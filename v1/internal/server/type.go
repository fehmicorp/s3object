package server

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fehmicorp/s3object/v1/internal/config"
)

type NewServer struct {
	Cfg *config.Config
}

type AppServer struct {
	HTTP          *http.Server
	Cfg           *config.Config
	DB            *sql.DB
	SigningSecret string
}

// S3 bucket naming rule: 3–63 chars, lowercase letters, numbers, dots, and hyphens.
var validBucketRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

// BucketInfo represents metadata for a bucket stored on the local filesystem.
type BucketInfo struct {
	Name         string    `json:"name"`
	CreationDate time.Time `json:"creation_date"`
}

type ObjectInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag,omitempty"`
}

type MultipartPart struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
}

type CompleteMultipartRequest struct {
	Parts []MultipartPart `json:"parts"`
}

type multipartSessionMeta struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

// resolveMultipartDir safely locates the staging folder for a given uploadId.
func (app *AppServer) resolveMultipartDir(uploadID string) (string, bool) {
	if uploadID == "" || strings.Contains(uploadID, "..") || strings.ContainsAny(uploadID, `/`) {
		return "", false
	}
	base := app.Cfg.Storage.Path
	if base == "" {
		base = os.TempDir()
	}
	root := filepath.Clean(filepath.Join(base, ".mpc"))
	target := filepath.Join(root, uploadID)
	if filepath.Dir(target) != root {
		return "", false
	}
	return target, true
}

func generateUploadID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type PresignRequest struct {
	Bucket        string `json:"bucket"`
	Key           string `json:"key"`
	Method        string `json:"method"`         // "GET" or "PUT" (defaults to "GET")
	ExpirySeconds int64  `json:"expiry_seconds"` // Optional override
}

type SearchResultItem struct {
	Bucket       string    `json:"bucket"`
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
}

// resolveBucketPath validates the bucket name and safely resolves its directory path.
func (app *AppServer) resolveBucketPath(bucket string) (string, bool) {
	if !validBucketRegex.MatchString(bucket) || strings.Contains(bucket, "..") {
		return "", false
	}
	base := filepath.Clean(app.Cfg.Storage.Path)
	target := filepath.Join(base, bucket)

	// Prevent directory traversal outside PersistentPath
	if filepath.Dir(target) != base {
		return "", false
	}
	return target, true
}

// resolveObjectPath validates both the bucket and object key, preventing directory traversal.
func (app *AppServer) resolveObjectPath(bucket, key string) (bucketPath, objectPath, cleanKey string, ok bool) {
	bucketPath, ok = app.resolveBucketPath(bucket)
	if !ok || key == "" {
		return "", "", "", false
	}

	// Normalize key slashes and disallow null bytes or ".." segments
	cleaned := filepath.ToSlash(filepath.Clean("/" + key))
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" || cleaned == "." || strings.Contains(cleaned, "..") {
		return "", "", "", false
	}

	fullPath := filepath.Join(bucketPath, filepath.FromSlash(cleaned))
	rel, err := filepath.Rel(bucketPath, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", "", "", false
	}

	return bucketPath, fullPath, cleaned, true
}
