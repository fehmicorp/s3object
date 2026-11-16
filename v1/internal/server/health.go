package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/fehmicorp/s3object/v1/internal/config"
)

func healthHandler(mux *http.ServeMux, h *config.Config) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		jsonOut(w, http.StatusOK, map[string]any{
			"status":  "alive",
			"app":     h.App.Name,
			"version": h.App.Version,
			"storage": h.Storage.Path,
		})
	}
	mux.HandleFunc("GET /s3", handler)
	mux.HandleFunc("HEAD /s3", handler)
}

func ListingPath(path string) string {
	if slug != "" && !strings.HasPrefix(slug, "/") {
		slug = "/" + slug
	}
	if path == "/" {
		return slug + "/{$}"
	}
	return slug + path
}

func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
