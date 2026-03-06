package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	opts, err := parseCLIArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	if opts.Command == "sessions" {
		if err := runSessionsList(opts.WorkDir, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	app, err := Bootstrap(BootstrapConfig{
		Demo:            opts.Bootstrap.Demo,
		ConfigPath:      opts.Bootstrap.ConfigPath,
		URL:             opts.Bootstrap.URL,
		Model:           opts.Bootstrap.Model,
		Key:             opts.Bootstrap.Key,
		ResumeSessionID: opts.Bootstrap.ResumeSessionID,
	})
	if err != nil {
		log.Fatalf("Failed to bootstrap: %v", err)
	}

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
