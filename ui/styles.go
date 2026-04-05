package ui

import "github.com/charmbracelet/lipgloss"

var (
	headerStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#5B3EC4")).
		Padding(0, 2)

	userStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#36B3E0")).
		Bold(true)

	assistantStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C084FC")).
		Bold(true)

	toolCallStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Italic(true)

	toolResultStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6EE7B7"))

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444")).
		Bold(true)

	statusBarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Background(lipgloss.Color("#1F2937")).
		Padding(0, 1)

	permissionStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#F59E0B")).
		Padding(1, 2)

	dimStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))
)
