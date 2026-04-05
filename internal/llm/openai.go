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
	openaiAPIURL     = "https://api.openai.com/v1/chat/completions"
	openaiDefaultModel = "gpt-4o"
)

func init() {
	Register("openai", NewOpenAIProvider)
}

type openaiProvider struct {
	apiKey     string
	httpClient *http.Client
}

// NewOpenAIProvider creates a new OpenAI provider. If apiKey is empty it falls
// back to the OPENAI_API_KEY environment variable.
func NewOpenAIProvider(apiKey string) (Provider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("openai: API key is required; set OPENAI_API_KEY or pass it explicitly")
	}
	return &openaiProvider{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}, nil
}

func (p *openaiProvider) Name() string {
	return "openai"
}

// ---------------------------------------------------------------------------
// OpenAI API request / response wire types
// ---------------------------------------------------------------------------

type openaiRequest struct {
	Model       string              `json:"model"`
	Messages    []openaiMessage     `json:"messages"`
	Tools       []openaiTool        `json:"tools,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature *float64            `json:"temperature,omitempty"`
	Stream      bool                `json:"stream,omitempty"`
	StreamOptions *openaiStreamOpts `json:"stream_options,omitempty"`
}

type openaiStreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type openaiMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openaiToolCall struct {
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type"`
	Function openaiFunctionCall `json:"function"`
	Index    *int               `json:"index,omitempty"`
}

type openaiFunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments"`
}

type openaiTool struct {
	Type     string             `json:"type"`
	Function openaiToolFunction `json:"function"`
}

type openaiToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type openaiResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Choices []openaiChoice `json:"choices"`
	Usage   openaiUsage    `json:"usage"`
}

type openaiChoice struct {
	Index        int           `json:"index"`
	Message      openaiMessage `json:"message"`
	Delta        openaiMessage `json:"delta"`
	FinishReason *string       `json:"finish_reason"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openaiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error"`
}

// ---------------------------------------------------------------------------
// Convert internal types → OpenAI API format
// ---------------------------------------------------------------------------

