package security

import (
	"os"
	"regexp"
	"strings"
)

// sensitiveEnvPatterns matches environment variable names that commonly contain secrets.
// These are stripped from subprocess environments by default.
var sensitiveEnvPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)_KEY$`),
	regexp.MustCompile(`(?i)_SECRET$`),
	regexp.MustCompile(`(?i)_TOKEN$`),
	regexp.MustCompile(`(?i)_PASSWORD$`),
	regexp.MustCompile(`(?i)_PASS$`),
	regexp.MustCompile(`(?i)_CREDENTIAL`),
	regexp.MustCompile(`(?i)^DATABASE_URL$`),
	regexp.MustCompile(`(?i)_DSN$`),
	regexp.MustCompile(`(?i)^AWS_`),
	regexp.MustCompile(`(?i)^AZURE_`),
	regexp.MustCompile(`(?i)^GCP_`),
	regexp.MustCompile(`(?i)^GOOGLE_APPLICATION_CREDENTIALS$`),
	regexp.MustCompile(`(?i)^OPENAI_API_KEY$`),
	regexp.MustCompile(`(?i)^ANTHROPIC_API_KEY$`),
	regexp.MustCompile(`(?i)^GITHUB_TOKEN$`),
	regexp.MustCompile(`(?i)^GH_TOKEN$`),
	regexp.MustCompile(`(?i)^GITLAB_TOKEN$`),
	regexp.MustCompile(`(?i)^NPM_TOKEN$`),
	regexp.MustCompile(`(?i)^NUGET_API_KEY$`),
	regexp.MustCompile(`(?i)^DOCKER_PASSWORD$`),
	regexp.MustCompile(`(?i)^HEROKU_API_KEY$`),
	regexp.MustCompile(`(?i)^SLACK_TOKEN$`),
	regexp.MustCompile(`(?i)^SLACK_WEBHOOK`),
	regexp.MustCompile(`(?i)^SENDGRID_API_KEY$`),
	regexp.MustCompile(`(?i)^TWILIO_`),
	regexp.MustCompile(`(?i)^STRIPE_`),
	regexp.MustCompile(`(?i)^PRIVATE_KEY`),
	regexp.MustCompile(`(?i)^SSH_`),
}

// alwaysAllowEnv is a whitelist of env vars that should never be scrubbed
// even if they match a sensitive pattern.
var alwaysAllowEnv = map[string]bool{
	"PATH":             true,
	"HOME":             true,
	"USER":             true,
	"SHELL":            true,
	"TERM":             true,
	"LANG":             true,
	"LC_ALL":           true,
	"EDITOR":           true,
	"VISUAL":           true,
	"TMPDIR":           true,
	"TMP":              true,
	"TEMP":             true,
	"PWD":              true,
	"OLDPWD":           true,
	"LOGNAME":          true,
	"HOSTNAME":         true,
	"DISPLAY":          true,
	"XDG_RUNTIME_DIR":  true,
	"XDG_CONFIG_HOME":  true,
	"XDG_DATA_HOME":    true,
	"XDG_CACHE_HOME":   true,
	"GOPATH":           true,
	"GOROOT":           true,
	"GOBIN":            true,
	"CARGO_HOME":       true,
	"RUSTUP_HOME":      true,
	"NVM_DIR":          true,
	"VIRTUAL_ENV":      true,
	"CONDA_PREFIX":     true,
	"PYENV_ROOT":       true,
}

// IsSensitiveEnvVar returns true if the given environment variable name
// matches a pattern that commonly contains secrets.
func IsSensitiveEnvVar(name string) bool {
	if alwaysAllowEnv[name] {
		return false
	}
	for _, pattern := range sensitiveEnvPatterns {
		if pattern.MatchString(name) {
			return true
		}
	}
	return false
}

// SanitizeEnvironment returns a copy of the current process environment
// with sensitive variables removed. Each entry is in "KEY=VALUE" format.
func SanitizeEnvironment() []string {
	return SanitizeEnvList(os.Environ())
}

// SanitizeEnvList filters a list of "KEY=VALUE" strings, removing entries
// whose keys match sensitive patterns.
func SanitizeEnvList(env []string) []string {
	result := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if !IsSensitiveEnvVar(key) {
			result = append(result, entry)
		}
	}
	return result
}

// ScrubCount returns how many environment variables from the current process
// would be scrubbed. Useful for informational messages.
func ScrubCount() int {
	count := 0
	for _, entry := range os.Environ() {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if IsSensitiveEnvVar(key) {
			count++
		}
	}
	return count
}

// ScrubNames returns the names of environment variables that would be scrubbed.
func ScrubNames() []string {
	var names []string
	for _, entry := range os.Environ() {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if IsSensitiveEnvVar(key) {
			names = append(names, key)
		}
	}
	return names
}
