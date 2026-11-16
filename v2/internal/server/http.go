package server

import (
	"context"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fehmicorp/s3object/v2/internal/auth"
	"github.com/fehmicorp/s3object/v2/types"
)

var slug string

type AppServer struct {
	HTTP *http.Server
	Auth *auth.Authenticator
}

type NSvr struct {
	ctx *context.Context
	cfg *types.Config
	ath *auth.Authenticator
	adt *Audit
	mtc *Metrics
	hth *Health
}

func NewServer(ctx context.Context, c *types.Config, a *auth.Authenticator, adt *Audit, mtc *Metrics, h *Health) *AppServer {
	mux := http.NewServeMux()
	slug = strings.TrimSuffix(c.Server.InitialSlug, "/")
	h.HealthHandler(mux)
	mtc.MetricHandler(mux)
	app := &AppServer{Auth: a}
	// ops :=
	mux.HandleFunc(ListingPath("/"), func(w http.ResponseWriter, r *http.Request) {
		fn, err := h.HttpMethods(w, r, slug)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		resp, err := fn()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		Return(w, resp)
	})

	app.HTTP = &http.Server{
		Addr:              c.Server.Host + ":" + strconv.Itoa(c.Server.Port),
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       c.Server.ReadTimeout,
		WriteTimeout:      c.Server.WriteTimeout,
		IdleTimeout:       c.Server.IdleTimeout,
		MaxHeaderBytes:    c.Server.MaxHeaderBytes,
	}
	return app
}

func Return(w http.ResponseWriter, resp *http.Response) {
	defer resp.Body.Close()
	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (a *AppServer) Start() error {
	log.Printf("Fehmi Blob Storage listening at %s", a.HTTP.Addr)
	e := a.HTTP.ListenAndServe()
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}

func (a *AppServer) Shutdown(c context.Context) error { return a.HTTP.Shutdown(c) }

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s (%s)", r.Method, r.URL.Path, r.RemoteAddr, time.Since(t))
	})
}
