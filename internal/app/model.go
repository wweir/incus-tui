package app

import (
	"context"
	"errors"
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
	"github.com/example/incus-tui/internal/tui/detailview"
	"github.com/example/incus-tui/internal/tui/overlay"
	"github.com/example/incus-tui/internal/tui/tableinput"
)

const (
	sidebarWidth   = 21
	sidebarFirstY  = 2
	tableFirstRowY = 4
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
	keys    []string
	status  string
}

type focusArea int

const (
	focusContent focusArea = iota
	focusSidebar
)

type appMode int

const (
	modeNormal appMode = iota
	modeForm
	modeConfirm
	modeSwitchInput
	modeDetail
)

type sectionTableSpec[T any] struct {
	columns      []table.Column
	loadedStatus string
	keyOf        func(item T) string
	toRow        func(item T) table.Row
}

type formFieldSpec struct {
	key         string
	prompt      string
	placeholder string
	value       string
}

type sectionFormSpec struct {
	title  string
	fields []formFieldSpec
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

type contextSwitchedMsg struct {
	svc     client.InstanceService
	remote  string
	project string
	err     error
}

type eventReceivedMsg struct {
	eventType string
	err       error
}

type detailLoadedMsg struct {
	section Section
	target  string
	detail  client.ResourceDetail
	err     error
}

type Model struct {
	instances     instances.Model
	svc           client.InstanceService
	newService    func(remote, project string) (client.InstanceService, error)
	timeout       time.Duration
	remote        string
	project       string
	active        int
	sidebarCursor int
	focus         focusArea
	mode          appMode
	table         table.Model
	status        map[Section]string
	cache         map[Section]tablePayload
	loading       bool
	loaded        map[Section]bool
	formTitle     string
	formIndex     int
	form          []textinput.Model
	formKeys      []string
	formErrors    map[int]string
	actionSection Section
	actionType    string
	actionTarget  string
	actionValues  client.ResourceValues
	switchKind    string
	switchInput   textinput.Model
	detailView    detailview.Model
	windowWidth   int
	windowHeight  int
}

func New(
	svc client.InstanceService,
	timeout time.Duration,
	instancesModel instances.Model,
	factory func(remote, project string) (client.InstanceService, error),
	remote string,
	project string,
) Model {
	t := table.New(table.WithFocused(true), table.WithHeight(16))
	t.SetStyles(defaultTableStyles())

	status := make(map[Section]string, len(orderedSections))
	for _, section := range orderedSections {
		status[section] = "Ready"
	}

	return Model{
		instances:  instancesModel,
		svc:        svc,
		newService: factory,
		timeout:    timeout,
		remote:     remote,
		project:    project,
		focus:      focusContent,
		table:      t,
		status:     status,
		cache:      make(map[Section]tablePayload, len(orderedSections)),
		loaded:     make(map[Section]bool, len(orderedSections)),
		detailView: detailview.New(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.instances.Init(), m.waitEventCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		m.table.SetWidth(max(40, msg.Width-24))
		m.resizeDetailView(msg.Width-12, msg.Height-12)
	case tea.MouseMsg:
		if m.mode == modeDetail {
			var cmd tea.Cmd
			m.detailView, cmd = m.detailView.Update(msg)
			return m, cmd
		}
		return m.handleMouse(msg)
	case tea.KeyMsg:
		switch m.mode {
		case modeSwitchInput:
			return m.handleSwitchInput(msg)
		case modeForm:
			return m.handleForm(msg)
		case modeConfirm:
			return m.handleConfirm(msg)
		case modeDetail:
			return m.handleDetail(msg)
		}
		updated, cmd, handled := m.handleNormalKey(msg)
		if handled {
			return updated, cmd
		}
		m = updated
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
			m.setTablePayload(msg.payload)
		}
		return m, nil
	case sectionActionDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.status[msg.section] = fmt.Sprintf("%s failed on %s: %v", titleWord(msg.action), msg.target, msg.err)
			return m, nil
		}
		m.status[msg.section] = fmt.Sprintf("%s completed: %s", titleWord(msg.action), msg.target)
		return m, m.refreshSectionCmd(msg.section)
	case contextSwitchedMsg:
		m.loading = false
		if msg.err != nil {
			m.status[m.currentSection()] = fmt.Sprintf("Switch failed: %v", msg.err)
			return m, nil
		}
		m.svc = msg.svc
		m.remote = msg.remote
		m.project = msg.project
		m.instances = instances.New(msg.svc, m.timeout)
		m.cache = make(map[Section]tablePayload, len(orderedSections))
		m.loaded = make(map[Section]bool, len(orderedSections))
		m.detailView.Clear()
		m.sidebarCursor = m.active
		if m.windowWidth > 0 && m.windowHeight > 0 {
			m.instances, _ = m.instances.Update(tea.WindowSizeMsg{Width: m.windowWidth, Height: m.windowHeight})
		}
		m.status[m.currentSection()] = fmt.Sprintf("Switched to remote=%q project=%q", renderContext(msg.remote), renderContext(msg.project))
		return m, tea.Batch(m.refreshSectionCmd(m.currentSection()), m.waitEventCmd())
	case detailLoadedMsg:
		m.loading = false
		if msg.section != m.currentSection() {
			return m, nil
		}
		if msg.err != nil {
			m.mode = modeNormal
			m.status[msg.section] = fmt.Sprintf("Detail failed on %s: %v", msg.target, msg.err)
			return m, nil
		}
		m.mode = modeDetail
		m.detailView.SetDetail(msg.detail)
		m.status[msg.section] = fmt.Sprintf("Viewing detail: %s", msg.target)
		return m, nil
	case eventReceivedMsg:
		if msg.err != nil {
			if !errors.Is(msg.err, context.Canceled) {
				m.status[m.currentSection()] = fmt.Sprintf("Monitor error: %v", msg.err)
			}
			return m, m.waitEventCmd()
		}
		if m.loading || m.mode != modeNormal {
			return m, m.waitEventCmd()
		}
		m.status[m.currentSection()] = fmt.Sprintf("Monitor event: %s", msg.eventType)
		return m, tea.Batch(m.refreshSectionCmd(m.currentSection()), m.waitEventCmd())
	}

	if m.focus == focusContent && m.currentSection() == SectionInstances {
		updated, cmd := m.instances.Update(msg)
		m.instances = updated
		return m, cmd
	}

	if m.focus != focusContent {
		return m, nil
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) handleNormalKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit, true
	case "O":
		m.startSwitchInput("remote")
		return m, nil, true
	case "P":
		m.startSwitchInput("project")
		return m, nil, true
	case "tab", "shift+tab":
		m.toggleFocus()
		return m, nil, true
	case "esc":
		m.setFocus(focusContent)
		return m, nil, true
	case "r":
		return m, m.refreshSectionCmd(m.currentSection()), true
	}

	if m.focus == focusSidebar {
		switch msg.String() {
		case "up", "k":
			if m.sidebarCursor > 0 {
				m.sidebarCursor--
			}
			return m, nil, true
		case "down", "j":
			if m.sidebarCursor < len(orderedSections)-1 {
				m.sidebarCursor++
			}
			return m, nil, true
		case "enter":
			return m, m.activateSection(m.sidebarCursor), true
		case "right", "l":
			cmd := m.activateSection(m.sidebarCursor)
			m.setFocus(focusContent)
			return m, cmd, true
		case "left", "h":
			return m, nil, true
		}
		return m, nil, true
	}

	switch msg.String() {
	case "left", "h":
		m.focusSidebar()
		return m, nil, true
	case "enter":
		if m.currentSection() == SectionInstances {
			return m, nil, false
		}
		target := m.currentRowKey()
		if target == "" {
			m.status[m.currentSection()] = "No row selected"
			return m, nil, true
		}
		m.loading = true
		return m, m.detailCmd(m.currentSection(), target), true
	case "c", "u", "d":
		if m.currentSection() == SectionInstances {
			return m, nil, false
		}
		if !supportsSectionAction(m.currentSection(), mapActionType(msg.String())) {
			m.status[m.currentSection()] = fmt.Sprintf("%s is not supported for %s", titleWord(mapActionType(msg.String())), m.currentSection())
			return m, nil, true
		}
		m.initSectionForm(msg.String())
		return m, nil, true
	}

	return m, nil, false
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeNormal {
		return m, nil
	}

	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && msg.X < sidebarWidth {
		next := msg.Y - sidebarFirstY
		if next >= 0 && next < len(orderedSections) {
			m.setFocus(focusSidebar)
			return m, m.activateSection(next)
		}
		return m, nil
	}

	if msg.X >= sidebarWidth {
		m.setFocus(focusContent)
	}

	if m.currentSection() == SectionInstances {
		updated, cmd := m.instances.Update(msg)
		m.instances = updated
		return m, cmd
	}

	if msg.X >= sidebarWidth && !m.loading {
		m.table = tableinput.HandleMouse(m.table, msg, tableFirstRowY)
	}
	return m, nil
}

