package instances

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/example/incus-tui/internal/client"
)

type Action int

const (
	ActionNone Action = iota
	ActionCreate
	ActionUpdate
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
	formOpen       bool
	formInputs     []textinput.Model
	formIndex      int
	formTitle      string
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
	columns := []table.Column{{Title: "Name", Width: 24}, {Title: "Status", Width: 14}, {Title: "Type", Width: 10}, {Title: "IPv4", Width: 30}}
	t := table.New(table.WithColumns(columns), table.WithFocused(true), table.WithHeight(14))
	t.SetStyles(defaultTableStyles())
	return Model{table: t, svc: svc, timeout: timeout, status: "Loading instances..."}
}

func (m Model) Init() tea.Cmd { return m.refreshCmd() }

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
				m.status = fmt.Sprintf("Applying %s...", m.selectedTarget)
				return m, m.submitPendingActionCmd()
			case "n", "N", "esc":
				m.confirming = false
				m.pendingAction = ActionNone
				m.selectedTarget = ""
				m.status = "Action cancelled"
				return m, nil
			}
		}

		if m.formOpen {
			return m.handleFormKey(msg)
		}

		if m.busy {
			return m, nil
		}

		switch msg.String() {
		case "r":
			m.busy = true
			m.status = "Refreshing instances..."
			return m, m.refreshCmd()
		case "c":
			m.initCreateForm()
			return m, nil
		case "u":
			target := m.currentName()
			if target == "" {
				return m, nil
			}
			m.initUpdateForm(target)
			return m, nil
		case "s":
			target := m.currentName()
			if target == "" {
				return m, nil
			}
			m.busy = true
			m.status = fmt.Sprintf("Starting %s...", target)
			return m, m.actionCmd(ActionStart, target, nil)
		case "x":
			target := m.currentName()
			if target == "" {
				return m, nil
			}
			m.busy = true
			m.status = fmt.Sprintf("Stopping %s...", target)
			return m, m.actionCmd(ActionStop, target, nil)
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

func (m Model) handleFormKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.formOpen = false
		m.status = "Form cancelled"
		return m, nil
	case "tab", "shift+tab", "up", "down":
		delta := 1
		if msg.String() == "shift+tab" || msg.String() == "up" {
			delta = -1
		}
		m.formIndex = (m.formIndex + delta + len(m.formInputs)) % len(m.formInputs)
		for i := range m.formInputs {
			if i == m.formIndex {
				m.formInputs[i].Focus()
			} else {
				m.formInputs[i].Blur()
			}
		}
		return m, nil
	case "enter":
		m.formOpen = false
		m.confirming = true
		m.pendingAction = m.formAction()
		m.selectedTarget = m.formInputs[0].Value()
		m.status = fmt.Sprintf("Confirm %s on %s? (y/n)", strings.ToLower(actionName(m.pendingAction)), m.selectedTarget)
		return m, nil
	}
	var cmd tea.Cmd
	m.formInputs[m.formIndex], cmd = m.formInputs[m.formIndex].Update(msg)
	return m, cmd
}

func (m Model) View() string {
	header := lipgloss.NewStyle().Bold(true).Render("Incus TUI - Instances")
	if m.formOpen {
		return fmt.Sprintf("%s\n\n%s\n\n%s\n%s", header, m.renderForm(), "[tab] next [enter] submit [esc] cancel", m.status)
	}
	help := "[j/k or ↑/↓] navigate [r] refresh [c] create [u] update [s] start [x] stop [d] delete [q] quit"
	if m.confirming {
		help = "Confirm action: [y] yes [n] no"
	}
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(m.status)
	return fmt.Sprintf("%s\n\n%s\n\n%s\n%s", header, m.table.View(), help, footer)
}

func (m Model) renderForm() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(m.formTitle))
	b.WriteString("\n\n")
	for i, input := range m.formInputs {
		b.WriteString(input.Prompt)
		b.WriteString(input.View())
		if i < len(m.formInputs)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m *Model) initCreateForm() {
	m.formTitle = "Create instance"
	m.formInputs = []textinput.Model{newInput("Name: ", "instance name", ""), newInput("Image: ", "image alias/fingerprint", "images:alpine/edge"), newInput("Type: ", "container or virtual-machine", "container")}
	m.focusFormInput(0)
	m.formOpen = true
	m.pendingAction = ActionCreate
	m.status = "Fill create form then press enter"
}

func (m *Model) initUpdateForm(target string) {
	m.formTitle = "Update instance config"
	m.formInputs = []textinput.Model{newInput("Name: ", "instance name", target), newInput("Config key: ", "e.g. limits.cpu", ""), newInput("Config value: ", "new value", "")}
	m.focusFormInput(1)
	m.formOpen = true
	m.pendingAction = ActionUpdate
	m.status = "Fill update form then press enter"
}

func (m *Model) focusFormInput(index int) {
	m.formIndex = index
	for i := range m.formInputs {
		if i == index {
			m.formInputs[i].Focus()
		} else {
			m.formInputs[i].Blur()
		}
	}
}

func (m Model) formAction() Action { return m.pendingAction }

func (m Model) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
		defer cancel()
		items, err := m.svc.ListInstances(ctx)
		return instancesLoadedMsg{items: items, err: err}
	}
}

func (m Model) submitPendingActionCmd() tea.Cmd {
	action := m.pendingAction
	target := m.selectedTarget
	var payload map[string]string
	if action == ActionCreate || action == ActionUpdate {
		payload = map[string]string{}
		for _, input := range m.formInputs {
			key := strings.TrimSpace(strings.TrimSuffix(input.Prompt, ": "))
			payload[strings.ToLower(strings.ReplaceAll(key, " ", "_"))] = strings.TrimSpace(input.Value())
		}
	}
	m.pendingAction = ActionNone
	return m.actionCmd(action, target, payload)
}

func (m Model) actionCmd(action Action, target string, payload map[string]string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
		defer cancel()
		var err error
		switch action {
		case ActionCreate:
			err = m.svc.CreateInstance(ctx, payload["name"], payload["image"], payload["type"])
		case ActionUpdate:
			err = m.svc.UpdateInstanceConfig(ctx, payload["name"], payload["config_key"], payload["config_value"])
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

func actionName(action Action) string {
	switch action {
	case ActionCreate:
		return "CREATE"
	case ActionUpdate:
		return "UPDATE"
	case ActionStart:
		return "START"
	case ActionStop:
		return "STOP"
	case ActionDelete:
		return "DELETE"
	default:
		return "NONE"
	}
}

func newInput(prompt, placeholder, value string) textinput.Model {
	in := textinput.New()
	in.Prompt = prompt
	in.Placeholder = placeholder
	in.SetValue(value)
	in.CharLimit = 256
	in.Width = 48
	return in
}

func defaultTableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true).BorderStyle(lipgloss.NormalBorder()).BorderBottom(true)
	s.Selected = s.Selected.Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Bold(true)
	return s
}
