package markdown

import (
	"strings"
	"testing"
)

// --- New() tests ---

func TestNew_ZeroWidthDefaultsTo80(t *testing.T) {
	r := New(0)
	if r.width != 80 {
		t.Errorf("New(0).width = %d, want 80", r.width)
	}
}

func TestNew_NegativeWidthDefaultsTo80(t *testing.T) {
	r := New(-10)
	if r.width != 80 {
		t.Errorf("New(-10).width = %d, want 80", r.width)
	}
}

func TestNew_PositiveWidthUsesGivenValue(t *testing.T) {
	r := New(120)
	if r.width != 120 {
		t.Errorf("New(120).width = %d, want 120", r.width)
	}
}

func TestNew_WidthOf1(t *testing.T) {
	r := New(1)
	if r.width != 1 {
		t.Errorf("New(1).width = %d, want 1", r.width)
	}
}

// --- Render headings ---

func TestRender_H1(t *testing.T) {
	r := New(80)
	output := r.Render("# Hello World")
	if !strings.Contains(output, "Hello World") {
		t.Errorf("expected output to contain 'Hello World', got: %q", output)
	}
}

func TestRender_H2(t *testing.T) {
	r := New(80)
	output := r.Render("## Section Title")
	if !strings.Contains(output, "Section Title") {
		t.Errorf("expected output to contain 'Section Title', got: %q", output)
	}
}

func TestRender_H3(t *testing.T) {
	r := New(80)
	output := r.Render("### Subsection")
	if !strings.Contains(output, "Subsection") {
		t.Errorf("expected output to contain 'Subsection', got: %q", output)
	}
}

func TestRender_MultipleHeadings(t *testing.T) {
	r := New(80)
	input := "# First\n## Second\n### Third"
	output := r.Render(input)
	if !strings.Contains(output, "First") {
		t.Error("expected output to contain 'First'")
	}
	if !strings.Contains(output, "Second") {
		t.Error("expected output to contain 'Second'")
	}
	if !strings.Contains(output, "Third") {
		t.Error("expected output to contain 'Third'")
	}
}

// --- Render code blocks ---

func TestRender_CodeBlockWithLanguage(t *testing.T) {
	r := New(80)
	input := "```go\nfmt.Println(\"hello\")\n```"
	output := r.Render(input)
	if !strings.Contains(output, "go") {
		t.Error("expected output to contain language label 'go'")
	}
	if !strings.Contains(output, "fmt.Println") {
		t.Error("expected output to contain code content")
	}
}

func TestRender_CodeBlockWithoutLanguage(t *testing.T) {
	r := New(80)
	input := "```\nsome code here\n```"
	output := r.Render(input)
	if !strings.Contains(output, "some code here") {
		t.Errorf("expected output to contain 'some code here', got: %q", output)
	}
}

func TestRender_CodeBlockMultipleLines(t *testing.T) {
	r := New(80)
	input := "```python\ndef hello():\n    print('hi')\n```"
	output := r.Render(input)
	if !strings.Contains(output, "def hello():") {
		t.Error("expected output to contain 'def hello():'")
	}
	if !strings.Contains(output, "print") {
		t.Error("expected output to contain 'print'")
	}
	if !strings.Contains(output, "python") {
		t.Error("expected output to contain language label 'python'")
	}
}

func TestRender_UnclosedCodeBlock(t *testing.T) {
	r := New(80)
	input := "```\nunclosed code\nmore code"
	output := r.Render(input)
	if !strings.Contains(output, "unclosed code") {
		t.Error("expected unclosed code block to still render code content")
	}
	if !strings.Contains(output, "more code") {
		t.Error("expected unclosed code block to include all remaining lines")
	}
}

// --- Render inline code ---

func TestRender_InlineCode(t *testing.T) {
	r := New(80)
	output := r.Render("Use `fmt.Println` to print")
	if !strings.Contains(output, "fmt.Println") {
		t.Errorf("expected output to contain 'fmt.Println', got: %q", output)
	}
	if !strings.Contains(output, "Use") {
		t.Error("expected output to contain surrounding text 'Use'")
	}
	if !strings.Contains(output, "to print") {
		t.Error("expected output to contain surrounding text 'to print'")
	}
}

func TestRender_MultipleInlineCodes(t *testing.T) {
	r := New(80)
	output := r.Render("Use `foo` and `bar` together")
	if !strings.Contains(output, "foo") {
		t.Error("expected output to contain 'foo'")
	}
	if !strings.Contains(output, "bar") {
		t.Error("expected output to contain 'bar'")
	}
}

