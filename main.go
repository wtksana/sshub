package main

import (
	"flag"
	"log"

	"sshub/internal/desktop"
)

func main() {
	dataDir := flag.String("data-dir", "", "application configuration directory; default user config/SSHub")
	flag.Parse()
	if err := desktop.Run(*dataDir); err != nil {
		log.Fatal(err)
	}
}
