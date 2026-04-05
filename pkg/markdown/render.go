package markdown

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Renderer renders markdown text to styled terminal output.
type Renderer struct {
	width int
}

// Styles used for rendering markdown elements.
var (
	h1Style = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#C084FC"))

	h2Style = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#818CF8"))

	h3Style = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#9CA3AF"))

	boldStyle = lipgloss.NewStyle().
		Bold(true)

	italicStyle = lipgloss.NewStyle().
		Italic(true)

	inlineCodeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9A8D4")).
		Background(lipgloss.Color("#1F2937"))

	codeBlockBorderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4B5563"))

	codeBlockStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D1D5DB"))

	codeBlockLangStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true)

	blockquoteBorderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	blockquoteTextStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Italic(true)

	hrStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4B5563"))

	bulletStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C084FC"))
)

// Regex patterns for inline formatting.
var (
	boldRegex       = regexp.MustCompile(`\*\*(.+?)\*\*`)
	boldUnderRegex  = regexp.MustCompile(`__(.+?)__`)
	italicRegex     = regexp.MustCompile(`\*(.+?)\*`)
	italicUnderRegex = regexp.MustCompile(`_(.+?)_`)
	inlineCodeRegex = regexp.MustCompile("`([^`]+)`")
)

// New creates a new markdown Renderer with the given terminal width.
func New(width int) *Renderer {
	if width <= 0 {
		width = 80
	}
	return &Renderer{width: width}
}

// Render takes a markdown string and returns styled terminal output.
func (r *Renderer) Render(markdown string) string {
	lines := strings.Split(markdown, "\n")
	var result []string

	i := 0
	for i < len(lines) {
		line := lines[i]

		// Code block (fenced with ```)
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			block, end := r.parseCodeBlock(lines, i)
			result = append(result, block)
			i = end + 1
			continue
		}

		// Horizontal rule
		trimmed := strings.TrimSpace(line)
		if r.isHorizontalRule(trimmed) {
			result = append(result, r.renderHorizontalRule())
			i++
			continue
		}

		// Headings
		if strings.HasPrefix(trimmed, "# ") {
			text := strings.TrimPrefix(trimmed, "# ")
			result = append(result, h1Style.Render(text))
			i++
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			text := strings.TrimPrefix(trimmed, "## ")
			result = append(result, h2Style.Render(text))
			i++
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			text := strings.TrimPrefix(trimmed, "### ")
			result = append(result, h3Style.Render(text))
			i++
			continue
		}

		// Blockquote
		if strings.HasPrefix(trimmed, "> ") || trimmed == ">" {
			block, end := r.parseBlockquote(lines, i)
			result = append(result, block)
			i = end + 1
			continue
		}

		// Unordered list item
		if r.isUnorderedListItem(trimmed) {
			block, end := r.parseUnorderedList(lines, i)
			result = append(result, block)
			i = end + 1
			continue
		}

		// Ordered list item
		if r.isOrderedListItem(trimmed) {
			block, end := r.parseOrderedList(lines, i)
			result = append(result, block)
			i = end + 1
			continue
		}

		// Blank line
		if trimmed == "" {
			result = append(result, "")
			i++
			continue
		}

		// Regular paragraph text — apply inline formatting
		result = append(result, r.renderInline(line))
		i++
	}

	return strings.Join(result, "\n")
}

// parseCodeBlock extracts and renders a fenced code block starting at line index i.
// Returns the rendered block and the index of the closing fence line.
func (r *Renderer) parseCodeBlock(lines []string, i int) (string, int) {
	opening := strings.TrimSpace(lines[i])
	lang := strings.TrimPrefix(opening, "```")
	lang = strings.TrimSpace(lang)

	var codeLines []string
	j := i + 1
	for j < len(lines) {
		if strings.TrimSpace(lines[j]) == "```" {
			break
		}
		codeLines = append(codeLines, lines[j])
		j++
	}

	// If we never found a closing fence, treat the rest as code
	endIdx := j
	if j >= len(lines) {
		endIdx = len(lines) - 1
	}

	code := strings.Join(codeLines, "\n")

	// Build the rendered code block
	var sb strings.Builder

	borderChar := "│"
	styledBorder := codeBlockBorderStyle.Render(borderChar)

	// Language label
	if lang != "" {
		sb.WriteString(codeBlockLangStyle.Render("  "+lang) + "\n")
	}

	// Top border
	codeWidth := r.width - 4
	if codeWidth < 20 {
		codeWidth = 20
	}
	topBorder := codeBlockBorderStyle.Render("  " + strings.Repeat("─", codeWidth))
	sb.WriteString(topBorder + "\n")

	// Code lines
	for _, cl := range strings.Split(code, "\n") {
		rendered := codeBlockStyle.Render(cl)
		sb.WriteString("  " + styledBorder + " " + rendered)
		sb.WriteString("\n")
	}

	// Bottom border
	bottomBorder := codeBlockBorderStyle.Render("  " + strings.Repeat("─", codeWidth))
	sb.WriteString(bottomBorder)

	return sb.String(), endIdx
}

