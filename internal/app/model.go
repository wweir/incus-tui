package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/example/incus-tui/internal/client"
	"github.com/example/incus-tui/internal/modules/instances"
)

type Section string

const (
	SectionInstances  Section = "Instances"
	SectionImages     Section = "Images"
	SectionStorage    Section = "Storage"
	SectionNetworks   Section = "Networks"
	SectionProfiles   Section = "Profiles"
	SectionProjects   Section = "Projects"
	SectionCluster    Section = "Cluster"
	SectionOperations Section = "Operations"
	SectionWarnings   Section = "Warnings"
)

var orderedSections = []Section{
	SectionInstances,
	SectionImages,
	SectionStorage,
	SectionNetworks,
	SectionProfiles,
	SectionProjects,
	SectionCluster,
	SectionOperations,
	SectionWarnings,
}

type tablePayload struct {
	columns []table.Column
	rows    []table.Row
	status  string
}

type sectionTableSpec[T any] struct {
	columns      []table.Column
	loadedStatus string
	toRow        func(item T) table.Row
}

type sectionLoadedMsg struct {
	section Section
	payload tablePayload
	err     error
}

type sectionActionDoneMsg struct {
	section Section
	action  string
	target  string
	err     error
}

type Model struct {
	instances     instances.Model
	svc           client.InstanceService
	timeout       time.Duration
	active        int
	table         table.Model
	status        map[Section]string
	cache         map[Section]tablePayload
	loading       bool
	loaded        map[Section]bool
	viewportW     int
	formOpen      bool
	formTitle     string
	formIndex     int
	form          []textinput.Model
	confirming    bool
	actionSection Section
	actionType    string
	actionTarget  string
}

func New(svc client.InstanceService, timeout time.Duration, instancesModel instances.Model) Model {
	t := table.New(table.WithFocused(true), table.WithHeight(16))
	t.SetStyles(defaultTableStyles())

	status := make(map[Section]string, len(orderedSections))
	for _, section := range orderedSections {
		status[section] = "Ready"
	}

	return Model{
		instances: instancesModel,
		svc:       svc,
		timeout:   timeout,
		table:     t,
		status:    status,
		cache:     make(map[Section]tablePayload, len(orderedSections)),
		loaded:    make(map[Section]bool, len(orderedSections)),
	}
}

func (m Model) Init() tea.Cmd {
	return m.instances.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewportW = msg.Width
		m.table.SetWidth(max(40, msg.Width-24))
	case tea.KeyMsg:
		if m.formOpen {
			return m.handleForm(msg)
		}
		if m.confirming {
			switch msg.String() {
			case "y", "Y":
				m.confirming = false
				m.loading = true
				m.status[m.actionSection] = fmt.Sprintf("%s %s...", strings.Title(m.actionType), m.actionTarget)
				return m, m.sectionActionCmd(m.actionSection, m.actionType, m.actionTarget, m.formValue(1))
			case "n", "N", "esc":
				m.confirming = false
				m.status[m.currentSection()] = "Action cancelled"
				return m, nil
			}
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "left", "h":
			if m.active > 0 {
				m.active--
			}
			if !m.loaded[m.currentSection()] {
				return m, m.refreshSectionCmd(m.currentSection())
			}
			m.applySectionCache()
			return m, nil
		case "right", "l", "tab":
			if m.active < len(orderedSections)-1 {
				m.active++
			}
			if !m.loaded[m.currentSection()] {
				return m, m.refreshSectionCmd(m.currentSection())
			}
			m.applySectionCache()
			return m, nil
		case "r":
			return m, m.refreshSectionCmd(m.currentSection())
		case "c", "u", "d":
			if m.currentSection() == SectionInstances {
				break
			}
			m.initSectionForm(msg.String())
			return m, nil
		}
	case sectionLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.status[msg.section] = fmt.Sprintf("Load failed: %v", msg.err)
			return m, nil
		}
		m.cache[msg.section] = msg.payload
		m.loaded[msg.section] = true
		m.status[msg.section] = msg.payload.status
		if msg.section == m.currentSection() {
			m.table.SetColumns(msg.payload.columns)
			m.table.SetRows(msg.payload.rows)
		}
		return m, nil
	case sectionActionDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.status[msg.section] = fmt.Sprintf("%s failed on %s: %v", strings.Title(msg.action), msg.target, msg.err)
			return m, nil
		}
		m.status[msg.section] = fmt.Sprintf("%s completed: %s", strings.Title(msg.action), msg.target)
		return m, m.refreshSectionCmd(msg.section)
	}

	if m.currentSection() == SectionInstances {
		updated, cmd := m.instances.Update(msg)
		m.instances = updated
		return m, cmd
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	sidebar := m.renderSidebar()
	body := m.renderBody()
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, body)
}

