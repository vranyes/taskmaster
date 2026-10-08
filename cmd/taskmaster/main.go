package main

import (
	"log"
	"net/http"

	"taskmaster"
)

func main() {
	cfg := taskmaster.LoadConfig()
	caller := &taskmaster.OpenAICaller{
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
		APIKey:  cfg.APIKey,
		HTTP:    &http.Client{},
	}
	log.Printf("taskmaster listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, taskmaster.NewHTTPHandler(caller)); err != nil {
		log.Fatal(err)
	}
}