// parseBlockquote collects consecutive blockquote lines starting at index i.
func (r *Renderer) parseBlockquote(lines []string, i int) (string, int) {
	var quoteLines []string
	j := i
	for j < len(lines) {
		trimmed := strings.TrimSpace(lines[j])
		if strings.HasPrefix(trimmed, "> ") {
			quoteLines = append(quoteLines, strings.TrimPrefix(trimmed, "> "))
		} else if trimmed == ">" {
			quoteLines = append(quoteLines, "")
		} else {
			break
		}
		j++
	}

	border := blockquoteBorderStyle.Render("│")
	var rendered []string
	for _, ql := range quoteLines {
		styled := blockquoteTextStyle.Render(r.renderInline(ql))
		rendered = append(rendered, "  "+border+" "+styled)
	}

	return strings.Join(rendered, "\n"), j - 1
}

// parseUnorderedList collects consecutive unordered list items starting at index i.
func (r *Renderer) parseUnorderedList(lines []string, i int) (string, int) {
	var rendered []string
	j := i
	for j < len(lines) {
		trimmed := strings.TrimSpace(lines[j])
		if r.isUnorderedListItem(trimmed) {
			// Determine indent level from original line
			indent := countLeadingSpaces(lines[j])
			indentLevel := indent / 2

			text := trimUnorderedPrefix(trimmed)
			bullet := bulletStyle.Render("•")
			prefix := strings.Repeat("  ", indentLevel)
			rendered = append(rendered, prefix+"  "+bullet+" "+r.renderInline(text))
		} else if trimmed == "" {
			// Allow blank lines within lists
			rendered = append(rendered, "")
		} else {
			break
		}
		j++
	}

	return strings.Join(rendered, "\n"), j - 1
}

// parseOrderedList collects consecutive ordered list items starting at index i.
func (r *Renderer) parseOrderedList(lines []string, i int) (string, int) {
	var rendered []string
	j := i
	for j < len(lines) {
		trimmed := strings.TrimSpace(lines[j])
		if r.isOrderedListItem(trimmed) {
			indent := countLeadingSpaces(lines[j])
			indentLevel := indent / 2

			// Extract the number and text
			num, text := splitOrderedItem(trimmed)
			prefix := strings.Repeat("  ", indentLevel)
			numStr := bulletStyle.Render(num + ".")
			rendered = append(rendered, prefix+"  "+numStr+" "+r.renderInline(text))
		} else if trimmed == "" {
			rendered = append(rendered, "")
		} else {
			break
		}
		j++
	}

	return strings.Join(rendered, "\n"), j - 1
}

