package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	s3 "github.com/fehmicorp/s3object/v1/pkg/init"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// engine, err := s3.NewFromFile(ctx, "config.yaml")
	engine, err := s3.New(ctx, s3.Options{
		Port:        9000,
		InitialSlug: "/dist",
		StoragePath: "./data",
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := engine.Start(ctx); err != nil {
		log.Fatal(err)
	}
}
