package llm

import (
	"context"
	"fmt"
	"sync"
)

// Provider defines the interface that all LLM providers must implement.
type Provider interface {
	// Name returns the provider name (e.g., "anthropic", "openai").
	Name() string

	// SendMessage sends a request and returns a complete response.
	SendMessage(ctx context.Context, req *Request) (*Message, *Usage, error)

	// StreamMessage sends a request and returns a channel of stream events.
	StreamMessage(ctx context.Context, req *Request) (<-chan StreamEvent, error)
}

// ProviderFactory is a constructor function for creating a Provider given an API key.
type ProviderFactory func(apiKey string) (Provider, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]ProviderFactory{}
)

// Register adds a provider factory to the global registry.
// It is safe for concurrent use and is typically called from init() functions.
func Register(name string, factory ProviderFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = factory
}

// NewProvider creates a new Provider instance by looking up the named factory
// in the global registry and calling it with the given API key.
func NewProvider(name string, apiKey string) (Provider, error) {
	registryMu.RLock()
	factory, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	return factory(apiKey)
}

// RegisteredProviders returns a list of all registered provider names.
func RegisteredProviders() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
