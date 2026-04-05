package security

import "testing"

func TestIsSensitivePath_Sensitive(t *testing.T) {
	sensitivePaths := []string{
		// Environment files
		".env",
		".env.local",
		".env.production",

		// SSH keys
		"id_rsa",
		"id_ed25519",

		// Private keys and certificates
		"server.key",
		"cert.pem",
		"private.p12",

		// SSH directory paths
		".ssh/config",
		"~/.ssh/id_rsa",
		"/home/user/.ssh/known_hosts",

		// Cloud credentials
		".aws/credentials",
		".config/gcloud/credentials.json",

		// Package manager tokens
		".npmrc",
		".pypirc",

		// Git credentials
		".git-credentials",
		".netrc",

		// GPG keys
		"secret.gpg",
	}

	for _, path := range sensitivePaths {
		t.Run(path, func(t *testing.T) {
			if !IsSensitivePath(path) {
				t.Errorf("IsSensitivePath(%q) = false, want true", path)
			}
		})
	}
}

func TestIsSensitivePath_NotSensitive(t *testing.T) {
	safePaths := []string{
		"main.go",
		"README.md",
		"package.json",
		"Makefile",
		"src/app.py",
		"internal/config/config.go",
		".gitignore",
		".eslintrc",
	}

	for _, path := range safePaths {
		t.Run(path, func(t *testing.T) {
			if IsSensitivePath(path) {
				t.Errorf("IsSensitivePath(%q) = true, want false", path)
			}
		})
	}
}

func TestDescribeSensitivePath_NonEmpty(t *testing.T) {
	sensitivePaths := []string{
		".env",
		".env.local",
		"id_rsa",
		"server.key",
		"cert.pem",
		".npmrc",
		".git-credentials",
		".netrc",
		"secret.gpg",
	}

	for _, path := range sensitivePaths {
		t.Run(path, func(t *testing.T) {
			desc := DescribeSensitivePath(path)
			if desc == "" {
				t.Errorf("DescribeSensitivePath(%q) returned empty string, want non-empty description", path)
			}
		})
	}
}

func TestDescribeSensitivePath_Empty(t *testing.T) {
	safePaths := []string{
		"main.go",
		"README.md",
		"package.json",
		"Makefile",
		"src/app.py",
		"internal/config/config.go",
		".gitignore",
		".eslintrc",
	}

	for _, path := range safePaths {
		t.Run(path, func(t *testing.T) {
			desc := DescribeSensitivePath(path)
			if desc != "" {
				t.Errorf("DescribeSensitivePath(%q) = %q, want empty string", path, desc)
			}
		})
	}
}

func TestDescribeSensitivePath_DirectoryBased(t *testing.T) {
	// Paths that are sensitive because of their directory component
	dirPaths := []string{
		".ssh/config",
		".aws/credentials",
		".config/gcloud/credentials.json",
	}

	for _, path := range dirPaths {
		t.Run(path, func(t *testing.T) {
			desc := DescribeSensitivePath(path)
			if desc == "" {
				t.Errorf("DescribeSensitivePath(%q) returned empty string, want description about sensitive directory", path)
			}
		})
	}
}
