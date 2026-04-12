package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-tmux-tunnel/internal/api"
	"github.com/pollenjp/cc-tunnel/apps/cc-tmux-tunnel/internal/session"
)

func main() {
	defaultAddr := ":9090"
	if p := os.Getenv("PORT"); p != "" {
		defaultAddr = ":" + p
	}
	addr := flag.String("addr", defaultAddr, "listen address")
	flag.Parse()

	mgr := session.NewManager()
	handler := api.NewHandler(mgr)

	mux := http.NewServeMux()
	api.HandlerFromMux(handler, mux)

	srv := &http.Server{Addr: *addr, Handler: mux}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		fmt.Printf("cc-tmux-tunnel listening on %s\n", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	fmt.Println("\nShutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	if err := mgr.Close(); err != nil {
		log.Printf("session cleanup errors: %v", err)
	}
	fmt.Println("Shutdown complete.")
}