func (m Model) View() string {
	if m.mode == modeSwitchInput {
		return overlay.RenderDialog(
			m.windowWidth,
			m.windowHeight,
			fmt.Sprintf("Switch %s context", titleWord(m.switchKind)),
			fmt.Sprintf("%s%s", m.switchInput.Prompt, m.switchInput.View()),
			"[enter] apply [esc] cancel",
			renderStatusBar(m.status[m.currentSection()]),
			lipgloss.Color("62"),
		)
	}

	if m.currentSection() == SectionInstances && m.instances.OverlayActive() {
		return m.instances.View()
	}

	if m.mode == modeForm {
		return overlay.RenderScreen(
			m.windowWidth,
			m.windowHeight,
			m.formTitle,
			lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(m.renderSectionMeta()),
			m.renderForm(),
			"[tab] next [enter] submit [esc] cancel",
			renderStatusBar(m.status[m.currentSection()]),
			lipgloss.Color("62"),
		)
	}

	if m.mode == modeDetail {
		return overlay.RenderScreen(
			m.windowWidth,
			m.windowHeight,
			m.detailView.Title(),
			lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(m.renderSectionMeta()),
			m.detailView.View(),
			m.detailView.HelpText("r refresh"),
			renderStatusBar(m.status[m.currentSection()]),
			lipgloss.Color("99"),
		)
	}

	if m.mode == modeConfirm {
		return overlay.RenderDialog(
			m.windowWidth,
			m.windowHeight,
			"Confirm action",
			fmt.Sprintf("%s %s\n\nProceed with this change?", titleWord(m.actionType), m.actionTarget),
			"[y] yes [n] no [esc] cancel",
			renderStatusBar(m.status[m.currentSection()]),
			lipgloss.Color("214"),
		)
	}

	sidebar := m.renderSidebar()
	body := m.renderBody()
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, body)
}

