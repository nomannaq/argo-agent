package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nomanqureshi/argo/internal/tools"
)

// MCPTool wraps an MCP server tool as an Argo Tool implementation.
// This allows MCP tools to be registered in the standard tool registry
// and used by the agent just like built-in tools.
type MCPTool struct {
	client     *Client
	info       MCPToolInfo
	serverName string
}

// NewMCPTool creates a new MCPTool that bridges an MCP tool to Argo's tool interface.
func NewMCPTool(client *Client, info MCPToolInfo) *MCPTool {
	return &MCPTool{
		client:     client,
		info:       info,
		serverName: client.ServerName(),
	}
}

func (t *MCPTool) Name() string {
	// Prefix with server name to avoid collisions with built-in tools
	if t.serverName != "" {
		return fmt.Sprintf("mcp_%s_%s", sanitizeName(t.serverName), t.info.Name)
	}
	return fmt.Sprintf("mcp_%s", t.info.Name)
}

func (t *MCPTool) Description() string {
	desc := t.info.Description
	if t.serverName != "" {
		desc = fmt.Sprintf("[MCP: %s] %s", t.serverName, desc)
	}
	return desc
}

func (t *MCPTool) InputSchema() map[string]any {
	if t.info.InputSchema != nil {
		return t.info.InputSchema
	}
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

// Permission returns PermissionDangerous because MCP tools execute external code.
func (t *MCPTool) Permission() tools.PermissionLevel {
	return tools.PermissionDangerous
}

func (t *MCPTool) Execute(ctx context.Context, input string) (*tools.Result, error) {
	// Parse the input JSON into arguments map
	var arguments map[string]any
	if input != "" && input != "{}" {
		if err := json.Unmarshal([]byte(input), &arguments); err != nil {
			return &tools.Result{
				Error:   fmt.Sprintf("failed to parse tool input: %s", err),
				IsError: true,
			}, nil
		}
	}

	// Call the MCP server
	result, err := t.client.CallTool(ctx, t.info.Name, arguments)
	if err != nil {
		return &tools.Result{
			Error:   fmt.Sprintf("MCP tool call failed: %s", err),
			IsError: true,
		}, nil
	}

	// Concatenate text content blocks
	var textParts []string
	for _, content := range result.Content {
		if content.Type == "text" && content.Text != "" {
			textParts = append(textParts, content.Text)
		}
	}

	output := strings.Join(textParts, "\n")

	if result.IsError {
		return &tools.Result{
			Output:  output,
			Error:   output,
			IsError: true,
		}, nil
	}

	return &tools.Result{
		Output:  output,
		IsError: false,
	}, nil
}

// RegisterMCPTools connects to an MCP server and registers all its tools
// in the given Argo tool registry. Returns the client (caller should
// close it when done) and the number of tools registered.
func RegisterMCPTools(ctx context.Context, config ServerConfig, registry *tools.Registry) (*Client, int, error) {
	client, err := NewClient(ctx, config)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	toolInfos, err := client.ListTools(ctx)
	if err != nil {
		_ = client.Close()
		return nil, 0, fmt.Errorf("failed to list MCP tools: %w", err)
	}

	for _, info := range toolInfos {
		mcpTool := NewMCPTool(client, info)
		registry.Register(mcpTool)
	}

	return client, len(toolInfos), nil
}

// sanitizeName converts a string to a safe identifier for tool naming.
func sanitizeName(s string) string {
	s = strings.ToLower(s)
	var result strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			result.WriteRune(c)
		} else if c == ' ' || c == '-' || c == '.' {
			result.WriteRune('_')
		}
	}
	return result.String()
}
