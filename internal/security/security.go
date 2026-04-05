// Package security provides proactive security controls for the Argo AI agent.
//
// Unlike Zed's reactive approach (which requires users to configure rules after the fact),
// Argo ships with sane defaults that protect secrets, environment variables, and sensitive
// paths out of the box.
//
// The security model has four layers:
//
//  1. Environment scrubbing — sensitive env vars are stripped from subprocess environments
//  2. Path guards — sensitive files (.env, .ssh, *.pem, etc.) are blocked by default
//  3. Command sandboxing — shell substitutions and exfiltration commands are flagged
//  4. Audit logging — every tool call is logged for accountability
package security