func (m Model) currentSection() Section {
	return orderedSections[m.active]
}

func (m *Model) toggleFocus() {
	if m.focus == focusSidebar {
		m.setFocus(focusContent)
		return
	}
	m.focusSidebar()
}

func (m *Model) setFocus(focus focusArea) {
	m.focus = focus
	if focus == focusContent {
		m.table.Focus()
		m.instances.Focus()
		return
	}
	m.table.Blur()
	m.instances.Blur()
}

func (m *Model) focusSidebar() {
	m.sidebarCursor = m.active
	m.setFocus(focusSidebar)
}

func (m *Model) activateSection(index int) tea.Cmd {
	if index < 0 || index >= len(orderedSections) {
		return nil
	}
	m.sidebarCursor = index
	m.mode = modeNormal
	m.detailView.Clear()
	section := orderedSections[index]
	if m.active == index {
		if section != SectionInstances && !m.loaded[section] {
			return m.refreshSectionCmd(section)
		}
		return nil
	}
	m.active = index
	if !m.loaded[section] {
		return m.refreshSectionCmd(section)
	}
	m.applySectionCache()
	return nil
}

func (m *Model) applySectionCache() {
	payload, ok := m.cache[m.currentSection()]
	if !ok {
		m.setTablePayload(tablePayload{})
		return
	}
	m.setTablePayload(payload)
}

func (m *Model) setTablePayload(payload tablePayload) {
	m.table.SetRows(nil)
	m.table.SetColumns(payload.columns)
	m.table.SetRows(normalizeTableRows(payload.columns, payload.rows))
}

func normalizeTableRows(columns []table.Column, rows []table.Row) []table.Row {
	columnCount := len(columns)
	if columnCount == 0 || len(rows) == 0 {
		return nil
	}

	normalized := make([]table.Row, 0, len(rows))
	for _, row := range rows {
		next := make(table.Row, columnCount)
		copy(next, row)
		normalized = append(normalized, next)
	}
	return normalized
}

func (m Model) renderSidebar() string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Padding(0, 1).Render("Incus TUI")
	items := make([]string, 0, len(orderedSections))
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("238")).Bold(true).Padding(0, 1)
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Bold(true).Padding(0, 1)
	for idx, section := range orderedSections {
		marker := "  "
		if idx == m.active {
			marker = "* "
		}
		if m.focus == focusSidebar && idx == m.sidebarCursor {
			marker = "> "
		}
		label := fmt.Sprintf("%s%s", marker, section)
		switch {
		case m.focus == focusSidebar && idx == m.sidebarCursor:
			label = cursorStyle.Render(label)
		case idx == m.active:
			label = activeStyle.Render(label)
		}
		items = append(items, label)
	}
	footerText := "[tab] sidebar [O] remote [P] project"
	if m.focus == focusSidebar {
		footerText = "[j/k] move [enter] open [tab] content"
	}
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(0, 1).Render(footerText)
	ctxLine := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Padding(0, 1).Render(
		fmt.Sprintf("R:%s P:%s", renderContext(m.remote), renderContext(m.project)),
	)
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("109")).Padding(0, 1).Render(fmt.Sprintf("Mode: %s", renderModeLabel(m.mode)))

	content := lipgloss.JoinVertical(lipgloss.Left, append([]string{header, ""}, append(items, "", ctxLine, status, footer)...)...)
	return lipgloss.NewStyle().
		Width(20).
		Padding(0, 0, 0, 0).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("238")).
		BorderRight(true).
		Render(content)
}

