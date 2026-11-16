package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fehmicorp/s3object/v1/internal/config"
	"github.com/fehmicorp/s3object/v1/internal/server"
)

func main() {
	conf := flag.String("config", "config.yaml", "configuration YAML path")
	flag.Parse()
	cfg, e := config.LoadConfig(*conf)
	if e != nil {
		log.Fatalf("config: %v", e)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := server.CreateServer(ctx, cfg)
	done := make(chan error, 1)
	go func() { done <- srv.HTTP.ListenAndServe() }()
	select {
	case e := <-done:
		if e != nil {
			log.Fatalf("HTTP server: %v", e)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if e := srv.Shutdown(shutdownCtx); e != nil {
			log.Printf("shutdown: %v", e)
		}
	}
}
