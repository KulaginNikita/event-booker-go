package main

import (
	"log"

	"github.com/KulaginNikita/event-booker/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
