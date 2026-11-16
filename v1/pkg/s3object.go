package v1

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/fehmicorp/s3object/v1/internal/config"
	"github.com/fehmicorp/s3object/v1/internal/server"
)

// Options lets external Go projects configure the S3 server programmatically without a YAML file.
type Options struct {
	Host        string
	Port        int
	InitialSlug string
	StoragePath string
	StagingPath string
}

// Engine wraps the underlying HTTP server so external callers can start, mount, or shut it down.
type Engine struct {
	srv server.AppServer
}

// NewFromFile initializes the S3 server using a YAML configuration file.
func NewFromFile(ctx context.Context, configPath string) (*Engine, error) {
	if configPath == "" {
		configPath = "config.yaml"
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load s3 config: %w", err)
	}
	return &Engine{
		srv: server.CreateServer(ctx, cfg),
	}, nil
}

// New initializes the S3 server directly from Go options with sensible defaults.
func New(ctx context.Context, opts Options) (*Engine, error) {
	if opts.StoragePath == "" {
		return nil, errors.New("StoragePath is required")
	}

	cfg := &config.Config{}
	cfg.App.Name = "fehmi-s3"
	cfg.App.Version = "1.0.0"

	cfg.Server.Host = opts.Host
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	cfg.Server.Port = opts.Port
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 9000
	}
	cfg.Server.InitialSlug = opts.InitialSlug
	cfg.Server.ReadTimeout = 300 * time.Second
	cfg.Server.WriteTimeout = 300 * time.Second
	cfg.Server.IdleTimeout = 120 * time.Second
	cfg.Server.MaxHeaderBytes = 1 << 20

	cfg.Storage.Driver = "filesystem"
	cfg.Storage.Path = opts.StoragePath

	return &Engine{
		srv: server.CreateServer(ctx, cfg),
	}, nil
}

// Start runs the HTTP server and blocks until ctx is canceled or a fatal server error occurs.
func (e *Engine) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := e.srv.HTTP.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return e.srv.Shutdown(shutdownCtx)
	}
}

// Handler exposes the underlying http.Handler so another project can embed S3 routes into its own router.
func (e *Engine) Handler() http.Handler {
	return e.srv.HTTP.Handler
}
