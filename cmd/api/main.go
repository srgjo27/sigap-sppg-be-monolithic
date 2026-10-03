package main

import (
	"log"

	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/config"
	"github.com/srgjo27/sigap-sppg-be-monolithic/internal/server"
)

func main() {
	cfg := config.Load()

	router := server.New(cfg)

	if err := router.Run(cfg.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}
