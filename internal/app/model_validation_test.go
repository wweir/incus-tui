package app

import (
	"testing"

	"github.com/example/incus-tui/internal/client"
)

func TestValidateSectionForm(t *testing.T) {
	tests := []struct {
		name    string
		section Section
		action  string
		values  client.ResourceValues
		wantErr bool
	}{
		{name: "delete with name", section: SectionProfiles, action: "delete", values: client.ResourceValues{"name": "p1"}, wantErr: false},
		{name: "delete missing name", section: SectionProfiles, action: "delete", values: client.ResourceValues{}, wantErr: true},
		{name: "storage create requires driver", section: SectionStorage, action: "create", values: client.ResourceValues{"name": "pool1"}, wantErr: true},
		{name: "network create requires type", section: SectionNetworks, action: "create", values: client.ResourceValues{"name": "net1"}, wantErr: true},
		{name: "images create valid", section: SectionImages, action: "create", values: client.ResourceValues{"server": "https://images.linuxcontainers.org", "alias": "alpine/edge", "public": "false", "auto_update": "true"}, wantErr: false},
		{name: "images update invalid bool", section: SectionImages, action: "update", values: client.ResourceValues{"name": "img", "public": "x"}, wantErr: true},
		{name: "warnings update requires status", section: SectionWarnings, action: "update", values: client.ResourceValues{"name": "warn", "status": "foo"}, wantErr: true},
		{name: "warnings update valid", section: SectionWarnings, action: "update", values: client.ResourceValues{"name": "warn", "status": "acknowledged"}, wantErr: false},
		{name: "cluster create blocked", section: SectionCluster, action: "create", values: client.ResourceValues{"name": "member"}, wantErr: true},
		{name: "project update ok", section: SectionProjects, action: "update", values: client.ResourceValues{"name": "p1", "description": "keep"}, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateSectionForm(tt.section, tt.action, tt.values)
			gotErr := len(errs) > 0
			if gotErr != tt.wantErr {
				t.Fatalf("validateSectionForm() gotErr=%v wantErr=%v errs=%v", gotErr, tt.wantErr, errs)
			}
		})
	}
}
