package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev"
	"next.orly.dev/pkg/database"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <data-dir> [--truncate]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Repairs Badger database by truncating corrupted value log files\n")
		fmt.Fprintf(os.Stderr, "Use --truncate flag to actually perform truncation (otherwise just reports issues)\n")
		os.Exit(1)
	}

	dataDir := os.Args[1]
	truncate := false
	if len(os.Args) > 2 && os.Args[2] == "--truncate" {
		truncate = true
	}

	if !filepath.IsAbs(dataDir) {
		var err error
		dataDir, err = filepath.Abs(dataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid data directory: %v\n", err)
			os.Exit(1)
		}
	}

	// Check if directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: data directory does not exist: %s\n", dataDir)
		os.Exit(1)
	}

	fmt.Printf("Checking database: %s\n", dataDir)
	if !truncate {
		fmt.Println("DRY RUN MODE - use --truncate to actually repair")
	}

	// Try to open the database to see what error we get
	opts := badger.DefaultOptions(dataDir)
	opts.ReadOnly = true
	opts.Logger = database.NewLogger(lol.Error, dataDir)

	// Try different value log sizes
	valueLogSizes := []int{20, 128}
	if v := os.Getenv("ORLY_DB_VALUE_LOG_FILE_SIZE_MB"); v != "" {
		if n, perr := strconv.Atoi(v); perr == nil && n > 0 {
			valueLogSizes = []int{n, 20, 128}
		}
	}

	var openErr error
	for _, sizeMB := range valueLogSizes {
		opts.ValueLogFileSize = int64(sizeMB * 1024 * 1024)
		_, err := badger.Open(opts)
		if err == nil {
			fmt.Println("Database opens successfully - no repair needed!")
			return
		}
		openErr = err
		fmt.Printf("Failed with %d MB: %v\n", sizeMB, err)
	}

	// Parse the error to find the problematic file
	errStr := openErr.Error()
	fmt.Printf("\nError detected: %s\n", errStr)

	// Look for "fid: XX" in the error message
	if strings.Contains(errStr, "fid:") {
		// Extract file ID from error
		parts := strings.Split(errStr, "fid: ")
		if len(parts) > 1 {
			fidPart := strings.Split(parts[1], " ")[0]
			fid, err := strconv.Atoi(fidPart)
			if err == nil {
				fmt.Printf("Problematic value log file ID: %d\n", fid)

				// List all .vlog files to see what's actually there
				fmt.Println("\nScanning for value log files...")
				var largeFiles []string
				maxExpectedSize := int64(128 * 1024 * 1024) // 128 MB
				entries, err := os.ReadDir(dataDir)
				if err == nil {
					var vlogFiles []string

					for _, entry := range entries {
						if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".vlog") {
							vlogFiles = append(vlogFiles, entry.Name())
							if info, err := os.Stat(filepath.Join(dataDir, entry.Name())); err == nil {
								if info.Size() > maxExpectedSize {
									largeFiles = append(largeFiles, entry.Name())
								}
							}
						}
					}
					if len(vlogFiles) > 0 {
						fmt.Printf("Found %d value log files:\n", len(vlogFiles))
						for _, f := range vlogFiles {
							if info, err := os.Stat(filepath.Join(dataDir, f)); err == nil {
								sizeMB := float64(info.Size()) / (1024 * 1024)
								marker := ""
								if info.Size() > maxExpectedSize {
									marker = " ⚠️  (ABNORMALLY LARGE - may need GC)"
								}
								fmt.Printf("  %s: %d bytes (%.2f MB)%s\n", f, info.Size(), sizeMB, marker)
							}
						}
						if len(largeFiles) > 0 {
							fmt.Printf("\n⚠️  Warning: Found %d abnormally large value log file(s).\n", len(largeFiles))
							fmt.Println("   These may be due to a previous memory leak or corruption.")
							fmt.Println("   After fixing MANIFEST, run GC to clean them up.")
						}
					} else {
						fmt.Println("No .vlog files found in directory")
					}
				}

				// Try to find the file with the matching ID
				// Badger uses 6-digit zero-padded format: 000039.vlog
				vlogPattern := fmt.Sprintf("%06d.vlog", fid)
				vlogPath := filepath.Join(dataDir, vlogPattern)

				// Also try alternative patterns
				altPatterns := []string{
					fmt.Sprintf("%d.vlog", fid),
					fmt.Sprintf("%05d.vlog", fid),
					fmt.Sprintf("%07d.vlog", fid),
				}

				var foundPath string
				var actualSize int64

				if info, err := os.Stat(vlogPath); err == nil {
					foundPath = vlogPath
					actualSize = info.Size()
					fmt.Printf("\nFound value log file: %s, size: %d bytes (%.2f MB)\n",
						vlogPattern, actualSize, float64(actualSize)/(1024*1024))
				} else {
					// Try alternative patterns
					for _, pattern := range altPatterns {
						altPath := filepath.Join(dataDir, pattern)
						if info, err := os.Stat(altPath); err == nil {
							foundPath = altPath
							actualSize = info.Size()
							fmt.Printf("\nFound value log file: %s, size: %d bytes (%.2f MB)\n",
								pattern, actualSize, float64(actualSize)/(1024*1024))
							break
						}
					}
				}

				if foundPath != "" {
					// Try to extract expected size from error
					if strings.Contains(errStr, "end offset:") && strings.Contains(errStr, "< size:") {
						// Parse "end offset: XXXXX < size: YYYYY"
						offsetParts := strings.Split(errStr, "end offset: ")
						if len(offsetParts) > 1 {
							offsetSizeParts := strings.Split(offsetParts[1], " < size: ")
							if len(offsetSizeParts) == 2 {
								endOffsetStr := strings.Split(offsetSizeParts[0], " ")[0]
								expectedSizeStr := strings.Split(offsetSizeParts[1], " ")[0]

								endOffset, _ := strconv.ParseInt(endOffsetStr, 10, 64)
								expectedSize, _ := strconv.ParseInt(expectedSizeStr, 10, 64)

								fmt.Printf("End offset: %d bytes (%.2f MB)\n", endOffset, float64(endOffset)/(1024*1024))
								fmt.Printf("Expected size: %d bytes (%.2f MB)\n", expectedSize, float64(expectedSize)/(1024*1024))

								if endOffset < expectedSize {
									if endOffset < actualSize {
										fmt.Printf("\nFile needs truncation to %d bytes (%.2f MB)\n", endOffset, float64(endOffset)/(1024*1024))

										if truncate {
											fmt.Printf("Truncating file %s to %d bytes...\n", foundPath, endOffset)
											file, err := os.OpenFile(foundPath, os.O_RDWR, 0644)
											if err != nil {
												fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
												os.Exit(1)
											}
											defer file.Close()

											if err := file.Truncate(endOffset); err != nil {
												fmt.Fprintf(os.Stderr, "Error truncating file: %v\n", err)
												os.Exit(1)
											}

											fmt.Println("File truncated successfully!")
											fmt.Println("\nNow try running gc-db again to perform garbage collection.")
										} else {
											fmt.Println("\nDRY RUN: Would truncate file to end offset")
											fmt.Println("Run with --truncate flag to actually perform truncation")
										}
									} else {
										fmt.Printf("\nFile size (%d bytes) is already smaller than end offset (%d bytes)\n", actualSize, endOffset)
										fmt.Println("The file may need to be deleted or the database may need a different repair approach.")
									}
								} else {
									fmt.Printf("\nEnd offset (%d) >= expected size (%d) - no truncation needed\n", endOffset, expectedSize)
								}
							}
						}
					}
				} else {
					fmt.Printf("\nValue log file with ID %d not found (tried: %s and alternatives)\n", fid, vlogPattern)
					fmt.Println("The file may have been deleted or the database structure is different.")
					fmt.Println("This usually means the MANIFEST file is out of sync with actual files.")

					// Check if MANIFEST exists
					manifestPath := filepath.Join(dataDir, "MANIFEST")
					if info, err := os.Stat(manifestPath); err == nil {
						fmt.Printf("\nMANIFEST file exists (%d bytes)\n", info.Size())
						fmt.Println("Deleting MANIFEST will force Badger to rebuild it from actual files.")

						if truncate {
							fmt.Printf("Deleting MANIFEST file: %s\n", manifestPath)
							if err := os.Remove(manifestPath); err != nil {
								fmt.Fprintf(os.Stderr, "Error deleting MANIFEST: %v\n", err)
								os.Exit(1)
							}
							fmt.Println("MANIFEST deleted successfully!")
							fmt.Println("Badger will rebuild the MANIFEST on next open based on actual files.")
							if len(largeFiles) > 0 {
								fmt.Printf("\n⚠️  Note: You have %d abnormally large value log file(s) that need cleanup.\n", len(largeFiles))
								fmt.Println("   After the relay starts, run 'gc-db' to perform garbage collection")
								fmt.Println("   and free space in these oversized files.")
							}
							fmt.Println("\nNext steps:")
							fmt.Println("1. Try starting the relay - it should rebuild the MANIFEST automatically")
							fmt.Println("2. If it starts successfully, run 'gc-db' to clean up large value log files")
						} else {
							fmt.Println("\nDRY RUN: Would delete MANIFEST file to force rebuild")
							fmt.Println("Run with --truncate flag to actually delete MANIFEST")
						}
					} else {
						fmt.Println("\nMANIFEST file not found - database may be in an inconsistent state.")
					}
				}
			}
		}
	}

	if !truncate {
		fmt.Fprintf(os.Stderr, "\nDatabase requires repair. Run with --truncate flag to repair.\n")
		fmt.Fprintf(os.Stderr, "WARNING: Truncation may result in data loss!\n")
		os.Exit(1)
	}
}
