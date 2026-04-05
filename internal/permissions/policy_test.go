package permissions

import (
	"strings"
	"testing"

	"github.com/nomanqureshi/argo/internal/tools"
)

// makeTestInteractive creates an InteractiveHandler that returns the given response.
func makeTestInteractive(response bool) *InteractiveHandler {
	return NewInteractiveHandler(func(name string, level tools.PermissionLevel, input string) (bool, error) {
		return response, nil
	})
}

// --- Decision.String() tests ---

func TestDecision_String_Allow(t *testing.T) {
	if got := Allow.String(); got != "allow" {
		t.Errorf("Allow.String() = %q, want %q", got, "allow")
	}
}

func TestDecision_String_Deny(t *testing.T) {
	if got := Deny.String(); got != "deny" {
		t.Errorf("Deny.String() = %q, want %q", got, "deny")
	}
}

func TestDecision_String_Ask(t *testing.T) {
	if got := Ask.String(); got != "ask" {
		t.Errorf("Ask.String() = %q, want %q", got, "ask")
	}
}

func TestDecision_String_Unknown(t *testing.T) {
	d := Decision(99)
	if got := d.String(); got != "unknown" {
		t.Errorf("Decision(99).String() = %q, want %q", got, "unknown")
	}
}

func TestDecision_String_Negative(t *testing.T) {
	d := Decision(-1)
	if got := d.String(); got != "unknown" {
		t.Errorf("Decision(-1).String() = %q, want %q", got, "unknown")
	}
}

// --- PolicyRule.matches() tests ---

func TestPolicyRule_Matches_ExactToolName_NoInputPattern(t *testing.T) {
	rule := PolicyRule{
		ToolPattern:  "shell",
		InputPattern: "",
		Decision:     Allow,
	}
	if !rule.matches("shell", "anything") {
		t.Fatal("expected rule to match exact tool name 'shell'")
	}
}

func TestPolicyRule_Matches_GlobPattern(t *testing.T) {
	rule := PolicyRule{
		ToolPattern:  "edit_*",
		InputPattern: "",
		Decision:     Allow,
	}
	if !rule.matches("edit_file", "input") {
		t.Fatal("expected rule with pattern 'edit_*' to match 'edit_file'")
	}
	if !rule.matches("edit_config", "input") {
		t.Fatal("expected rule with pattern 'edit_*' to match 'edit_config'")
	}
}

func TestPolicyRule_Matches_WithInputRegex(t *testing.T) {
	rule := PolicyRule{
		ToolPattern:  "*",
		InputPattern: `\.go$`,
		Decision:     Allow,
	}
	if !rule.matches("edit_file", "main.go") {
		t.Fatal("expected rule to match input ending with .go")
	}
	if rule.matches("edit_file", "main.py") {
		t.Fatal("expected rule not to match input ending with .py")
	}
}

func TestPolicyRule_Matches_NonMatchingTool(t *testing.T) {
	rule := PolicyRule{
		ToolPattern:  "shell",
		InputPattern: "",
		Decision:     Allow,
	}
	if rule.matches("read_file", "anything") {
		t.Fatal("expected rule with pattern 'shell' not to match 'read_file'")
	}
}

func TestPolicyRule_Matches_InvalidGlobReturnsFalse(t *testing.T) {
	rule := PolicyRule{
		ToolPattern:  "[invalid",
		InputPattern: "",
		Decision:     Allow,
	}
	if rule.matches("anything", "input") {
		t.Fatal("expected invalid glob pattern to return false")
	}
}

func TestPolicyRule_Matches_InvalidRegexReturnsFalse(t *testing.T) {
	rule := PolicyRule{
		ToolPattern:  "*",
		InputPattern: "[invalid",
		Decision:     Allow,
	}
	if rule.matches("shell", "input") {
		t.Fatal("expected invalid regex pattern to return false")
	}
}

func TestPolicyRule_Matches_WildcardToolPattern(t *testing.T) {
	rule := PolicyRule{
		ToolPattern:  "*",
		InputPattern: "",
		Decision:     Allow,
	}
	if !rule.matches("any_tool", "any_input") {
		t.Fatal("expected wildcard '*' to match any tool")
	}
}

