package llm

import (
	"context"
	"sort"
	"testing"
)

// mockProvider implements the Provider interface for testing.
type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) SendMessage(ctx context.Context, req *Request) (*Message, *Usage, error) {
	return nil, nil, nil
}

func (m *mockProvider) StreamMessage(ctx context.Context, req *Request) (<-chan StreamEvent, error) {
	return nil, nil
}

// mockFactory returns a ProviderFactory that creates a mockProvider with the given name.
func mockFactory(name string) ProviderFactory {
	return func(apiKey string) (Provider, error) {
		return &mockProvider{name: name}, nil
	}
}

// builtinProviders returns the set of providers registered by init() functions
// (anthropic.go and openai.go), so tests can account for them.
func builtinProviders() map[string]bool {
	return map[string]bool{
		"anthropic": true,
		"openai":    true,
	}
}

func TestRegisterAndNewProvider(t *testing.T) {
	const name = "test_provider_register"
	Register(name, mockFactory(name))

	provider, err := NewProvider(name, "fake-key")
	if err != nil {
		t.Fatalf("NewProvider(%q) returned unexpected error: %v", name, err)
	}
	if provider == nil {
		t.Fatal("NewProvider returned nil provider")
	}
	if got := provider.Name(); got != name {
		t.Errorf("provider.Name() = %q, want %q", got, name)
	}
}

func TestNewProviderUnknown(t *testing.T) {
	_, err := NewProvider("nonexistent_provider_xyz", "key")
	if err == nil {
		t.Fatal("NewProvider for unknown provider should return an error")
	}
}

func TestRegisteredProvidersContainsRegistered(t *testing.T) {
	const name = "test_provider_list"
	Register(name, mockFactory(name))

	providers := RegisteredProviders()
	found := false
	for _, p := range providers {
		if p == name {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("RegisteredProviders() = %v, expected to contain %q", providers, name)
	}
}

func TestRegisteredProvidersContainsBuiltins(t *testing.T) {
	providers := RegisteredProviders()
	builtins := builtinProviders()

	providerSet := make(map[string]bool)
	for _, p := range providers {
		providerSet[p] = true
	}

	for name := range builtins {
		if !providerSet[name] {
			t.Errorf("RegisteredProviders() missing built-in provider %q; got %v", name, providers)
		}
	}
}

func TestRegisterOverwritesExistingFactory(t *testing.T) {
	const name = "test_provider_overwrite"

	// Register with first factory.
	Register(name, func(apiKey string) (Provider, error) {
		return &mockProvider{name: "original"}, nil
	})

	// Overwrite with second factory.
	Register(name, func(apiKey string) (Provider, error) {
		return &mockProvider{name: "overwritten"}, nil
	})

	provider, err := NewProvider(name, "key")
	if err != nil {
		t.Fatalf("NewProvider(%q) returned unexpected error: %v", name, err)
	}
	if got := provider.Name(); got != "overwritten" {
		t.Errorf("provider.Name() = %q, want %q (factory was not overwritten)", got, "overwritten")
	}
}

func TestMultipleProvidersCanBeRegistered(t *testing.T) {
	names := []string{
		"test_provider_multi_a",
		"test_provider_multi_b",
		"test_provider_multi_c",
	}

	for _, name := range names {
		Register(name, mockFactory(name))
	}

	for _, name := range names {
		provider, err := NewProvider(name, "key")
		if err != nil {
			t.Errorf("NewProvider(%q) returned unexpected error: %v", name, err)
			continue
		}
		if got := provider.Name(); got != name {
			t.Errorf("provider.Name() = %q, want %q", got, name)
		}
	}

	// Verify all are listed.
	registered := RegisteredProviders()
	sort.Strings(registered)
	for _, name := range names {
		idx := sort.SearchStrings(registered, name)
		if idx >= len(registered) || registered[idx] != name {
			t.Errorf("RegisteredProviders() missing %q; got %v", name, registered)
		}
	}
}

func TestNewProviderPassesAPIKey(t *testing.T) {
	const name = "test_provider_apikey"
	var receivedKey string

	Register(name, func(apiKey string) (Provider, error) {
		receivedKey = apiKey
		return &mockProvider{name: name}, nil
	})

	_, err := NewProvider(name, "my-secret-key-123")
	if err != nil {
		t.Fatalf("NewProvider returned unexpected error: %v", err)
	}
	if receivedKey != "my-secret-key-123" {
		t.Errorf("factory received apiKey = %q, want %q", receivedKey, "my-secret-key-123")
	}
}

func TestNewProviderPropagatesFactoryError(t *testing.T) {
	const name = "test_provider_factory_error"

	Register(name, func(apiKey string) (Provider, error) {
		return nil, context.DeadlineExceeded
	})

	_, err := NewProvider(name, "key")
	if err == nil {
		t.Fatal("NewProvider should propagate factory error, got nil")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("NewProvider error = %v, want %v", err, context.DeadlineExceeded)
	}
}
