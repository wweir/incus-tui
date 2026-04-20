package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	incus "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

type Instance struct {
	Name   string
	Status string
	Type   string
	IP4    []string
}

type InstanceService interface {
	ListInstances(ctx context.Context) ([]Instance, error)
	StartInstance(ctx context.Context, name string) error
	StopInstance(ctx context.Context, name string) error
	DeleteInstance(ctx context.Context, name string) error
}

type IncusClient struct {
	server incus.InstanceServer
}

func NewIncusClient(remote, project string) (*IncusClient, error) {
	server, err := connectServer(remote)
	if err != nil {
		return nil, err
	}
	if project != "" {
		server = server.UseProject(project)
	}
	return &IncusClient{server: server}, nil
}

func connectServer(remote string) (incus.InstanceServer, error) {
	if strings.TrimSpace(remote) == "" {
		server, err := incus.ConnectIncusUnix("", nil)
		if err != nil {
			return nil, fmt.Errorf("connect default incus unix socket: %w", err)
		}
		return server, nil
	}

	endpoint, err := normalizeEndpoint(remote)
	if err != nil {
		return nil, err
	}

	server, err := incus.ConnectIncus(endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("connect remote endpoint %q: %w", endpoint, err)
	}
	return server, nil
}

func normalizeEndpoint(remote string) (string, error) {
	trimmed := strings.TrimSpace(remote)
	if trimmed == "" {
		return "", fmt.Errorf("remote endpoint is empty")
	}
	if strings.HasPrefix(trimmed, "https://") || strings.HasPrefix(trimmed, "http://") {
		parsed, err := url.ParseRequestURI(trimmed)
		if err != nil {
			return "", fmt.Errorf("invalid remote endpoint %q: %w", trimmed, err)
		}
		if parsed.Hostname() == "" {
			return "", fmt.Errorf("invalid remote endpoint %q: missing host", trimmed)
		}
		return trimmed, nil
	}
	return "", fmt.Errorf("unsupported remote %q: only URL endpoints are supported, e.g. https://127.0.0.1:8443", trimmed)
}

func (c *IncusClient) ListInstances(ctx context.Context) ([]Instance, error) {
	all, err := c.server.GetInstancesFullWithFilter(api.InstanceTypeAny, []string{})
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}

	instances := make([]Instance, 0, len(all))
	for _, item := range all {
		ins := Instance{Name: item.Name, Status: item.Status, Type: string(item.Type)}
		for _, network := range item.State.Network {
			for _, address := range network.Addresses {
				if strings.EqualFold(address.Family, "inet") {
					ins.IP4 = append(ins.IP4, address.Address)
				}
			}
		}
		instances = append(instances, ins)
	}
	return instances, nil
}

func (c *IncusClient) StartInstance(ctx context.Context, name string) error {
	op, err := c.server.UpdateInstanceState(name, api.InstanceStatePut{Action: "start", Timeout: -1}, "")
	if err != nil {
		return fmt.Errorf("start instance %q: %w", name, err)
	}
	if err := op.WaitContext(ctx); err != nil {
		return fmt.Errorf("wait start instance %q: %w", name, err)
	}
	return nil
}

func (c *IncusClient) StopInstance(ctx context.Context, name string) error {
	op, err := c.server.UpdateInstanceState(name, api.InstanceStatePut{Action: "stop", Force: true, Timeout: -1}, "")
	if err != nil {
		return fmt.Errorf("stop instance %q: %w", name, err)
	}
	if err := op.WaitContext(ctx); err != nil {
		return fmt.Errorf("wait stop instance %q: %w", name, err)
	}
	return nil
}

func (c *IncusClient) DeleteInstance(ctx context.Context, name string) error {
	op, err := c.server.DeleteInstance(name)
	if err != nil {
		return fmt.Errorf("delete instance %q: %w", name, err)
	}
	if err := op.WaitContext(ctx); err != nil {
		return fmt.Errorf("wait delete instance %q: %w", name, err)
	}
	return nil
}
