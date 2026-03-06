package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/vigo999/ms-cli/agent/session"
)

type cliOptions struct {
	Command   string // run | resume | sessions
	WorkDir   string
	Bootstrap BootstrapConfig
}

func parseCLIArgs(args []string) (cliOptions, error) {
	opts := cliOptions{
		Command: "run",
	}

	if len(args) > 0 {
		switch args[0] {
		case "resume":
			opts.Command = "resume"
			args = args[1:]
		case "sessions":
			opts.Command = "sessions"
			args = args[1:]
		}
	}

	flagArgs, positional, err := splitFlagArgs(args)
	if err != nil {
		return opts, err
	}

	fs := flag.NewFlagSet("ms-cli", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&opts.Bootstrap.Demo, "demo", false, "Run in demo mode")
	fs.StringVar(&opts.Bootstrap.ConfigPath, "config", "", "Path to config file")
	fs.StringVar(&opts.Bootstrap.URL, "url", "", "Model API base URL")
	fs.StringVar(&opts.Bootstrap.Model, "model", "", "Model name")
	fs.StringVar(&opts.Bootstrap.Key, "api-key", "", "API key")
	if err := fs.Parse(flagArgs); err != nil {
		return opts, err
	}

	workDir, err := os.Getwd()
	if err != nil {
		workDir = "."
	}
	opts.WorkDir, _ = filepath.Abs(workDir)

	switch opts.Command {
	case "run":
		if len(positional) > 0 {
			return opts, fmt.Errorf("unexpected arguments: %s", strings.Join(positional, " "))
		}
	case "resume":
		if len(positional) == 0 {
			return opts, errors.New("resume requires session id: ms-cli resume <session-id>")
		}
		if len(positional) > 1 {
			return opts, fmt.Errorf("resume accepts exactly one session id, got: %s", strings.Join(positional, " "))
		}
		opts.Bootstrap.ResumeSessionID = positional[0]
	case "sessions":
		if len(positional) != 1 || positional[0] != "list" {
			return opts, errors.New("usage: ms-cli sessions list")
		}
	}

	return opts, nil
}

func splitFlagArgs(args []string) ([]string, []string, error) {
	flagArgs := make([]string, 0, len(args))
	positional := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}

		flagArgs = append(flagArgs, arg)
		name, hasEq := splitFlagName(arg)
		if !needsValue(name) || hasEq {
			continue
		}
		if i+1 >= len(args) {
			return nil, nil, fmt.Errorf("flag %s requires a value", arg)
		}
		i++
		flagArgs = append(flagArgs, args[i])
	}
	return flagArgs, positional, nil
}

func splitFlagName(arg string) (string, bool) {
	trimmed := strings.TrimLeft(arg, "-")
	if trimmed == "" {
		return "", false
	}
	parts := strings.SplitN(trimmed, "=", 2)
	return parts[0], len(parts) == 2
}

func needsValue(name string) bool {
	switch name {
	case "config", "url", "model", "api-key":
		return true
	default:
		return false
	}
}

func runSessionsList(workDir string, out io.Writer) error {
	storePath := filepath.Join(workDir, ".mscli", "sessions")
	store, err := session.NewFileStore(storePath)
	if err != nil {
		return fmt.Errorf("init session store: %w", err)
	}

	infos, err := store.List()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tUpdatedAt\tMessages\tArchived\tWorkDir")
	for _, info := range infos {
		fmt.Fprintf(
			w,
			"%s\t%s\t%d\t%t\t%s\n",
			info.ID,
			info.UpdatedAt.Format(time.DateTime),
			info.MessageCount,
			info.Archived,
			info.WorkDir,
		)
	}
	return w.Flush()
}
