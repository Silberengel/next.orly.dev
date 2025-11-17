package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev"
	"next.orly.dev/pkg/database"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <data-dir>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Runs garbage collection on Badger database to free space in value log files\n")
		os.Exit(1)
	}

	dataDir := os.Args[1]
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

	fmt.Printf("Opening database in read-only mode: %s\n", dataDir)

	// Try multiple value log file sizes to handle mismatches
	// The error shows files are ~35MB, so we'll try: 20MB, 128MB (larger than file)
	valueLogSizes := []int{20, 128}
	if v := os.Getenv("ORLY_DB_VALUE_LOG_FILE_SIZE_MB"); v != "" {
		if n, perr := strconv.Atoi(v); perr == nil && n > 0 {
			// Use the specified size first, then try others if it fails
			valueLogSizes = []int{n, 20, 128}
		}
	}

	var db *badger.DB
	var err error
	var opened bool

	for _, sizeMB := range valueLogSizes {
		opts := badger.DefaultOptions(dataDir)
		opts.ReadOnly = true
		opts.Logger = database.NewLogger(lol.Error, dataDir) // error level
		opts.ValueLogFileSize = int64(sizeMB * 1024 * 1024)  // Convert MB to bytes

		fmt.Printf("Trying to open with value log file size: %d MB...\n", sizeMB)
		db, err = badger.Open(opts)
		if err == nil {
			fmt.Printf("Successfully opened database with %d MB value log file size\n", sizeMB)
			opened = true
			break
		}
		fmt.Printf("Failed with %d MB: %v\n", sizeMB, err)
	}

	if !opened {
		fmt.Fprintf(os.Stderr, "\nError: failed to open database with any value log file size\n")
		fmt.Fprintf(os.Stderr, "The database may be corrupted or require manual repair.\n")
		fmt.Fprintf(os.Stderr, "You may need to use Badger's repair tool or truncate value log files manually.\n")
		os.Exit(1)
	}
	defer db.Close()

	fmt.Println("Running value log garbage collection...")
	fmt.Println("This may take several minutes for large databases...")

	// Run GC multiple times until no more can be collected
	gcCount := 0
	for i := 0; i < 20; i++ {
		err := db.RunValueLogGC(0.5)
		if err != nil {
			// ErrNoRewrite means no more GC needed
			if err == badger.ErrNoRewrite {
				fmt.Printf("GC cycle %d: No more garbage to collect\n", i+1)
				break
			}
			fmt.Printf("GC cycle %d: Error: %v\n", i+1, err)
			break
		}
		gcCount++
		fmt.Printf("GC cycle %d: Completed successfully\n", i+1)
	}

	if gcCount > 0 {
		fmt.Printf("\nSuccessfully completed %d GC cycles. Space should be freed in value log files.\n", gcCount)
	} else {
		fmt.Println("\nNo garbage collection was needed or possible.")
	}

	fmt.Println("Database GC completed. You can now start the relay.")
}
