package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
}

type Model struct {
	instances instances.Model
	active    int
}

func New(instancesModel instances.Model) Model {
	return Model{instances: instancesModel}
}

func (m Model) Init() tea.Cmd {
	return m.instances.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "left", "h":
			if m.active > 0 {
				m.active--
			}
			return m, nil
		case "right", "l", "tab":
			if m.active < len(orderedSections)-1 {
				m.active++
			}
			return m, nil
		}
	}

	if m.currentSection() == SectionInstances {
		updated, cmd := m.instances.Update(msg)
		m.instances = updated
		return m, cmd
	}
	return m, nil
}

func (m Model) View() string {
	sidebar := m.renderSidebar()
	body := m.renderBody()
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, body)
}

func (m Model) currentSection() Section {
	return orderedSections[m.active]
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

	placeholder := lipgloss.NewStyle().Bold(true).Render(string(m.currentSection())) + "\n\n" +
		"该模块尚未实现，按 h/l 或方向键切换到 Instances。"
	return lipgloss.NewStyle().Padding(1, 2).Render(placeholder)
}