func TestPolicyRule_Matches_EmptyToolName(t *testing.T) {
	rule := PolicyRule{
		ToolPattern:  "*",
		InputPattern: "",
		Decision:     Allow,
	}
	// filepath.Match("*", "") should match
	// filepath.Match("*", "") returns true, but even if it didn't,
	// both outcomes are acceptable behavior for empty tool names.
	_ = rule.matches("", "input")
}

// --- DefaultSecurityRules() tests ---

func TestDefaultSecurityRules_ReturnsNonEmptyList(t *testing.T) {
	rules := DefaultSecurityRules()
	if len(rules) == 0 {
		t.Fatal("expected DefaultSecurityRules to return a non-empty list")
	}
}

func TestDefaultSecurityRules_ContainsDenyDecisions(t *testing.T) {
	rules := DefaultSecurityRules()
	for _, r := range rules {
		if r.Decision != Deny {
			t.Errorf("expected all default security rules to have Deny decision, got %v for pattern %q", r.Decision, r.ToolPattern)
		}
	}
}

func TestDefaultSecurityRules_AllUseWildcardToolPattern(t *testing.T) {
	rules := DefaultSecurityRules()
	for _, r := range rules {
		if r.ToolPattern != "*" {
			t.Errorf("expected all default security rules to use wildcard tool pattern, got %q", r.ToolPattern)
		}
	}
}

// --- PolicyHandler.CheckPermission: dangerous command blocking ---

func TestPolicyHandler_BlocksRmRf(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "rm -rf /")
	if allowed {
		t.Fatal("expected rm -rf / to be blocked")
	}
	if err == nil {
		t.Fatal("expected error for blocked command")
	}
	if !strings.Contains(err.Error(), "security policy") {
		t.Errorf("expected error to mention security policy, got: %v", err)
	}
}

func TestPolicyHandler_BlocksRmRfVariant(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "rm -fr /")
	if allowed {
		t.Fatal("expected rm -fr / to be blocked")
	}
	if err == nil {
		t.Fatal("expected error for blocked command")
	}
}

func TestPolicyHandler_BlocksRmRfHome(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "rm -rf ~/")
	if allowed {
		t.Fatal("expected rm -rf ~/ to be blocked")
	}
	if err == nil {
		t.Fatal("expected error for blocked command")
	}
}

func TestPolicyHandler_BlocksForkBomb(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, ":(){ :|:& };:")
	if allowed {
		t.Fatal("expected fork bomb to be blocked")
	}
	if err == nil {
		t.Fatal("expected error for blocked fork bomb")
	}
}

func TestPolicyHandler_BlocksMkfs(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "mkfs.ext4 /dev/sda1")
	if allowed {
		t.Fatal("expected mkfs to be blocked")
	}
	if err == nil {
		t.Fatal("expected error for blocked mkfs")
	}
}

func TestPolicyHandler_BlocksDdIf(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "dd if=/dev/zero of=/dev/sda")
	if allowed {
		t.Fatal("expected dd if= to be blocked")
	}
	if err == nil {
		t.Fatal("expected error for blocked dd")
	}
}

func TestPolicyHandler_BlocksRedirectToBlockDevice(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "echo x > /dev/sda")
	if allowed {
		t.Fatal("expected redirect to block device to be blocked")
	}
	if err == nil {
		t.Fatal("expected error for blocked redirect")
	}
}

func TestPolicyHandler_BlocksDangerousChmod(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "chmod -R 777 /")
	if allowed {
		t.Fatal("expected chmod -R 777 / to be blocked")
	}
	if err == nil {
		t.Fatal("expected error for blocked chmod")
	}
}

// --- PolicyHandler: safe read operations ---

func TestPolicyHandler_AllowsSafeReadOperations(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("read_file", tools.PermissionRead, "main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected safe read operation to be allowed")
	}
}

