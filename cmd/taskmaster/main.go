package main

import (
	"net/http"
	"os"

	"taskmaster"
)

func main() {
	log := taskmaster.SetupDefaultLogger()
	cfg := taskmaster.LoadConfig()
	// No resolve bearer by design: in-cluster transport trusts
	// NetworkPolicy, not tokens. The edge JWT gate stays — caller
	// identity is not transport auth.
	if cfg.EdgeSecret == "" {
		log.Error("config.invalid", "reason", "missing TASKMASTER_EDGE_SECRET: refusing to serve unauthenticated")
		os.Exit(1)
	}
	dir := &taskmaster.HTTPDirectory{
		BaseURL: cfg.DirectoryURL, HTTP: &http.Client{}, Logger: log,
	}
	h := &taskmaster.AuthHandler{
		EdgeSecret: []byte(cfg.EdgeSecret),
		Directory:  dir,
		Logger:     log,
		NewCaller: func(apiKey string) taskmaster.DownstreamCaller {
			// apiKey is per-user key material: never logged.
			return &taskmaster.OpenAICaller{
				BaseURL: cfg.BaseURL, Model: cfg.Model, APIKey: apiKey, HTTP: &http.Client{}, Logger: log,
			}
		},
	}
	// Startup facts only: no secrets, keys, or tokens.
	log.Info("server.start",
		"addr", cfg.Addr,
		"model", cfg.Model,
		"base_url", cfg.BaseURL,
		"directory_url", cfg.DirectoryURL,
	)
	if err := http.ListenAndServe(cfg.Addr, taskmaster.NewAuthHTTPHandler(h)); err != nil {
		log.Error("server.stopped", "error", err.Error())
		os.Exit(1)
	}
}
