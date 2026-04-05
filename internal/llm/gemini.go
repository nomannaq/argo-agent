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
	geminiBaseURL     = "https://generativelanguage.googleapis.com/v1beta/models"
	geminiDefaultModel = "gemini-2.5-flash"
)

func init() {
	Register("gemini", NewGeminiProvider)
}

type geminiProvider struct {
	apiKey     string
	httpClient *http.Client
}

// NewGeminiProvider creates a new Google Gemini provider. If apiKey is empty it
// falls back to the GEMINI_API_KEY and then GOOGLE_API_KEY environment variables.
func NewGeminiProvider(apiKey string) (Provider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("gemini: API key is required; set GEMINI_API_KEY or GOOGLE_API_KEY")
	}
	return &geminiProvider{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}, nil
}

func (p *geminiProvider) Name() string {
	return "gemini"
}

// ---------------------------------------------------------------------------
// Gemini API request / response wire types
// ---------------------------------------------------------------------------

type geminiRequest struct {
	Contents         []geminiContent         `json:"contents"`
	Tools            []geminiToolDeclaration  `json:"tools,omitempty"`
	SystemInstruction *geminiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig *geminiGenerationConfig  `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall   `json:"functionCall,omitempty"`
	FunctionResponse *geminiFuncResponse   `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type geminiFuncResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiToolDeclaration struct {
	FunctionDeclarations []geminiFunctionDecl `json:"functionDeclarations"`
}

type geminiFunctionDecl struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type geminiGenerationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate  `json:"candidates"`
	UsageMetadata *geminiUsage       `json:"usageMetadata,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason,omitempty"`
}

type geminiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type geminiErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// ---------------------------------------------------------------------------
// Convert internal types → Gemini API format
// ---------------------------------------------------------------------------

func (p *geminiProvider) buildRequest(req *Request) *geminiRequest {
	gr := &geminiRequest{}

	// System instruction.
	if req.SystemPrompt != "" {
		gr.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: req.SystemPrompt}},
		}
	}

	// Generation config.
	gc := &geminiGenerationConfig{}
	hasConfig := false
	if req.MaxTokens > 0 {
		gc.MaxOutputTokens = req.MaxTokens
		hasConfig = true
	}
	if req.Temperature != 0 {
		t := req.Temperature
		gc.Temperature = &t
		hasConfig = true
	}
	if hasConfig {
		gr.GenerationConfig = gc
	}

	// Tools.
	if len(req.Tools) > 0 {
		var decls []geminiFunctionDecl
		for _, t := range req.Tools {
			decls = append(decls, geminiFunctionDecl{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			})
		}
		gr.Tools = []geminiToolDeclaration{{FunctionDeclarations: decls}}
	}

	// Messages.
	gr.Contents = convertMessagesToGemini(req.Messages)

	return gr
}

// buildToolCallIDMap scans messages and builds a mapping from synthetic tool
// call IDs back to function names. This is needed because Gemini does not use
// tool call IDs — it matches function responses by name.
func buildToolCallIDMap(messages []Message) map[string]string {
	m := make(map[string]string)
	for _, msg := range messages {
		for _, tc := range msg.ToolCalls {
			m[tc.ID] = tc.Name
		}
	}
	return m
}

