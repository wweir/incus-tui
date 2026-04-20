package instances

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/example/incus-tui/internal/client"
)

type Action int

const (
	ActionNone Action = iota
	ActionStart
	ActionStop
	ActionDelete
)

type Model struct {
	table          table.Model
	svc            client.InstanceService
	timeout        time.Duration
	status         string
	busy           bool
	confirming     bool
	pendingAction  Action
	selectedTarget string
	width          int
}

type instancesLoadedMsg struct {
	items []client.Instance
	err   error
}

type instanceActionDoneMsg struct {
	action Action
	target string
	err    error
}

func New(svc client.InstanceService, timeout time.Duration) Model {
	columns := []table.Column{
		{Title: "Name", Width: 24},
		{Title: "Status", Width: 14},
		{Title: "Type", Width: 10},
		{Title: "IPv4", Width: 30},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(14),
	)
	t.SetStyles(defaultTableStyles())

	return Model{
		table:   t,
		svc:     svc,
		timeout: timeout,
		status:  "Loading instances...",
	}
}

func (m Model) Init() tea.Cmd {
	return m.refreshCmd()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.table.SetWidth(msg.Width - 2)
	case tea.KeyMsg:
		if m.confirming {
			switch msg.String() {
			case "y", "Y":
				m.confirming = false
				m.busy = true
				m.status = fmt.Sprintf("Deleting %s...", m.selectedTarget)
				return m, m.actionCmd(ActionDelete, m.selectedTarget)
			case "n", "N", "esc":
				m.confirming = false
				m.pendingAction = ActionNone
				m.selectedTarget = ""
				m.status = "Delete cancelled"
				return m, nil
			}
		}

		if m.busy {
			return m, nil
		}

		switch msg.String() {
		case "r":
			m.busy = true
			m.status = "Refreshing instances..."
			return m, m.refreshCmd()
		case "s":
			target := m.currentName()
			if target == "" {
				return m, nil
			}
			m.busy = true
			m.status = fmt.Sprintf("Starting %s...", target)
			return m, m.actionCmd(ActionStart, target)
		case "x":
			target := m.currentName()
			if target == "" {
				return m, nil
			}
			m.busy = true
			m.status = fmt.Sprintf("Stopping %s...", target)
			return m, m.actionCmd(ActionStop, target)
		case "d":
			target := m.currentName()
			if target == "" {
				return m, nil
			}
			m.confirming = true
			m.pendingAction = ActionDelete
			m.selectedTarget = target
			m.status = fmt.Sprintf("Confirm delete %s? (y/n)", target)
			return m, nil
		}
	case instancesLoadedMsg:
		m.busy = false
		if msg.err != nil {
			m.status = fmt.Sprintf("Load failed: %v", msg.err)
			return m, nil
		}

		rows := make([]table.Row, 0, len(msg.items))
		for _, item := range msg.items {
			rows = append(rows, table.Row{item.Name, item.Status, item.Type, strings.Join(item.IP4, ",")})
		}
		m.table.SetRows(rows)
		m.status = fmt.Sprintf("Loaded %d instances", len(rows))
	case instanceActionDoneMsg:
		m.busy = false
		if msg.err != nil {
			m.status = fmt.Sprintf("Action failed on %s: %v", msg.target, msg.err)
			return m, nil
		}
		m.status = fmt.Sprintf("Action completed: %s", msg.target)
		return m, m.refreshCmd()
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	header := lipgloss.NewStyle().Bold(true).Render("Incus TUI - Instances")
	help := "[j/k or ↑/↓] navigate [r] refresh [s] start [x] stop [d] delete [q] quit"
	if m.confirming {
		help = "Confirm delete: [y] yes [n] no"
	}
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(m.status)
	return fmt.Sprintf("%s\n\n%s\n\n%s\n%s", header, m.table.View(), help, footer)
}

func (m Model) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
		defer cancel()

		items, err := m.svc.ListInstances(ctx)
		return instancesLoadedMsg{items: items, err: err}
	}
}

func (m Model) actionCmd(action Action, target string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
		defer cancel()

		var err error
		switch action {
		case ActionStart:
			err = m.svc.StartInstance(ctx, target)
		case ActionStop:
			err = m.svc.StopInstance(ctx, target)
		case ActionDelete:
			err = m.svc.DeleteInstance(ctx, target)
		}
		return instanceActionDoneMsg{action: action, target: target, err: err}
	}
}

func (m Model) currentName() string {
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return ""
	}
	return row[0]
}

func defaultTableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true).BorderStyle(lipgloss.NormalBorder()).BorderBottom(true)
	s.Selected = s.Selected.Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Bold(true)
	return s
}