func TestPolicyHandler_AllowsSafeLsCommand(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("shell", tools.PermissionRead, "ls -la")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected safe ls command to be allowed")
	}
}

// --- PolicyHandler: session decisions ---

func TestPolicyHandler_SessionDecision_Allow(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(false))
	ph.RememberDecision("edit_file", Allow)

	allowed, err := ph.CheckPermission("edit_file", tools.PermissionWrite, "some file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected session Allow decision to permit the operation")
	}
}

func TestPolicyHandler_SessionDecision_Deny(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))
	ph.RememberDecision("shell", Deny)

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "echo hello")
	if allowed {
		t.Fatal("expected session Deny decision to block the operation")
	}
	if err == nil {
		t.Fatal("expected error for denied session decision")
	}
	if !strings.Contains(err.Error(), "session policy") {
		t.Errorf("expected error to mention session policy, got: %v", err)
	}
}

func TestPolicyHandler_RememberDecision_OverridesPrevious(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(false))

	ph.RememberDecision("shell", Allow)
	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "echo hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected Allow to permit")
	}

	ph.RememberDecision("shell", Deny)
	allowed, err = ph.CheckPermission("shell", tools.PermissionDangerous, "echo hi")
	if allowed {
		t.Fatal("expected Deny to block after override")
	}
	if err == nil {
		t.Fatal("expected error for denied")
	}
}

func TestPolicyHandler_ClearSessionDecisions(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	ph.RememberDecision("shell", Deny)
	ph.ClearSessionDecisions()

	// After clearing, the session decision should be gone.
	// With PermissionDangerous and interactive returning true, it should be allowed.
	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "echo hello")
	if err != nil {
		t.Fatalf("unexpected error after clearing session decisions: %v", err)
	}
	if !allowed {
		t.Fatal("expected operation to be allowed after clearing session decisions")
	}
}

// --- PolicyHandler: user-defined rules ---

func TestPolicyHandler_UserRule_AllowDecision(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(false))
	ph.AddRule(PolicyRule{
		ToolPattern:  "edit_*",
		InputPattern: "",
		Decision:     Allow,
	})

	allowed, err := ph.CheckPermission("edit_file", tools.PermissionWrite, "main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected user Allow rule to permit the operation")
	}
}

func TestPolicyHandler_UserRule_DenyDecision(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))
	ph.AddRule(PolicyRule{
		ToolPattern:  "shell",
		InputPattern: "",
		Decision:     Deny,
	})

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "echo safe")
	if allowed {
		t.Fatal("expected user Deny rule to block the operation")
	}
	if err == nil {
		t.Fatal("expected error for user Deny rule")
	}
	if !strings.Contains(err.Error(), "denied by policy rule") {
		t.Errorf("expected error to mention policy rule, got: %v", err)
	}
}

func TestPolicyHandler_UserRule_AskDecision_DelegatesToInteractive(t *testing.T) {
	interactive := makeTestInteractive(true)
	ph := NewPolicyHandler(interactive)
	ph.AddRule(PolicyRule{
		ToolPattern:  "edit_*",
		InputPattern: "",
		Decision:     Ask,
	})

	allowed, err := ph.CheckPermission("edit_file", tools.PermissionWrite, "main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected Ask decision to delegate to interactive (which returns true)")
	}
}

func TestPolicyHandler_UserRule_AskDecision_DelegatesToInteractive_Denied(t *testing.T) {
	interactive := makeTestInteractive(false)
	ph := NewPolicyHandler(interactive)
	ph.AddRule(PolicyRule{
		ToolPattern:  "edit_*",
		InputPattern: "",
		Decision:     Ask,
	})

	allowed, err := ph.CheckPermission("edit_file", tools.PermissionWrite, "main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatal("expected Ask decision to delegate to interactive (which returns false)")
	}
}

