package security

import (
	"testing"
)

func TestIsSensitiveEnvVar_Sensitive(t *testing.T) {
	sensitiveVars := []string{
		"ANTHROPIC_API_KEY",
		"OPENAI_API_KEY",
		"AWS_SECRET_ACCESS_KEY",
		"DATABASE_URL",
		"MY_TOKEN",
		"GITHUB_TOKEN",
		"SSH_PRIVATE_KEY",
		"STRIPE_SECRET_KEY",
	}

	for _, name := range sensitiveVars {
		t.Run(name, func(t *testing.T) {
			if !IsSensitiveEnvVar(name) {
				t.Errorf("IsSensitiveEnvVar(%q) = false, want true", name)
			}
		})
	}
}

func TestIsSensitiveEnvVar_NotSensitive(t *testing.T) {
	safeVars := []string{
		"PATH",
		"HOME",
		"USER",
		"SHELL",
		"GOPATH",
		"EDITOR",
		"MY_VARIABLE",
		"NODE_ENV",
		"DEBUG",
	}

	for _, name := range safeVars {
		t.Run(name, func(t *testing.T) {
			if IsSensitiveEnvVar(name) {
				t.Errorf("IsSensitiveEnvVar(%q) = true, want false", name)
			}
		})
	}
}

func TestSanitizeEnvList_RemovesSensitive(t *testing.T) {
	input := []string{
		"PATH=/usr/bin",
		"OPENAI_API_KEY=sk-xxx",
		"HOME=/home/user",
		"AWS_SECRET_ACCESS_KEY=secret",
	}

	result := SanitizeEnvList(input)

	// Should keep PATH and HOME
	expected := map[string]bool{
		"PATH=/usr/bin":    true,
		"HOME=/home/user":  true,
	}

	if len(result) != len(expected) {
		t.Fatalf("SanitizeEnvList returned %d entries, want %d\nGot: %v", len(result), len(expected), result)
	}

	for _, entry := range result {
		if !expected[entry] {
			t.Errorf("SanitizeEnvList kept unexpected entry: %q", entry)
		}
	}

	// Verify sensitive vars were removed
	for _, entry := range result {
		if entry == "OPENAI_API_KEY=sk-xxx" || entry == "AWS_SECRET_ACCESS_KEY=secret" {
			t.Errorf("SanitizeEnvList should have removed sensitive entry: %q", entry)
		}
	}
}

func TestSanitizeEnvList_PreservesOrder(t *testing.T) {
	input := []string{
		"ALPHA=1",
		"OPENAI_API_KEY=sk-xxx",
		"BETA=2",
		"GAMMA=3",
		"AWS_SECRET_ACCESS_KEY=secret",
		"DELTA=4",
	}

	result := SanitizeEnvList(input)

	expectedOrder := []string{
		"ALPHA=1",
		"BETA=2",
		"GAMMA=3",
		"DELTA=4",
	}

	if len(result) != len(expectedOrder) {
		t.Fatalf("SanitizeEnvList returned %d entries, want %d\nGot: %v", len(result), len(expectedOrder), result)
	}

	for i, entry := range result {
		if entry != expectedOrder[i] {
			t.Errorf("SanitizeEnvList[%d] = %q, want %q", i, entry, expectedOrder[i])
		}
	}
}

func TestSanitizeEnvList_EmptyList(t *testing.T) {
	result := SanitizeEnvList([]string{})

	if len(result) != 0 {
		t.Errorf("SanitizeEnvList(empty) returned %d entries, want 0", len(result))
	}
}

func TestSanitizeEnvList_SkipsEntriesWithoutEquals(t *testing.T) {
	input := []string{
		"PATH=/usr/bin",
		"MALFORMED_ENTRY",
		"HOME=/home/user",
		"ANOTHER_BAD",
	}

	result := SanitizeEnvList(input)

	expectedOrder := []string{
		"PATH=/usr/bin",
		"HOME=/home/user",
	}

	if len(result) != len(expectedOrder) {
		t.Fatalf("SanitizeEnvList returned %d entries, want %d\nGot: %v", len(result), len(expectedOrder), result)
	}

	for i, entry := range result {
		if entry != expectedOrder[i] {
			t.Errorf("SanitizeEnvList[%d] = %q, want %q", i, entry, expectedOrder[i])
		}
	}
}
