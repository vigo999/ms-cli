// Package slash provides slash command definitions and autocomplete functionality.
package slash

import (
	"strings"
)

// Command represents a slash command.
type Command struct {
	Name        string
	Description string
	Usage       string
	Handler     func(args []string) string
}

// Registry holds all available slash commands.
type Registry struct {
	commands map[string]Command
}

// NewRegistry creates a new slash command registry.
func NewRegistry() *Registry {
	r := &Registry{
		commands: make(map[string]Command),
	}
	r.registerDefaults()
	return r
}

// Register adds a command to the registry.
func (r *Registry) Register(cmd Command) {
	r.commands[cmd.Name] = cmd
}

// Get retrieves a command by name.
func (r *Registry) Get(name string) (Command, bool) {
	cmd, ok := r.commands[name]
	return cmd, ok
}

// List returns all registered commands.
func (r *Registry) List() []Command {
	cmds := make([]Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		cmds = append(cmds, cmd)
	}
	return cmds
}

// Match returns commands that match the given prefix.
func (r *Registry) Match(prefix string) []Command {
	if prefix == "" {
		return r.List()
	}

	var matches []Command
	for name, cmd := range r.commands {
		if strings.HasPrefix(name, prefix) {
			matches = append(matches, cmd)
		}
	}
	return matches
}

// Suggestions returns command names that match the given input.
func (r *Registry) Suggestions(input string) []string {
	// If input is just "/", return all commands
	if input == "/" {
		names := make([]string, 0, len(r.commands))
		for name := range r.commands {
			names = append(names, name)
		}
		return names
	}

	// Otherwise match by prefix
	var matches []string
	for name := range r.commands {
		if strings.HasPrefix(name, input) {
			matches = append(matches, name)
		}
	}
	return matches
}

// IsSlashCommand checks if input starts with "/".
func IsSlashCommand(input string) bool {
	return strings.HasPrefix(strings.TrimSpace(input), "/")
}

// Parse parses a slash command input into name and arguments.
func Parse(input string) (string, []string) {
	input = strings.TrimSpace(input)
	if !IsSlashCommand(input) {
		return "", nil
	}

	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", nil
	}

	name := parts[0]
	args := []string{}
	if len(parts) > 1 {
		args = parts[1:]
	}

	return name, args
}

func (r *Registry) registerDefaults() {
	r.Register(Command{
		Name:        "/roadmap",
		Description: "Check roadmap status",
		Usage:       "/roadmap status [path]",
	})

	r.Register(Command{
		Name:        "/weekly",
		Description: "Check weekly update status",
		Usage:       "/weekly status [path]",
	})

	r.Register(Command{
		Name:        "/model",
		Description: "Show or switch model",
		Usage:       "/model [openai:]model",
	})

	r.Register(Command{
		Name:        "/exit",
		Description: "Exit the application",
		Usage:       "/exit",
	})

	r.Register(Command{
		Name:        "/compact",
		Description: "Compact conversation context",
		Usage:       "/compact",
	})

	r.Register(Command{
		Name:        "/clear",
		Description: "Clear the chat history",
		Usage:       "/clear",
	})

	r.Register(Command{
		Name:        "/test",
		Description: "Test API connectivity",
		Usage:       "/test",
	})

	r.Register(Command{
		Name:        "/permission",
		Description: "Manage tool permissions",
		Usage:       "/permission [tool] [level]",
	})

	r.Register(Command{
		Name:        "/yolo",
		Description: "Toggle yolo mode (auto-approve all)",
		Usage:       "/yolo",
	})

	r.Register(Command{
		Name:        "/mouse",
		Description: "Toggle mouse wheel scrolling",
		Usage:       "/mouse [on|off|toggle|status]",
	})

	r.Register(Command{
		Name:        "/help",
		Description: "Show available commands",
		Usage:       "/help",
	})
}

// DefaultRegistry is the global slash command registry.
var DefaultRegistry = NewRegistry()

// Register registers a command to the default registry.
func Register(cmd Command) {
	DefaultRegistry.Register(cmd)
}

// Get retrieves a command from the default registry.
func Get(name string) (Command, bool) {
	return DefaultRegistry.Get(name)
}

// List returns all commands from the default registry.
func List() []Command {
	return DefaultRegistry.List()
}

// Suggestions returns suggestions from the default registry.
func Suggestions(input string) []string {
	return DefaultRegistry.Suggestions(input)
}
