package app

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/table"
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

type sectionLoadedMsg struct {
	section Section
	payload tablePayload
	err     error
}

type Model struct {
	instances instances.Model
	svc       client.InstanceService
	timeout   time.Duration
	active    int
	table     table.Model
	status    map[Section]string
	cache     map[Section]tablePayload
	loading   bool
	loaded    map[Section]bool
	viewportW int
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
	return tea.Batch(m.instances.Init(), m.refreshSectionCmd(m.currentSection()))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewportW = msg.Width
		m.table.SetWidth(max(40, msg.Width-24))
	case tea.KeyMsg:
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
	help := "[j/k or ↑/↓] navigate [r] refresh [q] quit"
	if m.loading {
		help = "Loading..."
	}
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(m.status[m.currentSection()])
	content := fmt.Sprintf("%s\n\n%s\n\n%s\n%s", title, m.table.View(), help, status)
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
		items, err := m.svc.ListImages(ctx)
		if err != nil {
			return tablePayload{}, err
		}
		rows := make([]table.Row, 0, len(items))
		for _, item := range items {
			rows = append(rows, table.Row{shortFingerprint(item.Fingerprint), item.Type, item.Architecture, humanSize(item.Size), item.UploadedAt.Format(time.RFC3339)})
		}
		return tablePayload{columns: []table.Column{{Title: "Fingerprint", Width: 18}, {Title: "Type", Width: 12}, {Title: "Arch", Width: 10}, {Title: "Size", Width: 12}, {Title: "Uploaded", Width: 22}}, rows: rows, status: fmt.Sprintf("Loaded %d images", len(rows))}, nil
	case SectionStorage:
		items, err := m.svc.ListStoragePools(ctx)
		if err != nil {
			return tablePayload{}, err
		}
		rows := make([]table.Row, 0, len(items))
		for _, item := range items {
			rows = append(rows, table.Row{item.Name, item.Driver, item.Status, strconv.Itoa(item.UsedBy)})
		}
		return tablePayload{columns: []table.Column{{Title: "Name", Width: 24}, {Title: "Driver", Width: 12}, {Title: "Status", Width: 12}, {Title: "UsedBy", Width: 10}}, rows: rows, status: fmt.Sprintf("Loaded %d storage pools", len(rows))}, nil
	case SectionNetworks:
		items, err := m.svc.ListNetworks(ctx)
		if err != nil {
			return tablePayload{}, err
		}
		rows := make([]table.Row, 0, len(items))
		for _, item := range items {
			rows = append(rows, table.Row{item.Name, item.Type, strconv.FormatBool(item.Managed), item.Status, strconv.Itoa(item.UsedBy)})
		}
		return tablePayload{columns: []table.Column{{Title: "Name", Width: 24}, {Title: "Type", Width: 12}, {Title: "Managed", Width: 10}, {Title: "Status", Width: 12}, {Title: "UsedBy", Width: 10}}, rows: rows, status: fmt.Sprintf("Loaded %d networks", len(rows))}, nil
	case SectionProfiles:
		items, err := m.svc.ListProfiles(ctx)
		if err != nil {
			return tablePayload{}, err
		}
		rows := make([]table.Row, 0, len(items))
		for _, item := range items {
			rows = append(rows, table.Row{item.Name, item.Project, strconv.Itoa(item.UsedBy)})
		}
		return tablePayload{columns: []table.Column{{Title: "Name", Width: 24}, {Title: "Project", Width: 18}, {Title: "UsedBy", Width: 10}}, rows: rows, status: fmt.Sprintf("Loaded %d profiles", len(rows))}, nil
	case SectionProjects:
		items, err := m.svc.ListProjects(ctx)
		if err != nil {
			return tablePayload{}, err
		}
		rows := make([]table.Row, 0, len(items))
		for _, item := range items {
			rows = append(rows, table.Row{item.Name, truncateText(item.Description, 48), strconv.Itoa(item.UsedBy)})
		}
		return tablePayload{columns: []table.Column{{Title: "Name", Width: 24}, {Title: "Description", Width: 48}, {Title: "UsedBy", Width: 10}}, rows: rows, status: fmt.Sprintf("Loaded %d projects", len(rows))}, nil
	case SectionCluster:
		items, err := m.svc.ListClusterMembers(ctx)
		if err != nil {
			return tablePayload{}, err
		}
		rows := make([]table.Row, 0, len(items))
		for _, item := range items {
			rows = append(rows, table.Row{item.Name, item.Status, truncateText(item.Message, 40), truncateText(item.URL, 40)})
		}
		return tablePayload{columns: []table.Column{{Title: "Name", Width: 22}, {Title: "Status", Width: 12}, {Title: "Message", Width: 42}, {Title: "URL", Width: 42}}, rows: rows, status: fmt.Sprintf("Loaded %d cluster members", len(rows))}, nil
	case SectionOperations:
		items, err := m.svc.ListOperations(ctx)
		if err != nil {
			return tablePayload{}, err
		}
		rows := make([]table.Row, 0, len(items))
		for _, item := range items {
			rows = append(rows, table.Row{shortFingerprint(item.ID), item.Class, item.Status, truncateText(item.Description, 36), item.CreatedAt.Format(time.RFC3339)})
		}
		return tablePayload{columns: []table.Column{{Title: "ID", Width: 18}, {Title: "Class", Width: 12}, {Title: "Status", Width: 12}, {Title: "Description", Width: 38}, {Title: "Created", Width: 22}}, rows: rows, status: fmt.Sprintf("Loaded %d operations", len(rows))}, nil
	case SectionWarnings:
		items, err := m.svc.ListWarnings(ctx)
		if err != nil {
			return tablePayload{}, err
		}
		rows := make([]table.Row, 0, len(items))
		for _, item := range items {
			rows = append(rows, table.Row{shortFingerprint(item.UUID), item.Severity, item.Type, item.Project, strconv.Itoa(item.Count), item.LastSeenAt.Format(time.RFC3339), truncateText(item.Message, 24)})
		}
		return tablePayload{columns: []table.Column{{Title: "UUID", Width: 14}, {Title: "Severity", Width: 10}, {Title: "Type", Width: 18}, {Title: "Project", Width: 14}, {Title: "Count", Width: 8}, {Title: "LastSeen", Width: 22}, {Title: "Message", Width: 26}}, rows: rows, status: fmt.Sprintf("Loaded %d warnings", len(rows))}, nil
	default:
		return tablePayload{}, fmt.Errorf("unsupported section %s", section)
	}
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