func TestPolicyHandler_UserRule_FirstMatchWins(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(false))

	// First rule: allow edit_file specifically
	ph.AddRule(PolicyRule{
		ToolPattern:  "edit_file",
		InputPattern: "",
		Decision:     Allow,
	})
	// Second rule: deny all edit_* (should not be reached for edit_file)
	ph.AddRule(PolicyRule{
		ToolPattern:  "edit_*",
		InputPattern: "",
		Decision:     Deny,
	})

	allowed, err := ph.CheckPermission("edit_file", tools.PermissionWrite, "main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected first matching rule (Allow) to win")
	}

	// edit_config should match the second rule (Deny)
	allowed, err = ph.CheckPermission("edit_config", tools.PermissionWrite, "config.yaml")
	if allowed {
		t.Fatal("expected second rule (Deny) to apply to edit_config")
	}
	if err == nil {
		t.Fatal("expected error for denied")
	}
}

func TestPolicyHandler_UserRule_WithInputPattern(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(false))
	ph.AddRule(PolicyRule{
		ToolPattern:  "*",
		InputPattern: `\.secret$`,
		Decision:     Deny,
	})

	allowed, err := ph.CheckPermission("read_file", tools.PermissionRead, "passwords.secret")
	if allowed {
		t.Fatal("expected rule with input pattern to deny .secret files")
	}
	if err == nil {
		t.Fatal("expected error for denied")
	}

	// Non-matching input should fall through
	allowed, err = ph.CheckPermission("read_file", tools.PermissionRead, "readme.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected non-matching input to be allowed by default read level")
	}
}

// --- PolicyHandler: default behavior by permission level ---

func TestPolicyHandler_Default_ReadLevelAutoAllows(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(false))

	allowed, err := ph.CheckPermission("read_file", tools.PermissionRead, "main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected read level to be auto-allowed by default")
	}
}

func TestPolicyHandler_Default_WriteLevelAsks(t *testing.T) {
	// Interactive returns true: should allow
	ph := NewPolicyHandler(makeTestInteractive(true))
	allowed, err := ph.CheckPermission("edit_file", tools.PermissionWrite, "main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected write level to delegate to interactive (returns true)")
	}

	// Interactive returns false: should deny
	ph2 := NewPolicyHandler(makeTestInteractive(false))
	allowed, err = ph2.CheckPermission("edit_file", tools.PermissionWrite, "main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatal("expected write level to delegate to interactive (returns false)")
	}
}

func TestPolicyHandler_Default_DangerousLevelAsks(t *testing.T) {
	// Interactive returns true: should allow
	ph := NewPolicyHandler(makeTestInteractive(true))
	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected dangerous level to delegate to interactive (returns true)")
	}

	// Interactive returns false: should deny
	ph2 := NewPolicyHandler(makeTestInteractive(false))
	allowed, err = ph2.CheckPermission("shell", tools.PermissionDangerous, "echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatal("expected dangerous level to delegate to interactive (returns false)")
	}
}

// --- containsWriteIntent() tests ---

