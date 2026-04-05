package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/nomanqureshi/argo/internal/llm"
)

// CompactMessages summarizes older messages in the thread, keeping
// the last keepRecent messages intact. This helps manage the context
// window by replacing older messages with a single summary message.
//
// Strategy:
//   - Keep the last `keepRecent` messages untouched
//   - Replace all older messages with a single system-style summary
//   - The summary preserves key context: what files were discussed,
//     what changes were made, what tools were used, key decisions
func (a *Agent) CompactMessages(ctx context.Context, keepRecent int) error {
	msgs := a.thread.Messages()

	if len(msgs) <= keepRecent {
		return nil // nothing to compact
	}

	oldMessages := msgs[:len(msgs)-keepRecent]
	recentMessages := msgs[len(msgs)-keepRecent:]

	summary := buildCompactionSummary(oldMessages)

	summaryMsg := llm.Message{
		Role:    llm.RoleUser,
		Content: summary,
	}

	newMessages := make([]llm.Message, 0, 1+len(recentMessages))
	newMessages = append(newMessages, summaryMsg)
	newMessages = append(newMessages, recentMessages...)

	a.thread.SetMessages(newMessages)

	return nil
}

// buildCompactionSummary creates a structured summary of the given messages.
func buildCompactionSummary(messages []llm.Message) string {
	var b strings.Builder

	b.WriteString("[CONVERSATION SUMMARY - Earlier messages have been compacted]\n\n")

	// Collect statistics
	var userQuestions []string
	var assistantSummaries []string
	toolUsage := make(map[string]int)
	filesReferenced := make(map[string]bool)

	filePathRegex := regexp.MustCompile(`(?:^|[\s"'/])([a-zA-Z0-9_\-./]+\.[a-zA-Z0-9]+)`)

	for _, msg := range messages {
		switch msg.Role {
		case llm.RoleUser:
			q := msg.Content
			if len(q) > 100 {
				q = q[:100] + "..."
			}
			if q != "" {
				userQuestions = append(userQuestions, q)
			}

		case llm.RoleAssistant:
			if msg.Content != "" {
				s := msg.Content
				if len(s) > 150 {
					s = s[:150] + "..."
				}
				assistantSummaries = append(assistantSummaries, s)
			}
			for _, tc := range msg.ToolCalls {
				toolUsage[tc.Name]++
				// Extract file paths from tool arguments
				matches := filePathRegex.FindAllStringSubmatch(tc.Arguments, -1)
				for _, m := range matches {
					if len(m) > 1 {
						filesReferenced[m[1]] = true
					}
				}
			}

		case llm.RoleTool:
			// Extract file paths from tool results
			matches := filePathRegex.FindAllStringSubmatch(msg.Content, -1)
			for _, m := range matches {
				if len(m) > 1 {
					filesReferenced[m[1]] = true
				}
			}
		}
	}

	// Format the summary
	if len(userQuestions) > 0 {
		b.WriteString("### User Questions/Requests\n")
		for i, q := range userQuestions {
			if i >= 10 {
				fmt.Fprintf(&b, "  ... and %d more\n", len(userQuestions)-10)
				break
			}
			fmt.Fprintf(&b, "  %d. %s\n", i+1, q)
		}
		b.WriteString("\n")
	}

	if len(assistantSummaries) > 0 {
		b.WriteString("### Key Assistant Responses\n")
		for i, s := range assistantSummaries {
			if i >= 5 {
				fmt.Fprintf(&b, "  ... and %d more responses\n", len(assistantSummaries)-5)
				break
			}
			fmt.Fprintf(&b, "  - %s\n", s)
		}
		b.WriteString("\n")
	}

	if len(toolUsage) > 0 {
		b.WriteString("### Tools Used\n")
		for name, count := range toolUsage {
			fmt.Fprintf(&b, "  - %s: %d calls\n", name, count)
		}
		b.WriteString("\n")
	}

	if len(filesReferenced) > 0 {
		b.WriteString("### Files Referenced\n")
		count := 0
		for path := range filesReferenced {
			if count >= 20 {
				fmt.Fprintf(&b, "  ... and %d more files\n", len(filesReferenced)-20)
				break
			}
			fmt.Fprintf(&b, "  - %s\n", path)
			count++
		}
		b.WriteString("\n")
	}

	b.WriteString("[End of summary. Continue the conversation from here.]\n")

	return b.String()
}
