package server

import (
	"net/http"
	"os"
	"strings"
)

// ListBuckets handles GET /
func (app *AppServer) ListBuckets(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(app.Cfg.Storage.Path)
	if err != nil {
		if os.IsNotExist(err) {
			jsonOut(w, http.StatusOK, map[string]any{"buckets": []BucketInfo{}})
			return
		}
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed to read storage directory"})
		return
	}
	buckets := make([]BucketInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		buckets = append(buckets, BucketInfo{
			Name:         entry.Name(),
			CreationDate: info.ModTime().UTC(),
		})
	}
	jsonOut(w, http.StatusOK, map[string]any{
		"buckets": buckets,
		"count":   len(buckets),
	})
}

// CreateBucket handles PUT /{bucket}
func (app *AppServer) CreateBucket(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	bucketPath, ok := app.resolveBucketPath(bucket)
	if !ok {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "InvalidBucketName"})
		return
	}
	if _, err := os.Stat(bucketPath); err == nil {
		jsonOut(w, http.StatusConflict, map[string]any{"error": "BucketAlreadyOwnedByYou"})
		return
	}
	if err := os.MkdirAll(bucketPath, 0o750); err != nil {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed to create bucket directory"})
		return
	}
	w.Header().Set("Location", "/"+bucket)
	jsonOut(w, http.StatusOK, map[string]any{
		"message": "bucket created",
		"bucket":  bucket,
	})
}

// HeadBucket handles HEAD /{bucket}
func (app *AppServer) HeadBucket(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	bucketPath, ok := app.resolveBucketPath(bucket)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	info, err := os.Stat(bucketPath)
	if err != nil || !info.IsDir() {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("X-Bucket-Region", "local")
	w.WriteHeader(http.StatusOK)
}

// DeleteBucket handles DELETE /{bucket}
func (app *AppServer) DeleteBucket(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	bucketPath, ok := app.resolveBucketPath(bucket)
	if !ok {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "InvalidBucketName"})
		return
	}
	info, err := os.Stat(bucketPath)
	if err != nil || !info.IsDir() {
		jsonOut(w, http.StatusNotFound, map[string]any{"error": "NoSuchBucket"})
		return
	}
	// os.Remove only removes empty directories, matching S3's BucketNotEmpty rule
	if err := os.Remove(bucketPath); err != nil {
		jsonOut(w, http.StatusConflict, map[string]any{"error": "BucketNotEmpty"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
