package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// StreamingEditTool makes edits to files using fuzzy line matching.
// Unlike EditFileTool which requires exact text matching, this tool
// uses approximate matching to find the target location, making it
// more robust when the LLM doesn't perfectly recall whitespace or
// minor details of the existing code.
//
// Inspired by Zed's streaming_edit_file_tool which streams edits
// incrementally. In this implementation, the fuzzy matching is done
// at execution time using the complete input.
type StreamingEditTool struct{}

type streamingEditInput struct {
	Path               string `json:"path"`
	OldText            string `json:"old_text"`
	NewText            string `json:"new_text"`
	DisplayDescription string `json:"display_description"`
}

func (t *StreamingEditTool) Name() string {
	return "streaming_edit"
}

func (t *StreamingEditTool) Description() string {
	return `Make edits to a file using fuzzy line matching. This tool is more forgiving
than exact-match editing — it uses approximate matching to find the target
location even if whitespace or minor details differ slightly.

The old_text does not need to match the file contents exactly. The tool will
find the best-matching region in the file and replace it with new_text.

Use this tool when you want to edit a file but aren't 100% certain of the
exact current contents (e.g., whitespace, indentation). For exact matches,
use edit_file instead.`
}

func (t *StreamingEditTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to edit",
			},
			"old_text": map[string]any{
				"type":        "string",
				"description": "The approximate text to find in the file. Does not need to match exactly — fuzzy matching is used.",
			},
			"new_text": map[string]any{
				"type":        "string",
				"description": "The text to replace the matched region with.",
			},
			"display_description": map[string]any{
				"type":        "string",
				"description": "A brief description of the edit being made.",
			},
		},
		"required": []string{"path", "old_text", "new_text"},
	}
}

func (t *StreamingEditTool) Permission() PermissionLevel {
	return PermissionWrite
}

func (t *StreamingEditTool) Execute(ctx context.Context, input string) (*Result, error) {
	var params streamingEditInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return &Result{
			Error:   fmt.Sprintf("failed to parse input: %s", err),
			IsError: true,
		}, nil
	}

	if params.Path == "" {
		return &Result{Error: "path is required", IsError: true}, nil
	}
	if params.OldText == "" {
		return &Result{Error: "old_text is required", IsError: true}, nil
	}

	// Read the file
	data, err := os.ReadFile(params.Path)
	if err != nil {
		return &Result{
			Error:   fmt.Sprintf("failed to read file %s: %s", params.Path, err),
			IsError: true,
		}, nil
	}

	content := string(data)
	fileLines := strings.Split(content, "\n")
	searchLines := strings.Split(params.OldText, "\n")

	// Try exact match first (fast path)
	if strings.Contains(content, params.OldText) {
		count := strings.Count(content, params.OldText)
		if count == 1 {
			newContent := strings.Replace(content, params.OldText, params.NewText, 1)
			if err := os.WriteFile(params.Path, []byte(newContent), 0644); err != nil {
				return &Result{
					Error:   fmt.Sprintf("failed to write file: %s", err),
					IsError: true,
				}, nil
			}
			desc := params.DisplayDescription
			if desc == "" {
				desc = "Edit applied (exact match)"
			}
			return &Result{
				Output: fmt.Sprintf("%s\n\nReplaced %d lines with %d lines in %s",
					desc, len(searchLines), len(strings.Split(params.NewText, "\n")), params.Path),
				IsError: false,
			}, nil
		}
		// Multiple exact matches — fall through to fuzzy matching for best location
	}

	// Fuzzy matching: find the best matching region in the file
	bestStart, bestEnd, bestScore := findBestFuzzyMatch(fileLines, searchLines)

	if bestScore < 0.4 {
		return &Result{
			Error: fmt.Sprintf(
				"could not find a sufficiently similar region in %s (best similarity: %.1f%%). "+
					"The old_text may be too different from the actual file contents.",
				params.Path, bestScore*100),
			IsError: true,
		}, nil
	}

	// Replace the matched region
	var newLines []string
	newLines = append(newLines, fileLines[:bestStart]...)
	newLines = append(newLines, strings.Split(params.NewText, "\n")...)
	newLines = append(newLines, fileLines[bestEnd:]...)

	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(params.Path, []byte(newContent), 0644); err != nil {
		return &Result{
			Error:   fmt.Sprintf("failed to write file: %s", err),
			IsError: true,
		}, nil
	}

	desc := params.DisplayDescription
	if desc == "" {
		desc = "Edit applied (fuzzy match)"
	}

	matchInfo := fmt.Sprintf("Matched lines %d-%d (similarity: %.1f%%)",
		bestStart+1, bestEnd, bestScore*100)

	return &Result{
		Output: fmt.Sprintf("%s\n\n%s\nReplaced %d lines with %d lines in %s",
			desc, matchInfo,
			bestEnd-bestStart, len(strings.Split(params.NewText, "\n")), params.Path),
		IsError: false,
	}, nil
}