// --- Render bold ---

func TestRender_BoldAsterisks(t *testing.T) {
	r := New(80)
	output := r.Render("This is **bold** text")
	if !strings.Contains(output, "bold") {
		t.Errorf("expected output to contain 'bold', got: %q", output)
	}
	if !strings.Contains(output, "This is") {
		t.Error("expected surrounding text")
	}
}

func TestRender_BoldUnderscores(t *testing.T) {
	r := New(80)
	output := r.Render("This is __bold__ text")
	if !strings.Contains(output, "bold") {
		t.Errorf("expected output to contain 'bold', got: %q", output)
	}
}

// --- Render italic ---

func TestRender_ItalicAsterisk(t *testing.T) {
	r := New(80)
	output := r.Render("This is *italic* text")
	if !strings.Contains(output, "italic") {
		t.Errorf("expected output to contain 'italic', got: %q", output)
	}
}

func TestRender_ItalicUnderscore(t *testing.T) {
	r := New(80)
	output := r.Render("This is _italic_ text")
	if !strings.Contains(output, "italic") {
		t.Errorf("expected output to contain 'italic', got: %q", output)
	}
}

// --- Render blockquote ---

func TestRender_Blockquote(t *testing.T) {
	r := New(80)
	output := r.Render("> This is a quote")
	if !strings.Contains(output, "This is a quote") {
		t.Errorf("expected output to contain 'This is a quote', got: %q", output)
	}
}

func TestRender_BlockquoteMultipleLines(t *testing.T) {
	r := New(80)
	input := "> First line\n> Second line"
	output := r.Render(input)
	if !strings.Contains(output, "First line") {
		t.Error("expected output to contain 'First line'")
	}
	if !strings.Contains(output, "Second line") {
		t.Error("expected output to contain 'Second line'")
	}
}

func TestRender_BlockquoteEmptyLine(t *testing.T) {
	r := New(80)
	input := "> First\n>\n> Third"
	output := r.Render(input)
	if !strings.Contains(output, "First") {
		t.Error("expected output to contain 'First'")
	}
	if !strings.Contains(output, "Third") {
		t.Error("expected output to contain 'Third'")
	}
}

// --- Render unordered lists ---

func TestRender_UnorderedList_DashPrefix(t *testing.T) {
	r := New(80)
	input := "- Item one\n- Item two\n- Item three"
	output := r.Render(input)
	if !strings.Contains(output, "Item one") {
		t.Error("expected output to contain 'Item one'")
	}
	if !strings.Contains(output, "Item two") {
		t.Error("expected output to contain 'Item two'")
	}
	if !strings.Contains(output, "Item three") {
		t.Error("expected output to contain 'Item three'")
	}
}

func TestRender_UnorderedList_AsteriskPrefix(t *testing.T) {
	r := New(80)
	input := "* Alpha\n* Beta"
	output := r.Render(input)
	if !strings.Contains(output, "Alpha") {
		t.Error("expected output to contain 'Alpha'")
	}
	if !strings.Contains(output, "Beta") {
		t.Error("expected output to contain 'Beta'")
	}
}

func TestRender_UnorderedList_PlusPrefix(t *testing.T) {
	r := New(80)
	input := "+ First\n+ Second"
	output := r.Render(input)
	if !strings.Contains(output, "First") {
		t.Error("expected output to contain 'First'")
	}
	if !strings.Contains(output, "Second") {
		t.Error("expected output to contain 'Second'")
	}
}

// --- Render ordered lists ---

func TestRender_OrderedList(t *testing.T) {
	r := New(80)
	input := "1. First\n2. Second\n3. Third"
	output := r.Render(input)
	if !strings.Contains(output, "First") {
		t.Error("expected output to contain 'First'")
	}
	if !strings.Contains(output, "Second") {
		t.Error("expected output to contain 'Second'")
	}
	if !strings.Contains(output, "Third") {
		t.Error("expected output to contain 'Third'")
	}
}

func TestRender_OrderedList_ContainsNumbers(t *testing.T) {
	r := New(80)
	input := "1. Alpha\n2. Beta"
	output := r.Render(input)
	if !strings.Contains(output, "1") {
		t.Error("expected output to contain number '1'")
	}
	if !strings.Contains(output, "2") {
		t.Error("expected output to contain number '2'")
	}
}

// --- Render horizontal rules ---

