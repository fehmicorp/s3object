package server

import (
	"net/http"
)

type Metrics struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

func (m *Metrics) MetricHandler(mux *http.ServeMux) {
	if m.Enabled && m.Path != "" {
		mux.HandleFunc(ListingPath(m.Path), func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("# Fehmi Blob Storage metrics endpoint\n"))
		})
	}
}
