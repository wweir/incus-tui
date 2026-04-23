package app

import (
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/example/incus-tui/internal/client"
	"github.com/example/incus-tui/internal/modules/instances"
)

type noopService struct{}

func (noopService) ListInstances(context.Context) ([]client.Instance, error)           { return nil, nil }
func (noopService) CreateInstance(context.Context, string, string, string) error       { return nil }
func (noopService) UpdateInstanceConfig(context.Context, string, string, string) error { return nil }
func (noopService) StartInstance(context.Context, string) error                        { return nil }
func (noopService) StopInstance(context.Context, string) error                         { return nil }
func (noopService) DeleteInstance(context.Context, string) error                       { return nil }
func (noopService) GetInstanceDetail(context.Context, string) (client.ResourceDetail, error) {
	return client.ResourceDetail{}, nil
}
func (noopService) ListImages(context.Context) ([]client.Image, error)             { return nil, nil }
func (noopService) ListStoragePools(context.Context) ([]client.StoragePool, error) { return nil, nil }
func (noopService) ListNetworks(context.Context) ([]client.Network, error)         { return nil, nil }
func (noopService) ListProfiles(context.Context) ([]client.Profile, error)         { return nil, nil }
func (noopService) ListProjects(context.Context) ([]client.Project, error)         { return nil, nil }
func (noopService) ListClusterMembers(context.Context) ([]client.ClusterMember, error) {
	return nil, nil
}
func (noopService) ListOperations(context.Context) ([]client.Operation, error)          { return nil, nil }
func (noopService) ListWarnings(context.Context) ([]client.Warning, error)              { return nil, nil }
func (noopService) CreateResource(context.Context, string, client.ResourceValues) error { return nil }
func (noopService) UpdateResource(context.Context, string, client.ResourceValues) error { return nil }
func (noopService) DeleteResource(context.Context, string, string) error                { return nil }
func (noopService) GetResourceDetail(context.Context, string, string) (client.ResourceDetail, error) {
	return client.ResourceDetail{}, nil
}
func (noopService) WaitForEvent(context.Context) (string, error) { return "", context.Canceled }

func TestSwitchContextCmd(t *testing.T) {
	factory := func(remote, project string) (client.InstanceService, error) {
		return noopService{}, nil
	}

	m := Model{newService: factory, remote: "r1", project: "p1"}
	cmd := m.switchContextCmd("remote", "r2")
	msg := cmd()
	switched, ok := msg.(contextSwitchedMsg)
	if !ok {
		t.Fatalf("switchContextCmd() message type=%T", msg)
	}
	if switched.remote != "r2" || switched.project != "p1" {
		t.Fatalf("switchContextCmd() remote/project = %q/%q", switched.remote, switched.project)
	}
}

func TestRenderContext(t *testing.T) {
	if got := renderContext(""); got != "default" {
		t.Fatalf("renderContext(empty) = %q", got)
	}
	if got := renderContext("prod"); got != "prod" {
		t.Fatalf("renderContext(prod) = %q", got)
	}
}

func TestHandleSwitchInputCancel(t *testing.T) {
	m := Model{mode: modeSwitchInput, status: map[Section]string{SectionInstances: "Ready"}}
	updated, cmd := m.handleSwitchInput(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatalf("expected nil cmd on cancel")
	}
	next := updated.(Model)
	if next.mode != modeNormal {
		t.Fatalf("mode after esc = %v, want normal", next.mode)
	}
}

func TestModePreventsOverlappingSwitchAndFormStates(t *testing.T) {
	m := New(noopService{}, 0, instances.New(noopService{}, 0), nil, "", "")
	m.active = 1
	m.startSwitchInput("remote")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	next := updated.(Model)
	if next.mode != modeSwitchInput {
		t.Fatalf("mode = %v, want switch input", next.mode)
	}
	if len(next.form) != 0 {
		t.Fatalf("form should not open while switch input mode is active")
	}
}

func TestHandleMouseSidebarSwitchesSection(t *testing.T) {
	m := New(noopService{}, 0, instances.New(noopService{}, 0), nil, "", "")
	updated, cmd := m.handleMouse(tea.MouseMsg{
		X:      1,
		Y:      sidebarFirstY + 1,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if cmd == nil {
		t.Fatalf("expected refresh cmd for unloaded section")
	}
	next := updated.(Model)
	if got := next.currentSection(); got != SectionImages {
		t.Fatalf("current section = %s, want %s", got, SectionImages)
	}
	if next.focus != focusSidebar {
		t.Fatalf("focus = %v, want sidebar", next.focus)
	}
}

func TestHandleMouseTableWheelMovesSelection(t *testing.T) {
	m := New(noopService{}, 0, instances.New(noopService{}, 0), nil, "", "")
	m.active = 1
	m.loaded[SectionImages] = true
	m.cache[SectionImages] = tablePayload{
		columns: nil,
		rows:    nil,
	}
	m.table.SetColumns([]table.Column{{Title: "Name", Width: 8}})
	m.table.SetRows([]table.Row{{"a"}, {"b"}, {"c"}, {"d"}})

	updated, cmd := m.handleMouse(tea.MouseMsg{
		X:      sidebarWidth + 1,
		Button: tea.MouseButtonWheelDown,
	})
	if cmd != nil {
		t.Fatalf("expected nil cmd")
	}
	next := updated.(Model)
	if got := next.table.Cursor(); got != 3 {
		t.Fatalf("cursor after wheel down = %d, want 3", got)
	}
}

func TestTabTogglesBetweenContentAndSidebarFocus(t *testing.T) {
	m := New(noopService{}, 0, instances.New(noopService{}, 0), nil, "", "")
	m.active = 2

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd != nil {
		t.Fatalf("expected nil cmd")
	}
	next := updated.(Model)
	if next.focus != focusSidebar {
		t.Fatalf("focus after tab = %v, want sidebar", next.focus)
	}
	if next.sidebarCursor != 2 {
		t.Fatalf("sidebar cursor = %d, want active section 2", next.sidebarCursor)
	}

	updated, cmd = next.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd != nil {
		t.Fatalf("expected nil cmd")
	}
	next = updated.(Model)
	if next.focus != focusContent {
		t.Fatalf("focus after second tab = %v, want content", next.focus)
	}
}

func TestSidebarFocusMovesCursorWithoutActivatingSection(t *testing.T) {
	m := New(noopService{}, 0, instances.New(noopService{}, 0), nil, "", "")
	m.focus = focusSidebar

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Fatalf("expected nil cmd while moving sidebar cursor")
	}
	next := updated.(Model)
	if next.sidebarCursor != 1 {
		t.Fatalf("sidebar cursor = %d, want 1", next.sidebarCursor)
	}
	if next.currentSection() != SectionInstances {
		t.Fatalf("current section = %s, want %s", next.currentSection(), SectionInstances)
	}
}

func TestSidebarEnterActivatesCursorSection(t *testing.T) {
	m := New(noopService{}, 0, instances.New(noopService{}, 0), nil, "", "")
	m.focus = focusSidebar
	m.sidebarCursor = 1

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected refresh cmd for unloaded section")
	}
	next := updated.(Model)
	if next.currentSection() != SectionImages {
		t.Fatalf("current section = %s, want %s", next.currentSection(), SectionImages)
	}
}

