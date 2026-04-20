package client

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNormalizeEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "https endpoint", input: "https://127.0.0.1:8443", want: "https://127.0.0.1:8443"},
		{name: "http endpoint", input: "http://localhost:8443", want: "http://localhost:8443"},
		{name: "empty", input: "", wantErr: true},
		{name: "remote name not supported", input: "local", wantErr: true},
		{name: "bad url", input: "https://:8443", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeEndpoint(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeEndpoint() err=%v wantErr=%v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("normalizeEndpoint()=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestRunWithContext(t *testing.T) {
	t.Run("context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		_, err := runWithContext(ctx, func() (string, error) {
			time.Sleep(100 * time.Millisecond)
			return "late", nil
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("runWithContext() err=%v, want deadline exceeded", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		got, err := runWithContext(ctx, func() (int, error) {
			return 7, nil
		})
		if err != nil {
			t.Fatalf("runWithContext() err=%v", err)
		}
		if got != 7 {
			t.Fatalf("runWithContext() got=%d, want=7", got)
		}
	})
}

func TestLoadCLIConfigWithMissingFile(t *testing.T) {
	confDir := filepath.Join(t.TempDir(), "incus")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	t.Setenv("INCUS_CONF", confDir)

	cfg, err := loadCLIConfig()
	if err != nil {
		t.Fatalf("loadCLIConfig() err=%v", err)
	}
	if cfg == nil {
		t.Fatalf("loadCLIConfig() cfg is nil")
	}
}