// findBestFuzzyMatch finds the region in fileLines that best matches searchLines.
// Returns the start index (inclusive), end index (exclusive), and similarity score.
func findBestFuzzyMatch(fileLines, searchLines []string) (int, int, float64) {
	if len(searchLines) == 0 || len(fileLines) == 0 {
		return 0, 0, 0
	}

	bestStart := 0
	bestEnd := 0
	bestScore := 0.0

	searchLen := len(searchLines)

	// Slide a window of size searchLen (and ±2 lines for flexibility) over the file
	for windowSize := max(1, searchLen-2); windowSize <= min(len(fileLines), searchLen+2); windowSize++ {
		for start := 0; start <= len(fileLines)-windowSize; start++ {
			end := start + windowSize
			window := fileLines[start:end]

			score := computeBlockSimilarity(window, searchLines)
			if score > bestScore {
				bestScore = score
				bestStart = start
				bestEnd = end
			}
		}
	}

	return bestStart, bestEnd, bestScore
}

// computeBlockSimilarity computes the similarity between two blocks of lines.
// Returns a value between 0 (no similarity) and 1 (identical).
func computeBlockSimilarity(block1, block2 []string) float64 {
	if len(block1) == 0 && len(block2) == 0 {
		return 1.0
	}
	if len(block1) == 0 || len(block2) == 0 {
		return 0.0
	}

	// Use a combination of:
	// 1. Line-by-line similarity (for aligned blocks)
	// 2. Overall content similarity

	// Line-by-line similarity using Levenshtein-based approach
	totalLineSim := 0.0
	maxLines := max(len(block1), len(block2))
	minLines := min(len(block1), len(block2))

	for i := 0; i < minLines; i++ {
		totalLineSim += lineSimilarity(block1[i], block2[i])
	}

	// Penalize for different number of lines
	lineSim := totalLineSim / float64(maxLines)

	// Content similarity: compare the full text
	text1 := strings.Join(block1, "\n")
	text2 := strings.Join(block2, "\n")
	contentSim := stringSimilarity(text1, text2)

	// Weighted combination
	return 0.7*lineSim + 0.3*contentSim
}

// lineSimilarity computes similarity between two lines (0 to 1).
// Uses normalized edit distance for short strings and token overlap for longer ones.
func lineSimilarity(a, b string) float64 {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)

	if a == b {
		return 1.0
	}
	if a == "" && b == "" {
		return 1.0
	}
	if a == "" || b == "" {
		return 0.0
	}

	// For short strings, use character-level Levenshtein
	if len(a) < 200 && len(b) < 200 {
		dist := levenshtein(a, b)
		maxLen := max(len(a), len(b))
		return 1.0 - float64(dist)/float64(maxLen)
	}

	// For longer strings, use token-based Jaccard similarity
	return tokenJaccard(a, b)
}

// stringSimilarity computes similarity between two strings using a combination
// of techniques suitable for code comparison.
func stringSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if a == "" && b == "" {
		return 1.0
	}
	if a == "" || b == "" {
		return 0.0
	}

	// Use token-based similarity for efficiency
	return tokenJaccard(a, b)
}

// tokenJaccard computes the Jaccard similarity coefficient between the
// whitespace-separated token multisets of two strings.
func tokenJaccard(a, b string) float64 {
	tokensA := tokenize(a)
	tokensB := tokenize(b)

	if len(tokensA) == 0 && len(tokensB) == 0 {
		return 1.0
	}

	intersection := 0
	setB := make(map[string]int)
	for _, t := range tokensB {
		setB[t]++
	}

	usedB := make(map[string]int)
	for _, t := range tokensA {
		if setB[t] > usedB[t] {
			intersection++
			usedB[t]++
		}
	}

	union := len(tokensA) + len(tokensB) - intersection
	if union == 0 {
		return 1.0
	}

	return float64(intersection) / float64(union)
}

// tokenize splits a string into tokens (words) for comparison.
func tokenize(s string) []string {
	return strings.Fields(s)
}

// levenshtein computes the Levenshtein edit distance between two strings.
func levenshtein(a, b string) int {
	la := len(a)
	lb := len(b)

	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// Use two-row optimization to save memory
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = minOf(
				prev[j]+1,      // deletion
				curr[j-1]+1,    // insertion
				prev[j-1]+cost, // substitution
			)
		}
		prev, curr = curr, prev
	}

	return prev[lb]
}

// minOf returns the minimum of three integers.
func minOf(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