func (m Model) renderBody() string {
	if m.currentSection() == SectionInstances {
		return lipgloss.NewStyle().Padding(0, 1).Render(m.instances.View())
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Render(fmt.Sprintf("Incus TUI - %s", m.currentSection()))
	help := renderSectionHelp(m.currentSection())
	if m.focus == focusSidebar {
		help = "[tab] focus content [j/k or ↑/↓] move section [enter] open [q] quit"
	}
	body := renderPanel("", m.table.View(), lipgloss.Color("238"))
	if m.loading {
		help = "Loading..."
	}
	meta := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(m.renderSectionMeta())
	status := renderStatusBar(m.status[m.currentSection()])
	content := fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n%s", title, meta, body, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(help), status)
	return lipgloss.NewStyle().Padding(0, 1).Render(content)
}

func (m *Model) startSwitchInput(kind string) {
	m.mode = modeSwitchInput
	m.switchKind = kind
	value := m.remote
	if kind == "project" {
		value = m.project
	}
	m.switchInput = newTextInput("Value: ", fmt.Sprintf("new %s", kind), value)
	m.switchInput.Focus()
	m.status[m.currentSection()] = fmt.Sprintf("Switch %s then press enter", kind)
}

func (m Model) handleSwitchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.status[m.currentSection()] = "Context switch cancelled"
		return m, nil
	case "enter":
		m.mode = modeNormal
		m.loading = true
		value := strings.TrimSpace(m.switchInput.Value())
		return m, m.switchContextCmd(m.switchKind, value)
	}
	var cmd tea.Cmd
	m.switchInput, cmd = m.switchInput.Update(msg)
	return m, cmd
}

func (m Model) renderSwitchInput() string {
	title := fmt.Sprintf("Switch %s context", titleWord(m.switchKind))
	return fmt.Sprintf("%s\n\n%s%s", lipgloss.NewStyle().Bold(true).Render(title), m.switchInput.Prompt, m.switchInput.View())
}

func (m Model) handleDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "enter":
		m.mode = modeNormal
		m.detailView.Clear()
		m.status[m.currentSection()] = "Back to list"
		return m, nil
	case "r":
		m.mode = modeNormal
		m.detailView.Clear()
		return m, m.refreshSectionCmd(m.currentSection())
	}
	var cmd tea.Cmd
	m.detailView, cmd = m.detailView.Update(msg)
	return m, cmd
}

func (m Model) handleConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.mode = modeNormal
		m.loading = true
		m.status[m.actionSection] = fmt.Sprintf("%s %s...", titleWord(m.actionType), m.actionTarget)
		return m, m.sectionActionCmd(m.actionSection, m.actionType, m.actionTarget, m.actionValues)
	case "n", "N", "esc":
		m.mode = modeNormal
		m.actionValues = nil
		m.status[m.currentSection()] = "Action cancelled"
		return m, nil
	}
	return m, nil
}

func (m Model) switchContextCmd(kind, value string) tea.Cmd {
	return func() tea.Msg {
		nextRemote, nextProject := m.remote, m.project
		if kind == "remote" {
			nextRemote = value
		} else {
			nextProject = value
		}

		svc, err := m.newService(nextRemote, nextProject)
		return contextSwitchedMsg{svc: svc, remote: nextRemote, project: nextProject, err: err}
	}
}

func (m Model) waitEventCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		eventType, err := m.svc.WaitForEvent(ctx)
		return eventReceivedMsg{eventType: eventType, err: err}
	}
}

