package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	var (
		demo       = flag.Bool("demo", false, "Run in demo mode")
		configPath = flag.String("config", "", "Path to config file")
		url        = flag.String("url", "", "OpenAI-compatible base URL")
		model      = flag.String("model", "", "Model name")
		apiKey     = flag.String("api-key", "", "API key")
	)
	flag.Parse()

	app, err := Bootstrap(BootstrapConfig{
		Demo:       *demo,
		ConfigPath: *configPath,
		URL:        *url,
		Model:      *model,
		Key:        *apiKey,
	})
	if err != nil {
		log.Fatalf("Failed to bootstrap: %v", err)
	}

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
