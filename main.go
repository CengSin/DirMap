package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cengsin/system-agent-rag/internal/agent"
	"github.com/cengsin/system-agent-rag/internal/config"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("system-agent-rag %s\n", version)
		os.Exit(0)
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Set log level based on config
	switch cfg.LogLevel {
	case "debug":
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	case "info":
		log.SetFlags(log.LstdFlags)
	case "warn", "error":
		log.SetFlags(0)
	}

	a, err := agent.New(cfg)
	if err != nil {
		log.Fatalf("agent: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("received signal: %v", sig)
		cancel()
	}()

	if err := a.Run(ctx); err != nil {
		log.Fatalf("agent: %v", err)
	}
}
