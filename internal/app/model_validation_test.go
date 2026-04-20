package app

import "testing"

func TestValidateSectionForm(t *testing.T) {
	tests := []struct {
		name      string
		section   Section
		action    string
		fieldName string
		fieldVal  string
		wantErr   bool
	}{
		{name: "delete with name", section: SectionProfiles, action: "delete", fieldName: "p1", wantErr: false},
		{name: "delete missing name", section: SectionProfiles, action: "delete", fieldName: "", wantErr: true},
		{name: "storage create requires driver", section: SectionStorage, action: "create", fieldName: "pool1", fieldVal: "", wantErr: true},
		{name: "network create requires type", section: SectionNetworks, action: "create", fieldName: "net1", fieldVal: "", wantErr: true},
		{name: "images update blocked", section: SectionImages, action: "update", fieldName: "img", fieldVal: "x", wantErr: true},
		{name: "project update ok", section: SectionProjects, action: "update", fieldName: "p1", fieldVal: "desc", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateSectionForm(tt.section, tt.action, tt.fieldName, tt.fieldVal)
			gotErr := len(errs) > 0
			if gotErr != tt.wantErr {
				t.Fatalf("validateSectionForm() gotErr=%v wantErr=%v errs=%v", gotErr, tt.wantErr, errs)
			}
		})
	}
}
