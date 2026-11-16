package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// InitMultipartUpload handles POST /{bucket}/multipart/{key...}
func (app *AppServer) InitMultipartUpload(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	bucketPath, _, cleanKey, ok := app.resolveObjectPath(bucket, key)
	if !ok {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "InvalidObjectKey"})
		return
	}

	if info, err := os.Stat(bucketPath); err != nil || !info.IsDir() {
		jsonOut(w, http.StatusNotFound, map[string]any{"error": "NoSuchBucket"})
		return
	}

	uploadID, err := generateUploadID()
	if err != nil {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed to generate upload id"})
		return
	}

	sessionDir, ok := app.resolveMultipartDir(uploadID)
	if !ok {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "invalid staging path"})
		return
	}

	if err := os.MkdirAll(sessionDir, 0o750); err != nil {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed to initialize multipart staging"})
		return
	}

	// Save session metadata to verify bucket & key on later requests
	metaBytes, _ := json.Marshal(multipartSessionMeta{Bucket: bucket, Key: cleanKey})
	if err := os.WriteFile(filepath.Join(sessionDir, "meta.json"), metaBytes, 0o640); err != nil {
		_ = os.RemoveAll(sessionDir)
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed to write session metadata"})
		return
	}

	jsonOut(w, http.StatusOK, map[string]any{
		"bucket":    bucket,
		"key":       cleanKey,
		"upload_id": uploadID,
	})
}

// UploadPart handles PUT /{bucket}/multipart/{key...}?uploadId=...&p=1
func (app *AppServer) UploadPart(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	_, _, cleanKey, ok := app.resolveObjectPath(bucket, key)
	if !ok {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "InvalidObjectKey"})
		return
	}

	uploadID := r.URL.Query().Get("uid")
	partNum, err := strconv.Atoi(r.URL.Query().Get("pn"))
	if uploadID == "" || err != nil || partNum < 1 || partNum > 10000 {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "invalid uploadId or partNumber (1-10000)"})
		return
	}

	sessionDir, ok := app.resolveMultipartDir(uploadID)
	if !ok {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "InvalidUploadID"})
		return
	}

	// Validate session exists and matches bucket + key
	metaRaw, err := os.ReadFile(filepath.Join(sessionDir, "meta.json"))
	if err != nil {
		jsonOut(w, http.StatusNotFound, map[string]any{"error": "NoSuchUpload"})
		return
	}
	var meta multipartSessionMeta
	if err := json.Unmarshal(metaRaw, &meta); err != nil || meta.Bucket != bucket || meta.Key != cleanKey {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "upload session does not match bucket/key"})
		return
	}

	// Enforce per-part size limit if configured
	body := io.Reader(r.Body)
	if app.Cfg.Storage.MaxUploadSize > 0 {
		body = http.MaxBytesReader(w, r.Body, app.Cfg.Storage.MaxUploadSize)
	}

	// Write part to a temp file first, then rename so retries are atomic
	tmpPart, err := os.CreateTemp(sessionDir, fmt.Sprintf("part-%05d-*.tmp", partNum))
	if err != nil {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed to create part temp file"})
		return
	}
	tmpPath := tmpPart.Name()
	cleanup := true
	defer func() {
		_ = tmpPart.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmpPart, hasher), body)
	if err != nil {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "failed reading part body: " + err.Error()})
		return
	}
	if err := tmpPart.Close(); err != nil {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed closing part file"})
		return
	}

	finalPartPath := filepath.Join(sessionDir, fmt.Sprintf("part-%05d", partNum))
	if err := os.Rename(tmpPath, finalPartPath); err != nil {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed committing part"})
		return
	}
	cleanup = false

	etag := fmt.Sprintf(`"%s"`, hex.EncodeToString(hasher.Sum(nil)))
	w.Header().Set("ETag", etag)
	jsonOut(w, http.StatusOK, map[string]any{
		"part_number": partNum,
		"etag":        etag,
		"size":        written,
	})
}