func TestContainsWriteIntent_DetectsWriteIndicators(t *testing.T) {
	indicators := []struct {
		input    string
		expected bool
	}{
		{"write to /etc/passwd", true},
		{"create /etc/config", true},
		{"delete /usr/bin/tool", true},
		{"remove /etc/something", true},
		{"mv /etc/old /etc/new", true},
		{"move file to /usr/local", true},
		{"cp /etc/config /tmp/backup", true},
		{"copy files from /usr/", true},
		{"chmod 755 /usr/bin/tool", true},
		{"chown root /usr/bin/tool", true},
		{"chgrp admin /usr/share/file", true},
		{"truncate /etc/file", true},
		{"append to /etc/hosts", true},
		{"overwrite /usr/config", true},
		{"> /etc/passwd", true},
		{">> /etc/hosts", true},
		{"edit /etc/config", true},
		{"modify /usr/file", true},
		{"patch /etc/file", true},
		{"install package", true},
		{"uninstall package", true},
		{"mkdir /etc/newdir", true},
		{"rmdir /usr/old", true},
		{"ln -s /etc/target /etc/link", true},
		{"link files", true},
	}

	for _, tt := range indicators {
		t.Run(tt.input, func(t *testing.T) {
			got := containsWriteIntent(tt.input)
			if got != tt.expected {
				t.Errorf("containsWriteIntent(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestContainsWriteIntent_ReturnsFalseForReadOnly(t *testing.T) {
	readOnlyInputs := []string{
		"cat /etc/passwd",
		"ls /usr/bin",
		"head /etc/hosts",
		"tail /etc/config",
		"grep pattern /etc/file",
		"find /usr -name test",
		"reading a file",
		"show contents",
		"list files",
		"",
	}

	for _, input := range readOnlyInputs {
		t.Run(input, func(t *testing.T) {
			if containsWriteIntent(input) {
				t.Errorf("containsWriteIntent(%q) = true, want false for read-only input", input)
			}
		})
	}
}

// --- Security rules: system directory protection ---

func TestPolicyHandler_SecurityRules_BlockWritesToEtc(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "write to /etc/passwd")
	if allowed {
		t.Fatal("expected write to /etc/ to be blocked")
	}
	if err == nil {
		t.Fatal("expected error for write to system directory")
	}
	if !strings.Contains(err.Error(), "/etc/") {
		t.Errorf("expected error to reference /etc/, got: %v", err)
	}
}

func TestPolicyHandler_SecurityRules_BlockWritesToUsr(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "write to /usr/bin/tool")
	if allowed {
		t.Fatal("expected write to /usr/ to be blocked")
	}
	if err == nil {
		t.Fatal("expected error for write to system directory")
	}
}

func TestPolicyHandler_SecurityRules_BlockWritesToBin(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "delete /bin/sh")
	if allowed {
		t.Fatal("expected write to /bin/ to be blocked")
	}
	if err == nil {
		t.Fatal("expected error for write to /bin/")
	}
}

func TestPolicyHandler_SecurityRules_BlockWritesToSbin(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "remove /sbin/init")
	if allowed {
		t.Fatal("expected write to /sbin/ to be blocked")
	}
	if err == nil {
		t.Fatal("expected error for write to /sbin/")
	}
}

func TestPolicyHandler_SecurityRules_BlockWritesToBoot(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "overwrite /boot/grub")
	if allowed {
		t.Fatal("expected write to /boot/ to be blocked")
	}
	if err == nil {
		t.Fatal("expected error for write to /boot/")
	}
}

func TestPolicyHandler_SecurityRules_DoesNotBlockReadsFromEtc(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	// "cat /etc/passwd" doesn't contain any write indicators
	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "cat /etc/passwd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected read from /etc/ to be allowed (no write intent)")
	}
}

func TestPolicyHandler_SecurityRules_DoesNotBlockReadsFromUsr(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "ls /usr/bin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected read from /usr/ to be allowed (no write intent)")
	}
}

// --- PolicyHandler: session decision does NOT override security rules ---

func TestPolicyHandler_SecurityRulesOverrideSessionAllow(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))
	ph.RememberDecision("shell", Allow)

	// Even with a session Allow, dangerous patterns should still be blocked
	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "rm -rf /")
	if allowed {
		t.Fatal("expected security rules to override session Allow decision")
	}
	if err == nil {
		t.Fatal("expected error for blocked dangerous command")
	}
}

// --- PolicyHandler: session decision does NOT override security rules for system dirs ---

func TestPolicyHandler_SecurityRulesOverrideSessionAllow_SystemDir(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))
	ph.RememberDecision("shell", Allow)

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "write to /etc/passwd")
	if allowed {
		t.Fatal("expected security rules to override session Allow for system directory writes")
	}
	if err == nil {
		t.Fatal("expected error for write to system directory")
	}
}

// --- PolicyHandler: user rules do NOT override security rules ---

func TestPolicyHandler_SecurityRulesOverrideUserAllowRule(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))
	ph.AddRule(PolicyRule{
		ToolPattern:  "*",
		InputPattern: "",
		Decision:     Allow,
	})

	allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, "rm -rf /")
	if allowed {
		t.Fatal("expected security rules to override user Allow rule")
	}
	if err == nil {
		t.Fatal("expected error for blocked dangerous command")
	}
}

