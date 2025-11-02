package blossom

import (
	"encoding/json"
	"time"
)

// BlobDescriptor represents a blob descriptor as defined in BUD-02
type BlobDescriptor struct {
	URL      string     `json:"url"`
	SHA256   string     `json:"sha256"`
	Size     int64      `json:"size"`
	Type     string     `json:"type"`
	Uploaded int64      `json:"uploaded"`
	NIP94    [][]string `json:"nip94,omitempty"`
}

// BlobMetadata stores metadata about a blob in the database
type BlobMetadata struct {
	Pubkey    []byte `json:"pubkey"`
	MimeType  string `json:"mime_type"`
	Uploaded  int64  `json:"uploaded"`
	Size      int64  `json:"size"`
	Extension string `json:"extension"` // File extension (e.g., ".png", ".pdf")
}

// NewBlobDescriptor creates a new blob descriptor
func NewBlobDescriptor(
	url, sha256 string, size int64, mimeType string, uploaded int64,
) *BlobDescriptor {
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return &BlobDescriptor{
		URL:      url,
		SHA256:   sha256,
		Size:     size,
		Type:     mimeType,
		Uploaded: uploaded,
	}
}

// NewBlobMetadata creates a new blob metadata struct
func NewBlobMetadata(pubkey []byte, mimeType string, size int64) *BlobMetadata {
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return &BlobMetadata{
		Pubkey:    pubkey,
		MimeType:  mimeType,
		Uploaded:  time.Now().Unix(),
		Size:      size,
		Extension: "", // Will be set by SaveBlob
	}
}

// Serialize serializes blob metadata to JSON
func (bm *BlobMetadata) Serialize() (data []byte, err error) {
	return json.Marshal(bm)
}

// DeserializeBlobMetadata deserializes blob metadata from JSON
func DeserializeBlobMetadata(data []byte) (bm *BlobMetadata, err error) {
	bm = &BlobMetadata{}
	err = json.Unmarshal(data, bm)
	return
}
