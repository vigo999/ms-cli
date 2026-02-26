package main

import "log"

func main() {
	app, err := Bootstrap()
	if err != nil {
		log.Fatal(err)
	}
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
