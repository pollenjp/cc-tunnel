package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/api"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/tmuxclient"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	runnerURL := flag.String("runner-url", "http://localhost:9090", "cc-tmux-tunnel runner URL")
	flag.Parse()

	client, err := tmuxclient.NewClientWithResponses(*runnerURL)
	if err != nil {
		log.Fatalf("failed to create runner client: %v", err)
	}

	handler := api.NewHandler(client)

	mux := http.NewServeMux()
	api.HandlerFromMux(handler, mux)

	fmt.Printf("cc-tunnel listening on %s (runner: %s)\n", *addr, *runnerURL)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
