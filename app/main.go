package main

import (
	"log"
	"os"
)

func main() {
	demo := len(os.Args) > 1 && os.Args[1] == "--demo"

	app, err := Bootstrap(demo)
	if err != nil {
		log.Fatal(err)
	}
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
