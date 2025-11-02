package blossom

import (
	"encoding/json"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/crypto/sha256"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/utils"
)

const (
	// Database key prefixes
	prefixBlobData  = "blob:data:"
	prefixBlobMeta  = "blob:meta:"
	prefixBlobIndex = "blob:index:"
	prefixBlobReport = "blob:report:"
)

// Storage provides blob storage operations
type Storage struct {
	db *database.D
}

// NewStorage creates a new storage instance
func NewStorage(db *database.D) *Storage {
	return &Storage{db: db}
}

// SaveBlob stores a blob with its metadata
func (s *Storage) SaveBlob(
	sha256Hash []byte, data []byte, pubkey []byte, mimeType string,
) (err error) {
	sha256Hex := hex.Enc(sha256Hash)

	// Verify SHA256 matches
	calculatedHash := sha256.Sum256(data)
	if !utils.FastEqual(calculatedHash[:], sha256Hash) {
		err = errorf.E(
			"SHA256 mismatch: calculated %x, provided %x",
			calculatedHash[:], sha256Hash,
		)
		return
	}

	// Create metadata
	metadata := NewBlobMetadata(pubkey, mimeType, int64(len(data)))
	var metaData []byte
	if metaData, err = metadata.Serialize(); chk.E(err) {
		return
	}

	// Store blob data
	dataKey := prefixBlobData + sha256Hex
	if err = s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set([]byte(dataKey), data); err != nil {
			return err
		}

		// Store metadata
		metaKey := prefixBlobMeta + sha256Hex
		if err := txn.Set([]byte(metaKey), metaData); err != nil {
			return err
		}

		// Index by pubkey
		indexKey := prefixBlobIndex + hex.Enc(pubkey) + ":" + sha256Hex
		if err := txn.Set([]byte(indexKey), []byte{1}); err != nil {
			return err
		}

		return nil
	}); chk.E(err) {
		return
	}

	log.D.F("saved blob %s (%d bytes) for pubkey %s", sha256Hex, len(data), hex.Enc(pubkey))
	return
}

// GetBlob retrieves blob data by SHA256 hash
func (s *Storage) GetBlob(sha256Hash []byte) (data []byte, metadata *BlobMetadata, err error) {
	sha256Hex := hex.Enc(sha256Hash)
	dataKey := prefixBlobData + sha256Hex

	var blobData []byte
	if err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(dataKey))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			blobData = make([]byte, len(val))
			copy(blobData, val)
			return nil
		})
	}); chk.E(err) {
		return
	}

	// Get metadata
	metaKey := prefixBlobMeta + sha256Hex
	if err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(metaKey))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			if metadata, err = DeserializeBlobMetadata(val); err != nil {
				return err
			}
			return nil
		})
	}); chk.E(err) {
		return
	}

	data = blobData
	return
}

// HasBlob checks if a blob exists
func (s *Storage) HasBlob(sha256Hash []byte) (exists bool, err error) {
	sha256Hex := hex.Enc(sha256Hash)
	dataKey := prefixBlobData + sha256Hex

	if err = s.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(dataKey))
		if err == badger.ErrKeyNotFound {
			exists = false
			return nil
		}
		if err != nil {
			return err
		}
		exists = true
		return nil
	}); chk.E(err) {
		return
	}

	return
}

// DeleteBlob deletes a blob and its metadata
func (s *Storage) DeleteBlob(sha256Hash []byte, pubkey []byte) (err error) {
	sha256Hex := hex.Enc(sha256Hash)
	dataKey := prefixBlobData + sha256Hex
	metaKey := prefixBlobMeta + sha256Hex
	indexKey := prefixBlobIndex + hex.Enc(pubkey) + ":" + sha256Hex

	if err = s.db.Update(func(txn *badger.Txn) error {
		// Verify blob exists
		_, err := txn.Get([]byte(dataKey))
		if err == badger.ErrKeyNotFound {
			return errorf.E("blob %s not found", sha256Hex)
		}
		if err != nil {
			return err
		}

		// Delete blob data
		if err := txn.Delete([]byte(dataKey)); err != nil {
			return err
		}

		// Delete metadata
		if err := txn.Delete([]byte(metaKey)); err != nil {
			return err
		}

		// Delete index entry
		if err := txn.Delete([]byte(indexKey)); err != nil {
			return err
		}

		return nil
	}); chk.E(err) {
		return
	}

	log.D.F("deleted blob %s for pubkey %s", sha256Hex, hex.Enc(pubkey))
	return
}

// ListBlobs lists all blobs for a given pubkey
func (s *Storage) ListBlobs(
	pubkey []byte, since, until int64,
) (descriptors []*BlobDescriptor, err error) {
	pubkeyHex := hex.Enc(pubkey)
	prefix := prefixBlobIndex + pubkeyHex + ":"

	descriptors = make([]*BlobDescriptor, 0)

	if err = s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			
			// Extract SHA256 from key: prefixBlobIndex + pubkeyHex + ":" + sha256Hex
			sha256Hex := string(key[len(prefix):])
			
			// Get blob metadata
			metaKey := prefixBlobMeta + sha256Hex
			metaItem, err := txn.Get([]byte(metaKey))
			if err != nil {
				continue
			}

			var metadata *BlobMetadata
			if err = metaItem.Value(func(val []byte) error {
				if metadata, err = DeserializeBlobMetadata(val); err != nil {
					return err
				}
				return nil
			}); err != nil {
				continue
			}

			// Filter by time range
			if since > 0 && metadata.Uploaded < since {
				continue
			}
			if until > 0 && metadata.Uploaded > until {
				continue
			}

			// Verify blob exists
			dataKey := prefixBlobData + sha256Hex
			_, errGet := txn.Get([]byte(dataKey))
			if errGet != nil {
				continue
			}

			// Create descriptor (URL will be set by handler)
			descriptor := NewBlobDescriptor(
				"", // URL will be set by handler
				sha256Hex,
				metadata.Size,
				metadata.MimeType,
				metadata.Uploaded,
			)

			descriptors = append(descriptors, descriptor)
		}

		return nil
	}); chk.E(err) {
		return
	}

	return
}

// SaveReport stores a report for a blob (BUD-09)
func (s *Storage) SaveReport(sha256Hash []byte, reportData []byte) (err error) {
	sha256Hex := hex.Enc(sha256Hash)
	reportKey := prefixBlobReport + sha256Hex

	// Get existing reports
	var existingReports [][]byte
	if err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(reportKey))
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			if err = json.Unmarshal(val, &existingReports); err != nil {
				return err
			}
			return nil
		})
	}); chk.E(err) {
		return
	}

	// Append new report
	existingReports = append(existingReports, reportData)

	// Store updated reports
	var reportsData []byte
	if reportsData, err = json.Marshal(existingReports); chk.E(err) {
		return
	}

	if err = s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(reportKey), reportsData)
	}); chk.E(err) {
		return
	}

	log.D.F("saved report for blob %s", sha256Hex)
	return
}

// GetBlobMetadata retrieves only metadata for a blob
func (s *Storage) GetBlobMetadata(sha256Hash []byte) (metadata *BlobMetadata, err error) {
	sha256Hex := hex.Enc(sha256Hash)
	metaKey := prefixBlobMeta + sha256Hex

	if err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(metaKey))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			if metadata, err = DeserializeBlobMetadata(val); err != nil {
				return err
			}
			return nil
		})
	}); chk.E(err) {
		return
	}

	return
}

