package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
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

type DetailField struct {
	Label string
	Value string
}

type ResourceDetail struct {
	Title  string
	Fields []DetailField
}

type ResourceValues map[string]string

func (v ResourceValues) Get(key string) string {
	return strings.TrimSpace(v[key])
}

type InstanceService interface {
	ListInstances(ctx context.Context) ([]Instance, error)
	CreateInstance(ctx context.Context, name, image, instanceType string) error
	UpdateInstanceConfig(ctx context.Context, name, key, value string) error
	StartInstance(ctx context.Context, name string) error
	StopInstance(ctx context.Context, name string) error
	DeleteInstance(ctx context.Context, name string) error
	GetInstanceDetail(ctx context.Context, name string) (ResourceDetail, error)
	ListImages(ctx context.Context) ([]Image, error)
	ListStoragePools(ctx context.Context) ([]StoragePool, error)
	ListNetworks(ctx context.Context) ([]Network, error)
	ListProfiles(ctx context.Context) ([]Profile, error)
	ListProjects(ctx context.Context) ([]Project, error)
	ListClusterMembers(ctx context.Context) ([]ClusterMember, error)
	ListOperations(ctx context.Context) ([]Operation, error)
	ListWarnings(ctx context.Context) ([]Warning, error)
	CreateResource(ctx context.Context, section string, values ResourceValues) error
	UpdateResource(ctx context.Context, section string, values ResourceValues) error
	DeleteResource(ctx context.Context, section, name string) error
	GetResourceDetail(ctx context.Context, section, name string) (ResourceDetail, error)
	WaitForEvent(ctx context.Context) (string, error)
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

func (c *IncusClient) CheckAccess(ctx context.Context) error {
	_, err := runWithContext(ctx, func() (*api.Server, error) {
		server, _, getErr := c.server.GetServer()
		return server, getErr
	})
	if err != nil {
		return fmt.Errorf("probe incus access: %w", err)
	}
	return nil
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

func IsLocalSocketPermissionError(err error) bool {
	if err == nil || !errors.Is(err, os.ErrPermission) {
		return false
	}

	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "unix.socket") || strings.Contains(lower, "connect unix")
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

func (c *IncusClient) GetInstanceDetail(ctx context.Context, name string) (ResourceDetail, error) {
	instance, err := runWithContext(ctx, func() (*api.Instance, error) {
		item, _, getErr := c.server.GetInstance(name)
		return item, getErr
	})
	if err != nil {
		return ResourceDetail{}, fmt.Errorf("get instance %q: %w", name, err)
	}

	return ResourceDetail{
		Title: fmt.Sprintf("Instance %s", instance.Name),
		Fields: []DetailField{
			{Label: "Name", Value: instance.Name},
			{Label: "Project", Value: instance.Project},
			{Label: "Status", Value: instance.Status},
			{Label: "Type", Value: instance.Type},
			{Label: "Location", Value: detailValue(instance.Location)},
			{Label: "Architecture", Value: detailValue(instance.Architecture)},
			{Label: "Description", Value: detailValue(instance.Description)},
			{Label: "Profiles", Value: formatStringList(instance.Profiles)},
			{Label: "Ephemeral", Value: formatBool(instance.Ephemeral)},
			{Label: "Stateful", Value: formatBool(instance.Stateful)},
			{Label: "Created", Value: formatTime(instance.CreatedAt)},
			{Label: "Last Used", Value: formatTime(instance.LastUsedAt)},
			{Label: "Config", Value: formatStringMap(instance.Config)},
			{Label: "Devices", Value: formatNestedStringMap(instance.Devices)},
			{Label: "Expanded Config", Value: formatStringMap(instance.ExpandedConfig)},
			{Label: "Expanded Devices", Value: formatNestedStringMap(instance.ExpandedDevices)},
		},
	}, nil
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

func (c *IncusClient) CreateResource(ctx context.Context, section string, values ResourceValues) error {
	name := values.Get("name")
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("create %s %q: %w", section, name, err)
	}

	switch section {
	case "Images":
		public, err := parseBoolValue(values.Get("public"))
		if err != nil {
			return fmt.Errorf("create image %q: %w", values.Get("alias"), err)
		}
		autoUpdate, err := parseBoolValue(values.Get("auto_update"))
		if err != nil {
			return fmt.Errorf("create image %q: %w", values.Get("alias"), err)
		}

		req := api.ImagesPost{
			ImagePut: api.ImagePut{
				Public:     public,
				AutoUpdate: autoUpdate,
			},
			Source: &api.ImagesPostSource{
				ImageSource: api.ImageSource{
					Server:   values.Get("server"),
					Protocol: defaultString(values.Get("protocol"), "simplestreams"),
					Alias:    values.Get("alias"),
				},
				Type: "image",
				Mode: "pull",
			},
		}
		if localAlias := values.Get("local_alias"); localAlias != "" {
			req.Aliases = []api.ImageAlias{{Name: localAlias}}
		}

		op, err := c.server.CreateImage(req, nil)
		if err != nil {
			return fmt.Errorf("create image from %q: %w", req.Source.Alias, err)
		}
		if err := op.WaitContext(ctx); err != nil {
			return fmt.Errorf("wait create image from %q: %w", req.Source.Alias, err)
		}
		return nil
	case "Storage":
		req := api.StoragePoolsPost{
			Name:   name,
			Driver: values.Get("driver"),
			StoragePoolPut: api.StoragePoolPut{
				Description: values.Get("description"),
			},
		}
		return c.server.CreateStoragePool(req)
	case "Networks":
		req := api.NetworksPost{
			Name: name,
			Type: values.Get("type"),
			NetworkPut: api.NetworkPut{
				Description: values.Get("description"),
			},
		}
		return c.server.CreateNetwork(req)
	case "Profiles":
		return c.server.CreateProfile(api.ProfilesPost{
			Name: name,
			ProfilePut: api.ProfilePut{
				Description: values.Get("description"),
			},
		})
	case "Projects":
		return c.server.CreateProject(api.ProjectsPost{
			Name: name,
			ProjectPut: api.ProjectPut{
				Description: values.Get("description"),
			},
		})
	default:
		return fmt.Errorf("create %s is not supported", section)
	}
}

func (c *IncusClient) UpdateResource(ctx context.Context, section string, values ResourceValues) error {
	name := values.Get("name")
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("update %s %q: %w", section, name, err)
	}

	switch section {
	case "Images":
		image, etag, err := c.server.GetImage(name)
		if err != nil {
			return fmt.Errorf("get image %q: %w", name, err)
		}
		put := image.Writable()
		if public, ok, err := parseOptionalBoolValue(values.Get("public")); err != nil {
			return fmt.Errorf("update image %q: %w", name, err)
		} else if ok {
			put.Public = public
		}
		if autoUpdate, ok, err := parseOptionalBoolValue(values.Get("auto_update")); err != nil {
			return fmt.Errorf("update image %q: %w", name, err)
		} else if ok {
			put.AutoUpdate = autoUpdate
		}
		if profiles, ok := parseOptionalCSV(values.Get("profiles")); ok {
			put.Profiles = profiles
		}
		return c.server.UpdateImage(name, put, etag)
	case "Storage":
		pool, etag, err := c.server.GetStoragePool(name)
		if err != nil {
			return fmt.Errorf("get storage pool %q: %w", name, err)
		}
		put := pool.Writable()
		if description, ok := optionalOverride(values.Get("description")); ok {
			put.Description = description
		}
		return c.server.UpdateStoragePool(name, put, etag)
	case "Networks":
		network, etag, err := c.server.GetNetwork(name)
		if err != nil {
			return fmt.Errorf("get network %q: %w", name, err)
		}
		put := network.Writable()
		if description, ok := optionalOverride(values.Get("description")); ok {
			put.Description = description
		}
		return c.server.UpdateNetwork(name, put, etag)
	case "Profiles":
		profile, etag, err := c.server.GetProfile(name)
		if err != nil {
			return fmt.Errorf("get profile %q: %w", name, err)
		}
		put := profile.Writable()
		if description, ok := optionalOverride(values.Get("description")); ok {
			put.Description = description
		}
		return c.server.UpdateProfile(name, put, etag)
	case "Projects":
		project, etag, err := c.server.GetProject(name)
		if err != nil {
			return fmt.Errorf("get project %q: %w", name, err)
		}
		put := project.Writable()
		if description, ok := optionalOverride(values.Get("description")); ok {
			put.Description = description
		}
		return c.server.UpdateProject(name, put, etag)
	case "Cluster":
		member, etag, err := c.server.GetClusterMember(name)
		if err != nil {
			return fmt.Errorf("get cluster member %q: %w", name, err)
		}
		put := member.Writable()
		if description, ok := optionalOverride(values.Get("description")); ok {
			put.Description = description
		}
		if failureDomain, ok := optionalOverride(values.Get("failure_domain")); ok {
			put.FailureDomain = failureDomain
		}
		if groups, ok := parseOptionalCSV(values.Get("groups")); ok {
			put.Groups = groups
		}
		if roles, ok := parseOptionalCSV(values.Get("roles")); ok {
			put.Roles = roles
		}
		return c.server.UpdateClusterMember(name, put, etag)
	case "Warnings":
		warning, etag, err := c.server.GetWarning(name)
		if err != nil {
			return fmt.Errorf("get warning %q: %w", name, err)
		}
		put := warning.WarningPut
		put.Status = values.Get("status")
		return c.server.UpdateWarning(name, put, etag)
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

func (c *IncusClient) GetResourceDetail(ctx context.Context, section, name string) (ResourceDetail, error) {
	switch section {
	case "Images":
		image, err := runWithContext(ctx, func() (*api.Image, error) {
			item, _, getErr := c.server.GetImage(name)
			return item, getErr
		})
		if err != nil {
			return ResourceDetail{}, fmt.Errorf("get image %q: %w", name, err)
		}
		return ResourceDetail{
			Title: fmt.Sprintf("Image %s", image.Fingerprint),
			Fields: []DetailField{
				{Label: "Fingerprint", Value: image.Fingerprint},
				{Label: "Project", Value: detailValue(image.Project)},
				{Label: "Type", Value: image.Type},
				{Label: "Architecture", Value: detailValue(image.Architecture)},
				{Label: "Public", Value: formatBool(image.Public)},
				{Label: "Cached", Value: formatBool(image.Cached)},
				{Label: "Auto Update", Value: formatBool(image.AutoUpdate)},
				{Label: "Size", Value: humanSize(image.Size)},
				{Label: "Filename", Value: detailValue(image.Filename)},
				{Label: "Aliases", Value: formatImageAliases(image.Aliases)},
				{Label: "Profiles", Value: formatStringList(image.Profiles)},
				{Label: "Properties", Value: formatStringMap(image.Properties)},
				{Label: "Uploaded", Value: formatTime(image.UploadedAt)},
				{Label: "Created", Value: formatTime(image.CreatedAt)},
				{Label: "Last Used", Value: formatTime(image.LastUsedAt)},
				{Label: "Expires", Value: formatTime(image.ExpiresAt)},
			},
		}, nil
	case "Storage":
		pool, err := runWithContext(ctx, func() (*api.StoragePool, error) {
			item, _, getErr := c.server.GetStoragePool(name)
			return item, getErr
		})
		if err != nil {
			return ResourceDetail{}, fmt.Errorf("get storage pool %q: %w", name, err)
		}
		return ResourceDetail{
			Title: fmt.Sprintf("Storage %s", pool.Name),
			Fields: []DetailField{
				{Label: "Name", Value: pool.Name},
				{Label: "Driver", Value: pool.Driver},
				{Label: "Status", Value: pool.Status},
				{Label: "Description", Value: detailValue(pool.Description)},
				{Label: "Locations", Value: formatStringList(pool.Locations)},
				{Label: "Used By", Value: fmt.Sprintf("%d", len(pool.UsedBy))},
				{Label: "Config", Value: formatStringMap(pool.Config)},
			},
		}, nil
	case "Networks":
		network, err := runWithContext(ctx, func() (*api.Network, error) {
			item, _, getErr := c.server.GetNetwork(name)
			return item, getErr
		})
		if err != nil {
			return ResourceDetail{}, fmt.Errorf("get network %q: %w", name, err)
		}
		return ResourceDetail{
			Title: fmt.Sprintf("Network %s", network.Name),
			Fields: []DetailField{
				{Label: "Name", Value: network.Name},
				{Label: "Project", Value: detailValue(network.Project)},
				{Label: "Type", Value: network.Type},
				{Label: "Status", Value: detailValue(network.Status)},
				{Label: "Managed", Value: formatBool(network.Managed)},
				{Label: "Description", Value: detailValue(network.Description)},
				{Label: "Locations", Value: formatStringList(network.Locations)},
				{Label: "Used By", Value: fmt.Sprintf("%d", len(network.UsedBy))},
				{Label: "Config", Value: formatStringMap(network.Config)},
			},
		}, nil
	case "Profiles":
		profile, err := runWithContext(ctx, func() (*api.Profile, error) {
			item, _, getErr := c.server.GetProfile(name)
			return item, getErr
		})
		if err != nil {
			return ResourceDetail{}, fmt.Errorf("get profile %q: %w", name, err)
		}
		return ResourceDetail{
			Title: fmt.Sprintf("Profile %s", profile.Name),
			Fields: []DetailField{
				{Label: "Name", Value: profile.Name},
				{Label: "Project", Value: detailValue(profile.Project)},
				{Label: "Description", Value: detailValue(profile.Description)},
				{Label: "Used By", Value: fmt.Sprintf("%d", len(profile.UsedBy))},
				{Label: "Config", Value: formatStringMap(profile.Config)},
				{Label: "Devices", Value: formatNestedStringMap(profile.Devices)},
			},
		}, nil
	case "Projects":
		project, err := runWithContext(ctx, func() (*api.Project, error) {
			item, _, getErr := c.server.GetProject(name)
			return item, getErr
		})
		if err != nil {
			return ResourceDetail{}, fmt.Errorf("get project %q: %w", name, err)
		}
		return ResourceDetail{
			Title: fmt.Sprintf("Project %s", project.Name),
			Fields: []DetailField{
				{Label: "Name", Value: project.Name},
				{Label: "Description", Value: detailValue(project.Description)},
				{Label: "Used By", Value: fmt.Sprintf("%d", len(project.UsedBy))},
				{Label: "Config", Value: formatStringMap(project.Config)},
			},
		}, nil
	case "Cluster":
		member, err := runWithContext(ctx, func() (*api.ClusterMember, error) {
			item, _, getErr := c.server.GetClusterMember(name)
			return item, getErr
		})
		if err != nil {
			return ResourceDetail{}, fmt.Errorf("get cluster member %q: %w", name, err)
		}
		return ResourceDetail{
			Title: fmt.Sprintf("Cluster %s", member.ServerName),
			Fields: []DetailField{
				{Label: "Name", Value: member.ServerName},
				{Label: "URL", Value: member.URL},
				{Label: "Status", Value: member.Status},
				{Label: "Message", Value: detailValue(member.Message)},
				{Label: "Description", Value: detailValue(member.Description)},
				{Label: "Architecture", Value: detailValue(member.Architecture)},
				{Label: "Database", Value: formatBool(member.Database)},
				{Label: "Failure Domain", Value: detailValue(member.FailureDomain)},
				{Label: "Roles", Value: formatStringList(member.Roles)},
				{Label: "Groups", Value: formatStringList(member.Groups)},
				{Label: "Config", Value: formatStringMap(member.Config)},
			},
		}, nil
	case "Operations":
		operation, err := runWithContext(ctx, func() (*api.Operation, error) {
			item, _, getErr := c.server.GetOperation(name)
			return item, getErr
		})
		if err != nil {
			return ResourceDetail{}, fmt.Errorf("get operation %q: %w", name, err)
		}
		return ResourceDetail{
			Title: fmt.Sprintf("Operation %s", operation.ID),
			Fields: []DetailField{
				{Label: "ID", Value: operation.ID},
				{Label: "Class", Value: operation.Class},
				{Label: "Status", Value: operation.Status},
				{Label: "Description", Value: detailValue(operation.Description)},
				{Label: "Location", Value: detailValue(operation.Location)},
				{Label: "May Cancel", Value: formatBool(operation.MayCancel)},
				{Label: "Error", Value: detailValue(operation.Err)},
				{Label: "Created", Value: formatTime(operation.CreatedAt)},
				{Label: "Updated", Value: formatTime(operation.UpdatedAt)},
				{Label: "Resources", Value: formatResourceMap(operation.Resources)},
				{Label: "Metadata", Value: formatAnyMap(operation.Metadata)},
			},
		}, nil
	case "Warnings":
		warning, err := runWithContext(ctx, func() (*api.Warning, error) {
			item, _, getErr := c.server.GetWarning(name)
			return item, getErr
		})
		if err != nil {
			return ResourceDetail{}, fmt.Errorf("get warning %q: %w", name, err)
		}
		return ResourceDetail{
			Title: fmt.Sprintf("Warning %s", warning.UUID),
			Fields: []DetailField{
				{Label: "UUID", Value: warning.UUID},
				{Label: "Severity", Value: warning.Severity},
				{Label: "Status", Value: warning.Status},
				{Label: "Type", Value: warning.Type},
				{Label: "Project", Value: detailValue(warning.Project)},
				{Label: "Location", Value: detailValue(warning.Location)},
				{Label: "Count", Value: fmt.Sprintf("%d", warning.Count)},
				{Label: "Entity URL", Value: detailValue(warning.EntityURL)},
				{Label: "First Seen", Value: formatTime(warning.FirstSeenAt)},
				{Label: "Last Seen", Value: formatTime(warning.LastSeenAt)},
				{Label: "Message", Value: detailValue(warning.LastMessage)},
			},
		}, nil
	default:
		return ResourceDetail{}, fmt.Errorf("detail %s is not supported", section)
	}
}

func (c *IncusClient) WaitForEvent(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	listener, err := c.server.GetEventsByType([]string{api.EventTypeLifecycle, api.EventTypeOperation})
	if err != nil {
		return "", fmt.Errorf("create event listener: %w", err)
	}
	defer listener.Disconnect()

	events := make(chan api.Event, 1)
	target, err := listener.AddHandler(nil, func(event api.Event) {
		select {
		case events <- event:
		default:
		}
	})
	if err != nil {
		return "", fmt.Errorf("add event handler: %w", err)
	}
	defer func() { _ = listener.RemoveHandler(target) }()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case event := <-events:
		return event.Type, nil
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

func detailValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "-"
	}
	return trimmed
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format(time.RFC3339)
}

func formatBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func formatStringList(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return strings.Join(sorted, "\n")
}

func formatImageAliases(values []api.ImageAlias) string {
	if len(values) == 0 {
		return "-"
	}

	lines := make([]string, 0, len(values))
	for _, alias := range values {
		lines = append(lines, alias.Name)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func formatStringMap(values map[string]string) string {
	if len(values) == 0 {
		return "-"
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", key, redactSensitiveValue(key, values[key])))
	}
	return strings.Join(lines, "\n")
}

func formatNestedStringMap(values map[string]map[string]string) string {
	if len(values) == 0 {
		return "-"
	}

	outerKeys := make([]string, 0, len(values))
	for key := range values {
		outerKeys = append(outerKeys, key)
	}
	sort.Strings(outerKeys)

	lines := make([]string, 0, len(values))
	for _, outerKey := range outerKeys {
		lines = append(lines, fmt.Sprintf("%s:", outerKey))
		innerKeys := make([]string, 0, len(values[outerKey]))
		for key := range values[outerKey] {
			innerKeys = append(innerKeys, key)
		}
		sort.Strings(innerKeys)
		for _, innerKey := range innerKeys {
			lines = append(lines, fmt.Sprintf("  %s=%s", innerKey, redactSensitiveValue(innerKey, values[outerKey][innerKey])))
		}
	}
	return strings.Join(lines, "\n")
}

func formatResourceMap(values map[string][]string) string {
	if len(values) == 0 {
		return "-"
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(values))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s:", key))
		items := append([]string(nil), values[key]...)
		sort.Strings(items)
		for _, item := range items {
			lines = append(lines, fmt.Sprintf("  %s", item))
		}
	}
	return strings.Join(lines, "\n")
}

func formatAnyMap(values map[string]any) string {
	if len(values) == 0 {
		return "-"
	}

	raw, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return fmt.Sprintf("marshal metadata: %v", err)
	}
	return string(raw)
}

func defaultString(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func optionalOverride(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.EqualFold(trimmed, "keep") {
		return "", false
	}
	return trimmed, true
}

func parseBoolValue(value string) (bool, error) {
	switch strings.TrimSpace(value) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("boolean value must be true or false")
	}
}

func parseOptionalBoolValue(value string) (bool, bool, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.EqualFold(trimmed, "keep") {
		return false, false, nil
	}
	parsed, err := parseBoolValue(trimmed)
	if err != nil {
		return false, false, err
	}
	return parsed, true, nil
}

func parseOptionalCSV(value string) ([]string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.EqualFold(trimmed, "keep") {
		return nil, false
	}

	parts := strings.Split(trimmed, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}
		items = append(items, entry)
	}
	return items, true
}

func humanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%dB", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(size)/float64(div), "KMGTPE"[exp])
}

func redactSensitiveValue(key, value string) string {
	lowerKey := strings.ToLower(key)
	switch {
	case strings.Contains(lowerKey, "secret"),
		strings.Contains(lowerKey, "token"),
		strings.Contains(lowerKey, "password"),
		strings.Contains(lowerKey, "passphrase"),
		strings.Contains(lowerKey, "private"),
		strings.Contains(lowerKey, "certificate"):
		if strings.TrimSpace(value) == "" {
			return "-"
		}
		return "<redacted>"
	default:
		return detailValue(value)
	}
}