func (m Model) currentSection() Section {
	return orderedSections[m.active]
}

func (m *Model) applySectionCache() {
	payload, ok := m.cache[m.currentSection()]
	if !ok {
		m.table.SetColumns([]table.Column{})
		m.table.SetRows([]table.Row{})
		return
	}
	m.table.SetColumns(payload.columns)
	m.table.SetRows(payload.rows)
}

func (m Model) renderSidebar() string {
	header := lipgloss.NewStyle().Bold(true).Padding(0, 1).Render("Incus TUI")
	items := make([]string, 0, len(orderedSections))
	for idx, section := range orderedSections {
		label := fmt.Sprintf(" %s", section)
		if idx == m.active {
			label = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Bold(true).Render(label)
		}
		items = append(items, label)
	}
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(0, 1).Render("[h/l|←/→|tab] 切换")

	content := lipgloss.JoinVertical(lipgloss.Left, append([]string{header, ""}, append(items, "", footer)...)...)
	return lipgloss.NewStyle().Width(20).BorderStyle(lipgloss.NormalBorder()).BorderRight(true).Render(content)
}

func (m Model) renderBody() string {
	if m.currentSection() == SectionInstances {
		return lipgloss.NewStyle().Padding(0, 1).Render(m.instances.View())
	}

	title := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Incus TUI - %s", m.currentSection()))
	help := "[j/k or ↑/↓] navigate [r] refresh [c/u/d] write [q] quit"
	body := m.table.View()
	if m.formOpen {
		body = m.renderForm()
		help = "[tab] next [enter] submit [esc] cancel"
	}
	if m.confirming {
		help = "Confirm action: [y] yes [n] no"
	}
	if m.loading {
		help = "Loading..."
	}
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(m.status[m.currentSection()])
	content := fmt.Sprintf("%s\n\n%s\n\n%s\n%s", title, body, help, status)
	return lipgloss.NewStyle().Padding(0, 1).Render(content)
}

func (m *Model) refreshSectionCmd(section Section) tea.Cmd {
	m.loading = true
	m.status[section] = "Loading..."

	if section == SectionInstances {
		return m.instances.Init()
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
		defer cancel()

		payload, err := m.loadTablePayload(ctx, section)
		return sectionLoadedMsg{section: section, payload: payload, err: err}
	}
}

