package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

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

	slog.Info("starting incus-tui", "version", version, "build_date", buildDate)

	incusSvc, err := client.NewIncusClient(cfg.Remote, cfg.Project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init incus client failed: %v\n", err)
		os.Exit(1)
	}

	instanceModule := instances.New(incusSvc, cfg.CommandTimeout)
	application := app.New(incusSvc, cfg.CommandTimeout, instanceModule)

	program := tea.NewProgram(application, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "run program failed: %v\n", err)
		os.Exit(1)
	}
}
