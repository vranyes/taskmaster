package main

import (
	"log"
	"net/http"

	"taskmaster"
)

func main() {
	cfg := taskmaster.LoadConfig()
	if cfg.EdgeSecret == "" || cfg.ResolveSecret == "" {
		log.Fatal("missing TASKMASTER_EDGE_SECRET or TASKMASTER_RESOLVE_SECRET: refusing to serve unauthenticated")
	}
	h := &taskmaster.AuthHandler{
		EdgeSecret: []byte(cfg.EdgeSecret),
		Directory: &taskmaster.HTTPDirectory{
			BaseURL: cfg.DirectoryURL, Secret: cfg.ResolveSecret, HTTP: &http.Client{},
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
