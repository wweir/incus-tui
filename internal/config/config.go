package config

import (
	"fmt"
	"time"
)

// Config keeps runtime options for the TUI process.
type Config struct {
	Remote         string
	Project        string
	CommandTimeout time.Duration
}

func (c Config) Validate() error {
	if c.CommandTimeout <= 0 {
		return fmt.Errorf("command timeout must be greater than 0: %s", c.CommandTimeout)
	}

	if c.CommandTimeout > 5*time.Minute {
		return fmt.Errorf("command timeout must be <= 5m: %s", c.CommandTimeout)
	}

	return nil
}

func Default() Config {
	return Config{
		Remote:         "",
		Project:        "default",
		CommandTimeout: 15 * time.Second,
	}
}
