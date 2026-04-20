package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	incus "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/cliconfig"
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
	CreateInstance(ctx context.Context, name, image, instanceType string) error
	UpdateInstanceConfig(ctx context.Context, name, key, value string) error
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
	CreateResource(ctx context.Context, section, name, value string) error
	UpdateResource(ctx context.Context, section, name, value string) error
	DeleteResource(ctx context.Context, section, name string) error
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
	trimmed := strings.TrimSpace(remote)

	if trimmed == "" {
		cfg, err := loadCLIConfig()
		if err != nil {
			return nil, err
		}

		server, err := cfg.GetInstanceServer(cfg.DefaultRemote)
		if err == nil {
			return server, nil
		}

		server, err = incus.ConnectIncusUnix("", nil)
		if err != nil {
			return nil, fmt.Errorf("connect default incus unix socket: %w", err)
		}
		return server, nil
	}

	if strings.HasPrefix(trimmed, "https://") || strings.HasPrefix(trimmed, "http://") {
		endpoint, err := normalizeEndpoint(trimmed)
		if err != nil {
			return nil, err
		}

		server, err := incus.ConnectIncus(endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("connect remote endpoint %q: %w", endpoint, err)
		}
		return server, nil
	}

	cfg, err := loadCLIConfig()
	if err != nil {
		return nil, err
	}

	server, err := cfg.GetInstanceServer(trimmed)
	if err != nil {
		return nil, fmt.Errorf("connect remote %q from incus config: %w", trimmed, err)
	}
	return server, nil
}

func loadCLIConfig() (*cliconfig.Config, error) {
	baseDir := os.Getenv("INCUS_CONF")
	baseDir = strings.TrimSpace(baseDir)
	cfg := cliconfig.NewConfig(baseDir, true)
	cfgPath := cfg.ConfigPath("config.yml")
	loaded, err := cliconfig.LoadConfig(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return nil, fmt.Errorf("load incus config %q: %w", cfgPath, err)
	}
	return loaded, nil
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
	return "", fmt.Errorf("unsupported remote endpoint %q: only URL endpoints are supported, e.g. https://127.0.0.1:8443", trimmed)
}

func (c *IncusClient) ListInstances(ctx context.Context) ([]Instance, error) {
	all, err := runWithContext(ctx, func() ([]api.InstanceFull, error) {
		return c.server.GetInstancesFullWithFilter(api.InstanceTypeAny, []string{})
	})
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

func (c *IncusClient) CreateInstance(ctx context.Context, name, image, instanceType string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("create instance %q: %w", name, err)
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("create instance: name is required")
	}
	if strings.TrimSpace(image) == "" {
		return fmt.Errorf("create instance %q: image is required", name)
	}
	req := api.InstancesPost{
		Name: name,
		Source: api.InstanceSource{
			Type:  "image",
			Alias: image,
		},
	}
	if strings.TrimSpace(instanceType) == "virtual-machine" {
		req.Type = api.InstanceTypeVM
	} else {
		req.Type = api.InstanceTypeContainer
	}
	op, err := c.server.CreateInstance(req)
	if err != nil {
		return fmt.Errorf("create instance %q: %w", name, err)
	}
	if err := op.WaitContext(ctx); err != nil {
		return fmt.Errorf("wait create instance %q: %w", name, err)
	}
	return nil
}

func (c *IncusClient) UpdateInstanceConfig(ctx context.Context, name, key, value string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("update instance %q: %w", name, err)
	}
	if strings.TrimSpace(name) == "" || strings.TrimSpace(key) == "" {
		return fmt.Errorf("update instance: name and key are required")
	}
	instance, etag, err := c.server.GetInstance(name)
	if err != nil {
		return fmt.Errorf("get instance %q: %w", name, err)
	}
	put := instance.Writable()
	if put.Config == nil {
		put.Config = map[string]string{}
	}
	put.Config[key] = value
	op, err := c.server.UpdateInstance(name, put, etag)
	if err != nil {
		return fmt.Errorf("update instance %q: %w", name, err)
	}
	if err := op.WaitContext(ctx); err != nil {
		return fmt.Errorf("wait update instance %q: %w", name, err)
	}
	return nil
}

func (c *IncusClient) StartInstance(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("start instance %q: %w", name, err)
	}

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
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("stop instance %q: %w", name, err)
	}

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
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("delete instance %q: %w", name, err)
	}

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
	return listAndMapWithContext(
		ctx,
		"list images",
		c.server.GetImages,
		func(img api.Image) Image {
			return Image{
				Fingerprint:  img.Fingerprint,
				Type:         img.Type,
				Size:         img.Size,
				Architecture: img.Architecture,
				UploadedAt:   img.UploadedAt,
			}
		},
	)
}

func (c *IncusClient) ListStoragePools(ctx context.Context) ([]StoragePool, error) {
	return listAndMapWithContext(
		ctx,
		"list storage pools",
		c.server.GetStoragePools,
		func(pool api.StoragePool) StoragePool {
			return StoragePool{Name: pool.Name, Driver: pool.Driver, Status: pool.Status, UsedBy: len(pool.UsedBy)}
		},
	)
}

func (c *IncusClient) ListNetworks(ctx context.Context) ([]Network, error) {
	return listAndMapWithContext(
		ctx,
		"list networks",
		c.server.GetNetworks,
		func(network api.Network) Network {
			return Network{Name: network.Name, Type: network.Type, Managed: network.Managed, Status: network.Status, UsedBy: len(network.UsedBy)}
		},
	)
}

func (c *IncusClient) ListProfiles(ctx context.Context) ([]Profile, error) {
	return listAndMapWithContext(
		ctx,
		"list profiles",
		c.server.GetProfiles,
		func(profile api.Profile) Profile {
			return Profile{Name: profile.Name, Project: profile.Project, UsedBy: len(profile.UsedBy)}
		},
	)
}

