package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (app *AppServer) signPresignedURL(method, objectRoutePath string, expiresUnix int64) string {
	secret := app.SigningSecret
	if secret == "" {
		secret = "ymVFLSoMrZXxQ55kwEQQbSSNuJhNF15xM8klYjWvzYu7jp4V"
	}

	payload := fmt.Sprintf("%s\n%s\n%d", strings.ToUpper(method), objectRoutePath, expiresUnix)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// GeneratePresignedURL handles POST /presign
func (app *AppServer) GeneratePresignedURL(w http.ResponseWriter, r *http.Request) {
	var req PresignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON request body"})
		return
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet && method != http.MethodPut && method != http.MethodDelete {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "method must be GET, PUT, or DELETE"})
		return
	}

	bucketPath, _, cleanKey, ok := app.resolveObjectPath(req.Bucket, req.Key)
	if !ok {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "invalid bucket or key"})
		return
	}
	if info, err := os.Stat(bucketPath); err != nil || !info.IsDir() {
		jsonOut(w, http.StatusNotFound, map[string]any{"error": "NoSuchBucket"})
		return
	}

	// Default expiry: 400 seconds (or fallback to 15 minutes if <= 0)
	expiry := 9 * time.Minute
	if expiry <= 0 {
		expiry = 5 * time.Minute
	}
	if req.ExpirySeconds > 0 {
		custom := time.Duration(req.ExpirySeconds) * time.Second
		if custom > 7*24*time.Hour {
			jsonOut(w, http.StatusBadRequest, map[string]any{"error": "expiry_seconds cannot exceed 604800 (7 days)"})
			return
		}
		expiry = custom
	}

	expiresAt := time.Now().UTC().Add(expiry)
	expiresUnix := expiresAt.Unix()

	objectRoute := ListingPath(fmt.Sprintf("/%s/objects/%s", req.Bucket, cleanKey))
	signature := app.signPresignedURL(method, objectRoute, expiresUnix)

	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}

	q := url.Values{}
	q.Set("X-Fehmi-Expires", strconv.FormatInt(expiresUnix, 10))
	q.Set("X-Fehmi-Signature", signature)

	fullURL := fmt.Sprintf("%s://%s%s?%s", scheme, r.Host, objectRoute, q.Encode())

	jsonOut(w, http.StatusOK, map[string]any{
		"url":        fullURL,
		"method":     method,
		"bucket":     req.Bucket,
		"key":        cleanKey,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

// SearchAssets handles GET /search?q=logo&bucket=my-assets&limit=50
func (app *AppServer) SearchAssets(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if query == "" {
		jsonOut(w, http.StatusBadRequest, map[string]any{"error": "query parameter 'q' is required"})
		return
	}

	filterBucket := strings.TrimSpace(r.URL.Query().Get("bucket"))
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	baseStorage := app.Cfg.Storage.Path
	var targetBuckets []string

	if filterBucket != "" {
		bPath, ok := app.resolveBucketPath(filterBucket)
		if !ok {
			jsonOut(w, http.StatusBadRequest, map[string]any{"error": "InvalidBucketName"})
			return
		}
		if info, err := os.Stat(bPath); err != nil || !info.IsDir() {
			jsonOut(w, http.StatusNotFound, map[string]any{"error": "NoSuchBucket"})
			return
		}
		targetBuckets = []string{filterBucket}
	} else {
		entries, err := os.ReadDir(baseStorage)
		if err != nil {
			jsonOut(w, http.StatusInternalServerError, map[string]any{"error": "failed to read storage directory"})
			return
		}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				targetBuckets = append(targetBuckets, e.Name())
			}
		}
	}

	results := make([]SearchResultItem, 0)

	for _, bName := range targetBuckets {
		if len(results) >= limit {
			break
		}
		bPath := filepath.Join(baseStorage, bName)

		_ = filepath.WalkDir(bPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || strings.HasPrefix(d.Name(), ".") {
				return nil
			}
			if len(results) >= limit {
				return filepath.SkipAll
			}

			rel, err := filepath.Rel(bPath, path)
			if err != nil {
				return nil
			}
			key := filepath.ToSlash(rel)

			if strings.Contains(strings.ToLower(key), query) {
				info, err := d.Info()
				if err != nil {
					return nil
				}
				results = append(results, SearchResultItem{
					Bucket:       bName,
					Key:          key,
					Size:         info.Size(),
					LastModified: info.ModTime().UTC(),
				})
			}
			return nil
		})
	}

	jsonOut(w, http.StatusOK, map[string]any{
		"query":   query,
		"count":   len(results),
		"results": results,
	})
}
