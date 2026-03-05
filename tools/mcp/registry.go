package mcp

import (
	"context"
	"fmt"
	"sync"

	"github.com/vigo999/ms-cli/tools"
)

// Registry manages MCP connections and tools
type Registry struct {
	mu      sync.RWMutex
	clients map[string]Client // name -> client
	tools   map[string]*Adapter
}

// NewRegistry creates a new MCP registry
func NewRegistry() *Registry {
	return &Registry{
		clients: make(map[string]Client),
		tools:   make(map[string]*Adapter),
	}
}

// Connect connects to an MCP server
func (r *Registry) Connect(name string, client Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.clients[name]; exists {
		return fmt.Errorf("MCP connection %q already exists", name)
	}

	r.clients[name] = client

	// Fetch and register tools
	ctx := context.Background()
	mcpTools, err := client.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("list MCP tools: %w", err)
	}

	for _, mcpTool := range mcpTools {
		adapter, err := NewAdapter(mcpTool, client)
		if err != nil {
			// Log error but continue with other tools
			continue
		}
		// Prefix tool name with server name to avoid collisions
		toolName := fmt.Sprintf("%s_%s", name, adapter.Name())
		r.tools[toolName] = adapter
	}

	return nil
}

// Disconnect disconnects from an MCP server
func (r *Registry) Disconnect(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	client, ok := r.clients[name]
	if !ok {
		return fmt.Errorf("MCP connection %q not found", name)
	}

	// Remove tools from this connection
	prefix := name + "_"
	for toolName := range r.tools {
		if len(toolName) > len(prefix) && toolName[:len(prefix)] == prefix {
			delete(r.tools, toolName)
		}
	}

	delete(r.clients, name)
	return client.Close()
}

// Get retrieves an MCP tool by name
func (r *Registry) Get(name string) (tools.ToolWithSchema, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapter, ok := r.tools[name]
	if !ok {
		return nil, false
	}
	return adapter, true
}

// List returns all MCP tools
func (r *Registry) List() []tools.ToolWithSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]tools.ToolWithSchema, 0, len(r.tools))
	for _, adapter := range r.tools {
		list = append(list, adapter)
	}
	return list
}

// GetAllTools returns all tool names
func (r *Registry) GetAllTools() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Status returns the connection status
func (r *Registry) Status() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := make(map[string]string)
	for name := range r.clients {
		status[name] = "connected"
	}
	return status
}
