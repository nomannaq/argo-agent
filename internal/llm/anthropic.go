package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	anthropicAPIURL     = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersion = "2023-06-01"
	defaultMaxTokens    = 4096
)

func init() {
	Register("anthropic", NewAnthropicProvider)
}

type anthropicProvider struct {
	apiKey     string
	httpClient *http.Client
}

// NewAnthropicProvider creates a new Anthropic Claude provider. If apiKey is
// empty it falls back to the ANTHROPIC_API_KEY environment variable.
func NewAnthropicProvider(apiKey string) (Provider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic: API key is required; set ANTHROPIC_API_KEY or pass it explicitly")
	}
	return &anthropicProvider{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}, nil
}

func (p *anthropicProvider) Name() string {
	return "anthropic"
}

// ---------------------------------------------------------------------------
// Anthropic API request / response wire types
// ---------------------------------------------------------------------------

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	Temperature *float64           `json:"temperature,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []anthropicContentBlock
}

type anthropicContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   any             `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type anthropicTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema"`
}

type anthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []anthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence *string                 `json:"stop_sequence"`
	Usage        anthropicUsage          `json:"usage"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// ---------------------------------------------------------------------------
// Streaming SSE wire types
// ---------------------------------------------------------------------------

type anthropicStreamMessageStart struct {
	Type    string            `json:"type"`
	Message anthropicResponse `json:"message"`
}

type anthropicStreamContentBlockStart struct {
	Type         string                `json:"type"`
	Index        int                   `json:"index"`
	ContentBlock anthropicContentBlock `json:"content_block"`
}

type anthropicStreamContentBlockDelta struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text,omitempty"`
		PartialJSON string `json:"partial_json,omitempty"`
	} `json:"delta"`
}

type anthropicStreamContentBlockStop struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

type anthropicStreamMessageDelta struct {
	Type  string `json:"type"`
	Delta struct {
		StopReason   string  `json:"stop_reason"`
		StopSequence *string `json:"stop_sequence"`
	} `json:"delta"`
	Usage anthropicUsage `json:"usage"`
}

// ---------------------------------------------------------------------------
// Convert internal types → Anthropic API format
// ---------------------------------------------------------------------------

func (p *anthropicProvider) buildRequest(req *Request, stream bool) (*anthropicRequest, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultMaxTokens
	}

	model := req.Model
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}

	ar := &anthropicRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    req.SystemPrompt,
		Stream:    stream,
	}

	if req.Temperature != 0 {
		t := req.Temperature
		ar.Temperature = &t
	}

	for _, t := range req.Tools {
		ar.Tools = append(ar.Tools, anthropicTool(t))
	}

	msgs, err := convertMessagesToAnthropic(req.Messages)
	if err != nil {
		return nil, err
	}
	ar.Messages = msgs

	return ar, nil
}

func convertMessagesToAnthropic(messages []Message) ([]anthropicMessage, error) {
	var result []anthropicMessage

	for _, msg := range messages {
		switch msg.Role {
		case RoleUser:
			result = append(result, anthropicMessage{
				Role:    "user",
				Content: msg.Content,
			})

		case RoleAssistant:
			if len(msg.ToolCalls) > 0 {
				var blocks []anthropicContentBlock
				if msg.Content != "" {
					blocks = append(blocks, anthropicContentBlock{
						Type: "text",
						Text: msg.Content,
					})
				}
				for _, tc := range msg.ToolCalls {
					blocks = append(blocks, anthropicContentBlock{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Name,
						Input: json.RawMessage(tc.Arguments),
					})
				}
				result = append(result, anthropicMessage{
					Role:    "assistant",
					Content: blocks,
				})
			} else {
				result = append(result, anthropicMessage{
					Role:    "assistant",
					Content: msg.Content,
				})
			}

		case RoleTool:
			// Anthropic expects tool results as user messages with tool_result blocks.
			// Consecutive tool-role messages are merged into a single user message.
			block := anthropicContentBlock{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   msg.Content,
			}

			if len(result) > 0 {
				last := &result[len(result)-1]
				if last.Role == "user" {
					if blocks, ok := last.Content.([]anthropicContentBlock); ok {
						if len(blocks) > 0 && blocks[0].Type == "tool_result" {
							last.Content = append(blocks, block)
							continue
						}
					}
				}
			}

			result = append(result, anthropicMessage{
				Role:    "user",
				Content: []anthropicContentBlock{block},
			})

		case RoleSystem:
			// System messages are handled via the top-level "system" field.
			// If one appears inline, send it as a user message as a fallback.
			result = append(result, anthropicMessage{
				Role:    "user",
				Content: msg.Content,
			})

		default:
			return nil, fmt.Errorf("anthropic: unsupported role: %s", msg.Role)
		}
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Convert Anthropic API response → internal types
// ---------------------------------------------------------------------------

func convertAnthropicResponse(resp *anthropicResponse) (*Message, *Usage) {
	msg := &Message{
		Role: RoleAssistant,
	}

	var textParts []string
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "tool_use":
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: string(block.Input),
			})
		}
	}

	msg.Content = strings.Join(textParts, "")

	usage := &Usage{
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
	}

	return msg, usage
}

