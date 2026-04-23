package tableinput

import (
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

const ScrollRows = 3

func HandleMouse(tbl table.Model, msg tea.MouseMsg, firstRowY int) table.Model {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		tbl.MoveUp(ScrollRows)
	case tea.MouseButtonWheelDown:
		tbl.MoveDown(ScrollRows)
	case tea.MouseButtonLeft:
		if msg.Action == tea.MouseActionPress {
			selectVisibleRow(&tbl, msg.Y-firstRowY)
		}
	}
	return tbl
}

func selectVisibleRow(tbl *table.Model, visibleRow int) {
	if visibleRow < 0 || visibleRow >= tbl.Height() {
		return
	}
	rows := tbl.Rows()
	if len(rows) == 0 {
		return
	}

	cursor := tbl.Cursor()
	start := cursor - tbl.Height()
	if start < 0 {
		start = 0
	}
	next := start + visibleRow
	if next >= len(rows) {
		return
	}
	tbl.SetCursor(next)
}
