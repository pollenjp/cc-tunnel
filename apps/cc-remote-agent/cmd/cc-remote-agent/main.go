package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/pollenjp/cc-tunnel/apps/cc-remote-agent/internal/api"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}
	addr := ":" + port

	handler := api.NewHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("/execute", handler.Execute)
	mux.HandleFunc("/health", handler.Health)

	fmt.Printf("cc-remote-agent listening on %s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