func (c *IncusClient) ListProjects(ctx context.Context) ([]Project, error) {
	return listAndMapWithContext(
		ctx,
		"list projects",
		c.server.GetProjects,
		func(project api.Project) Project {
			return Project{Name: project.Name, Description: project.Description, UsedBy: len(project.UsedBy)}
		},
	)
}

func (c *IncusClient) ListClusterMembers(ctx context.Context) ([]ClusterMember, error) {
	return listAndMapWithContext(
		ctx,
		"list cluster members",
		c.server.GetClusterMembers,
		func(member api.ClusterMember) ClusterMember {
			return ClusterMember{Name: member.ServerName, URL: member.URL, Status: member.Status, Message: member.Message}
		},
	)
}

func (c *IncusClient) ListOperations(ctx context.Context) ([]Operation, error) {
	return listAndMapWithContext(
		ctx,
		"list operations",
		c.server.GetOperations,
		func(operation api.Operation) Operation {
			return Operation{ID: operation.ID, Class: operation.Class, Status: operation.Status, Description: operation.Description, CreatedAt: operation.CreatedAt}
		},
	)
}

func (c *IncusClient) ListWarnings(ctx context.Context) ([]Warning, error) {
	return listAndMapWithContext(
		ctx,
		"list warnings",
		c.server.GetWarnings,
		func(warning api.Warning) Warning {
			return Warning{UUID: warning.UUID, Severity: warning.Severity, Type: warning.Type, Location: warning.Location, Project: warning.Project, Count: warning.Count, Message: warning.LastMessage, LastSeenAt: warning.LastSeenAt}
		},
	)
}

func (c *IncusClient) CreateResource(ctx context.Context, section, name, value string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("create %s %q: %w", section, name, err)
	}
	switch section {
	case "Storage":
		return c.server.CreateStoragePool(api.StoragePoolsPost{Name: name, Driver: value})
	case "Networks":
		return c.server.CreateNetwork(api.NetworksPost{Name: name, Type: value})
	case "Profiles":
		return c.server.CreateProfile(api.ProfilesPost{Name: name, ProfilePut: api.ProfilePut{Description: value}})
	case "Projects":
		return c.server.CreateProject(api.ProjectsPost{Name: name, ProjectPut: api.ProjectPut{Description: value}})
	default:
		return fmt.Errorf("create %s is not supported", section)
	}
}

func (c *IncusClient) UpdateResource(ctx context.Context, section, name, value string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("update %s %q: %w", section, name, err)
	}
	switch section {
	case "Storage":
		pool, etag, err := c.server.GetStoragePool(name)
		if err != nil {
			return fmt.Errorf("get storage pool %q: %w", name, err)
		}
		put := pool.Writable()
		put.Description = value
		return c.server.UpdateStoragePool(name, put, etag)
	case "Networks":
		network, etag, err := c.server.GetNetwork(name)
		if err != nil {
			return fmt.Errorf("get network %q: %w", name, err)
		}
		put := network.Writable()
		put.Description = value
		return c.server.UpdateNetwork(name, put, etag)
	case "Profiles":
		profile, etag, err := c.server.GetProfile(name)
		if err != nil {
			return fmt.Errorf("get profile %q: %w", name, err)
		}
		put := profile.Writable()
		put.Description = value
		return c.server.UpdateProfile(name, put, etag)
	case "Projects":
		project, etag, err := c.server.GetProject(name)
		if err != nil {
			return fmt.Errorf("get project %q: %w", name, err)
		}
		put := project.Writable()
		put.Description = value
		return c.server.UpdateProject(name, put, etag)
	case "Cluster":
		member, etag, err := c.server.GetClusterMember(name)
		if err != nil {
			return fmt.Errorf("get cluster member %q: %w", name, err)
		}
		put := member.Writable()
		put.Description = value
		return c.server.UpdateClusterMember(name, put, etag)
	default:
		return fmt.Errorf("update %s is not supported", section)
	}
}

func (c *IncusClient) DeleteResource(ctx context.Context, section, name string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("delete %s %q: %w", section, name, err)
	}
	switch section {
	case "Images":
		op, err := c.server.DeleteImage(name)
		if err != nil {
			return fmt.Errorf("delete image %q: %w", name, err)
		}
		return op.WaitContext(ctx)
	case "Storage":
		return c.server.DeleteStoragePool(name)
	case "Networks":
		return c.server.DeleteNetwork(name)
	case "Profiles":
		return c.server.DeleteProfile(name)
	case "Projects":
		return c.server.DeleteProject(name)
	case "Cluster":
		return c.server.DeleteClusterMember(name, false)
	case "Operations":
		return c.server.DeleteOperation(name)
	case "Warnings":
		return c.server.DeleteWarning(name)
	default:
		return fmt.Errorf("delete %s is not supported", section)
	}
}

func runWithContext[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	type result struct {
		value T
		err   error
	}

	if err := ctx.Err(); err != nil {
		var zero T
		return zero, err
	}

	done := make(chan result, 1)
	go func() {
		value, err := fn()
		done <- result{value: value, err: err}
	}()

	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case res := <-done:
		return res.value, res.err
	}
}

func listAndMapWithContext[From any, To any](
	ctx context.Context,
	opName string,
	listFn func() ([]From, error),
	mapFn func(From) To,
) ([]To, error) {
	items, err := runWithContext(ctx, listFn)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", opName, err)
	}

	out := make([]To, 0, len(items))
	for _, item := range items {
		out = append(out, mapFn(item))
	}
	return out, nil
}
