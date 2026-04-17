package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/api"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

func main() {
	defaultAddr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		defaultAddr = ":" + p
	}
	addr := flag.String("addr", defaultAddr, "listen address")
	agentURL := flag.String("agent-url", "http://localhost:9091", "cc-remote-agent URL")
	dbURL := flag.String("db-url", "", "PostgreSQL connection URL")
	flag.Parse()

	if *dbURL == "" {
		if v := os.Getenv("DATABASE_URL"); v != "" {
			*dbURL = v
		} else {
			*dbURL = "postgres://cctunnel:cctunnel_dev@localhost:5432/cctunnel?sslmode=disable"
		}
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, *dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	repo := db.NewRepository(pool)
	remote := remoteclient.NewClient(*agentURL)
	handler := api.NewHandler(repo, remote)

	mux := http.NewServeMux()
	api.HandlerFromMux(handler, mux)

	fmt.Printf("cc-tunnel listening on %s (agent: %s)\n", *addr, *agentURL)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