// ---------------------------------------------------------------------------
// SendMessage (non-streaming)
// ---------------------------------------------------------------------------

func (p *anthropicProvider) SendMessage(ctx context.Context, req *Request) (*Message, *Usage, error) {
	ar, err := p.buildRequest(req, false)
	if err != nil {
		return nil, nil, fmt.Errorf("anthropic: failed to build request: %w", err)
	}

	body, err := json.Marshal(ar)
	if err != nil {
		return nil, nil, fmt.Errorf("anthropic: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("anthropic: failed to create HTTP request: %w", err)
	}
	p.setHeaders(httpReq, false)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("anthropic: HTTP request failed: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("anthropic: failed to read response body: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, nil, parseAnthropicError(httpResp.StatusCode, respBody)
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, nil, fmt.Errorf("anthropic: failed to unmarshal response: %w", err)
	}

	msg, usage := convertAnthropicResponse(&apiResp)
	return msg, usage, nil
}

// ---------------------------------------------------------------------------
// StreamMessage (SSE streaming)
// ---------------------------------------------------------------------------

func (p *anthropicProvider) StreamMessage(ctx context.Context, req *Request) (<-chan StreamEvent, error) {
	ar, err := p.buildRequest(req, true)
	if err != nil {
		return nil, fmt.Errorf("anthropic: failed to build request: %w", err)
	}

	body, err := json.Marshal(ar)
	if err != nil {
		return nil, fmt.Errorf("anthropic: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: failed to create HTTP request: %w", err)
	}
	p.setHeaders(httpReq, true)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: HTTP request failed: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		defer func() { _ = httpResp.Body.Close() }()
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, parseAnthropicError(httpResp.StatusCode, respBody)
	}

	ch := make(chan StreamEvent, 64)

	go func() {
		defer close(ch)
		defer func() { _ = httpResp.Body.Close() }()
		p.processSSEStream(ctx, httpResp.Body, ch)
	}()

	return ch, nil
}

// activeToolCall tracks tool call state while streaming.
type activeToolCall struct {
	id   string
	name string
	args strings.Builder
}

func (p *anthropicProvider) processSSEStream(ctx context.Context, body io.Reader, ch chan<- StreamEvent) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	// Track active tool calls by content block index.
	toolCalls := map[int]*activeToolCall{}
	var totalUsage Usage

	var currentEvent string
	var dataLines []string

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			ch <- StreamEvent{Type: EventError, Error: ctx.Err()}
			return
		default:
		}

		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			dataLines = nil
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
			continue
		}

		// Empty line marks end of an SSE event.
		if line == "" && currentEvent != "" {
			data := strings.Join(dataLines, "\n")
			p.dispatchSSEEvent(currentEvent, data, ch, toolCalls, &totalUsage)
			currentEvent = ""
			dataLines = nil
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamEvent{Type: EventError, Error: fmt.Errorf("anthropic: stream read error: %w", err)}
	}
}

