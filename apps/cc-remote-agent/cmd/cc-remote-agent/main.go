package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/pollenjp/cc-tunnel/apps/cc-remote-agent/internal/api"
	"github.com/pollenjp/cc-tunnel/apps/cc-remote-agent/internal/auth"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}
	addr := ":" + port

	authMgr := auth.NewAuthManager()
	handler := api.NewHandler(authMgr)

	mux := http.NewServeMux()
	mux.HandleFunc("/execute", handler.Execute)
	mux.HandleFunc("/health", handler.Health)
	mux.HandleFunc("/auth/status", handler.AuthStatus)
	mux.HandleFunc("/auth/login", handler.AuthLogin)
	mux.HandleFunc("/auth/logout", handler.AuthLogout)
	mux.HandleFunc("/auth/input", handler.AuthInput)
	mux.HandleFunc("/auth/output", handler.AuthOutput)
	mux.HandleFunc("/auth/cancel", handler.AuthCancel)

	fmt.Printf("cc-remote-agent listening on %s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
