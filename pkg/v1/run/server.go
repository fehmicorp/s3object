package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	s3 "github.com/fehmicorp/s3object/v1/pkg"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	port := 9000
	if p := os.Getenv("S3_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	slug := os.Getenv("S3_SLUG")
	if slug == "" {
		slug = "/dist"
	}

	storagePath := os.Getenv("S3_STORAGE_PATH")
	if storagePath == "" {
		storagePath = "/data"
	}
	configPath := getConfigPath()

	log.Printf("Operating System: %s", runtime.GOOS)
	log.Printf("Configuration: %s", configPath)
	log.Printf("Storage: %s", storagePath)

	engine, err := s3.New(ctx, s3.Options{
		Port:        port,
		InitialSlug: slug,
		StoragePath: storagePath,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := engine.Start(ctx); err != nil {
		log.Fatal(err)
	}
}

func getConfigPath() string {
	// Explicit configuration path takes priority.
	if path := os.Getenv("S3_CONFIG"); path != "" {
		return path
	}

	if runtime.GOOS == "windows" {
		exe, err := os.Executable()
		if err == nil {
			return filepath.Join(filepath.Dir(exe), "config.yaml")
		}
	}

	// Linux production configuration.
	if runtime.GOOS == "linux" {
		return "/etc/fehmi-s3/config.yaml"
	}

	// Development fallback.
	return "config.yaml"
}
