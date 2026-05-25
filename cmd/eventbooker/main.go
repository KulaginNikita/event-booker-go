package main

import (
	"flag"
	"log"

	"github.com/KulaginNikita/event-booker/internal/app"
)

func main() {
	configPath := flag.String("config", "config/config.yml", "path to config file")
	flag.Parse()

	if err := app.Run(*configPath); err != nil {
		log.Fatal(err)
	}
}
