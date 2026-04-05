package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/pollenjp/cc-tunnel/internal/api"
	"github.com/pollenjp/cc-tunnel/internal/session"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	mgr := session.NewManager()
	handler := api.NewHandler(mgr)

	mux := http.NewServeMux()
	api.HandlerFromMux(handler, mux)

	fmt.Printf("cc-tunnel listening on %s\n", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
