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
	"github.com/example/incus-tui/internal/tui/detailview"
	"github.com/example/incus-tui/internal/tui/overlay"
	"github.com/example/incus-tui/internal/tui/tableinput"
)

const tableFirstRowY = 4

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
	formOpen       bool
	formInputs     []textinput.Model
	formIndex      int
	formTitle      string
	formErrors     map[int]string
	detailView     detailview.Model
	windowWidth    int
	windowHeight   int
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

type instanceDetailLoadedMsg struct {
	target string
	detail client.ResourceDetail
	err    error
}

func New(svc client.InstanceService, timeout time.Duration) Model {
	columns := []table.Column{{Title: "Name", Width: 24}, {Title: "Status", Width: 14}, {Title: "Type", Width: 10}, {Title: "IPv4", Width: 30}}
	t := table.New(table.WithColumns(columns), table.WithFocused(true), table.WithHeight(14))
	t.SetStyles(defaultTableStyles())
	model := Model{table: t, svc: svc, timeout: timeout, status: "Loading instances...", detailView: detailview.New()}
	model.resizeDetailView(80, 24)
	return model
}

func (m Model) Init() tea.Cmd { return m.refreshCmd() }

func (m *Model) Focus() {
	m.table.Focus()
}

func (m *Model) Blur() {
	m.table.Blur()
}

func (m Model) Focused() bool {
	return m.table.Focused()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		m.table.SetWidth(msg.Width - 2)
		m.resizeDetailView(msg.Width-12, msg.Height-12)
	case tea.MouseMsg:
		if m.showingDetail() {
			var cmd tea.Cmd
			m.detailView, cmd = m.detailView.Update(msg)
			return m, cmd
		}
		if !m.busy && !m.formOpen && !m.confirming {
			m.table = tableinput.HandleMouse(m.table, msg, tableFirstRowY)
		}
		return m, nil
	case tea.KeyMsg:
		if m.showingDetail() {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "esc", "enter":
				m.detailView.Clear()
				m.status = "Back to list"
				return m, nil
			case "r":
				m.detailView.Clear()
				m.busy = true
				m.status = "Refreshing instances..."
				return m, m.refreshCmd()
			}
			var cmd tea.Cmd
			m.detailView, cmd = m.detailView.Update(msg)
			return m, cmd
		}

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
		case "enter":
			target := m.currentName()
			if target == "" {
				return m, nil
			}
			m.busy = true
			m.status = fmt.Sprintf("Loading detail for %s...", target)
			return m, m.detailCmd(target)
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
	case instanceDetailLoadedMsg:
		m.busy = false
		if msg.err != nil {
			m.status = fmt.Sprintf("Detail failed on %s: %v", msg.target, msg.err)
			return m, nil
		}
		m.detailView.SetDetail(msg.detail)
		m.status = fmt.Sprintf("Viewing detail: %s", msg.target)
		return m, nil
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
		m.formIndex = nextInputIndex(msg.String(), m.formIndex, len(m.formInputs))
		setFocusedInput(m.formInputs, m.formIndex)
		return m, nil
	case "enter":
		values := collectFormValues(m.formInputs)
		if errs := validateInstanceForm(m.pendingAction, values); len(errs) > 0 {
			m.formErrors = errs
			m.status = "Please fix invalid fields"
			return m, nil
		}
		m.formErrors = nil
		m.formOpen = false
		m.confirming = true
		m.selectedTarget = values["name"]
		m.status = fmt.Sprintf("Confirm %s on %s? (y/n)", strings.ToLower(actionName(m.pendingAction)), m.selectedTarget)
		return m, nil
	}
	var cmd tea.Cmd
	m.formInputs[m.formIndex], cmd = m.formInputs[m.formIndex].Update(msg)
	return m, cmd
}

func (m Model) View() string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Render("Incus TUI - Instances")
	meta := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(m.renderMeta())
	body := renderPanel("", m.table.View(), lipgloss.Color("238"))
	help := "[j/k or ↑/↓] navigate [enter] detail [r] refresh [c] create [u] update [s] start [x] stop [d] delete [q] quit"
	base := fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n%s", header, meta, body, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(help), renderStatusBar(m.status))

	switch {
	case m.formOpen:
		return overlay.RenderScreen(
			m.windowWidth,
			m.windowHeight,
			m.formTitle,
			meta,
			m.renderForm(),
			"[tab] next [enter] submit [esc] cancel",
			renderStatusBar(m.status),
			lipgloss.Color("62"),
		)
	case m.showingDetail():
		return overlay.RenderScreen(
			m.windowWidth,
			m.windowHeight,
			m.detailView.Title(),
			meta,
			m.detailView.View(),
			m.detailView.HelpText("r refresh"),
			renderStatusBar(m.status),
			lipgloss.Color("99"),
		)
	case m.confirming:
		return overlay.RenderDialog(
			m.windowWidth,
			m.windowHeight,
			"Confirm action",
			fmt.Sprintf("%s %s\n\nProceed with this change?", strings.ToLower(actionName(m.pendingAction)), m.selectedTarget),
			"[y] yes [n] no [esc] cancel",
			renderStatusBar(m.status),
			lipgloss.Color("214"),
		)
	default:
		return base
	}
}