// renderInline applies inline markdown formatting (bold, italic, code) to a line.
func (r *Renderer) renderInline(line string) string {
	// Process inline code first to prevent inner formatting
	// Replace inline code spans with placeholders, render at the end
	type placeholder struct {
		key  string
		text string
	}

	var placeholders []placeholder
	counter := 0

	// Extract inline code to protect from further processing
	processed := inlineCodeRegex.ReplaceAllStringFunc(line, func(match string) string {
		inner := inlineCodeRegex.FindStringSubmatch(match)
		if len(inner) < 2 {
			return match
		}
		key := "\x00CODE" + string(rune(counter)) + "\x00"
		counter++
		placeholders = append(placeholders, placeholder{key: key, text: inner[1]})
		return key
	})

	// Bold (must come before italic since ** contains *)
	processed = boldRegex.ReplaceAllStringFunc(processed, func(match string) string {
		inner := boldRegex.FindStringSubmatch(match)
		if len(inner) < 2 {
			return match
		}
		return boldStyle.Render(inner[1])
	})
	processed = boldUnderRegex.ReplaceAllStringFunc(processed, func(match string) string {
		inner := boldUnderRegex.FindStringSubmatch(match)
		if len(inner) < 2 {
			return match
		}
		return boldStyle.Render(inner[1])
	})

	// Italic
	processed = italicRegex.ReplaceAllStringFunc(processed, func(match string) string {
		inner := italicRegex.FindStringSubmatch(match)
		if len(inner) < 2 {
			return match
		}
		return italicStyle.Render(inner[1])
	})
	processed = italicUnderRegex.ReplaceAllStringFunc(processed, func(match string) string {
		inner := italicUnderRegex.FindStringSubmatch(match)
		if len(inner) < 2 {
			return match
		}
		return italicStyle.Render(inner[1])
	})

	// Restore inline code placeholders
	for _, p := range placeholders {
		styled := inlineCodeStyle.Render(" " + p.text + " ")
		processed = strings.Replace(processed, p.key, styled, 1)
	}

	return processed
}

// renderHorizontalRule renders a full-width horizontal rule.
func (r *Renderer) renderHorizontalRule() string {
	w := r.width
	if w <= 0 {
		w = 80
	}
	return hrStyle.Render(strings.Repeat("─", w))
}

// isHorizontalRule checks if a trimmed line is a horizontal rule.
func (r *Renderer) isHorizontalRule(trimmed string) bool {
	if len(trimmed) < 3 {
		return false
	}
	// Must be only dashes, asterisks, or underscores (with optional spaces)
	cleaned := strings.ReplaceAll(trimmed, " ", "")
	if len(cleaned) < 3 {
		return false
	}
	allDashes := true
	allAsterisks := true
	allUnderscores := true
	for _, c := range cleaned {
		if c != '-' {
			allDashes = false
		}
		if c != '*' {
			allAsterisks = false
		}
		if c != '_' {
			allUnderscores = false
		}
	}
	return allDashes || allAsterisks || allUnderscores
}

// isUnorderedListItem checks if a trimmed line is an unordered list item.
func (r *Renderer) isUnorderedListItem(trimmed string) bool {
	return strings.HasPrefix(trimmed, "- ") ||
		strings.HasPrefix(trimmed, "* ") ||
		strings.HasPrefix(trimmed, "+ ")
}

// isOrderedListItem checks if a trimmed line is an ordered list item.
func (r *Renderer) isOrderedListItem(trimmed string) bool {
	for i, c := range trimmed {
		if c >= '0' && c <= '9' {
			continue
		}
		if c == '.' && i > 0 && i < len(trimmed)-1 && trimmed[i+1] == ' ' {
			return true
		}
		return false
	}
	return false
}

// trimUnorderedPrefix removes the leading "- ", "* ", or "+ " from a list item.
func trimUnorderedPrefix(s string) string {
	if strings.HasPrefix(s, "- ") {
		return strings.TrimPrefix(s, "- ")
	}
	if strings.HasPrefix(s, "* ") {
		return strings.TrimPrefix(s, "* ")
	}
	if strings.HasPrefix(s, "+ ") {
		return strings.TrimPrefix(s, "+ ")
	}
	return s
}

// splitOrderedItem splits "1. text" into ("1", "text").
func splitOrderedItem(s string) (string, string) {
	dotIdx := strings.Index(s, ". ")
	if dotIdx < 0 {
		return "", s
	}
	return s[:dotIdx], s[dotIdx+2:]
}

// countLeadingSpaces returns the number of leading space characters.
func countLeadingSpaces(s string) int {
	count := 0
loop:
	for _, c := range s {
		switch c {
		case ' ':
			count++
		case '\t':
			count += 4
		default:
			break loop
		}
	}
	return count
}

// RenderToTerminal is a convenience function that renders markdown text
// to styled terminal output with the given width.
func RenderToTerminal(markdown string, width int) string {
	return New(width).Render(markdown)
}
