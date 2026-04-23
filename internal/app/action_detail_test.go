package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/example/incus-tui/internal/client"
	"github.com/example/incus-tui/internal/modules/instances"
)

type recordingService struct {
	detailSection string
	detailTarget  string
	updateSection string
	updateTarget  string
	updateValue   string
}

func (*recordingService) ListInstances(context.Context) ([]client.Instance, error) { return nil, nil }
func (*recordingService) CreateInstance(context.Context, string, string, string) error {
	return nil
}
func (*recordingService) UpdateInstanceConfig(context.Context, string, string, string) error {
	return nil
}
func (*recordingService) StartInstance(context.Context, string) error  { return nil }
func (*recordingService) StopInstance(context.Context, string) error   { return nil }
func (*recordingService) DeleteInstance(context.Context, string) error { return nil }
func (*recordingService) GetInstanceDetail(context.Context, string) (client.ResourceDetail, error) {
	return client.ResourceDetail{}, nil
}
func (*recordingService) ListImages(context.Context) ([]client.Image, error) { return nil, nil }
func (*recordingService) ListStoragePools(context.Context) ([]client.StoragePool, error) {
	return nil, nil
}
func (*recordingService) ListNetworks(context.Context) ([]client.Network, error) { return nil, nil }
func (*recordingService) ListProfiles(context.Context) ([]client.Profile, error) { return nil, nil }
func (*recordingService) ListProjects(context.Context) ([]client.Project, error) { return nil, nil }
func (*recordingService) ListClusterMembers(context.Context) ([]client.ClusterMember, error) {
	return nil, nil
}
func (*recordingService) ListOperations(context.Context) ([]client.Operation, error) { return nil, nil }
func (*recordingService) ListWarnings(context.Context) ([]client.Warning, error)     { return nil, nil }
func (*recordingService) CreateResource(context.Context, string, client.ResourceValues) error {
	return nil
}
func (s *recordingService) UpdateResource(_ context.Context, section string, values client.ResourceValues) error {
	s.updateSection = section
	s.updateTarget = values.Get("name")
	s.updateValue = values.Get("status")
	return nil
}
func (*recordingService) DeleteResource(context.Context, string, string) error { return nil }
func (s *recordingService) GetResourceDetail(_ context.Context, section, name string) (client.ResourceDetail, error) {
	s.detailSection = section
	s.detailTarget = name
	return client.ResourceDetail{
		Title: "detail",
		Fields: []client.DetailField{
			{Label: "Name", Value: name},
		},
	}, nil
}
func (*recordingService) WaitForEvent(context.Context) (string, error) { return "", context.Canceled }

func TestInitSectionFormUsesFullKeyForTruncatedRows(t *testing.T) {
	svc := &recordingService{}
	m := New(svc, time.Second, instances.New(svc, time.Second), nil, "", "")
	m.active = 1
	payload := tablePayload{
		columns: []table.Column{{Title: "Fingerprint", Width: 18}},
		rows:    []table.Row{{"1234567890ab"}},
		keys:    []string{"1234567890abcdef1234567890abcdef"},
	}
	m.cache[SectionImages] = payload
	m.loaded[SectionImages] = true
	m.setTablePayload(payload)

	m.initSectionForm("d")

	if got := m.form[0].Value(); got != "1234567890abcdef1234567890abcdef" {
		t.Fatalf("delete target = %q, want full fingerprint", got)
	}
}

