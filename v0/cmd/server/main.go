package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/99designs/gqlgen/codegen/config"
)

func main() {
	conf := flag.String("config", "config.yaml", "configuration YAML path")
	flag.Parse()
	c, e := config.LoadConfig(*conf)
	if e != nil {
		log.Fatalf("config: %v", e)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
}