func (m Model) loadTablePayload(ctx context.Context, section Section) (tablePayload, error) {
	switch section {
	case SectionImages:
		return buildSectionPayload(ctx, m.svc.ListImages, sectionTableSpec[client.Image]{
			columns:      []table.Column{{Title: "Fingerprint", Width: 18}, {Title: "Type", Width: 12}, {Title: "Arch", Width: 10}, {Title: "Size", Width: 12}, {Title: "Uploaded", Width: 22}},
			loadedStatus: "Loaded %d images",
			toRow: func(item client.Image) table.Row {
				return table.Row{shortFingerprint(item.Fingerprint), item.Type, item.Architecture, humanSize(item.Size), item.UploadedAt.Format(time.RFC3339)}
			},
		})
	case SectionStorage:
		return buildSectionPayload(ctx, m.svc.ListStoragePools, sectionTableSpec[client.StoragePool]{
			columns:      []table.Column{{Title: "Name", Width: 24}, {Title: "Driver", Width: 12}, {Title: "Status", Width: 12}, {Title: "UsedBy", Width: 10}},
			loadedStatus: "Loaded %d storage pools",
			toRow: func(item client.StoragePool) table.Row {
				return table.Row{item.Name, item.Driver, item.Status, strconv.Itoa(item.UsedBy)}
			},
		})
	case SectionNetworks:
		return buildSectionPayload(ctx, m.svc.ListNetworks, sectionTableSpec[client.Network]{
			columns:      []table.Column{{Title: "Name", Width: 24}, {Title: "Type", Width: 12}, {Title: "Managed", Width: 10}, {Title: "Status", Width: 12}, {Title: "UsedBy", Width: 10}},
			loadedStatus: "Loaded %d networks",
			toRow: func(item client.Network) table.Row {
				return table.Row{item.Name, item.Type, strconv.FormatBool(item.Managed), item.Status, strconv.Itoa(item.UsedBy)}
			},
		})
	case SectionProfiles:
		return buildSectionPayload(ctx, m.svc.ListProfiles, sectionTableSpec[client.Profile]{
			columns:      []table.Column{{Title: "Name", Width: 24}, {Title: "Project", Width: 18}, {Title: "UsedBy", Width: 10}},
			loadedStatus: "Loaded %d profiles",
			toRow: func(item client.Profile) table.Row {
				return table.Row{item.Name, item.Project, strconv.Itoa(item.UsedBy)}
			},
		})
	case SectionProjects:
		return buildSectionPayload(ctx, m.svc.ListProjects, sectionTableSpec[client.Project]{
			columns:      []table.Column{{Title: "Name", Width: 24}, {Title: "Description", Width: 48}, {Title: "UsedBy", Width: 10}},
			loadedStatus: "Loaded %d projects",
			toRow: func(item client.Project) table.Row {
				return table.Row{item.Name, truncateText(item.Description, 48), strconv.Itoa(item.UsedBy)}
			},
		})
	case SectionCluster:
		return buildSectionPayload(ctx, m.svc.ListClusterMembers, sectionTableSpec[client.ClusterMember]{
			columns:      []table.Column{{Title: "Name", Width: 22}, {Title: "Status", Width: 12}, {Title: "Message", Width: 42}, {Title: "URL", Width: 42}},
			loadedStatus: "Loaded %d cluster members",
			toRow: func(item client.ClusterMember) table.Row {
				return table.Row{item.Name, item.Status, truncateText(item.Message, 40), truncateText(item.URL, 40)}
			},
		})
	case SectionOperations:
		return buildSectionPayload(ctx, m.svc.ListOperations, sectionTableSpec[client.Operation]{
			columns:      []table.Column{{Title: "ID", Width: 18}, {Title: "Class", Width: 12}, {Title: "Status", Width: 12}, {Title: "Description", Width: 38}, {Title: "Created", Width: 22}},
			loadedStatus: "Loaded %d operations",
			toRow: func(item client.Operation) table.Row {
				return table.Row{shortFingerprint(item.ID), item.Class, item.Status, truncateText(item.Description, 36), item.CreatedAt.Format(time.RFC3339)}
			},
		})
	case SectionWarnings:
		return buildSectionPayload(ctx, m.svc.ListWarnings, sectionTableSpec[client.Warning]{
			columns:      []table.Column{{Title: "UUID", Width: 14}, {Title: "Severity", Width: 10}, {Title: "Type", Width: 18}, {Title: "Project", Width: 14}, {Title: "Count", Width: 8}, {Title: "LastSeen", Width: 22}, {Title: "Message", Width: 26}},
			loadedStatus: "Loaded %d warnings",
			toRow: func(item client.Warning) table.Row {
				return table.Row{shortFingerprint(item.UUID), item.Severity, item.Type, item.Project, strconv.Itoa(item.Count), item.LastSeenAt.Format(time.RFC3339), truncateText(item.Message, 24)}
			},
		})
	default:
		return tablePayload{}, fmt.Errorf("unsupported section %s", section)
	}
}