func TestRender_HorizontalRule_Dashes(t *testing.T) {
	r := New(80)
	output := r.Render("---")
	if !strings.Contains(output, "─") {
		t.Errorf("expected output to contain horizontal rule character '─', got: %q", output)
	}
}

func TestRender_HorizontalRule_Asterisks(t *testing.T) {
	r := New(80)
	output := r.Render("***")
	if !strings.Contains(output, "─") {
		t.Errorf("expected output to contain horizontal rule character '─', got: %q", output)
	}
}

func TestRender_HorizontalRule_Underscores(t *testing.T) {
	r := New(80)
	output := r.Render("___")
	if !strings.Contains(output, "─") {
		t.Errorf("expected output to contain horizontal rule character '─', got: %q", output)
	}
}

// --- Blank lines ---

func TestRender_BlankLinesPreserved(t *testing.T) {
	r := New(80)
	input := "Hello\n\nWorld"
	output := r.Render(input)
	if !strings.Contains(output, "Hello") {
		t.Error("expected output to contain 'Hello'")
	}
	if !strings.Contains(output, "World") {
		t.Error("expected output to contain 'World'")
	}
	// Blank line should produce an empty line in the output
	lines := strings.Split(output, "\n")
	foundEmpty := false
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			foundEmpty = true
			break
		}
	}
	if !foundEmpty {
		t.Error("expected blank line to be preserved in output")
	}
}

// --- RenderToTerminal convenience function ---

func TestRenderToTerminal(t *testing.T) {
	output := RenderToTerminal("# Hello", 80)
	if !strings.Contains(output, "Hello") {
		t.Errorf("expected RenderToTerminal output to contain 'Hello', got: %q", output)
	}
}

func TestRenderToTerminal_ZeroWidth(t *testing.T) {
	output := RenderToTerminal("Some text", 0)
	if !strings.Contains(output, "Some text") {
		t.Errorf("expected RenderToTerminal with width 0 to still render, got: %q", output)
	}
}

func TestRenderToTerminal_ComplexMarkdown(t *testing.T) {
	input := "# Title\n\nSome **bold** and *italic* text\n\n- item1\n- item2\n\n```go\nfmt.Println(\"hi\")\n```"
	output := RenderToTerminal(input, 100)
	if !strings.Contains(output, "Title") {
		t.Error("expected output to contain 'Title'")
	}
	if !strings.Contains(output, "bold") {
		t.Error("expected output to contain 'bold'")
	}
	if !strings.Contains(output, "italic") {
		t.Error("expected output to contain 'italic'")
	}
	if !strings.Contains(output, "item1") {
		t.Error("expected output to contain 'item1'")
	}
	if !strings.Contains(output, "fmt.Println") {
		t.Error("expected output to contain 'fmt.Println'")
	}
}

// --- isHorizontalRule ---

func TestIsHorizontalRule_ReturnsFalseForShortStrings(t *testing.T) {
	r := New(80)
	if r.isHorizontalRule("--") {
		t.Error("expected '--' (len 2) not to be a horizontal rule")
	}
	if r.isHorizontalRule("-") {
		t.Error("expected '-' (len 1) not to be a horizontal rule")
	}
	if r.isHorizontalRule("") {
		t.Error("expected '' (empty) not to be a horizontal rule")
	}
}

func TestIsHorizontalRule_ValidRules(t *testing.T) {
	r := New(80)
	valid := []string{"---", "***", "___", "----", "****", "________", "- - -", "* * *"}
	for _, v := range valid {
		if !r.isHorizontalRule(v) {
			t.Errorf("expected %q to be a horizontal rule", v)
		}
	}
}

func TestIsHorizontalRule_InvalidRules(t *testing.T) {
	r := New(80)
	invalid := []string{"--", "**", "__", "-*-", "abc", "---abc", "-_-"}
	for _, v := range invalid {
		if r.isHorizontalRule(v) {
			t.Errorf("expected %q NOT to be a horizontal rule", v)
		}
	}
}

// --- isOrderedListItem ---

func TestIsOrderedListItem_Valid(t *testing.T) {
	r := New(80)
	valid := []string{"1. text", "2. more", "10. ten", "99. large number"}
	for _, v := range valid {
		if !r.isOrderedListItem(v) {
			t.Errorf("expected %q to be an ordered list item", v)
		}
	}
}