func (m Model) renderForm() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Render(m.formTitle))
	b.WriteString("\n\n")
	for i, input := range m.formInputs {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("109")).Bold(true).Render(strings.TrimSpace(input.Prompt)))
		b.WriteString("\n")
		b.WriteString(input.View())
		if errText, ok := m.formErrors[i]; ok {
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("  ! " + errText))
		}
		if i < len(m.formInputs)-1 {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

func (m *Model) initCreateForm() {
	m.formTitle = "Create instance"
	m.formInputs = []textinput.Model{newInput("Name: ", "instance name", ""), newInput("Image: ", "image alias/fingerprint", "images:alpine/edge"), newInput("Type: ", "container or virtual-machine", "container")}
	m.focusFormInput(0)
	m.formOpen = true
	m.formErrors = nil
	m.pendingAction = ActionCreate
	m.status = "Fill create form then press enter"
}

func (m *Model) initUpdateForm(target string) {
	m.formTitle = "Update instance config"
	m.formInputs = []textinput.Model{newInput("Name: ", "instance name", target), newInput("Config key: ", "e.g. limits.cpu", ""), newInput("Config value: ", "new value", "")}
	m.focusFormInput(1)
	m.formOpen = true
	m.formErrors = nil
	m.pendingAction = ActionUpdate
	m.status = "Fill update form then press enter"
}

func (m *Model) focusFormInput(index int) {
	m.formIndex = index
	setFocusedInput(m.formInputs, index)
}

func (m Model) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
		defer cancel()
		items, err := m.svc.ListInstances(ctx)
		return instancesLoadedMsg{items: items, err: err}
	}
}

func (m Model) detailCmd(target string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
		defer cancel()
		detail, err := m.svc.GetInstanceDetail(ctx, target)
		return instanceDetailLoadedMsg{target: target, detail: detail, err: err}
	}
}

func (m Model) submitPendingActionCmd() tea.Cmd {
	action := m.pendingAction
	target := m.selectedTarget
	var payload map[string]string
	if action == ActionCreate || action == ActionUpdate {
		payload = collectFormValues(m.formInputs)
	}
	m.pendingAction = ActionNone
	return m.actionCmd(action, target, payload)
}

func collectFormValues(inputs []textinput.Model) map[string]string {
	values := map[string]string{}
	for _, input := range inputs {
		key := strings.TrimSpace(strings.TrimSuffix(input.Prompt, ": "))
		values[strings.ToLower(strings.ReplaceAll(key, " ", "_"))] = strings.TrimSpace(input.Value())
	}
	return values
}

func validateInstanceForm(action Action, values map[string]string) map[int]string {
	errs := map[int]string{}
	name := values["name"]
	if name == "" {
		errs[0] = "name is required"
	}

	switch action {
	case ActionCreate:
		if values["image"] == "" {
			errs[1] = "image is required"
		}
		typ := values["type"]
		if typ != "container" && typ != "virtual-machine" {
			errs[2] = "type must be container or virtual-machine"
		}
	case ActionUpdate:
		if values["config_key"] == "" {
			errs[1] = "config key is required"
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
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

func (m Model) showingDetail() bool {
	return m.detailView.Active()
}

func (m *Model) resizeDetailView(width, height int) {
	m.detailView.SetSize(max(24, width), max(6, height))
}

func (m Model) OverlayActive() bool {
	return m.formOpen || m.confirming || m.showingDetail()
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

func nextInputIndex(key string, current, total int) int {
	delta := 1
	if key == "shift+tab" || key == "up" {
		delta = -1
	}
	return (current + delta + total) % total
}

func setFocusedInput(inputs []textinput.Model, index int) {
	for i := range inputs {
		if i == index {
			inputs[i].Focus()
			continue
		}
		inputs[i].Blur()
	}
}

func defaultTableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		Bold(true).
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("238")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true)
	s.Selected = s.Selected.Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Bold(true)
	s.Cell = s.Cell.Padding(0, 1)
	return s
}

func renderPanel(title, body string, borderColor lipgloss.Color) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(title) != "" {
		parts = append(parts, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Render(title))
	}
	parts = append(parts, body)
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(strings.Join(parts, "\n\n"))
}

func renderStatusBar(status string) string {
	color := lipgloss.Color("241")
	lower := strings.ToLower(status)
	switch {
	case strings.Contains(lower, "failed"), strings.Contains(lower, "error"):
		color = lipgloss.Color("196")
	case strings.Contains(lower, "completed"), strings.Contains(lower, "loaded"), strings.Contains(lower, "viewing"), strings.Contains(lower, "back to list"):
		color = lipgloss.Color("42")
	case strings.Contains(lower, "loading"), strings.Contains(lower, "confirm"), strings.Contains(lower, "refreshing"), strings.Contains(lower, "starting"), strings.Contains(lower, "stopping"):
		color = lipgloss.Color("214")
	}

	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true).
		Render(status)
}

func (m Model) renderMeta() string {
	selected := m.currentName()
	if selected == "" {
		selected = "-"
	}
	return fmt.Sprintf("Rows %d | Selected %s | Busy %t", len(m.table.Rows()), selected, m.busy)
}
