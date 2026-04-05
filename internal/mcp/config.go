package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MCPConfig holds configuration for MCP servers.
type MCPConfig struct {
	Servers map[string]ServerConfig `json:"mcpServers"`
}

// LoadMCPConfig reads MCP server configuration from ~/.argo/mcp.json.
// Returns an empty config (no error) if the file doesn't exist.
func LoadMCPConfig() (*MCPConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine home directory: %w", err)
	}

	configPath := filepath.Join(home, ".argo", "mcp.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &MCPConfig{Servers: make(map[string]ServerConfig)}, nil
		}
		return nil, fmt.Errorf("failed to read MCP config: %w", err)
	}

	var config MCPConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse MCP config: %w", err)
	}

	if config.Servers == nil {
		config.Servers = make(map[string]ServerConfig)
	}

	return &config, nil
}

// ExampleMCPConfig returns an example MCP configuration string for documentation.
func ExampleMCPConfig() string {
	return `{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/allowed/dir"]
    },
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": ["GITHUB_PERSONAL_ACCESS_TOKEN=your-token-here"]
    }
  }
}`
}
