package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/example/incus-tui/internal/modules/instances"
)

type Model struct {
	instances instances.Model
}

func New(instancesModel instances.Model) Model {
	return Model{instances: instancesModel}
}

func (m Model) Init() tea.Cmd {
	return m.instances.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "q" || keyMsg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	updated, cmd := m.instances.Update(msg)
	m.instances = updated
	return m, cmd
}

func (m Model) View() string {
	return m.instances.View()
}