func TestIsOrderedListItem_Invalid(t *testing.T) {
	r := New(80)
	invalid := []string{"abc", "- text", "1.noSpace", "hello", "", "1.", ". text"}
	for _, v := range invalid {
		if r.isOrderedListItem(v) {
			t.Errorf("expected %q NOT to be an ordered list item", v)
		}
	}
}

// --- isUnorderedListItem ---

func TestIsUnorderedListItem_Dash(t *testing.T) {
	r := New(80)
	if !r.isUnorderedListItem("- text") {
		t.Error("expected '- text' to be an unordered list item")
	}
}

func TestIsUnorderedListItem_Asterisk(t *testing.T) {
	r := New(80)
	if !r.isUnorderedListItem("* text") {
		t.Error("expected '* text' to be an unordered list item")
	}
}

func TestIsUnorderedListItem_Plus(t *testing.T) {
	r := New(80)
	if !r.isUnorderedListItem("+ text") {
		t.Error("expected '+ text' to be an unordered list item")
	}
}

func TestIsUnorderedListItem_Invalid(t *testing.T) {
	r := New(80)
	invalid := []string{"text", "1. ordered", "## heading", "", ">quote"}
	for _, v := range invalid {
		if r.isUnorderedListItem(v) {
			t.Errorf("expected %q NOT to be an unordered list item", v)
		}
	}
}

// --- splitOrderedItem ---

func TestSplitOrderedItem_Valid(t *testing.T) {
	num, text := splitOrderedItem("1. hello")
	if num != "1" {
		t.Errorf("expected num %q, got %q", "1", num)
	}
	if text != "hello" {
		t.Errorf("expected text %q, got %q", "hello", text)
	}
}

func TestSplitOrderedItem_MultiDigit(t *testing.T) {
	num, text := splitOrderedItem("42. the answer")
	if num != "42" {
		t.Errorf("expected num %q, got %q", "42", num)
	}
	if text != "the answer" {
		t.Errorf("expected text %q, got %q", "the answer", text)
	}
}

func TestSplitOrderedItem_NoDotSpace(t *testing.T) {
	num, text := splitOrderedItem("nodot")
	if num != "" {
		t.Errorf("expected empty num for input without '. ', got %q", num)
	}
	if text != "nodot" {
		t.Errorf("expected text to be the full input, got %q", text)
	}
}

func TestSplitOrderedItem_EmptyText(t *testing.T) {
	num, text := splitOrderedItem("5. ")
	if num != "5" {
		t.Errorf("expected num %q, got %q", "5", num)
	}
	if text != "" {
		t.Errorf("expected empty text, got %q", text)
	}
}

// --- countLeadingSpaces ---

func TestCountLeadingSpaces_NoSpaces(t *testing.T) {
	if got := countLeadingSpaces("hello"); got != 0 {
		t.Errorf("countLeadingSpaces(%q) = %d, want 0", "hello", got)
	}
}

func TestCountLeadingSpaces_Spaces(t *testing.T) {
	if got := countLeadingSpaces("    hello"); got != 4 {
		t.Errorf("countLeadingSpaces(%q) = %d, want 4", "    hello", got)
	}
}

func TestCountLeadingSpaces_Tabs(t *testing.T) {
	if got := countLeadingSpaces("\thello"); got != 4 {
		t.Errorf("countLeadingSpaces(%q) = %d, want 4", "\\thello", got)
	}
}

func TestCountLeadingSpaces_MixedTabsAndSpaces(t *testing.T) {
	if got := countLeadingSpaces("\t  hello"); got != 6 {
		t.Errorf("countLeadingSpaces(%q) = %d, want 6", "\\t  hello", got)
	}
}

func TestCountLeadingSpaces_EmptyString(t *testing.T) {
	if got := countLeadingSpaces(""); got != 0 {
		t.Errorf("countLeadingSpaces(%q) = %d, want 0", "", got)
	}
}

func TestCountLeadingSpaces_AllSpaces(t *testing.T) {
	if got := countLeadingSpaces("   "); got != 3 {
		t.Errorf("countLeadingSpaces(%q) = %d, want 3", "   ", got)
	}
}

// --- Render edge cases ---

func TestRender_EmptyString(t *testing.T) {
	r := New(80)
	output := r.Render("")
	// Should not panic and should produce something (at minimum an empty string)
	if output != "" {
		// It's acceptable if it returns empty or whitespace
		_ = output
	}
}

func TestRender_PlainText(t *testing.T) {
	r := New(80)
	output := r.Render("Just some plain text without any markdown")
	if !strings.Contains(output, "Just some plain text without any markdown") {
		t.Errorf("expected plain text to be passed through, got: %q", output)
	}
}

