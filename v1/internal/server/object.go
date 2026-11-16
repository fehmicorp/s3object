package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ListObjects handles GET /{bucket}/objects?prefix=...&max-keys=...
func (app *AppServer) ListObjects(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	bucketPath, ok := app.resolveBucketPath(bucket)
	if !ok {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "InvalidBucketName"})
		return
	}

	if info, err := os.Stat(bucketPath); err != nil || !info.IsDir() {
		jsonOut(w, http.StatusNotFound, map[string]any{"error": "NoSuchBucket"})
		return
	}

	prefix := r.URL.Query().Get("prefix")
	maxKeys := 100000
	if mk := r.URL.Query().Get("max-keys"); mk != "" {
		if parsed, err := strconv.Atoi(mk); err == nil && parsed > 0 && parsed <= 100000 {
			maxKeys = parsed
		}
	}

	objects := make([]ObjectInfo, 0)
	isTruncated := false

	err := filepath.WalkDir(bucketPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		// Skip hidden temporary files
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		relPath, err := filepath.Rel(bucketPath, path)
		if err != nil {
			return nil
		}
		key := filepath.ToSlash(relPath)

		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return nil
		}

		if len(objects) >= maxKeys {
			isTruncated = true
			return filepath.SkipAll
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		objects = append(objects, ObjectInfo{
			Key:          key,
			Size:         info.Size(),
			LastModified: info.ModTime().UTC(),
		})
		return nil
	})

	if err != nil {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed to list objects"})
		return
	}

	jsonOut(w, http.StatusOK, map[string]any{
		"bucket":       bucket,
		"prefix":       prefix,
		"key_count":    len(objects),
		"max_keys":     maxKeys,
		"is_truncated": isTruncated,
		"contents":     objects,
	})
}

// PutObject handles PUT /{bucket}/objects/{key...}
func (app *AppServer) PutObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	bucketPath, targetPath, cleanKey, ok := app.resolveObjectPath(bucket, key)
	if !ok {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "InvalidObjectKey"})
		return
	}

	if info, err := os.Stat(bucketPath); err != nil || !info.IsDir() {
		jsonOut(w, http.StatusNotFound, map[string]any{"error": "NoSuchBucket"})
		return
	}

	// Enforce max upload size if configured
	body := io.Reader(r.Body)
	if app.Cfg.Storage.MaxUploadSize > 0 {
		body = http.MaxBytesReader(w, r.Body, app.Cfg.Storage.MaxUploadSize)
	}

	// Write to staging directory first for atomic uploads
	stagingDir := app.Cfg.Storage.Path
	if stagingDir == "" {
		stagingDir = os.TempDir()
	}
	_ = os.MkdirAll(stagingDir, 0o750)

	tmpFile, err := os.CreateTemp(stagingDir, "upload-*")
	if err != nil {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed to create staging file"})
		return
	}
	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		_ = tmpFile.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	// Stream body while computing SHA-256 checksum
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmpFile, hasher), body)
	if err != nil {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "upload failed: " + err.Error()})
		return
	}
	if err := tmpFile.Close(); err != nil {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed to flush file"})
		return
	}

	// Ensure parent subdirectories exist inside the bucket
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o750); err != nil {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed to create key directory"})
		return
	}

	// Atomically move from staging to final destination (fallback to copy if cross-drive on Windows)
	if err := moveFile(tmpPath, targetPath); err != nil {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed to commit object"})
		return
	}
	cleanup = false

	etag := fmt.Sprintf(`"%s"`, hex.EncodeToString(hasher.Sum(nil)))
	w.Header().Set("ETag", etag)
	jsonOut(w, http.StatusOK, map[string]any{
		"bucket": bucket,
		"key":    cleanKey,
		"size":   written,
		"etag":   etag,
	})
}

// GetObject handles GET /{bucket}/objects/{key...}
func (app *AppServer) GetObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	_, targetPath, cleanKey, ok := app.resolveObjectPath(bucket, key)
	if !ok {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "InvalidObjectKey"})
		return
	}

	file, err := os.Open(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			jsonOut(w, http.StatusNotFound, map[string]any{"error": "NoSuchKey"})
			return
		}
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed to open object"})
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		jsonOut(w, http.StatusNotFound, map[string]any{"error": "NoSuchKey"})
		return
	}

	// Set Content-Type based on file extension if available
	if ctype := mime.TypeByExtension(filepath.Ext(cleanKey)); ctype != "" {
		w.Header().Set("Content-Type", ctype)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	// http.ServeContent handles Range requests (resumable downloads), Last-Modified, and Content-Length automatically
	http.ServeContent(w, r, filepath.Base(cleanKey), info.ModTime(), file)
}

// HeadObject handles HEAD /{bucket}/objects/{key...}
func (app *AppServer) HeadObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	_, targetPath, cleanKey, ok := app.resolveObjectPath(bucket, key)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	info, err := os.Stat(targetPath)
	if err != nil || info.IsDir() {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if ctype := mime.TypeByExtension(filepath.Ext(cleanKey)); ctype != "" {
		w.Header().Set("Content-Type", ctype)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("Last-Modified", info.ModTime().UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
}

// DeleteObject handles DELETE /{bucket}/objects/{key...}
func (app *AppServer) DeleteObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	bucketPath, targetPath, _, ok := app.resolveObjectPath(bucket, key)
	if !ok {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "InvalidObjectKey"})
		return
	}

	info, err := os.Stat(targetPath)
	if err != nil || info.IsDir() {
		// S3 returns 204 No Content even if the key does not exist (idempotent delete)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := os.Remove(targetPath); err != nil {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed to delete object"})
		return
	}

	// Clean up empty parent prefix folders up to the bucket root
	dir := filepath.Dir(targetPath)
	for dir != bucketPath && strings.HasPrefix(dir, bucketPath) {
		if err := os.Remove(dir); err != nil {
			break // Directory not empty
		}
		dir = filepath.Dir(dir)
	}

	w.WriteHeader(http.StatusNoContent)
}

// moveFile renames src to dst, falling back to stream copy if they are on different drives/partitions.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}
