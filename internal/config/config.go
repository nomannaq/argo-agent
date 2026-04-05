package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds all configuration for the Argo application.
type Config struct {
	// LLM settings
	Model       string  `json:"model" yaml:"model"`
	Provider    string  `json:"provider" yaml:"provider"`
	MaxTokens   int     `json:"max_tokens" yaml:"max_tokens"`
	Temperature float64 `json:"temperature" yaml:"temperature"`

	// API Keys (loaded from env, not stored in config file)
	APIKey string `json:"-" yaml:"-"`

	// UI settings
	Theme string `json:"theme" yaml:"theme"`

	// Permission settings
	AutoApproveRead bool `json:"auto_approve_read" yaml:"auto_approve_read"`

	// Paths
	DataDir string `json:"-" yaml:"-"` // ~/.argo
}

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Model:           "claude-sonnet-4-20250514",
		Provider:        "anthropic",
		MaxTokens:       16384,
		Temperature:     0,
		Theme:           "",
		AutoApproveRead: true,
	}
}

// Load reads configuration from disk, overlays environment variables, and
// resolves the API key. It returns a fully-initialised Config ready for use.
//
// Precedence (highest → lowest):
//  1. Environment variables (ARGO_MODEL, ARGO_PROVIDER, ARGO_MAX_TOKENS)
//  2. Config file (~/.argo/config.json)
//  3. Built-in defaults
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Resolve the data directory (~/.argo).
	dataDir, err := resolveDataDir()
	if err != nil {
		return nil, fmt.Errorf("config: resolve data dir: %w", err)
	}
	cfg.DataDir = dataDir

	// Ensure the data directory exists.
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("config: create data dir %s: %w", cfg.DataDir, err)
	}

	// Try to load the config file – it is fine if it does not exist yet.
	configPath := filepath.Join(cfg.DataDir, "config.json")
	if err := loadFromFile(configPath, cfg); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("config: load %s: %w", configPath, err)
	}

	// Overlay environment variables (they take priority over the file).
	applyEnv(cfg)

	// Resolve the API key from the environment based on the chosen provider.
	cfg.ResolveAPIKey()

	return cfg, nil
}

// Save persists the given Config to ~/.argo/config.json.
// Sensitive fields (APIKey) and runtime-only fields (DataDir) are excluded
// because they are tagged with `json:"-"`.
func Save(cfg *Config) error {
	dataDir := cfg.DataDir
	if dataDir == "" {
		var err error
		dataDir, err = resolveDataDir()
		if err != nil {
			return fmt.Errorf("config: resolve data dir: %w", err)
		}
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("config: create data dir %s: %w", dataDir, err)
	}

	configPath := filepath.Join(dataDir, "config.json")

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}

	// Append a trailing newline for friendliness.
	data = append(data, '\n')

	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("config: write %s: %w", configPath, err)
	}

	return nil
}

// DefaultModelForProvider returns the default model name for a given provider.
func DefaultModelForProvider(provider string) string {
	switch strings.ToLower(provider) {
	case "anthropic":
		return "claude-sonnet-4-20250514"
	case "openai":
		return "gpt-4o"
	case "gemini":
		return "gemini-2.5-flash"
	default:
		return ""
	}
}

// ApplyFlags applies CLI flag overrides to the Config.
// Only non-empty values are applied so that flags behave as optional overrides.
func (c *Config) ApplyFlags(model, provider string) {
	if model != "" {
		c.Model = model
	}
	if provider != "" {
		c.Provider = provider
		// When the provider changes but no explicit model was given,
		// switch to that provider's default model.
		if model == "" {
			if defaultModel := DefaultModelForProvider(provider); defaultModel != "" {
				c.Model = defaultModel
			}
		}
		// Re-resolve the API key when the provider changes.
		c.ResolveAPIKey()
	}
}

// ResolveAPIKey looks up the API key from environment variables based on the
// current provider. It checks provider-specific variables first, then falls
// back to the generic ARGO_API_KEY.
//
// Resolution order for provider "anthropic":
//  1. ANTHROPIC_API_KEY
//  2. ARGO_API_KEY
//
// Resolution order for provider "openai":
//  1. OPENAI_API_KEY
//  2. ARGO_API_KEY
//
// Resolution order for provider "gemini":
//  1. GEMINI_API_KEY
//  2. ARGO_API_KEY
//
// Note: The Gemini provider additionally supports GOOGLE_API_KEY as a
// fallback, but that is handled by the provider itself, not here.
//
// For any other provider "foo":
//  1. FOO_API_KEY
//  2. ARGO_API_KEY
func (c *Config) ResolveAPIKey() {
	provider := strings.ToLower(c.Provider)

	// Build the provider-specific env var name, e.g. ANTHROPIC_API_KEY.
	providerEnv := strings.ToUpper(provider) + "_API_KEY"

	if key := os.Getenv(providerEnv); key != "" {
		c.APIKey = key
		return
	}

	// Fallback to the generic key.
	if key := os.Getenv("ARGO_API_KEY"); key != "" {
		c.APIKey = key
		return
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// resolveDataDir returns the path to ~/.argo.
func resolveDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine home directory: %w", err)
	}
	return filepath.Join(home, ".argo"), nil
}

// loadFromFile reads a JSON config file and unmarshals it into cfg.
// Fields present in the file overwrite the values already in cfg; fields
// absent from the file leave the existing (default) values untouched.
func loadFromFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}

	return nil
}

// applyEnv overlays environment variables onto the Config.
func applyEnv(cfg *Config) {
	if v := os.Getenv("ARGO_MODEL"); v != "" {
		cfg.Model = v
	}

	if v := os.Getenv("ARGO_PROVIDER"); v != "" {
		cfg.Provider = v
		// When the provider changes via env but no explicit model was set,
		// switch to that provider's default model.
		if os.Getenv("ARGO_MODEL") == "" {
			if defaultModel := DefaultModelForProvider(v); defaultModel != "" {
				cfg.Model = defaultModel
			}
		}
	}

	if v := os.Getenv("ARGO_MAX_TOKENS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxTokens = n
		}
	}
}