func TestEnterLoadsDetailWithFullResourceKey(t *testing.T) {
	svc := &recordingService{}
	m := New(svc, time.Second, instances.New(svc, time.Second), nil, "", "")
	m.active = 1
	payload := tablePayload{
		columns: []table.Column{{Title: "Fingerprint", Width: 18}},
		rows:    []table.Row{{"1234567890ab"}},
		keys:    []string{"1234567890abcdef1234567890abcdef"},
	}
	m.cache[SectionImages] = payload
	m.loaded[SectionImages] = true
	m.setTablePayload(payload)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected detail cmd")
	}
	next := updated.(Model)
	if !next.loading {
		t.Fatalf("loading = false, want true")
	}

	msg := cmd()
	detailMsg, ok := msg.(detailLoadedMsg)
	if !ok {
		t.Fatalf("detail message type = %T", msg)
	}
	if detailMsg.target != "1234567890abcdef1234567890abcdef" {
		t.Fatalf("detail target = %q", detailMsg.target)
	}

	updated, _ = next.Update(detailMsg)
	next = updated.(Model)
	if next.mode != modeDetail {
		t.Fatalf("mode = %v, want detail", next.mode)
	}
	if svc.detailSection != string(SectionImages) || svc.detailTarget != "1234567890abcdef1234567890abcdef" {
		t.Fatalf("detail call = %s %s", svc.detailSection, svc.detailTarget)
	}
}

func TestCreateOnImagesOpensStructuredForm(t *testing.T) {
	svc := &recordingService{}
	m := New(svc, time.Second, instances.New(svc, time.Second), nil, "", "")
	m.active = 1

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd != nil {
		t.Fatalf("expected nil cmd")
	}
	next := updated.(Model)
	if next.mode != modeForm {
		t.Fatalf("mode = %v, want form", next.mode)
	}
	if len(next.form) != 6 {
		t.Fatalf("form length = %d, want 6", len(next.form))
	}
}

func TestSectionActionCmdRoutesWarningUpdate(t *testing.T) {
	svc := &recordingService{}
	m := New(svc, time.Second, instances.New(svc, time.Second), nil, "", "")

	msg := m.sectionActionCmd(SectionWarnings, "update", "warn-uuid", client.ResourceValues{
		"name":   "warn-uuid",
		"status": "acknowledged",
	})()
	done, ok := msg.(sectionActionDoneMsg)
	if !ok {
		t.Fatalf("action message type = %T", msg)
	}
	if done.err != nil {
		t.Fatalf("action err = %v", done.err)
	}
	if svc.updateSection != string(SectionWarnings) || svc.updateTarget != "warn-uuid" || svc.updateValue != "acknowledged" {
		t.Fatalf("update call = %s %s %s", svc.updateSection, svc.updateTarget, svc.updateValue)
	}
}

func TestDetailModeScrollsWithCompactViewport(t *testing.T) {
	svc := &recordingService{}
	m := New(svc, time.Second, instances.New(svc, time.Second), nil, "", "")
	m.active = 1

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	next := updated.(Model)

	updated, _ = next.Update(detailLoadedMsg{
		section: SectionImages,
		target:  "img",
		detail: client.ResourceDetail{
			Title: "detail",
			Fields: []client.DetailField{
				{Label: "Properties", Value: strings.Repeat("line\n", 20)},
			},
		},
	})
	next = updated.(Model)
	if next.mode != modeDetail {
		t.Fatalf("mode = %v, want detail", next.mode)
	}

	updated, _ = next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	next = updated.(Model)
	if next.detailView.YOffset() == 0 {
		t.Fatalf("detail viewport offset = %d, want > 0", next.detailView.YOffset())
	}
}

func TestDetailModeRendersOverlayWithoutSidebar(t *testing.T) {
	svc := &recordingService{}
	m := New(svc, time.Second, instances.New(svc, time.Second), nil, "", "")
	m.active = 1

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	next := updated.(Model)
	updated, _ = next.Update(detailLoadedMsg{
		section: SectionImages,
		target:  "img",
		detail: client.ResourceDetail{
			Title: "Image detail",
			Fields: []client.DetailField{
				{Label: "Name", Value: "img"},
			},
		},
	})
	next = updated.(Model)

	view := next.View()
	if !strings.Contains(view, "Image detail") {
		t.Fatalf("overlay view missing detail title: %q", view)
	}
	if strings.Contains(view, "Incus TUI - Images") {
		t.Fatalf("overlay should not render list page title: %q", view)
	}
	if strings.Contains(view, "Mode:") {
		t.Fatalf("overlay should not render sidebar: %q", view)
	}
}