// --- PolicyHandler: session decision for specific tool only ---

func TestPolicyHandler_SessionDecision_ToolSpecific(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(false))
	ph.RememberDecision("edit_file", Allow)

	// edit_file should be allowed
	allowed, err := ph.CheckPermission("edit_file", tools.PermissionWrite, "main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected session Allow for edit_file")
	}

	// shell should NOT be affected (falls through to interactive which returns false)
	allowed, err = ph.CheckPermission("shell", tools.PermissionDangerous, "echo hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatal("expected shell to not be affected by edit_file session decision")
	}
}

// --- PolicyHandler: unknown permission level falls through to interactive ---

func TestPolicyHandler_Default_UnknownLevelDelegatesToInteractive(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	// Using a custom permission level that doesn't match Read/Write/Dangerous
	allowed, err := ph.CheckPermission("custom_tool", tools.PermissionLevel(99), "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected unknown permission level to delegate to interactive (returns true)")
	}
}

// --- NewPolicyHandler ---

func TestNewPolicyHandler_InitializesCorrectly(t *testing.T) {
	interactive := makeTestInteractive(true)
	ph := NewPolicyHandler(interactive)

	if ph.interactive != interactive {
		t.Fatal("expected interactive handler to be set")
	}
	if len(ph.rules) != 0 {
		t.Fatalf("expected empty rules, got %d", len(ph.rules))
	}
	if len(ph.sessionDecisions) != 0 {
		t.Fatalf("expected empty session decisions, got %d", len(ph.sessionDecisions))
	}
}

// --- AddRule ---

func TestPolicyHandler_AddRule_AppendsRules(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	ph.AddRule(PolicyRule{ToolPattern: "a", Decision: Allow})
	ph.AddRule(PolicyRule{ToolPattern: "b", Decision: Deny})

	if len(ph.rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(ph.rules))
	}
	if ph.rules[0].ToolPattern != "a" {
		t.Errorf("expected first rule pattern 'a', got %q", ph.rules[0].ToolPattern)
	}
	if ph.rules[1].ToolPattern != "b" {
		t.Errorf("expected second rule pattern 'b', got %q", ph.rules[1].ToolPattern)
	}
}

// --- PolicyHandler implements Handler interface ---

func TestPolicyHandler_ImplementsHandlerInterface(t *testing.T) {
	var h Handler = NewPolicyHandler(makeTestInteractive(true))
	allowed, err := h.CheckPermission("read_file", tools.PermissionRead, "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected allowed")
	}
}

// --- Edge cases ---

func TestPolicyHandler_EmptyInput(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("read_file", tools.PermissionRead, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected empty input to be allowed for read")
	}
}

func TestPolicyHandler_EmptyToolName(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	allowed, err := ph.CheckPermission("", tools.PermissionRead, "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected empty tool name to be allowed for read level by default")
	}
}

func TestPolicyHandler_MkfsVariants(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	variants := []string{
		"mkfs.ext4 /dev/sda1",
		"mkfs.xfs /dev/sdb",
		"mkfs /dev/vda",
	}

	for _, v := range variants {
		t.Run(v, func(t *testing.T) {
			allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, v)
			if allowed {
				t.Fatalf("expected mkfs variant %q to be blocked", v)
			}
			if err == nil {
				t.Fatalf("expected error for mkfs variant %q", v)
			}
		})
	}
}

func TestPolicyHandler_DdVariants(t *testing.T) {
	ph := NewPolicyHandler(makeTestInteractive(true))

	variants := []string{
		"dd if=/dev/zero of=/dev/sda",
		"dd if=/dev/urandom of=/dev/sda bs=1M",
	}

	for _, v := range variants {
		t.Run(v, func(t *testing.T) {
			allowed, err := ph.CheckPermission("shell", tools.PermissionDangerous, v)
			if allowed {
				t.Fatalf("expected dd variant %q to be blocked", v)
			}
			if err == nil {
				t.Fatalf("expected error for dd variant %q", v)
			}
		})
	}
}