// CompleteMultipartUpload handles POST /{bucket}/multipart-complete/{key...}?uploadId=...
func (app *AppServer) CompleteMultipartUpload(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	_, targetPath, cleanKey, ok := app.resolveObjectPath(bucket, key)
	if !ok {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "InvalidObjectKey"})
		return
	}

	uploadID := r.URL.Query().Get("uploadId")
	sessionDir, ok := app.resolveMultipartDir(uploadID)
	if !ok {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "InvalidUploadID"})
		return
	}

	metaRaw, err := os.ReadFile(filepath.Join(sessionDir, "meta.json"))
	if err != nil {
		jsonOut(w, http.StatusNotFound, map[string]any{"error": "NoSuchUpload"})
		return
	}
	var meta multipartSessionMeta
	if err := json.Unmarshal(metaRaw, &meta); err != nil || meta.Bucket != bucket || meta.Key != cleanKey {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "upload session does not match bucket/key"})
		return
	}

	var req CompleteMultipartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Parts) == 0 {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "invalid or empty parts list"})
		return
	}

	// Sort parts by part_number ascending
	sort.Slice(req.Parts, func(i, j int) bool {
		return req.Parts[i].PartNumber < req.Parts[j].PartNumber
	})

	// Assemble all parts into a single staged file while computing final SHA-256 ETag
	assembledFile, err := os.CreateTemp(sessionDir, "assembled-*")
	if err != nil {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed to create assembled file"})
		return
	}
	assembledPath := assembledFile.Name()
	cleanup := true
	defer func() {
		_ = assembledFile.Close()
		if cleanup {
			_ = os.Remove(assembledPath)
		}
	}()

	fullHasher := sha256.New()
	var totalSize int64

	for i, part := range req.Parts {
		if i > 0 && part.PartNumber == req.Parts[i-1].PartNumber {
			jsonOut(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("duplicate part number %d", part.PartNumber)})
			return
		}

		partPath := filepath.Join(sessionDir, fmt.Sprintf("part-%05d", part.PartNumber))
		pf, err := os.Open(partPath)
		if err != nil {
			jsonOut(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("missing part %d", part.PartNumber)})
			return
		}

		partHasher := sha256.New()
		n, copyErr := io.Copy(io.MultiWriter(assembledFile, fullHasher, partHasher), pf)
		_ = pf.Close()
		if copyErr != nil {
			jsonOut(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("failed reading part %d", part.PartNumber)})
			return
		}

		// Verify ETag if provided by client
		actualETag := fmt.Sprintf(`"%s"`, hex.EncodeToString(partHasher.Sum(nil)))
		providedETag := strings.TrimSpace(part.ETag)
		if providedETag != "" && providedETag != actualETag && `"`+strings.Trim(providedETag, `"`)+`"` != actualETag {
			jsonOut(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("ETag mismatch for part %d", part.PartNumber)})
			return
		}

		totalSize += n
	}

	if err := assembledFile.Close(); err != nil {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed closing assembled file"})
		return
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o750); err != nil {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed creating target directory"})
		return
	}

	if err := moveFile(assembledPath, targetPath); err != nil {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed committing completed object"})
		return
	}
	cleanup = false

	// Clean up staging session folder
	_ = os.RemoveAll(sessionDir)

	finalETag := fmt.Sprintf(`"%s-%d"`, hex.EncodeToString(fullHasher.Sum(nil)), len(req.Parts))
	w.Header().Set("ETag", finalETag)
	jsonOut(w, http.StatusOK, map[string]any{
		"bucket": bucket,
		"key":    cleanKey,
		"size":   totalSize,
		"etag":   finalETag,
	})
}

// validateMultipartSession checks that the uploadId exists and belongs to the given bucket and key.
func (app *AppServer) validateMultipartSession(bucket, key, uploadID string) (string, int, string) {
	_, _, cleanKey, ok := app.resolveObjectPath(bucket, key)
	if !ok {
		return "", http.StatusBadRequest, "InvalidObjectKey"
	}

	sessionDir, ok := app.resolveMultipartDir(uploadID)
	if !ok {
		return "", http.StatusBadRequest, "InvalidUploadID"
	}

	metaRaw, err := os.ReadFile(filepath.Join(sessionDir, "meta.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", http.StatusNotFound, "NoSuchUpload"
		}
		return "", http.StatusInternalServerError, "failed to read upload session"
	}

	var meta multipartSessionMeta
	if err := json.Unmarshal(metaRaw, &meta); err != nil || meta.Bucket != bucket || meta.Key != cleanKey {
		return "", http.StatusBadRequest, "upload session does not match bucket or key"
	}

	return sessionDir, http.StatusOK, ""
}

// AbortMultipartUpload handles DELETE /{bucket}/multipart/{key...}?uploadId=...
func (app *AppServer) AbortMultipartUpload(w http.ResponseWriter, r *http.Request) {
	sessionDir, status, errMsg := app.validateMultipartSession(
		r.PathValue("bucket"),
		r.PathValue("key"),
		r.URL.Query().Get("uploadId"),
	)
	if errMsg != "" {
		jsonOut(w, status, map[string]any{"error": errMsg})
		return
	}

	if err := os.RemoveAll(sessionDir); err != nil {
		jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed to abort multipart upload"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
