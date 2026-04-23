package overlay

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

const (
	defaultWidth  = 100
	defaultHeight = 30
)

func RenderScreen(width, height int, title, meta, body, help, status string, borderColor lipgloss.Color) string {
	width = normalizeWidth(width)
	height = normalizeHeight(height)

	panelWidth := clamp(width-6, 56, width-2)
	panelHeight := clamp(height-4, 14, height-1)

	contentWidth := max(24, panelWidth-4)
	contentHeight := max(8, panelHeight-2)

	parts := make([]string, 0, 5)
	if title != "" {
		parts = append(parts, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Render(title))
	}
	if meta != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(meta))
	}
	parts = append(parts, lipgloss.NewStyle().Width(contentWidth).Render(body))
	if help != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(help))
	}
	if status != "" {
		parts = append(parts, status)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	framed := lipgloss.NewStyle().
		Width(panelWidth).
		Height(panelHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(lipgloss.Place(contentWidth, contentHeight, lipgloss.Left, lipgloss.Top, content))

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, framed)
}

func RenderDialog(width, height int, title, body, help, status string, borderColor lipgloss.Color) string {
	width = normalizeWidth(width)
	height = normalizeHeight(height)

	panelWidth := clamp(width-16, 44, 84)
	panelHeight := clamp(height/2, 10, 16)
	contentWidth := max(20, panelWidth-4)
	contentHeight := max(6, panelHeight-2)

	parts := []string{
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Render(title),
		body,
	}
	if help != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(help))
	}
	if status != "" {
		parts = append(parts, status)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	framed := lipgloss.NewStyle().
		Width(panelWidth).
		Height(panelHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(lipgloss.Place(contentWidth, contentHeight, lipgloss.Left, lipgloss.Top, content))

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, framed)
}

func normalizeWidth(width int) int {
	if width <= 0 {
		return defaultWidth
	}
	return max(40, width)
}

func normalizeHeight(height int) int {
	if height <= 0 {
		return defaultHeight
	}
	return max(12, height)
}

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}

func DebugSize(width, height int) string {
	return fmt.Sprintf("%dx%d", normalizeWidth(width), normalizeHeight(height))
}
