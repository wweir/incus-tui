package app

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/example/incus-tui/internal/client"
)

type noopService struct{}

func (noopService) ListInstances(context.Context) ([]client.Instance, error)           { return nil, nil }
func (noopService) CreateInstance(context.Context, string, string, string) error       { return nil }
func (noopService) UpdateInstanceConfig(context.Context, string, string, string) error { return nil }
func (noopService) StartInstance(context.Context, string) error                        { return nil }
func (noopService) StopInstance(context.Context, string) error                         { return nil }
func (noopService) DeleteInstance(context.Context, string) error                       { return nil }
func (noopService) ListImages(context.Context) ([]client.Image, error)                 { return nil, nil }
func (noopService) ListStoragePools(context.Context) ([]client.StoragePool, error)     { return nil, nil }
func (noopService) ListNetworks(context.Context) ([]client.Network, error)             { return nil, nil }
func (noopService) ListProfiles(context.Context) ([]client.Profile, error)             { return nil, nil }
func (noopService) ListProjects(context.Context) ([]client.Project, error)             { return nil, nil }
func (noopService) ListClusterMembers(context.Context) ([]client.ClusterMember, error) {
	return nil, nil
}
func (noopService) ListOperations(context.Context) ([]client.Operation, error)   { return nil, nil }
func (noopService) ListWarnings(context.Context) ([]client.Warning, error)       { return nil, nil }
func (noopService) CreateResource(context.Context, string, string, string) error { return nil }
func (noopService) UpdateResource(context.Context, string, string, string) error { return nil }
func (noopService) DeleteResource(context.Context, string, string) error         { return nil }
func (noopService) WaitForEvent(context.Context) (string, error)                 { return "", context.Canceled }

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
	m := Model{switching: true, status: map[Section]string{SectionInstances: "Ready"}}
	updated, cmd := m.handleSwitchInput(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatalf("expected nil cmd on cancel")
	}
	next := updated.(Model)
	if next.switching {
		t.Fatalf("switching should be false after esc")
	}
}
