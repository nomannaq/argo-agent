package llm

// Role represents the role of a message sender in a conversation.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Message represents a single message in a conversation.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool invocation requested by the model.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // raw JSON string
}

// ToolDefinition describes a tool that the model can invoke.
type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema"` // JSON schema object
}

// StreamEventType identifies the kind of event in a streaming response.
type StreamEventType int

const (
	EventText             StreamEventType = iota // A text content delta
	EventToolCallStart                           // A new tool call has started
	EventToolCallDelta                           // Incremental update to a tool call
	EventToolCallComplete                        // A tool call is fully received
	EventDone                                    // The stream has finished
	EventError                                   // An error occurred during streaming
)

// StreamEvent represents a single event from a streaming LLM response.
type StreamEvent struct {
	Type     StreamEventType
	Content  string    // text delta
	ToolCall *ToolCall // partial or complete tool call
	Done     bool      // true when stream is finished
	Error    error
	Usage    *Usage
}

// Usage tracks token consumption for a request/response pair.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Request contains all parameters needed to send a message to an LLM provider.
type Request struct {
	Model        string
	SystemPrompt string
	Messages     []Message
	Tools        []ToolDefinition
	MaxTokens    int
	Temperature  float64
}
