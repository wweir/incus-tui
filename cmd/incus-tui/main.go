package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/example/incus-tui/internal/app"
	"github.com/example/incus-tui/internal/client"
	"github.com/example/incus-tui/internal/config"
	"github.com/example/incus-tui/internal/modules/instances"
)

var (
	version   = "dev"
	buildDate = "unknown"
)

const elevationAttemptedEnv = "INCUS_TUI_ELEVATION_ATTEMPTED"
const elevationExecutableEnv = "INCUS_TUI_ELEVATION_EXECUTABLE"

func main() {
	cfg := config.Default()
	flag.StringVar(&cfg.Remote, "remote", cfg.Remote, "Incus remote name or endpoint URL")
	flag.StringVar(&cfg.Project, "project", cfg.Project, "Incus project name")
	flag.DurationVar(&cfg.CommandTimeout, "timeout", cfg.CommandTimeout, "Incus command timeout")
	flag.Parse()

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid config: %v\n", err)
		os.Exit(2)
	}

	incusSvc, err := client.NewIncusClient(cfg.Remote, cfg.Project)
	if err != nil {
		if shouldElevateForLocalSocket(cfg.Remote, err) {
			if relaunchErr := relaunchWithPrivileges(); relaunchErr != nil {
				fmt.Fprintf(os.Stderr, "elevate process failed: %v\n", relaunchErr)
			} else {
				return
			}
		}
		fmt.Fprintf(os.Stderr, "init incus client failed: %v\n", err)
		os.Exit(1)
	}
	if err := ensureLocalSocketAccess(cfg, incusSvc); err != nil {
		fmt.Fprintf(os.Stderr, "probe incus access failed: %v\n", err)
		os.Exit(1)
	}

	slog.Info("starting incus-tui", "version", version, "build_date", buildDate)

	instanceModule := instances.New(incusSvc, cfg.CommandTimeout)
	factory := func(remote, project string) (client.InstanceService, error) {
		return client.NewIncusClient(remote, project)
	}
	application := app.New(incusSvc, cfg.CommandTimeout, instanceModule, factory, cfg.Remote, cfg.Project)

	program := tea.NewProgram(application, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "run program failed: %v\n", err)
		os.Exit(1)
	}
}

func ensureLocalSocketAccess(cfg config.Config, incusSvc *client.IncusClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.CommandTimeout)
	defer cancel()

	err := incusSvc.CheckAccess(ctx)
	if err == nil {
		return nil
	}

	if !shouldElevateForLocalSocket(cfg.Remote, err) {
		return err
	}

	if relaunchErr := relaunchWithPrivileges(); relaunchErr != nil {
		return fmt.Errorf("elevate process: %w", relaunchErr)
	}

	return nil
}

func shouldElevateForLocalSocket(remote string, err error) bool {
	if strings.TrimSpace(remote) != "" {
		return false
	}
	if os.Geteuid() == 0 {
		return false
	}
	if os.Getenv(elevationAttemptedEnv) != "" {
		return false
	}
	return client.IsLocalSocketPermissionError(err)
}

func relaunchWithPrivileges() error {
	runner, args, err := buildElevationCommand(os.Args)
	if err != nil {
		return err
	}

	cmd := exec.Command(runner, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), elevationAttemptedEnv+"=1")

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		return err
	}

	os.Exit(0)
	return nil
}

func buildElevationCommand(argv []string) (string, []string, error) {
	executable, err := resolveElevationExecutable()
	if err != nil {
		return "", nil, fmt.Errorf("resolve elevation executable: %w", err)
	}

	path, err := exec.LookPath("sudo")
	if err != nil {
		return "", nil, fmt.Errorf("sudo is required for local socket elevation: %w", err)
	}
	return path, buildElevationArgs(executable, argv), nil
}

func buildElevationArgs(executable string, argv []string) []string {
	args := []string{"--preserve-env=INCUS_CONF," + elevationAttemptedEnv + "," + elevationExecutableEnv, executable}
	return append(args, argv[1:]...)
}

func resolveElevationExecutable() (string, error) {
	if configured := strings.TrimSpace(os.Getenv(elevationExecutableEnv)); configured != "" {
		if filepath.IsAbs(configured) {
			return configured, nil
		}

		absolute, err := filepath.Abs(configured)
		if err != nil {
			return "", fmt.Errorf("make %q absolute: %w", configured, err)
		}
		return absolute, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	return executable, nil
}