func (m Model) detailCmd(section Section, target string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
		defer cancel()

		detail, err := m.svc.GetResourceDetail(ctx, string(section), target)
		return detailLoadedMsg{section: section, target: target, detail: detail, err: err}
	}
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
			keyOf: func(item client.Image) string {
				return item.Fingerprint
			},
			toRow: func(item client.Image) table.Row {
				return table.Row{shortFingerprint(item.Fingerprint), item.Type, item.Architecture, humanSize(item.Size), item.UploadedAt.Format(time.RFC3339)}
			},
		})
	case SectionStorage:
		return buildSectionPayload(ctx, m.svc.ListStoragePools, sectionTableSpec[client.StoragePool]{
			columns:      []table.Column{{Title: "Name", Width: 24}, {Title: "Driver", Width: 12}, {Title: "Status", Width: 12}, {Title: "UsedBy", Width: 10}},
			loadedStatus: "Loaded %d storage pools",
			keyOf: func(item client.StoragePool) string {
				return item.Name
			},
			toRow: func(item client.StoragePool) table.Row {
				return table.Row{item.Name, item.Driver, item.Status, strconv.Itoa(item.UsedBy)}
			},
		})
	case SectionNetworks:
		return buildSectionPayload(ctx, m.svc.ListNetworks, sectionTableSpec[client.Network]{
			columns:      []table.Column{{Title: "Name", Width: 24}, {Title: "Type", Width: 12}, {Title: "Managed", Width: 10}, {Title: "Status", Width: 12}, {Title: "UsedBy", Width: 10}},
			loadedStatus: "Loaded %d networks",
			keyOf: func(item client.Network) string {
				return item.Name
			},
			toRow: func(item client.Network) table.Row {
				return table.Row{item.Name, item.Type, strconv.FormatBool(item.Managed), item.Status, strconv.Itoa(item.UsedBy)}
			},
		})
	case SectionProfiles:
		return buildSectionPayload(ctx, m.svc.ListProfiles, sectionTableSpec[client.Profile]{
			columns:      []table.Column{{Title: "Name", Width: 24}, {Title: "Project", Width: 18}, {Title: "UsedBy", Width: 10}},
			loadedStatus: "Loaded %d profiles",
			keyOf: func(item client.Profile) string {
				return item.Name
			},
			toRow: func(item client.Profile) table.Row {
				return table.Row{item.Name, item.Project, strconv.Itoa(item.UsedBy)}
			},
		})
	case SectionProjects:
		return buildSectionPayload(ctx, m.svc.ListProjects, sectionTableSpec[client.Project]{
			columns:      []table.Column{{Title: "Name", Width: 24}, {Title: "Description", Width: 48}, {Title: "UsedBy", Width: 10}},
			loadedStatus: "Loaded %d projects",
			keyOf: func(item client.Project) string {
				return item.Name
			},
			toRow: func(item client.Project) table.Row {
				return table.Row{item.Name, truncateText(item.Description, 48), strconv.Itoa(item.UsedBy)}
			},
		})
	case SectionCluster:
		return buildSectionPayload(ctx, m.svc.ListClusterMembers, sectionTableSpec[client.ClusterMember]{
			columns:      []table.Column{{Title: "Name", Width: 22}, {Title: "Status", Width: 12}, {Title: "Message", Width: 42}, {Title: "URL", Width: 42}},
			loadedStatus: "Loaded %d cluster members",
			keyOf: func(item client.ClusterMember) string {
				return item.Name
			},
			toRow: func(item client.ClusterMember) table.Row {
				return table.Row{item.Name, item.Status, truncateText(item.Message, 40), truncateText(item.URL, 40)}
			},
		})
	case SectionOperations:
		return buildSectionPayload(ctx, m.svc.ListOperations, sectionTableSpec[client.Operation]{
			columns:      []table.Column{{Title: "ID", Width: 18}, {Title: "Class", Width: 12}, {Title: "Status", Width: 12}, {Title: "Description", Width: 38}, {Title: "Created", Width: 22}},
			loadedStatus: "Loaded %d operations",
			keyOf: func(item client.Operation) string {
				return item.ID
			},
			toRow: func(item client.Operation) table.Row {
				return table.Row{shortFingerprint(item.ID), item.Class, item.Status, truncateText(item.Description, 36), item.CreatedAt.Format(time.RFC3339)}
			},
		})
	case SectionWarnings:
		return buildSectionPayload(ctx, m.svc.ListWarnings, sectionTableSpec[client.Warning]{
			columns:      []table.Column{{Title: "UUID", Width: 14}, {Title: "Severity", Width: 10}, {Title: "Type", Width: 18}, {Title: "Project", Width: 14}, {Title: "Count", Width: 8}, {Title: "LastSeen", Width: 22}, {Title: "Message", Width: 26}},
			loadedStatus: "Loaded %d warnings",
			keyOf: func(item client.Warning) string {
				return item.UUID
			},
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
		m.mode = modeNormal
		m.actionValues = nil
		m.status[m.currentSection()] = "Form cancelled"
		return m, nil
	case "tab", "shift+tab", "up", "down":
		m.formIndex = nextInputIndex(msg.String(), m.formIndex, len(m.form))
		setFocusedInput(m.form, m.formIndex)
		return m, nil
	case "enter":
		section := m.currentSection()
		values := collectNamedFormValues(m.form, m.formKeys)
		if errs := validateSectionForm(section, m.actionType, values); len(errs) > 0 {
			m.formErrors = indexFormErrors(errs, m.formKeys)
			m.status[section] = "Please fix invalid fields"
			return m, nil
		}
		m.formErrors = nil
		m.mode = modeConfirm
		m.actionSection = section
		m.actionValues = values
		m.actionTarget = describeActionTarget(section, m.actionType, values)
		m.status[section] = fmt.Sprintf("Confirm %s %s? (y/n)", m.actionType, m.actionTarget)
		return m, nil
	}
	var cmd tea.Cmd
	m.form[m.formIndex], cmd = m.form[m.formIndex].Update(msg)
	return m, cmd
}

