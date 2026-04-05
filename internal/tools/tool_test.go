package tools

import "testing"

func TestPermissionLevelString(t *testing.T) {
	tests := []struct {
		name     string
		level    PermissionLevel
		expected string
	}{
		{
			name:     "PermissionRead",
			level:    PermissionRead,
			expected: "read",
		},
		{
			name:     "PermissionWrite",
			level:    PermissionWrite,
			expected: "write",
		},
		{
			name:     "PermissionDangerous",
			level:    PermissionDangerous,
			expected: "dangerous",
		},
		{
			name:     "unknown permission level",
			level:    PermissionLevel(99),
			expected: "unknown",
		},
		{
			name:     "negative permission level",
			level:    PermissionLevel(-1),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.level.String()
			if got != tt.expected {
				t.Errorf("PermissionLevel(%d).String() = %q, want %q", tt.level, got, tt.expected)
			}
		})
	}
}

func TestPermissionLevelConstants(t *testing.T) {
	// Verify the iota ordering is correct
	if PermissionRead != 0 {
		t.Errorf("PermissionRead = %d, want 0", PermissionRead)
	}
	if PermissionWrite != 1 {
		t.Errorf("PermissionWrite = %d, want 1", PermissionWrite)
	}
	if PermissionDangerous != 2 {
		t.Errorf("PermissionDangerous = %d, want 2", PermissionDangerous)
	}
}

func TestResultConstruction(t *testing.T) {
	t.Run("success result", func(t *testing.T) {
		r := &Result{
			Output:  "some output",
			IsError: false,
		}
		if r.Output != "some output" {
			t.Errorf("Output = %q, want %q", r.Output, "some output")
		}
		if r.Error != "" {
			t.Errorf("Error = %q, want empty string", r.Error)
		}
		if r.IsError {
			t.Error("IsError = true, want false")
		}
	})

	t.Run("error result", func(t *testing.T) {
		r := &Result{
			Output:  "",
			Error:   "something went wrong",
			IsError: true,
		}
		if r.Output != "" {
			t.Errorf("Output = %q, want empty string", r.Output)
		}
		if r.Error != "something went wrong" {
			t.Errorf("Error = %q, want %q", r.Error, "something went wrong")
		}
		if !r.IsError {
			t.Error("IsError = false, want true")
		}
	})

	t.Run("result with output and error", func(t *testing.T) {
		r := &Result{
			Output:  "partial output",
			Error:   "timed out",
			IsError: true,
		}
		if r.Output != "partial output" {
			t.Errorf("Output = %q, want %q", r.Output, "partial output")
		}
		if r.Error != "timed out" {
			t.Errorf("Error = %q, want %q", r.Error, "timed out")
		}
		if !r.IsError {
			t.Error("IsError = false, want true")
		}
	})

	t.Run("zero value result", func(t *testing.T) {
		r := &Result{}
		if r.Output != "" {
			t.Errorf("Output = %q, want empty string", r.Output)
		}
		if r.Error != "" {
			t.Errorf("Error = %q, want empty string", r.Error)
		}
		if r.IsError {
			t.Error("IsError = true, want false")
		}
	})
}
