package detailview

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/example/incus-tui/internal/client"
)

const (
	defaultWidth  = 72
	defaultHeight = 16
)

type Model struct {
	title    string
	fields   []client.DetailField
	viewport viewport.Model
	width    int
	height   int
}

func New() Model {
	vp := viewport.New(defaultWidth, defaultHeight)
	return Model{
		viewport: vp,
		width:    defaultWidth,
		height:   defaultHeight,
	}
}

func (m Model) Active() bool {
	return strings.TrimSpace(m.title) != ""
}

func (m Model) Title() string {
	return m.title
}

func (m Model) View() string {
	return m.viewport.View()
}

func (m Model) YOffset() int {
	return m.viewport.YOffset
}

func (m Model) TotalLineCount() int {
	return m.viewport.TotalLineCount()
}

func (m Model) HelpText(refreshText string) string {
	scroll := "[j/k or ↑/↓] scroll [pgup/pgdn] page [g/G] top/bottom"
	if m.viewport.TotalLineCount() <= m.viewport.Height {
		scroll = "[j/k or ↑/↓] browse detail"
	}
	if strings.TrimSpace(refreshText) == "" {
		return fmt.Sprintf("%s [esc] back", scroll)
	}
	return fmt.Sprintf("%s [esc] back [%s]", scroll, refreshText)
}

func (m *Model) SetSize(width, height int) {
	m.width = max(24, width)
	m.height = max(6, height)
	m.viewport.Width = m.width
	m.viewport.Height = m.height
	m.rebuild()
}

func (m *Model) SetDetail(detail client.ResourceDetail) {
	m.title = strings.TrimSpace(detail.Title)
	m.fields = append([]client.DetailField(nil), detail.Fields...)
	m.rebuild()
	m.viewport.GotoTop()
}

func (m *Model) Clear() {
	m.title = ""
	m.fields = nil
	m.viewport.SetContent("")
	m.viewport.GotoTop()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.Active() {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j":
			m.viewport.ScrollDown(1)
			return m, nil
		case "k":
			m.viewport.ScrollUp(1)
			return m, nil
		case "g", "home":
			m.viewport.GotoTop()
			return m, nil
		case "G", "end":
			m.viewport.GotoBottom()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *Model) rebuild() {
	m.viewport.Width = max(24, m.width)
	m.viewport.Height = max(6, m.height)
	m.viewport.SetContent(renderFields(m.viewport.Width, m.fields))
	if m.viewport.PastBottom() {
		m.viewport.GotoBottom()
	}
}

func renderFields(width int, fields []client.DetailField) string {
	if len(fields) == 0 {
		return "No detail available."
	}

	useColumns := width >= 72
	labelWidth := measureLabelWidth(fields, width)
	blocks := make([]string, 0, len(fields))
	for _, field := range fields {
		blocks = append(blocks, renderField(width, labelWidth, useColumns, field))
	}
	return strings.Join(blocks, "\n\n")
}

func measureLabelWidth(fields []client.DetailField, width int) int {
	if width < 72 {
		return 0
	}

	labelWidth := 12
	for _, field := range fields {
		if strings.Contains(field.Value, "\n") {
			continue
		}
		candidate := len(strings.TrimSpace(field.Label)) + 1
		if candidate > labelWidth {
			labelWidth = candidate
		}
	}
	return min(20, labelWidth)
}

func renderField(width, labelWidth int, useColumns bool, field client.DetailField) string {
	label := strings.TrimSpace(field.Label)
	if label == "" {
		label = "Value"
	}

	value := strings.TrimSpace(field.Value)
	if value == "" {
		value = "-"
	}

	if useColumns && !strings.Contains(value, "\n") {
		return renderCompactRow(label, value, width, labelWidth)
	}

	wrapped := wrapBlock(value, max(12, width-2))
	return fmt.Sprintf("%s:\n%s", label, indentBlock(wrapped, "  "))
}

func renderCompactRow(label, value string, width, labelWidth int) string {
	valueWidth := max(12, width-labelWidth-2)
	lines := wrapLine(value, valueWidth)
	if len(lines) == 0 {
		lines = []string{"-"}
	}

	out := make([]string, 0, len(lines))
	for index, line := range lines {
		prefix := ""
		if index == 0 {
			prefix = fmt.Sprintf("%-*s ", labelWidth, label+":")
		} else {
			prefix = fmt.Sprintf("%-*s ", labelWidth, "")
		}
		out = append(out, prefix+line)
	}
	return strings.Join(out, "\n")
}

func wrapBlock(value string, width int) string {
	width = max(1, width)

	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
		if trimmed == "" {
			out = append(out, "")
			continue
		}
		indentWidth := len(line) - len(trimmed)
		indent := strings.Repeat(" ", indentWidth)
		for _, segment := range wrapLine(trimmed, max(1, width-indentWidth)) {
			out = append(out, indent+segment)
		}
	}

	return strings.Join(out, "\n")
}

func wrapLine(value string, width int) []string {
	width = max(1, width)

	if strings.TrimSpace(value) == "" {
		return []string{""}
	}

	runes := []rune(value)
	if len(runes) <= width {
		return []string{value}
	}

	lines := make([]string, 0, len(runes)/width+1)
	rest := runes
	for len(rest) > width {
		split := width
		for index := width; index > 0; index-- {
			if unicode.IsSpace(rest[index-1]) {
				split = index
				break
			}
		}

		line := strings.TrimRightFunc(string(rest[:split]), unicode.IsSpace)
		if line == "" {
			line = string(rest[:width])
			rest = rest[width:]
		} else {
			rest = trimLeftRunes(rest[split:])
		}
		lines = append(lines, line)
	}

	if len(rest) > 0 {
		lines = append(lines, string(rest))
	}
	return lines
}

func trimLeftRunes(input []rune) []rune {
	index := 0
	for index < len(input) && unicode.IsSpace(input[index]) {
		index++
	}
	return input[index:]
}

func indentBlock(value, indent string) string {
	lines := strings.Split(value, "\n")
	for index := range lines {
		lines[index] = indent + lines[index]
	}
	return strings.Join(lines, "\n")
}