func (m Model) handleForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.formOpen = false
		m.status[m.currentSection()] = "Form cancelled"
		return m, nil
	case "tab", "shift+tab", "up", "down":
		delta := 1
		if msg.String() == "shift+tab" || msg.String() == "up" {
			delta = -1
		}
		m.formIndex = (m.formIndex + delta + len(m.form)) % len(m.form)
		for i := range m.form {
			if i == m.formIndex {
				m.form[i].Focus()
			} else {
				m.form[i].Blur()
			}
		}
		return m, nil
	case "enter":
		m.formOpen = false
		m.confirming = true
		m.actionSection = m.currentSection()
		m.actionTarget = m.formValue(0)
		m.status[m.currentSection()] = fmt.Sprintf("Confirm %s %s? (y/n)", m.actionType, m.actionTarget)
		return m, nil
	}
	var cmd tea.Cmd
	m.form[m.formIndex], cmd = m.form[m.formIndex].Update(msg)
	return m, cmd
}

func (m *Model) initSectionForm(action string) {
	m.actionType = map[string]string{"c": "create", "u": "update", "d": "delete"}[action]
	target := m.currentRowValue(0)
	secondPrompt := "Value: "
	secondPlaceholder := "description/type"
	switch m.currentSection() {
	case SectionImages, SectionOperations, SectionWarnings:
		secondPrompt = "Reserved: "
		secondPlaceholder = "leave empty"
	}
	m.formTitle = fmt.Sprintf("%s %s", strings.Title(m.actionType), m.currentSection())
	m.form = []textinput.Model{
		newTextInput("Name: ", "resource name", target),
		newTextInput(secondPrompt, secondPlaceholder, ""),
	}
	if m.actionType == "delete" {
		m.form = m.form[:1]
	}
	m.formOpen = true
	m.formIndex = 0
	for i := range m.form {
		if i == 0 {
			m.form[i].Focus()
		} else {
			m.form[i].Blur()
		}
	}
	m.status[m.currentSection()] = "Fill form and press enter"
}

func (m Model) sectionActionCmd(section Section, action, target, value string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
		defer cancel()
		var err error
		switch action {
		case "create":
			err = m.svc.CreateResource(ctx, string(section), target, value)
		case "update":
			err = m.svc.UpdateResource(ctx, string(section), target, value)
		case "delete":
			err = m.svc.DeleteResource(ctx, string(section), target)
		default:
			err = fmt.Errorf("unsupported action %s", action)
		}
		return sectionActionDoneMsg{section: section, action: action, target: target, err: err}
	}
}

func (m Model) renderForm() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(m.formTitle))
	b.WriteString("\n\n")
	for i, input := range m.form {
		b.WriteString(input.Prompt)
		b.WriteString(input.View())
		if i < len(m.form)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m Model) currentRowValue(index int) string {
	row := m.table.SelectedRow()
	if len(row) <= index {
		return ""
	}
	return row[index]
}

func (m Model) formValue(index int) string {
	if len(m.form) <= index {
		return ""
	}
	return strings.TrimSpace(m.form[index].Value())
}

func buildSectionPayload[T any](
	ctx context.Context,
	loader func(context.Context) ([]T, error),
	spec sectionTableSpec[T],
) (tablePayload, error) {
	items, err := loader(ctx)
	if err != nil {
		return tablePayload{}, err
	}

	rows := make([]table.Row, 0, len(items))
	for _, item := range items {
		rows = append(rows, spec.toRow(item))
	}

	return tablePayload{
		columns: spec.columns,
		rows:    rows,
		status:  fmt.Sprintf(spec.loadedStatus, len(rows)),
	}, nil
}

func defaultTableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true).BorderStyle(lipgloss.NormalBorder()).BorderBottom(true)
	s.Selected = s.Selected.Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Bold(true)
	return s
}

func shortFingerprint(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

func truncateText(value string, n int) string {
	if len(value) <= n {
		return value
	}
	if n <= 3 {
		return value[:n]
	}
	return value[:n-3] + "..."
}

func humanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%dB", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(size)/float64(div), "KMGTPE"[exp])
}

func newTextInput(prompt, placeholder, value string) textinput.Model {
	in := textinput.New()
	in.Prompt = prompt
	in.Placeholder = placeholder
	in.SetValue(value)
	in.Width = 48
	in.CharLimit = 256
	return in
}
