package security

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// SensitivePathPatterns defines file path patterns that should be protected
// from agent read/write access by default.
var sensitivePathPatterns = []string{
	// Environment files
	".env",
	".env.*",
	".env.local",
	".env.production",
	".env.development",

	// SSH keys and config
	"id_rsa",
	"id_rsa.pub",
	"id_ed25519",
	"id_ed25519.pub",
	"id_ecdsa",
	"id_dsa",
	"known_hosts",
	"authorized_keys",

	// Private keys and certificates
	"*.pem",
	"*.key",
	"*.p12",
	"*.pfx",
	"*.jks",
	"*.keystore",

	// Cloud credentials
	"credentials",
	"credentials.json",
	"service-account*.json",

	// Package manager tokens
	".npmrc",
	".pypirc",
	".gem/credentials",

	// Docker secrets
	".docker/config.json",

	// Git credentials
	".git-credentials",
	".netrc",

	// GPG keys
	"*.gpg",
	"secring.*",
	"trustdb.gpg",
}

// sensitiveDirectoryPrefixes are directory paths that should never be accessed.
var sensitiveDirectoryPrefixes = []string{
	".ssh",
	".gnupg",
	".aws",
	".azure",
	".config/gcloud",
	".kube",
	".docker",
}

// sensitivePathRegexes are compiled patterns for more complex matching.
var sensitivePathRegexes []*regexp.Regexp

func init() {
	sensitivePathRegexes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)secret`),
		regexp.MustCompile(`(?i)private[_-]?key`),
		regexp.MustCompile(`(?i)\.env(\.[a-z]+)?$`),
	}
}

// IsSensitivePath checks if the given file path matches any sensitive pattern.
// It checks the basename against known sensitive file patterns and the full
// path against sensitive directory prefixes.
func IsSensitivePath(path string) bool {
	// Normalize the path
	path = filepath.Clean(path)
	base := filepath.Base(path)

	// Check basename against sensitive patterns
	for _, pattern := range sensitivePathPatterns {
		matched, err := filepath.Match(pattern, base)
		if err == nil && matched {
			return true
		}
	}

	// Check if path contains a sensitive directory
	normalizedPath := filepath.ToSlash(path)
	for _, prefix := range sensitiveDirectoryPrefixes {
		// Check if the path contains the sensitive directory as a component
		if strings.Contains(normalizedPath, "/"+prefix+"/") ||
			strings.HasPrefix(normalizedPath, prefix+"/") ||
			strings.HasSuffix(normalizedPath, "/"+prefix) ||
			normalizedPath == prefix {
			return true
		}
	}

	// Check if path is within home directory sensitive areas
	// Handle paths like ~/.ssh/id_rsa, /home/user/.aws/credentials
	for _, prefix := range sensitiveDirectoryPrefixes {
		if containsDirComponent(normalizedPath, prefix) {
			return true
		}
	}

	// Check against regex patterns
	for _, re := range sensitivePathRegexes {
		if re.MatchString(base) {
			return true
		}
	}

	return false
}

// containsDirComponent checks if the path contains the given directory component.
func containsDirComponent(path, dir string) bool {
	parts := strings.Split(path, "/")
	dirParts := strings.Split(dir, "/")
	if len(dirParts) == 0 {
		return false
	}

	for i := 0; i <= len(parts)-len(dirParts); i++ {
		match := true
		for j, dp := range dirParts {
			if parts[i+j] != dp {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// DescribeSensitivePath returns a human-readable reason why a path is considered sensitive,
// or an empty string if it's not sensitive.
func DescribeSensitivePath(path string) string {
	path = filepath.Clean(path)
	base := filepath.Base(path)
	normalizedPath := filepath.ToSlash(path)

	for _, pattern := range sensitivePathPatterns {
		matched, _ := filepath.Match(pattern, base)
		if matched {
			return fmt.Sprintf("file %q matches sensitive pattern %q", base, pattern)
		}
	}

	for _, prefix := range sensitiveDirectoryPrefixes {
		if containsDirComponent(normalizedPath, prefix) {
			return fmt.Sprintf("path is within sensitive directory %q", prefix)
		}
	}

	for _, re := range sensitivePathRegexes {
		if re.MatchString(base) {
			return fmt.Sprintf("filename matches sensitive pattern %q", re.String())
		}
	}

	return ""
}
