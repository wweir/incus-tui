package instances

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/example/incus-tui/internal/client"
)

func TestValidateInstanceForm(t *testing.T) {
	tests := []struct {
		name    string
		action  Action
		values  map[string]string
		wantErr bool
	}{
		{
			name:   "create valid",
			action: ActionCreate,
			values: map[string]string{"name": "c1", "image": "images:alpine/edge", "type": "container"},
		},
		{
			name:    "create missing image",
			action:  ActionCreate,
			values:  map[string]string{"name": "c1", "type": "container"},
			wantErr: true,
		},
		{
			name:    "create invalid type",
			action:  ActionCreate,
			values:  map[string]string{"name": "c1", "image": "images:alpine/edge", "type": "vm"},
			wantErr: true,
		},
		{
			name:   "update valid",
			action: ActionUpdate,
			values: map[string]string{"name": "c1", "config_key": "limits.cpu", "config_value": "2"},
		},
		{
			name:    "update missing key",
			action:  ActionUpdate,
			values:  map[string]string{"name": "c1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateInstanceForm(tt.action, tt.values)
			gotErr := len(errs) > 0
			if gotErr != tt.wantErr {
				t.Fatalf("validateInstanceForm() gotErr=%v wantErr=%v errs=%v", gotErr, tt.wantErr, errs)
			}
		})
	}
}

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

func TestUpdateHandlesKeyboardNavigation(t *testing.T) {
	m := New(noopService{}, time.Second)
	updated, _ := m.Update(instancesLoadedMsg{items: []client.Instance{
		{Name: "a"},
		{Name: "b"},
	}})

	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := updated.table.Cursor(); got != 1 {
		t.Fatalf("cursor after key down = %d, want 1", got)
	}
}

func TestUpdateHandlesMouseNavigation(t *testing.T) {
	m := New(noopService{}, time.Second)
	updated, _ := m.Update(instancesLoadedMsg{items: []client.Instance{
		{Name: "a"},
		{Name: "b"},
	}})

	updated, _ = updated.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	if got := updated.table.Cursor(); got != 1 {
		t.Fatalf("cursor after wheel down = %d, want 1", got)
	}
}

type detailService struct {
	noopService
	target string
}

func (s *detailService) GetInstanceDetail(_ context.Context, name string) (client.ResourceDetail, error) {
	s.target = name
	return client.ResourceDetail{
		Title: "detail",
		Fields: []client.DetailField{
			{Label: "Name", Value: name},
		},
	}, nil
}

func TestEnterLoadsInstanceDetail(t *testing.T) {
	svc := &detailService{}
	m := New(svc, time.Second)
	updated, _ := m.Update(instancesLoadedMsg{items: []client.Instance{
		{Name: "a"},
	}})

	updated, cmd := updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected detail cmd")
	}

	msg := cmd()
	detailMsg, ok := msg.(instanceDetailLoadedMsg)
	if !ok {
		t.Fatalf("detail message type = %T", msg)
	}
	if detailMsg.target != "a" {
		t.Fatalf("detail target = %q", detailMsg.target)
	}

	updated, _ = updated.Update(detailMsg)
	if !updated.showingDetail() {
		t.Fatalf("detail view should be active")
	}
	if svc.target != "a" {
		t.Fatalf("service target = %q", svc.target)
	}
}

func TestDetailViewScrollsWithJKey(t *testing.T) {
	m := New(noopService{}, time.Second)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	updated, _ = updated.Update(instanceDetailLoadedMsg{
		target: "a",
		detail: client.ResourceDetail{
			Title: "detail",
			Fields: []client.DetailField{
				{Label: "Config", Value: strings.Repeat("line\n", 20)},
			},
		},
	})
	if !updated.showingDetail() {
		t.Fatalf("detail view should be active")
	}

	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if updated.detailView.YOffset() == 0 {
		t.Fatalf("detail viewport offset = %d, want > 0", updated.detailView.YOffset())
	}
}

func TestFormRendersAsOverlay(t *testing.T) {
	m := New(noopService{}, time.Second)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	updated.initCreateForm()

	view := updated.View()
	if !strings.Contains(view, "Create instance") {
		t.Fatalf("overlay view missing form title: %q", view)
	}
	if strings.Contains(view, "Incus TUI - Instances\nRows") {
		t.Fatalf("form overlay should not render list layout: %q", view)
	}
}
