package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
)

// Client is an MCP client that communicates with an MCP server over stdio.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	stderr io.ReadCloser

	mu     sync.Mutex
	nextID atomic.Int64

	serverInfo   *Implementation
	capabilities *ServerCapabilities

	closed bool
}

// ServerConfig describes how to launch an MCP server.
type ServerConfig struct {
	// Command is the executable to run (e.g., "npx", "python").
	Command string `json:"command"`
	// Args are the command-line arguments.
	Args []string `json:"args,omitempty"`
	// Env are additional environment variables (KEY=VALUE format).
	Env []string `json:"env,omitempty"`
}

// NewClient creates and starts a new MCP client connected to the given server.
// It launches the subprocess and performs the MCP initialization handshake.
func NewClient(ctx context.Context, config ServerConfig) (*Client, error) {
	cmd := exec.CommandContext(ctx, config.Command, config.Args...)
	if len(config.Env) > 0 {
		cmd.Env = append(cmd.Environ(), config.Env...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp: failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp: failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp: failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp: failed to start server %q: %w", config.Command, err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	client := &Client{
		cmd:    cmd,
		stdin:  stdin,
		stdout: scanner,
		stderr: stderr,
	}

	// Perform initialization handshake
	if err := client.initialize(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("mcp: initialization failed: %w", err)
	}

	return client, nil
}

// initialize performs the MCP initialize handshake.
func (c *Client) initialize(ctx context.Context) error {
	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo: Implementation{
			Name:    "argo",
			Version: "1.0.0",
		},
		Capabilities: ClientCapabilities{},
	}

	var result InitializeResult
	if err := c.call(ctx, "initialize", params, &result); err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	c.serverInfo = &result.ServerInfo
	c.capabilities = &result.Capabilities

	// Send initialized notification (no response expected)
	if err := c.notify(ctx, "notifications/initialized", nil); err != nil {
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	return nil
}

// ListTools returns the list of tools available on the MCP server.
func (c *Client) ListTools(ctx context.Context) ([]MCPToolInfo, error) {
	var result ListToolsResult
	if err := c.call(ctx, "tools/list", nil, &result); err != nil {
		return nil, fmt.Errorf("list tools failed: %w", err)
	}
	return result.Tools, nil
}

// CallTool invokes a tool on the MCP server and returns the result.
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]any) (*CallToolResult, error) {
	params := CallToolParams{
		Name:      name,
		Arguments: arguments,
	}

	var result CallToolResult
	if err := c.call(ctx, "tools/call", params, &result); err != nil {
		return nil, fmt.Errorf("call tool %q failed: %w", name, err)
	}

	return &result, nil
}

// ServerName returns the server's name from initialization, or empty string.
func (c *Client) ServerName() string {
	if c.serverInfo != nil {
		return c.serverInfo.Name
	}
	return ""
}

// Close shuts down the MCP client and terminates the subprocess.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	// Close stdin to signal the server to shut down
	if c.stdin != nil {
		_ = c.stdin.Close()
	}

	// Wait for process to exit (with timeout)
	done := make(chan error, 1)
	go func() {
		done <- c.cmd.Wait()
	}()

	select {
	case <-done:
	default:
		// Force kill if it doesn't exit
		if c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
	}

	return nil
}

// call sends a JSON-RPC request and waits for the response.
func (c *Client) call(ctx context.Context, method string, params any, result any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	id := int(c.nextID.Add(1))
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// Encode and send request
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		return fmt.Errorf("failed to write request: %w", err)
	}

	// Read response
	if !c.stdout.Scan() {
		if err := c.stdout.Err(); err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
		return fmt.Errorf("server closed connection")
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(c.stdout.Bytes(), &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.Error != nil {
		return resp.Error
	}

	if result != nil && resp.Result != nil {
		if err := json.Unmarshal(resp.Result, result); err != nil {
			return fmt.Errorf("failed to parse result: %w", err)
		}
	}

	return nil
}

// notify sends a JSON-RPC notification (no ID, no response expected).
func (c *Client) notify(ctx context.Context, method string, params any) error {
	if c.closed {
		return fmt.Errorf("client is closed")
	}

	// Notifications have no ID
	type notification struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  any    `json:"params,omitempty"`
	}

	req := notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		return fmt.Errorf("failed to write notification: %w", err)
	}

	return nil
}