func (p *anthropicProvider) dispatchSSEEvent(
	event string,
	data string,
	ch chan<- StreamEvent,
	toolCalls map[int]*activeToolCall,
	totalUsage *Usage,
) {
	switch event {
	case "message_start":
		var msg anthropicStreamMessageStart
		if err := json.Unmarshal([]byte(data), &msg); err != nil {
			ch <- StreamEvent{Type: EventError, Error: fmt.Errorf("anthropic: failed to parse message_start: %w", err)}
			return
		}
		totalUsage.InputTokens = msg.Message.Usage.InputTokens
		totalUsage.OutputTokens = msg.Message.Usage.OutputTokens

	case "content_block_start":
		var block anthropicStreamContentBlockStart
		if err := json.Unmarshal([]byte(data), &block); err != nil {
			ch <- StreamEvent{Type: EventError, Error: fmt.Errorf("anthropic: failed to parse content_block_start: %w", err)}
			return
		}
		if block.ContentBlock.Type == "tool_use" {
			tc := &activeToolCall{
				id:   block.ContentBlock.ID,
				name: block.ContentBlock.Name,
			}
			toolCalls[block.Index] = tc
			ch <- StreamEvent{
				Type: EventToolCallStart,
				ToolCall: &ToolCall{
					ID:   tc.id,
					Name: tc.name,
				},
			}
		}

	case "content_block_delta":
		var delta anthropicStreamContentBlockDelta
		if err := json.Unmarshal([]byte(data), &delta); err != nil {
			ch <- StreamEvent{Type: EventError, Error: fmt.Errorf("anthropic: failed to parse content_block_delta: %w", err)}
			return
		}
		switch delta.Delta.Type {
		case "text_delta":
			ch <- StreamEvent{
				Type:    EventText,
				Content: delta.Delta.Text,
			}
		case "input_json_delta":
			if tc, ok := toolCalls[delta.Index]; ok {
				tc.args.WriteString(delta.Delta.PartialJSON)
			}
			ch <- StreamEvent{
				Type:    EventToolCallDelta,
				Content: delta.Delta.PartialJSON,
			}
		}

	case "content_block_stop":
		var stop anthropicStreamContentBlockStop
		if err := json.Unmarshal([]byte(data), &stop); err != nil {
			ch <- StreamEvent{Type: EventError, Error: fmt.Errorf("anthropic: failed to parse content_block_stop: %w", err)}
			return
		}
		if tc, ok := toolCalls[stop.Index]; ok {
			ch <- StreamEvent{
				Type: EventToolCallComplete,
				ToolCall: &ToolCall{
					ID:        tc.id,
					Name:      tc.name,
					Arguments: tc.args.String(),
				},
			}
			delete(toolCalls, stop.Index)
		}

	case "message_delta":
		var md anthropicStreamMessageDelta
		if err := json.Unmarshal([]byte(data), &md); err != nil {
			ch <- StreamEvent{Type: EventError, Error: fmt.Errorf("anthropic: failed to parse message_delta: %w", err)}
			return
		}
		totalUsage.OutputTokens += md.Usage.OutputTokens

	case "message_stop":
		ch <- StreamEvent{
			Type: EventDone,
			Done: true,
			Usage: &Usage{
				InputTokens:  totalUsage.InputTokens,
				OutputTokens: totalUsage.OutputTokens,
			},
		}

	case "ping":
		// Heartbeat – ignore.

	case "error":
		ch <- StreamEvent{
			Type:  EventError,
			Error: fmt.Errorf("anthropic: stream error: %s", data),
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (p *anthropicProvider) setHeaders(req *http.Request, stream bool) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
}

func parseAnthropicError(statusCode int, body []byte) error {
	var errResp anthropicErrorResponse
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		return fmt.Errorf("anthropic: API error (%d): %s: %s",
			statusCode, errResp.Error.Type, errResp.Error.Message)
	}
	return fmt.Errorf("anthropic: API error (%d): %s", statusCode, string(body))
}