func convertMessagesToGemini(messages []Message) []geminiContent {
	var result []geminiContent
	idMap := buildToolCallIDMap(messages)

	for _, msg := range messages {
		switch msg.Role {
		case RoleUser:
			result = append(result, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: msg.Content}},
			})

		case RoleAssistant:
			var parts []geminiPart
			if msg.Content != "" {
				parts = append(parts, geminiPart{Text: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				var args map[string]any
				if tc.Arguments != "" {
					// Best-effort parse; if it fails we send an empty map.
					_ = json.Unmarshal([]byte(tc.Arguments), &args)
				}
				parts = append(parts, geminiPart{
					FunctionCall: &geminiFunctionCall{
						Name: tc.Name,
						Args: args,
					},
				})
			}
			if len(parts) > 0 {
				result = append(result, geminiContent{
					Role:  "model",
					Parts: parts,
				})
			}

		case RoleTool:
			funcName := idMap[msg.ToolCallID]
			if funcName == "" {
				// Fallback: use the ToolCallID itself as the name.
				funcName = msg.ToolCallID
			}

			part := geminiPart{
				FunctionResponse: &geminiFuncResponse{
					Name: funcName,
					Response: map[string]any{
						"content": msg.Content,
					},
				},
			}

			// Merge consecutive tool results into a single user message,
			// since Gemini expects all function responses together.
			if len(result) > 0 {
				last := &result[len(result)-1]
				if last.Role == "user" && len(last.Parts) > 0 && last.Parts[0].FunctionResponse != nil {
					last.Parts = append(last.Parts, part)
					continue
				}
			}

			result = append(result, geminiContent{
				Role:  "user",
				Parts: []geminiPart{part},
			})

		case RoleSystem:
			// Inline system messages are sent as user messages as a fallback.
			result = append(result, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: msg.Content}},
			})
		}
	}

	return result
}

// ---------------------------------------------------------------------------
// Convert Gemini API response → internal types
// ---------------------------------------------------------------------------

func convertGeminiResponse(resp *geminiResponse) (*Message, *Usage) {
	msg := &Message{
		Role: RoleAssistant,
	}

	usage := &Usage{}
	if resp.UsageMetadata != nil {
		usage.InputTokens = resp.UsageMetadata.PromptTokenCount
		usage.OutputTokens = resp.UsageMetadata.CandidatesTokenCount
	}

	if len(resp.Candidates) == 0 {
		return msg, usage
	}

	candidate := resp.Candidates[0]
	var textParts []string
	callIndex := 0

	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			textParts = append(textParts, part.Text)
		}
		if part.FunctionCall != nil {
			argsJSON, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				argsJSON = []byte("{}")
			}
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID:        fmt.Sprintf("gemini_call_%d", callIndex),
				Name:      part.FunctionCall.Name,
				Arguments: string(argsJSON),
			})
			callIndex++
		}
	}

	msg.Content = strings.Join(textParts, "")

	return msg, usage
}

// ---------------------------------------------------------------------------
// SendMessage (non-streaming)
// ---------------------------------------------------------------------------

func (p *geminiProvider) SendMessage(ctx context.Context, req *Request) (*Message, *Usage, error) {
	gr := p.buildRequest(req)

	model := req.Model
	if model == "" {
		model = geminiDefaultModel
	}

	url := fmt.Sprintf("%s/%s:generateContent", geminiBaseURL, model)

	body, err := json.Marshal(gr)
	if err != nil {
		return nil, nil, fmt.Errorf("gemini: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("gemini: failed to create HTTP request: %w", err)
	}
	p.setHeaders(httpReq)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("gemini: HTTP request failed: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("gemini: failed to read response body: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, nil, parseGeminiError(httpResp.StatusCode, respBody)
	}

	var apiResp geminiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, nil, fmt.Errorf("gemini: failed to unmarshal response: %w", err)
	}

	msg, usage := convertGeminiResponse(&apiResp)
	return msg, usage, nil
}

// ---------------------------------------------------------------------------
// StreamMessage (SSE streaming)
// ---------------------------------------------------------------------------

func (p *geminiProvider) StreamMessage(ctx context.Context, req *Request) (<-chan StreamEvent, error) {
	gr := p.buildRequest(req)

	model := req.Model
	if model == "" {
		model = geminiDefaultModel
	}

	url := fmt.Sprintf("%s/%s:streamGenerateContent?alt=sse", geminiBaseURL, model)

	body, err := json.Marshal(gr)
	if err != nil {
		return nil, fmt.Errorf("gemini: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gemini: failed to create HTTP request: %w", err)
	}
	p.setHeaders(httpReq)
	httpReq.Header.Set("Accept", "text/event-stream")

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: HTTP request failed: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		defer func() { _ = httpResp.Body.Close() }()
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, parseGeminiError(httpResp.StatusCode, respBody)
	}

	ch := make(chan StreamEvent, 64)

	go func() {
		defer close(ch)
		defer func() { _ = httpResp.Body.Close() }()
		p.processSSEStream(ctx, httpResp.Body, ch)
	}()

	return ch, nil
}