func (p *openaiProvider) buildRequest(req *Request, stream bool) *openaiRequest {
	model := req.Model
	if model == "" {
		model = openaiDefaultModel
	}

	or := &openaiRequest{
		Model:  model,
		Stream: stream,
	}

	if stream {
		or.StreamOptions = &openaiStreamOpts{IncludeUsage: true}
	}

	if req.MaxTokens > 0 {
		or.MaxTokens = req.MaxTokens
	}

	if req.Temperature != 0 {
		t := req.Temperature
		or.Temperature = &t
	}

	for _, t := range req.Tools {
		or.Tools = append(or.Tools, openaiTool{
			Type: "function",
			Function: openaiToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	or.Messages = convertMessagesToOpenAI(req.SystemPrompt, req.Messages)

	return or
}

func convertMessagesToOpenAI(systemPrompt string, messages []Message) []openaiMessage {
	var result []openaiMessage

	if systemPrompt != "" {
		result = append(result, openaiMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	for _, msg := range messages {
		switch msg.Role {
		case RoleSystem:
			result = append(result, openaiMessage{
				Role:    "system",
				Content: msg.Content,
			})

		case RoleUser:
			result = append(result, openaiMessage{
				Role:    "user",
				Content: msg.Content,
			})

		case RoleAssistant:
			om := openaiMessage{
				Role:    "assistant",
				Content: msg.Content,
			}
			for _, tc := range msg.ToolCalls {
				om.ToolCalls = append(om.ToolCalls, openaiToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: openaiFunctionCall{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				})
			}
			result = append(result, om)

		case RoleTool:
			result = append(result, openaiMessage{
				Role:       "tool",
				Content:    msg.Content,
				ToolCallID: msg.ToolCallID,
			})
		}
	}

	return result
}

// ---------------------------------------------------------------------------
// Convert OpenAI API response → internal types
// ---------------------------------------------------------------------------

func convertOpenAIResponse(resp *openaiResponse) (*Message, *Usage) {
	msg := &Message{
		Role: RoleAssistant,
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		msg.Content = choice.Message.Content
		for _, tc := range choice.Message.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
	}

	usage := &Usage{
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
	}

	return msg, usage
}

// ---------------------------------------------------------------------------
// SendMessage (non-streaming)
// ---------------------------------------------------------------------------

func (p *openaiProvider) SendMessage(ctx context.Context, req *Request) (*Message, *Usage, error) {
	or := p.buildRequest(req, false)

	body, err := json.Marshal(or)
	if err != nil {
		return nil, nil, fmt.Errorf("openai: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, openaiAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("openai: failed to create HTTP request: %w", err)
	}
	p.setHeaders(httpReq, false)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("openai: HTTP request failed: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("openai: failed to read response body: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, nil, parseOpenAIError(httpResp.StatusCode, respBody)
	}

	var apiResp openaiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, nil, fmt.Errorf("openai: failed to unmarshal response: %w", err)
	}

	msg, usage := convertOpenAIResponse(&apiResp)
	return msg, usage, nil
}

// ---------------------------------------------------------------------------
// StreamMessage (SSE streaming)
// ---------------------------------------------------------------------------

func (p *openaiProvider) StreamMessage(ctx context.Context, req *Request) (<-chan StreamEvent, error) {
	or := p.buildRequest(req, true)

	body, err := json.Marshal(or)
	if err != nil {
		return nil, fmt.Errorf("openai: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, openaiAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: failed to create HTTP request: %w", err)
	}
	p.setHeaders(httpReq, true)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: HTTP request failed: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		defer func() { _ = httpResp.Body.Close() }()
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, parseOpenAIError(httpResp.StatusCode, respBody)
	}

	ch := make(chan StreamEvent, 64)

	go func() {
		defer close(ch)
		defer func() { _ = httpResp.Body.Close() }()
		p.processSSEStream(ctx, httpResp.Body, ch)
	}()

	return ch, nil
}

// openaiActiveToolCall tracks tool call state while streaming.
type openaiActiveToolCall struct {
	id   string
	name string
	args strings.Builder
}

func (p *openaiProvider) processSSEStream(ctx context.Context, body io.Reader, ch chan<- StreamEvent) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	// Track active tool calls by index.
	toolCalls := map[int]*openaiActiveToolCall{}
	var totalUsage Usage

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			ch <- StreamEvent{Type: EventError, Error: ctx.Err()}
			return
		default:
		}

		line := scanner.Text()

		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// The stream is done.
		if data == "[DONE]" {
			ch <- StreamEvent{
				Type: EventDone,
				Done: true,
				Usage: &Usage{
					InputTokens:  totalUsage.InputTokens,
					OutputTokens: totalUsage.OutputTokens,
				},
			}
			return
		}

		var chunk openaiResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			ch <- StreamEvent{
				Type:  EventError,
				Error: fmt.Errorf("openai: failed to parse stream chunk: %w", err),
			}
			continue
		}

		// Capture usage from the final chunk (stream_options.include_usage).
		if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
			totalUsage.InputTokens = chunk.Usage.PromptTokens
			totalUsage.OutputTokens = chunk.Usage.CompletionTokens
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		// Handle text content delta.
		if delta.Content != "" {
			ch <- StreamEvent{
				Type:    EventText,
				Content: delta.Content,
			}
		}

		// Handle tool call deltas.
		for _, tc := range delta.ToolCalls {
			idx := 0
			if tc.Index != nil {
				idx = *tc.Index
			}

			active, exists := toolCalls[idx]
			if !exists {
				// New tool call starting.
				active = &openaiActiveToolCall{
					id:   tc.ID,
					name: tc.Function.Name,
				}
				toolCalls[idx] = active

				ch <- StreamEvent{
					Type: EventToolCallStart,
					ToolCall: &ToolCall{
						ID:   active.id,
						Name: active.name,
					},
				}
			}

			// Accumulate argument fragments.
			if tc.Function.Arguments != "" {
				active.args.WriteString(tc.Function.Arguments)
				ch <- StreamEvent{
					Type:    EventToolCallDelta,
					Content: tc.Function.Arguments,
				}
			}
		}

		// If finish_reason is "tool_calls" or "stop", emit tool call completions.
		if chunk.Choices[0].FinishReason != nil {
			reason := *chunk.Choices[0].FinishReason
			if reason == "tool_calls" || reason == "stop" {
				for idx, active := range toolCalls {
					ch <- StreamEvent{
						Type: EventToolCallComplete,
						ToolCall: &ToolCall{
							ID:        active.id,
							Name:      active.name,
							Arguments: active.args.String(),
						},
					}
					delete(toolCalls, idx)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamEvent{Type: EventError, Error: fmt.Errorf("openai: stream read error: %w", err)}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (p *openaiProvider) setHeaders(req *http.Request, stream bool) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
}

func parseOpenAIError(statusCode int, body []byte) error {
	var errResp openaiErrorResponse
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		return fmt.Errorf("openai: API error (%d): %s: %s",
			statusCode, errResp.Error.Type, errResp.Error.Message)
	}
	return fmt.Errorf("openai: API error (%d): %s", statusCode, string(body))
}