func TestHorizontalFocusUsesActiveSectionWhenReturningToSidebar(t *testing.T) {
	m := New(noopService{}, 0, instances.New(noopService{}, 0), nil, "", "")
	m.active = 3
	m.sidebarCursor = 1

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if cmd != nil {
		t.Fatalf("expected nil cmd")
	}

	next := updated.(Model)
	if next.focus != focusSidebar {
		t.Fatalf("focus after left = %v, want sidebar", next.focus)
	}
	if next.sidebarCursor != next.active {
		t.Fatalf("sidebar cursor = %d, want active section %d", next.sidebarCursor, next.active)
	}
	if next.currentSection() != SectionNetworks {
		t.Fatalf("current section = %s, want %s", next.currentSection(), SectionNetworks)
	}
}

func TestHorizontalFocusSyncsInstancesTableFocus(t *testing.T) {
	m := New(noopService{}, 0, instances.New(noopService{}, 0), nil, "", "")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if cmd != nil {
		t.Fatalf("expected nil cmd")
	}

	next := updated.(Model)
	if next.instances.Focused() {
		t.Fatalf("instances table should be blurred when sidebar is focused")
	}

	updated, cmd = next.Update(tea.KeyMsg{Type: tea.KeyRight})
	if cmd != nil {
		t.Fatalf("expected nil cmd")
	}

	next = updated.(Model)
	if next.focus != focusContent {
		t.Fatalf("focus after right = %v, want content", next.focus)
	}
	if !next.instances.Focused() {
		t.Fatalf("instances table should be focused when content is focused")
	}
}

func TestSidebarRightActivatesCursorSectionAndMovesFocusToContent(t *testing.T) {
	m := New(noopService{}, 0, instances.New(noopService{}, 0), nil, "", "")
	m.focus = focusSidebar
	m.active = 0
	m.sidebarCursor = 2

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if cmd == nil {
		t.Fatalf("expected refresh cmd for unloaded section")
	}

	next := updated.(Model)
	if next.focus != focusContent {
		t.Fatalf("focus after right = %v, want content", next.focus)
	}
	if next.currentSection() != SectionStorage {
		t.Fatalf("current section = %s, want %s", next.currentSection(), SectionStorage)
	}
	if next.sidebarCursor != 2 {
		t.Fatalf("sidebar cursor = %d, want 2", next.sidebarCursor)
	}
}

func TestSectionLoadedReplacesTableWithoutMismatchedColumnPanic(t *testing.T) {
	m := New(noopService{}, 0, instances.New(noopService{}, 0), nil, "", "")
	m.active = 1
	m.table.SetColumns([]table.Column{
		{Title: "A", Width: 8},
		{Title: "B", Width: 8},
		{Title: "C", Width: 8},
		{Title: "D", Width: 8},
	})
	m.table.SetRows([]table.Row{{"a", "b", "c", "d"}})

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Update panicked while replacing table payload: %v", recovered)
		}
	}()

	_, _ = m.Update(sectionLoadedMsg{
		section: SectionImages,
		payload: tablePayload{
			columns: []table.Column{
				{Title: "Name", Width: 8},
				{Title: "Project", Width: 8},
				{Title: "UsedBy", Width: 8},
			},
			rows: []table.Row{{"default", "", "0"}},
		},
	})
}

func TestNormalizeTableRowsMatchesColumnCount(t *testing.T) {
	columns := []table.Column{
		{Title: "A", Width: 8},
		{Title: "B", Width: 8},
		{Title: "C", Width: 8},
	}

	rows := normalizeTableRows(columns, []table.Row{
		{"a", "b", "c", "d"},
		{"x"},
	})

	if len(rows) != 2 {
		t.Fatalf("normalized row count = %d, want 2", len(rows))
	}
	for index, row := range rows {
		if len(row) != len(columns) {
			t.Fatalf("row %d length = %d, want %d", index, len(row), len(columns))
		}
	}
	if rows[0][2] != "c" {
		t.Fatalf("long row was not preserved before truncation boundary: %q", rows[0][2])
	}
	if rows[1][1] != "" || rows[1][2] != "" {
		t.Fatalf("short row was not padded with empty cells: %#v", rows[1])
	}
}
