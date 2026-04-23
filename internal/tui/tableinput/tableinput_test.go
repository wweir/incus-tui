package tableinput

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

func TestHandleMouseWheelMovesSelection(t *testing.T) {
	tbl := table.New(
		table.WithColumns([]table.Column{{Title: "Name", Width: 8}}),
		table.WithFocused(true),
		table.WithHeight(5),
		table.WithRows([]table.Row{{"a"}, {"b"}, {"c"}, {"d"}, {"e"}}),
	)

	tbl = HandleMouse(tbl, tea.MouseMsg{Button: tea.MouseButtonWheelDown}, 4)
	if got := tbl.Cursor(); got != ScrollRows {
		t.Fatalf("cursor after wheel down = %d, want %d", got, ScrollRows)
	}

	tbl = HandleMouse(tbl, tea.MouseMsg{Button: tea.MouseButtonWheelUp}, 4)
	if got := tbl.Cursor(); got != 0 {
		t.Fatalf("cursor after wheel up = %d, want 0", got)
	}
}

func TestHandleMouseClickSelectsVisibleRow(t *testing.T) {
	tbl := table.New(
		table.WithColumns([]table.Column{{Title: "Name", Width: 8}}),
		table.WithFocused(true),
		table.WithHeight(5),
		table.WithRows([]table.Row{{"a"}, {"b"}, {"c"}}),
	)

	tbl = HandleMouse(tbl, tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: 5}, 4)
	if got := tbl.Cursor(); got != 1 {
		t.Fatalf("cursor after click = %d, want 1", got)
	}
}
