package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	incus "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

type Instance struct {
	Name   string
	Status string
	Type   string
	IP4    []string
}

type Image struct {
	Fingerprint  string
	Type         string
	Size         int64
	Architecture string
	UploadedAt   time.Time
}

type StoragePool struct {
	Name   string
	Driver string
	Status string
	UsedBy int
}

type Network struct {
	Name    string
	Type    string
	Managed bool
	Status  string
	UsedBy  int
}

type Profile struct {
	Name    string
	Project string
	UsedBy  int
}

type Project struct {
	Name        string
	Description string
	UsedBy      int
}

type ClusterMember struct {
	Name    string
	URL     string
	Status  string
	Message string
}

type Operation struct {
	ID          string
	Class       string
	Status      string
	Description string
	CreatedAt   time.Time
}

type Warning struct {
	UUID       string
	Severity   string
	Type       string
	Location   string
	Project    string
	Count      int
	Message    string
	LastSeenAt time.Time
}

type InstanceService interface {
	ListInstances(ctx context.Context) ([]Instance, error)
	StartInstance(ctx context.Context, name string) error
	StopInstance(ctx context.Context, name string) error
	DeleteInstance(ctx context.Context, name string) error
	ListImages(ctx context.Context) ([]Image, error)
	ListStoragePools(ctx context.Context) ([]StoragePool, error)
	ListNetworks(ctx context.Context) ([]Network, error)
	ListProfiles(ctx context.Context) ([]Profile, error)
	ListProjects(ctx context.Context) ([]Project, error)
	ListClusterMembers(ctx context.Context) ([]ClusterMember, error)
	ListOperations(ctx context.Context) ([]Operation, error)
	ListWarnings(ctx context.Context) ([]Warning, error)
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

func (c *IncusClient) ListImages(ctx context.Context) ([]Image, error) {
	images, err := c.server.GetImages()
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}

	items := make([]Image, 0, len(images))
	for _, img := range images {
		items = append(items, Image{
			Fingerprint:  img.Fingerprint,
			Type:         img.Type,
			Size:         img.Size,
			Architecture: img.Architecture,
			UploadedAt:   img.UploadedAt,
		})
	}
	return items, nil
}

func (c *IncusClient) ListStoragePools(ctx context.Context) ([]StoragePool, error) {
	pools, err := c.server.GetStoragePools()
	if err != nil {
		return nil, fmt.Errorf("list storage pools: %w", err)
	}

	items := make([]StoragePool, 0, len(pools))
	for _, pool := range pools {
		items = append(items, StoragePool{Name: pool.Name, Driver: pool.Driver, Status: pool.Status, UsedBy: len(pool.UsedBy)})
	}
	return items, nil
}

func (c *IncusClient) ListNetworks(ctx context.Context) ([]Network, error) {
	networks, err := c.server.GetNetworks()
	if err != nil {
		return nil, fmt.Errorf("list networks: %w", err)
	}

	items := make([]Network, 0, len(networks))
	for _, network := range networks {
		items = append(items, Network{Name: network.Name, Type: network.Type, Managed: network.Managed, Status: network.Status, UsedBy: len(network.UsedBy)})
	}
	return items, nil
}

func (c *IncusClient) ListProfiles(ctx context.Context) ([]Profile, error) {
	profiles, err := c.server.GetProfiles()
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}

	items := make([]Profile, 0, len(profiles))
	for _, profile := range profiles {
		items = append(items, Profile{Name: profile.Name, Project: profile.Project, UsedBy: len(profile.UsedBy)})
	}
	return items, nil
}

func (c *IncusClient) ListProjects(ctx context.Context) ([]Project, error) {
	projects, err := c.server.GetProjects()
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	items := make([]Project, 0, len(projects))
	for _, project := range projects {
		items = append(items, Project{Name: project.Name, Description: project.Description, UsedBy: len(project.UsedBy)})
	}
	return items, nil
}

func (c *IncusClient) ListClusterMembers(ctx context.Context) ([]ClusterMember, error) {
	members, err := c.server.GetClusterMembers()
	if err != nil {
		return nil, fmt.Errorf("list cluster members: %w", err)
	}

	items := make([]ClusterMember, 0, len(members))
	for _, member := range members {
		items = append(items, ClusterMember{Name: member.ServerName, URL: member.URL, Status: member.Status, Message: member.Message})
	}
	return items, nil
}

func (c *IncusClient) ListOperations(ctx context.Context) ([]Operation, error) {
	operations, err := c.server.GetOperations()
	if err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}

	items := make([]Operation, 0, len(operations))
	for _, operation := range operations {
		items = append(items, Operation{ID: operation.ID, Class: operation.Class, Status: operation.Status, Description: operation.Description, CreatedAt: operation.CreatedAt})
	}
	return items, nil
}

func (c *IncusClient) ListWarnings(ctx context.Context) ([]Warning, error) {
	warnings, err := c.server.GetWarnings()
	if err != nil {
		return nil, fmt.Errorf("list warnings: %w", err)
	}

	items := make([]Warning, 0, len(warnings))
	for _, warning := range warnings {
		items = append(items, Warning{UUID: warning.UUID, Severity: warning.Severity, Type: warning.Type, Location: warning.Location, Project: warning.Project, Count: warning.Count, Message: warning.LastMessage, LastSeenAt: warning.LastSeenAt})
	}
	return items, nil
}
