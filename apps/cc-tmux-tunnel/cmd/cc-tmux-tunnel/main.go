package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/pollenjp/cc-tunnel/apps/cc-tmux-tunnel/internal/api"
	"github.com/pollenjp/cc-tunnel/apps/cc-tmux-tunnel/internal/session"
)

func main() {
	addr := flag.String("addr", ":9090", "listen address")
	flag.Parse()

	mgr := session.NewManager()
	handler := api.NewHandler(mgr)

	mux := http.NewServeMux()
	api.HandlerFromMux(handler, mux)

	fmt.Printf("cc-tmux-tunnel listening on %s\n", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
