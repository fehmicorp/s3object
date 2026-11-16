package server

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fehmicorp/s3object/v1/internal/config"
)

var slug string

func CreateServer(ctx context.Context, cfg *config.Config) AppServer {
	slug = strings.TrimSuffix(cfg.Server.InitialSlug, "/")
	db, secret, err := InitSQLite(cfg.Storage.Path)
	if err != nil {
		log.Fatalf("failed to initialize sqlite: %v", err)
	}
	app := AppServer{
		Cfg:           cfg,
		DB:            db,
		SigningSecret: secret,
	}
	mux := http.NewServeMux()
	ListenRoutes(mux, &app)
	app.HTTP = &http.Server{
		Addr:              cfg.Server.Host + ":" + strconv.Itoa(cfg.Server.Port),
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
	}
	return app
}

func ListenRoutes(mux *http.ServeMux, app *AppServer) {
	healthHandler(mux, app.Cfg)

	// --- Presigned URLs & Asset Search ---
	mux.HandleFunc("POST "+ListingPath("/sign"), app.GeneratePresignedURL)
	mux.HandleFunc("GET "+ListingPath("/find"), app.SearchAssets)

	// --- Bucket Management (S3-Compatible & Admin) ---
	mux.HandleFunc("GET "+ListingPath("/list"), app.ListBuckets)
	mux.HandleFunc("PUT "+ListingPath("/create/{bucket}"), app.CreateBucket)
	mux.HandleFunc("HEAD "+ListingPath("/fetch/{bucket}"), app.HeadBucket)
	mux.HandleFunc("DELETE "+ListingPath("/delete/{bucket}"), app.DeleteBucket)

	// // --- Object / Asset Operations ---
	mux.HandleFunc("GET "+ListingPath("/objs/{bucket}"), app.ListObjects)
	mux.HandleFunc("PUT "+ListingPath("/obj/{bucket}/{key...}"), app.PutObject)
	mux.HandleFunc("GET "+ListingPath("/obj/{bucket}/{key...}"), app.GetObject)
	mux.HandleFunc("HEAD "+ListingPath("/obj/{bucket}/{key...}"), app.HeadObject)
	mux.HandleFunc("DELETE "+ListingPath("/obj/{bucket}/{key...}"), app.DeleteObject)

	// // --- Multipart Uploads (For Large Files & Desktop/Mobile Agent Sync) ---
	mux.HandleFunc("POST "+ListingPath("/begin/{bucket}/{key...}"), app.InitMultipartUpload)
	mux.HandleFunc("PUT "+ListingPath("/part/{bucket}/{key...}"), app.UploadPart)
	mux.HandleFunc("POST "+ListingPath("/done/bucket}/{key...}"), app.CompleteMultipartUpload)
	mux.HandleFunc("DELETE "+ListingPath("/end/{bucket}/{key...}"), app.AbortMultipartUpload)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s (%s)", r.Method, r.URL.Path, r.RemoteAddr, time.Since(t))
	})
}

func (a *AppServer) Shutdown(c context.Context) error { return a.HTTP.Shutdown(c) }