func (p *geminiProvider) processSSEStream(ctx context.Context, body io.Reader, ch chan<- StreamEvent) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	var totalUsage Usage
	callIndex := 0

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

		var chunk geminiResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			ch <- StreamEvent{
				Type:  EventError,
				Error: fmt.Errorf("gemini: failed to parse stream chunk: %w", err),
			}
			continue
		}

		// Update usage metadata from each chunk (last one wins with final counts).
		if chunk.UsageMetadata != nil {
			totalUsage.InputTokens = chunk.UsageMetadata.PromptTokenCount
			totalUsage.OutputTokens = chunk.UsageMetadata.CandidatesTokenCount
		}

		if len(chunk.Candidates) == 0 {
			continue
		}

		candidate := chunk.Candidates[0]

		for _, part := range candidate.Content.Parts {
			// Handle text content deltas.
			if part.Text != "" {
				ch <- StreamEvent{
					Type:    EventText,
					Content: part.Text,
				}
			}

			// Handle function calls (Gemini sends them as complete objects).
			if part.FunctionCall != nil {
				argsJSON, err := json.Marshal(part.FunctionCall.Args)
				if err != nil {
					argsJSON = []byte("{}")
				}

				tc := &ToolCall{
					ID:        fmt.Sprintf("gemini_call_%d", callIndex),
					Name:      part.FunctionCall.Name,
					Arguments: string(argsJSON),
				}
				callIndex++

				ch <- StreamEvent{
					Type:     EventToolCallStart,
					ToolCall: tc,
				}
				ch <- StreamEvent{
					Type:    EventToolCallDelta,
					Content: string(argsJSON),
				}
				ch <- StreamEvent{
					Type:     EventToolCallComplete,
					ToolCall: tc,
				}
			}
		}

		// Check for terminal finish reasons.
		if candidate.FinishReason == "STOP" || candidate.FinishReason == "MAX_TOKENS" {
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

		// Handle error finish reasons.
		switch candidate.FinishReason {
		case "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII":
			ch <- StreamEvent{
				Type:  EventError,
				Error: fmt.Errorf("gemini: generation stopped due to %s", candidate.FinishReason),
			}
			ch <- StreamEvent{
				Type: EventDone,
				Done: true,
				Usage: &Usage{
					InputTokens:  totalUsage.InputTokens,
					OutputTokens: totalUsage.OutputTokens,
				},
			}
			return
		case "MALFORMED_FUNCTION_CALL":
			ch <- StreamEvent{
				Type:  EventError,
				Error: fmt.Errorf("gemini: model produced a malformed function call"),
			}
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
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamEvent{Type: EventError, Error: fmt.Errorf("gemini: stream read error: %w", err)}
		return
	}

	// If we reach the end of the stream without an explicit finish reason,
	// emit a done event so the caller does not hang.
	ch <- StreamEvent{
		Type: EventDone,
		Done: true,
		Usage: &Usage{
			InputTokens:  totalUsage.InputTokens,
			OutputTokens: totalUsage.OutputTokens,
		},
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (p *geminiProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	// Gemini uses the API key as a query parameter.
	q := req.URL.Query()
	q.Set("key", p.apiKey)
	req.URL.RawQuery = q.Encode()
}

func parseGeminiError(statusCode int, body []byte) error {
	var errResp geminiErrorResponse
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		return fmt.Errorf("gemini: API error (%d): %s: %s",
			statusCode, errResp.Error.Status, errResp.Error.Message)
	}
	return fmt.Errorf("gemini: API error (%d): %s", statusCode, string(body))
}
