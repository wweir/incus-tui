package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type Instance struct {
	Name   string   `json:"name"`
	Status string   `json:"status"`
	Type   string   `json:"type"`
	IP4    []string `json:"ip4"`
}

type InstanceService interface {
	ListInstances(ctx context.Context) ([]Instance, error)
	StartInstance(ctx context.Context, name string) error
	StopInstance(ctx context.Context, name string) error
	DeleteInstance(ctx context.Context, name string) error
}

type IncusCLI struct {
	remote  string
	project string
}

func NewIncusCLI(remote, project string) *IncusCLI {
	return &IncusCLI{remote: remote, project: project}
}

func (c *IncusCLI) ListInstances(ctx context.Context) ([]Instance, error) {
	args := c.prefixArgs("list", "--format", "json")
	out, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}

	var payload []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Type   string `json:"type"`
		State  struct {
			Network map[string]struct {
				Addresses []struct {
					Address string `json:"address"`
					Family  string `json:"family"`
				} `json:"addresses"`
			} `json:"network"`
		} `json:"state"`
	}

	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("decode list output: %w", err)
	}

	instances := make([]Instance, 0, len(payload))
	for _, item := range payload {
		ins := Instance{Name: item.Name, Status: item.Status, Type: item.Type}
		for _, nic := range item.State.Network {
			for _, addr := range nic.Addresses {
				if strings.EqualFold(addr.Family, "inet") {
					ins.IP4 = append(ins.IP4, addr.Address)
				}
			}
		}
		instances = append(instances, ins)
	}

	return instances, nil
}

func (c *IncusCLI) StartInstance(ctx context.Context, name string) error {
	_, err := c.run(ctx, c.prefixArgs("start", name)...)
	if err != nil {
		return fmt.Errorf("start instance %q: %w", name, err)
	}
	return nil
}

func (c *IncusCLI) StopInstance(ctx context.Context, name string) error {
	_, err := c.run(ctx, c.prefixArgs("stop", name)...)
	if err != nil {
		return fmt.Errorf("stop instance %q: %w", name, err)
	}
	return nil
}

func (c *IncusCLI) DeleteInstance(ctx context.Context, name string) error {
	_, err := c.run(ctx, c.prefixArgs("delete", name, "--force")...)
	if err != nil {
		return fmt.Errorf("delete instance %q: %w", name, err)
	}
	return nil
}

func (c *IncusCLI) prefixArgs(args ...string) []string {
	base := make([]string, 0, len(args)+4)
	if c.remote != "" {
		base = append(base, "--remote", c.remote)
	}
	if c.project != "" {
		base = append(base, "--project", c.project)
	}
	base = append(base, args...)
	return base
}

func (c *IncusCLI) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "incus", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("incus %s failed: %w, output=%s", strings.Join(args, " "), err, sanitizeOutput(string(out)))
	}
	return out, nil
}

func sanitizeOutput(in string) string {
	trimmed := strings.TrimSpace(in)
	if len(trimmed) <= 1024 {
		return trimmed
	}
	return trimmed[:1024] + "..."
}