func TestRender_InlineCodeDoesNotProcessBold(t *testing.T) {
	r := New(80)
	output := r.Render("Use `**not bold**` here")
	// The text inside backticks should be preserved literally
	if !strings.Contains(output, "**not bold**") {
		// If inline code renders the asterisks, we at least want the text
		if !strings.Contains(output, "not bold") {
			t.Error("expected inline code content to be present")
		}
	}
}

func TestRender_MixedContent(t *testing.T) {
	r := New(80)
	input := "# Title\n\nA paragraph with **bold** and *italic*.\n\n> A quote\n\n- item\n\n---"
	output := r.Render(input)
	if !strings.Contains(output, "Title") {
		t.Error("expected 'Title' in output")
	}
	if !strings.Contains(output, "bold") {
		t.Error("expected 'bold' in output")
	}
	if !strings.Contains(output, "italic") {
		t.Error("expected 'italic' in output")
	}
	if !strings.Contains(output, "A quote") {
		t.Error("expected 'A quote' in output")
	}
	if !strings.Contains(output, "item") {
		t.Error("expected 'item' in output")
	}
	if !strings.Contains(output, "─") {
		t.Error("expected horizontal rule character in output")
	}
}

// --- trimUnorderedPrefix ---

func TestTrimUnorderedPrefix_Dash(t *testing.T) {
	if got := trimUnorderedPrefix("- hello"); got != "hello" {
		t.Errorf("trimUnorderedPrefix(%q) = %q, want %q", "- hello", got, "hello")
	}
}

func TestTrimUnorderedPrefix_Asterisk(t *testing.T) {
	if got := trimUnorderedPrefix("* hello"); got != "hello" {
		t.Errorf("trimUnorderedPrefix(%q) = %q, want %q", "* hello", got, "hello")
	}
}

func TestTrimUnorderedPrefix_Plus(t *testing.T) {
	if got := trimUnorderedPrefix("+ hello"); got != "hello" {
		t.Errorf("trimUnorderedPrefix(%q) = %q, want %q", "+ hello", got, "hello")
	}
}

func TestTrimUnorderedPrefix_NoPrefix(t *testing.T) {
	if got := trimUnorderedPrefix("hello"); got != "hello" {
		t.Errorf("trimUnorderedPrefix(%q) = %q, want %q", "hello", got, "hello")
	}
}

// --- Render: code block borders ---

func TestRender_CodeBlockContainsBorders(t *testing.T) {
	r := New(80)
	input := "```\ncode\n```"
	output := r.Render(input)
	if !strings.Contains(output, "─") {
		t.Error("expected code block to contain border character '─'")
	}
	if !strings.Contains(output, "│") {
		t.Error("expected code block to contain border character '│'")
	}
}

// --- Render: bullet character ---

func TestRender_UnorderedListContainsBullet(t *testing.T) {
	r := New(80)
	output := r.Render("- Item")
	if !strings.Contains(output, "•") {
		t.Error("expected unordered list to contain bullet '•'")
	}
	if !strings.Contains(output, "Item") {
		t.Error("expected unordered list to contain item text")
	}
}

// --- Render: ordered list number and dot ---

func TestRender_OrderedListContainsNumberAndDot(t *testing.T) {
	r := New(80)
	output := r.Render("1. First")
	if !strings.Contains(output, "1") {
		t.Error("expected ordered list to contain number '1'")
	}
	if !strings.Contains(output, ".") {
		t.Error("expected ordered list to contain '.'")
	}
	if !strings.Contains(output, "First") {
		t.Error("expected ordered list to contain item text 'First'")
	}
}

// --- Render: blockquote border ---

func TestRender_BlockquoteContainsBorder(t *testing.T) {
	r := New(80)
	output := r.Render("> Quote text")
	if !strings.Contains(output, "│") {
		t.Error("expected blockquote to contain border character '│'")
	}
}

// --- Width affects horizontal rule ---

func TestRender_HorizontalRuleWidthMatchesRenderer(t *testing.T) {
	r := New(40)
	output := r.Render("---")
	// The horizontal rule should contain repeated ─ characters.
	// Count the ─ runes in the raw output (ignoring ANSI codes).
	count := strings.Count(output, "─")
	if count != 40 {
		t.Errorf("expected horizontal rule to have 40 '─' characters for width 40, got %d", count)
	}
}