func (m *Model) initSectionForm(action string) {
	m.actionType = mapActionType(action)
	target := m.currentRowKey()
	spec := buildSectionFormSpec(m.currentSection(), m.actionType, target)
	m.formTitle = spec.title
	m.form = make([]textinput.Model, 0, len(spec.fields))
	m.formKeys = make([]string, 0, len(spec.fields))
	for _, field := range spec.fields {
		m.form = append(m.form, newTextInput(field.prompt, field.placeholder, field.value))
		m.formKeys = append(m.formKeys, field.key)
	}
	m.mode = modeForm
	m.formErrors = nil
	m.formIndex = 0
	m.actionValues = nil
	setFocusedInput(m.form, 0)
	m.status[m.currentSection()] = "Fill form and press enter"
}

func (m Model) sectionActionCmd(section Section, action, target string, values client.ResourceValues) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
		defer cancel()
		var err error
		switch action {
		case "create":
			err = m.svc.CreateResource(ctx, string(section), values)
		case "update":
			err = m.svc.UpdateResource(ctx, string(section), values)
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
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Render(m.formTitle))
	b.WriteString("\n\n")
	for i, input := range m.form {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("109")).Bold(true).Render(strings.TrimSpace(input.Prompt)))
		b.WriteString("\n")
		b.WriteString(input.View())
		if errText, ok := m.formErrors[i]; ok {
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("  ! " + errText))
		}
		if i < len(m.form)-1 {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

func (m Model) renderSectionMeta() string {
	selected := m.currentRowKey()
	if selected == "" {
		selected = "-"
	}

	rows := 0
	if payload, ok := m.cache[m.currentSection()]; ok {
		rows = len(payload.rows)
	}
	return fmt.Sprintf("Context %s/%s | Rows %d | Selected %s | Focus %s", renderContext(m.remote), renderContext(m.project), rows, selected, renderFocusLabel(m.focus))
}

func (m Model) currentRowValue(index int) string {
	row := m.table.SelectedRow()
	if len(row) <= index {
		return ""
	}
	return row[index]
}

func (m Model) currentRowKey() string {
	payload, ok := m.cache[m.currentSection()]
	if !ok || len(payload.keys) == 0 {
		return m.currentRowValue(0)
	}

	cursor := m.table.Cursor()
	if cursor < 0 || cursor >= len(payload.keys) {
		return ""
	}
	return payload.keys[cursor]
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
	keys := make([]string, 0, len(items))
	for _, item := range items {
		row := spec.toRow(item)
		rows = append(rows, row)
		if spec.keyOf != nil {
			keys = append(keys, spec.keyOf(item))
			continue
		}
		if len(row) == 0 {
			keys = append(keys, "")
			continue
		}
		keys = append(keys, row[0])
	}

	return tablePayload{
		columns: spec.columns,
		rows:    rows,
		keys:    keys,
		status:  fmt.Sprintf(spec.loadedStatus, len(rows)),
	}, nil
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

func collectNamedFormValues(inputs []textinput.Model, keys []string) client.ResourceValues {
	values := make(client.ResourceValues, len(keys))
	for index, key := range keys {
		if index >= len(inputs) {
			break
		}
		values[key] = strings.TrimSpace(inputs[index].Value())
	}
	return values
}

func indexFormErrors(errs map[string]string, keys []string) map[int]string {
	if len(errs) == 0 {
		return nil
	}

	indices := make(map[int]string, len(errs))
	for index, key := range keys {
		errText, ok := errs[key]
		if !ok {
			continue
		}
		indices[index] = errText
	}
	return indices
}

func mapActionType(key string) string {
	switch key {
	case "c":
		return "create"
	case "u":
		return "update"
	case "d":
		return "delete"
	default:
		return ""
	}
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

func titleWord(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func renderContext(value string) string {
	if strings.TrimSpace(value) == "" {
		return "default"
	}
	return value
}

func renderModeLabel(mode appMode) string {
	switch mode {
	case modeForm:
		return "FORM"
	case modeConfirm:
		return "CONFIRM"
	case modeSwitchInput:
		return "SWITCH"
	case modeDetail:
		return "DETAIL"
	default:
		return "BROWSE"
	}
}

func renderFocusLabel(focus focusArea) string {
	if focus == focusSidebar {
		return "SIDEBAR"
	}
	return "CONTENT"
}

func (m *Model) resizeDetailView(width, height int) {
	m.detailView.SetSize(max(24, width), max(6, height))
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
	case strings.Contains(lower, "completed"), strings.Contains(lower, "loaded"), strings.Contains(lower, "viewing"), strings.Contains(lower, "switched"):
		color = lipgloss.Color("42")
	case strings.Contains(lower, "loading"), strings.Contains(lower, "confirm"), strings.Contains(lower, "monitor"):
		color = lipgloss.Color("214")
	}

	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true).
		Render(status)
}

func renderSectionHelp(section Section) string {
	actions := make([]string, 0, 3)
	for _, candidate := range []struct {
		key    string
		action string
	}{
		{key: "c", action: "create"},
		{key: "u", action: "update"},
		{key: "d", action: "delete"},
	} {
		if supportsSectionAction(section, candidate.action) {
			actions = append(actions, fmt.Sprintf("[%s] %s", candidate.key, candidate.action))
		}
	}

	writeHelp := ""
	if len(actions) > 0 {
		writeHelp = " " + strings.Join(actions, " ")
	}
	return fmt.Sprintf("[tab] focus sidebar [j/k or ↑/↓] navigate [enter] detail [r] refresh%s [q] quit", writeHelp)
}

func supportsSectionAction(section Section, action string) bool {
	switch section {
	case SectionImages:
		return action == "create" || action == "update" || action == "delete"
	case SectionStorage, SectionNetworks, SectionProfiles, SectionProjects:
		return action == "create" || action == "update" || action == "delete"
	case SectionCluster:
		return action == "update" || action == "delete"
	case SectionWarnings:
		return action == "update" || action == "delete"
	case SectionOperations:
		return action == "delete"
	default:
		return false
	}
}

func buildSectionFormSpec(section Section, action, target string) sectionFormSpec {
	title := fmt.Sprintf("%s %s", titleWord(action), section)
	targetField := formFieldSpec{key: "name", prompt: "Name: ", placeholder: "resource name", value: target}

	switch section {
	case SectionImages:
		switch action {
		case "create":
			return sectionFormSpec{
				title: "Create Images",
				fields: []formFieldSpec{
					{key: "server", prompt: "Remote server: ", placeholder: "https://images.linuxcontainers.org", value: "https://images.linuxcontainers.org"},
					{key: "protocol", prompt: "Protocol: ", placeholder: "simplestreams or incus", value: "simplestreams"},
					{key: "alias", prompt: "Remote alias: ", placeholder: "e.g. alpine/edge", value: ""},
					{key: "local_alias", prompt: "Local alias: ", placeholder: "optional local alias", value: ""},
					{key: "public", prompt: "Public: ", placeholder: "true or false", value: "false"},
					{key: "auto_update", prompt: "Auto update: ", placeholder: "true or false", value: "false"},
				},
			}
		case "update":
			return sectionFormSpec{
				title: "Update Images",
				fields: []formFieldSpec{
					{key: "name", prompt: "Fingerprint: ", placeholder: "image fingerprint", value: target},
					{key: "public", prompt: "Public: ", placeholder: "keep | true | false", value: "keep"},
					{key: "auto_update", prompt: "Auto update: ", placeholder: "keep | true | false", value: "keep"},
					{key: "profiles", prompt: "Profiles: ", placeholder: "keep | comma separated | empty clears", value: "keep"},
				},
			}
		case "delete":
			return sectionFormSpec{
				title:  "Delete Images",
				fields: []formFieldSpec{{key: "name", prompt: "Fingerprint: ", placeholder: "image fingerprint", value: target}},
			}
		}
	case SectionStorage:
		switch action {
		case "create":
			return sectionFormSpec{
				title: "Create Storage",
				fields: []formFieldSpec{
					targetField,
					{key: "driver", prompt: "Driver: ", placeholder: "zfs | dir | btrfs | lvm ...", value: ""},
					{key: "description", prompt: "Description: ", placeholder: "optional description", value: ""},
				},
			}
		case "update":
			return sectionFormSpec{
				title: "Update Storage",
				fields: []formFieldSpec{
					targetField,
					{key: "description", prompt: "Description: ", placeholder: "keep | new description | empty clears", value: "keep"},
				},
			}
		}
	case SectionNetworks:
		switch action {
		case "create":
			return sectionFormSpec{
				title: "Create Networks",
				fields: []formFieldSpec{
					targetField,
					{key: "type", prompt: "Type: ", placeholder: "bridge | macvlan | ovs ...", value: ""},
					{key: "description", prompt: "Description: ", placeholder: "optional description", value: ""},
				},
			}
		case "update":
			return sectionFormSpec{
				title: "Update Networks",
				fields: []formFieldSpec{
					targetField,
					{key: "description", prompt: "Description: ", placeholder: "keep | new description | empty clears", value: "keep"},
				},
			}
		}
	case SectionProfiles:
		switch action {
		case "create":
			return sectionFormSpec{
				title: "Create Profiles",
				fields: []formFieldSpec{
					targetField,
					{key: "description", prompt: "Description: ", placeholder: "optional description", value: ""},
				},
			}
		case "update":
			return sectionFormSpec{
				title: "Update Profiles",
				fields: []formFieldSpec{
					targetField,
					{key: "description", prompt: "Description: ", placeholder: "keep | new description | empty clears", value: "keep"},
				},
			}
		}
	case SectionProjects:
		switch action {
		case "create":
			return sectionFormSpec{
				title: "Create Projects",
				fields: []formFieldSpec{
					targetField,
					{key: "description", prompt: "Description: ", placeholder: "optional description", value: ""},
				},
			}
		case "update":
			return sectionFormSpec{
				title: "Update Projects",
				fields: []formFieldSpec{
					targetField,
					{key: "description", prompt: "Description: ", placeholder: "keep | new description | empty clears", value: "keep"},
				},
			}
		}
	case SectionCluster:
		switch action {
		case "update":
			return sectionFormSpec{
				title: "Update Cluster",
				fields: []formFieldSpec{
					targetField,
					{key: "description", prompt: "Description: ", placeholder: "keep | new description | empty clears", value: "keep"},
					{key: "failure_domain", prompt: "Failure domain: ", placeholder: "keep | rack-1 | empty clears", value: "keep"},
					{key: "groups", prompt: "Groups: ", placeholder: "keep | comma separated | empty clears", value: "keep"},
					{key: "roles", prompt: "Roles: ", placeholder: "keep | comma separated | empty clears", value: "keep"},
				},
			}
		case "delete":
			return sectionFormSpec{
				title:  "Delete Cluster",
				fields: []formFieldSpec{targetField},
			}
		}
	case SectionOperations:
		if action == "delete" {
			return sectionFormSpec{
				title:  "Delete Operations",
				fields: []formFieldSpec{{key: "name", prompt: "Operation ID: ", placeholder: "operation id", value: target}},
			}
		}
	case SectionWarnings:
		switch action {
		case "update":
			return sectionFormSpec{
				title: "Update Warnings",
				fields: []formFieldSpec{
					{key: "name", prompt: "UUID: ", placeholder: "warning uuid", value: target},
					{key: "status", prompt: "Status: ", placeholder: "new | acknowledged | resolved", value: "acknowledged"},
				},
			}
		case "delete":
			return sectionFormSpec{
				title:  "Delete Warnings",
				fields: []formFieldSpec{{key: "name", prompt: "UUID: ", placeholder: "warning uuid", value: target}},
			}
		}
	}

	return sectionFormSpec{title: title, fields: []formFieldSpec{targetField}}
}

func describeActionTarget(section Section, action string, values client.ResourceValues) string {
	switch {
	case values.Get("name") != "":
		return values.Get("name")
	case section == SectionImages && action == "create":
		if alias := values.Get("local_alias"); alias != "" {
			return alias
		}
		if alias := values.Get("alias"); alias != "" {
			return alias
		}
	}
	return "resource"
}

func validateSectionForm(section Section, action string, values client.ResourceValues) map[string]string {
	errs := map[string]string{}
	if !supportsSectionAction(section, action) {
		errs["name"] = fmt.Sprintf("%s is not supported for this module", action)
		return errs
	}

	switch section {
	case SectionImages:
		if action == "create" {
			if values.Get("server") == "" {
				errs["server"] = "remote server is required"
			}
			if values.Get("alias") == "" {
				errs["alias"] = "remote alias is required"
			}
			if _, err := parseStrictBool(values.Get("public")); err != nil {
				errs["public"] = err.Error()
			}
			if _, err := parseStrictBool(values.Get("auto_update")); err != nil {
				errs["auto_update"] = err.Error()
			}
			break
		}
		if values.Get("name") == "" {
			errs["name"] = "fingerprint is required"
		}
		if values.Get("public") != "" && !strings.EqualFold(values.Get("public"), "keep") {
			if _, err := parseStrictBool(values.Get("public")); err != nil {
				errs["public"] = "public must be keep, true or false"
			}
		}
		if values.Get("auto_update") != "" && !strings.EqualFold(values.Get("auto_update"), "keep") {
			if _, err := parseStrictBool(values.Get("auto_update")); err != nil {
				errs["auto_update"] = "auto update must be keep, true or false"
			}
		}
		if action == "delete" {
			break
		}
	case SectionStorage:
		if values.Get("name") == "" {
			errs["name"] = "name is required"
		}
		if action == "create" && values.Get("driver") == "" {
			errs["driver"] = "driver is required"
		}
	case SectionNetworks:
		if values.Get("name") == "" {
			errs["name"] = "name is required"
		}
		if action == "create" && values.Get("type") == "" {
			errs["type"] = "network type is required"
		}
	case SectionProfiles, SectionProjects, SectionCluster, SectionOperations:
		if values.Get("name") == "" {
			errs["name"] = "name is required"
		}
	case SectionWarnings:
		if values.Get("name") == "" {
			errs["name"] = "uuid is required"
		}
		if action == "update" {
			normalized := values.Get("status")
			if normalized != "new" && normalized != "acknowledged" && normalized != "resolved" {
				errs["status"] = "status must be new, acknowledged or resolved"
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

func parseStrictBool(value string) (bool, error) {
	switch strings.TrimSpace(value) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("value must be true or false")
	}
}
