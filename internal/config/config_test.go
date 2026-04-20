package config

import (
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "valid default",
			cfg:     Default(),
			wantErr: false,
		},
		{
			name: "invalid timeout <= 0",
			cfg: Config{
				CommandTimeout: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid timeout too long",
			cfg: Config{
				CommandTimeout: 6 * time.Minute,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
