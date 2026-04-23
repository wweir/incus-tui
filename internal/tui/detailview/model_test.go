package detailview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/example/incus-tui/internal/client"
)

func TestUpdateSupportsScrollAndJumpKeys(t *testing.T) {
	m := New()
	m.SetSize(48, 4)
	m.SetDetail(client.ResourceDetail{
		Title: "detail",
		Fields: []client.DetailField{
			{Label: "Config", Value: strings.Repeat("line\n", 16)},
		},
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if updated.YOffset() != 1 {
		t.Fatalf("offset after j = %d, want 1", updated.YOffset())
	}

	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if updated.YOffset() == 0 {
		t.Fatalf("offset after G = %d, want > 0", updated.YOffset())
	}

	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if updated.YOffset() != 0 {
		t.Fatalf("offset after g = %d, want 0", updated.YOffset())
	}
}

func TestViewUsesCompactRowsForShortValues(t *testing.T) {
	m := New()
	m.SetSize(80, 8)
	m.SetDetail(client.ResourceDetail{
		Title: "detail",
		Fields: []client.DetailField{
			{Label: "Name", Value: "c1"},
			{Label: "Status", Value: "Running"},
		},
	})

	view := m.View()
	if !strings.Contains(view, "Name:") || !strings.Contains(view, "Status:") {
		t.Fatalf("view missing compact labels: %q", view)
	}
	if strings.Contains(view, "\n\n\n") {
		t.Fatalf("view contains excessive blank spacing: %q", view)
	}
}
