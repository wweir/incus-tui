package instances

import "testing"

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
