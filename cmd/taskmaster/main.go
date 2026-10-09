package main

import (
	"log"
	"net/http"

	"taskmaster"
)

func main() {
	cfg := taskmaster.LoadConfig()
	// No resolve bearer by design: in-cluster transport trusts
	// NetworkPolicy, not tokens. The edge JWT gate stays — caller
	// identity is not transport auth.
	if cfg.EdgeSecret == "" {
		log.Fatal("missing TASKMASTER_EDGE_SECRET: refusing to serve unauthenticated")
	}
	h := &taskmaster.AuthHandler{
		EdgeSecret: []byte(cfg.EdgeSecret),
		Directory: &taskmaster.HTTPDirectory{
			BaseURL: cfg.DirectoryURL, HTTP: &http.Client{},
		},
		NewCaller: func(apiKey string) taskmaster.DownstreamCaller {
			return &taskmaster.OpenAICaller{
				BaseURL: cfg.BaseURL, Model: cfg.Model, APIKey: apiKey, HTTP: &http.Client{},
			}
		},
	}
	log.Printf("taskmaster listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, taskmaster.NewAuthHTTPHandler(h)); err != nil {
		log.Fatal(err)
	}
}
