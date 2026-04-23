package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestShouldElevateForLocalSocket(t *testing.T) {
	socketErr := &os.PathError{
		Op:   "dial",
		Path: "/var/lib/incus/unix.socket",
		Err:  os.ErrPermission,
	}

	t.Run("local default socket permission denied", func(t *testing.T) {
		t.Setenv(elevationAttemptedEnv, "")
		if os.Geteuid() == 0 {
			t.Skip("requires non-root user")
		}
		if !shouldElevateForLocalSocket("", fmt.Errorf("probe incus access: %w", socketErr)) {
			t.Fatalf("shouldElevateForLocalSocket() = false, want true")
		}
	})

	t.Run("explicit remote never elevates", func(t *testing.T) {
		t.Setenv(elevationAttemptedEnv, "")
		if shouldElevateForLocalSocket("https://127.0.0.1:8443", fmt.Errorf("probe incus access: %w", socketErr)) {
			t.Fatalf("shouldElevateForLocalSocket() = true, want false")
		}
	})

	t.Run("prevent elevation loop", func(t *testing.T) {
		t.Setenv(elevationAttemptedEnv, "1")
		if shouldElevateForLocalSocket("", fmt.Errorf("probe incus access: %w", socketErr)) {
			t.Fatalf("shouldElevateForLocalSocket() = true, want false")
		}
	})
}

func TestBuildElevationArgs(t *testing.T) {
	executable := filepath.Join(string(os.PathSeparator), "tmp", "incus-tui")
	argv := []string{"incus-tui", "--project", "demo"}

	sudoArgs := buildElevationArgs(executable, argv)
	if len(sudoArgs) != 4 {
		t.Fatalf("sudo args len = %d, want 4", len(sudoArgs))
	}
	if sudoArgs[0] != "--preserve-env=INCUS_CONF,"+elevationAttemptedEnv+","+elevationExecutableEnv || sudoArgs[1] != executable {
		t.Fatalf("sudo args prefix = %v", sudoArgs[:2])
	}
}

func TestResolveElevationExecutable(t *testing.T) {
	t.Run("uses configured absolute path", func(t *testing.T) {
		want := filepath.Join(string(os.PathSeparator), "tmp", "bin", "incus-tui")
		t.Setenv(elevationExecutableEnv, want)

		got, err := resolveElevationExecutable()
		if err != nil {
			t.Fatalf("resolveElevationExecutable() err = %v", err)
		}
		if got != want {
			t.Fatalf("resolveElevationExecutable() = %q, want %q", got, want)
		}
	})

	t.Run("normalizes configured relative path", func(t *testing.T) {
		t.Setenv(elevationExecutableEnv, filepath.Join("bin", "incus-tui"))

		got, err := resolveElevationExecutable()
		if err != nil {
			t.Fatalf("resolveElevationExecutable() err = %v", err)
		}
		if !filepath.IsAbs(got) {
			t.Fatalf("resolveElevationExecutable() = %q, want absolute path", got)
		}
		if filepath.Base(got) != "incus-tui" {
			t.Fatalf("resolveElevationExecutable() base = %q, want incus-tui", filepath.Base(got))
		}
	})
}
