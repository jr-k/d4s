package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	dt "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

type DockerClient struct {
	cli *client.Client
	ctx context.Context
}

// Resource is a generic interface for displayable items
type Resource interface {
	GetID() string
	GetCells() []string
}

// HostStats represents basic host metrics
type HostStats struct {
	CPU      string
	Mem      string
	Name     string
	Version  string
	Context  string
}

// Container Model
type Container struct {
	ID      string
	Names   string
	Image   string
	Status  string
	State   string
	Ports   string
	Created string
	Compose string
	CPU     string
	Mem     string
}

func (c Container) GetID() string { return c.ID }
func (c Container) GetCells() []string {
	id := c.ID
	if len(id) > 12 {
		id = id[:12]
	}
	return []string{id, c.Names, c.Image, c.Status, c.Ports, c.CPU, c.Mem, c.Compose, c.Created}
}

// Image Model
type Image struct {
	ID      string
	Tags    string
	Size    string
	Created string
}

func (i Image) GetID() string { return i.ID }
func (i Image) GetCells() []string {
	return []string{i.ID[:12], i.Tags, i.Size, i.Created}
}

// Volume Model
type Volume struct {
	Name   string
	Driver string
	Mount  string
}

func (v Volume) GetID() string { return v.Name }
func (v Volume) GetCells() []string {
	return []string{v.Name, v.Driver, v.Mount}
}

// Network Model
type Network struct {
	ID     string
	Name   string
	Driver string
	Scope  string
}

func (n Network) GetID() string { return n.ID }
func (n Network) GetCells() []string {
	return []string{n.ID[:12], n.Name, n.Driver, n.Scope}
}

// Service Model
type Service struct {
	ID       string
	Name     string
	Image    string
	Mode     string
	Replicas string
	Ports    string
}

func (s Service) GetID() string { return s.ID }
func (s Service) GetCells() []string {
	return []string{s.ID[:12], s.Name, s.Image, s.Mode, s.Replicas, s.Ports}
}

// Node Model
type Node struct {
	ID       string
	Hostname string
	Status   string
	Avail    string
	Role     string
	Version  string
}

func (n Node) GetID() string { return n.ID }
func (n Node) GetCells() []string {
	return []string{n.ID[:12], n.Hostname, n.Status, n.Avail, n.Role, n.Version}
}

func NewDockerClient() (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerClient{cli: cli, ctx: context.Background()}, nil
}

func (d *DockerClient) GetHostStats() (HostStats, error) {
	info, err := d.cli.Info(d.ctx)
	if err != nil {
		return HostStats{}, err
	}
	
	// Basic Mock stats for now as real-time host stats require system access
	// or complex docker stats aggregation. We use Info for static data.
	memTotal := formatBytes(info.MemTotal)
	
	return HostStats{
		CPU:     fmt.Sprintf("%d CPUs", info.NCPU),
		Mem:     memTotal,
		Name:    info.Name,
		Version: info.ServerVersion,
		Context: "default", // TODO: Fetch real context name
	}, nil
}

func (d *DockerClient) ListContainers() ([]Resource, error) {
	list, err := d.cli.ContainerList(d.ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var res []Resource
	for _, c := range list {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		
		ports := ""
		if len(c.Ports) > 0 {
			ports = fmt.Sprintf("%d->%d", c.Ports[0].PublicPort, c.Ports[0].PrivatePort)
		}

		compose := ""
		if cf, ok := c.Labels["com.docker.compose.project.config_files"]; ok {
			compose = "📄 " + shortenPath(cf)
		} else if proj, ok := c.Labels["com.docker.compose.project"]; ok {
			compose = "📦 " + proj
		}

		res = append(res, Container{
			ID:      c.ID,
			Names:   name,
			Image:   c.Image,
			Status:  c.Status,
			State:   c.State,
			Ports:   ports,
			Created: formatTime(c.Created),
			Compose: compose,
			CPU:     "0%", // Mock until async stats implemented
			Mem:     "0% ([#6272a4]0 B[-])", // Mock
		})
	}
	return res, nil
}

func (d *DockerClient) ListImages() ([]Resource, error) {
	list, err := d.cli.ImageList(d.ctx, image.ListOptions{})
	if err != nil {
		return nil, err
	}

	var res []Resource
	for _, i := range list {
		tags := "<none>"
		if len(i.RepoTags) > 0 {
			tags = i.RepoTags[0]
		}
		res = append(res, Image{
			ID:      strings.TrimPrefix(i.ID, "sha256:"),
			Tags:    tags,
			Size:    formatBytes(i.Size),
			Created: formatTime(i.Created),
		})
	}
	return res, nil
}

func (d *DockerClient) ListVolumes() ([]Resource, error) {
	list, err := d.cli.VolumeList(d.ctx, volume.ListOptions{})
	if err != nil {
		return nil, err
	}

	var res []Resource
	for _, v := range list.Volumes {
		res = append(res, Volume{
			Name:   v.Name,
			Driver: v.Driver,
			Mount:  shortenPath(v.Mountpoint),
		})
	}
	return res, nil
}

func (d *DockerClient) ListNetworks() ([]Resource, error) {
	list, err := d.cli.NetworkList(d.ctx, network.ListOptions{})
	if err != nil {
		return nil, err
	}

	var res []Resource
	for _, n := range list {
		res = append(res, Network{
			ID:     n.ID,
			Name:   n.Name,
			Driver: n.Driver,
			Scope:  n.Scope,
		})
	}
	return res, nil
}

func (d *DockerClient) ListServices() ([]Resource, error) {
	list, err := d.cli.ServiceList(d.ctx, dt.ServiceListOptions{})
	if err != nil {
		return nil, err
	}

	var res []Resource
	for _, s := range list {
		mode := ""
		replicas := ""
		if s.Spec.Mode.Replicated != nil {
			mode = "Replicated"
			if s.Spec.Mode.Replicated.Replicas != nil {
				replicas = fmt.Sprintf("%d", *s.Spec.Mode.Replicated.Replicas)
			}
		} else if s.Spec.Mode.Global != nil {
			mode = "Global"
		}

		ports := ""
		if len(s.Endpoint.Ports) > 0 {
			ports = fmt.Sprintf("%d->%d", s.Endpoint.Ports[0].PublishedPort, s.Endpoint.Ports[0].TargetPort)
		}

		imageName := s.Spec.TaskTemplate.ContainerSpec.Image
		// Clean image name (remove sha)
		if idx := strings.LastIndex(imageName, "@"); idx != -1 {
			imageName = imageName[:idx]
		}

		res = append(res, Service{
			ID:       s.ID,
			Name:     s.Spec.Name,
			Image:    imageName,
			Mode:     mode,
			Replicas: replicas,
			Ports:    ports,
		})
	}
	return res, nil
}

func (d *DockerClient) ListNodes() ([]Resource, error) {
	list, err := d.cli.NodeList(d.ctx, dt.NodeListOptions{})
	if err != nil {
		return nil, err
	}

	var res []Resource
	for _, n := range list {
		res = append(res, Node{
			ID:       n.ID,
			Hostname: n.Description.Hostname,
			Status:   string(n.Status.State),
			Avail:    string(n.Spec.Availability),
			Role:     string(n.Spec.Role),
			Version:  n.Description.Engine.EngineVersion,
		})
	}
	return res, nil
}

func (d *DockerClient) Inspect(resourceType, id string) (string, error) {
	var data interface{}
	var err error

	switch resourceType {
	case "container":
		data, err = d.cli.ContainerInspect(d.ctx, id)
	case "image":
		data, _, err = d.cli.ImageInspectWithRaw(d.ctx, id)
	case "volume":
		data, err = d.cli.VolumeInspect(d.ctx, id)
	case "network":
		data, err = d.cli.NetworkInspect(d.ctx, id, network.InspectOptions{})
	case "service":
		data, _, err = d.cli.ServiceInspectWithRaw(d.ctx, id, swarm.ServiceInspectOptions{})
	case "node":
		data, _, err = d.cli.NodeInspectWithRaw(d.ctx, id)
	default:
		return "", fmt.Errorf("unknown resource type: %s", resourceType)
	}

	if err != nil {
		return "", err
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Container Actions

func (d *DockerClient) StopContainer(id string) error {
	timeout := 10 // seconds
	return d.cli.ContainerStop(d.ctx, id, container.StopOptions{Timeout: &timeout})
}

func (d *DockerClient) RestartContainer(id string) error {
	timeout := 10 // seconds
	return d.cli.ContainerRestart(d.ctx, id, container.StopOptions{Timeout: &timeout})
}

func (d *DockerClient) RemoveContainer(id string) error {
	return d.cli.ContainerRemove(d.ctx, id, container.RemoveOptions{Force: true})
}

func (d *DockerClient) GetContainerLogs(id string, timestamps bool) (io.ReadCloser, error) {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "200", // Enough to fill screen, optimized start
		Timestamps: timestamps,
	}
	return d.cli.ContainerLogs(d.ctx, id, opts)
}

func (d *DockerClient) GetServiceLogs(id string, timestamps bool) (io.ReadCloser, error) {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "200",
		Timestamps: timestamps,
	}
	return d.cli.ServiceLogs(d.ctx, id, opts)
}

func (d *DockerClient) HasTTY(id string) (bool, error) {
	c, err := d.cli.ContainerInspect(d.ctx, id)
	if err != nil {
		return false, err
	}
	return c.Config.Tty, nil
}

// Image Actions
func (d *DockerClient) RemoveImage(id string) error {
	_, err := d.cli.ImageRemove(d.ctx, id, image.RemoveOptions{Force: true, PruneChildren: true})
	return err
}

func (d *DockerClient) PruneImages() error {
	_, err := d.cli.ImagesPrune(d.ctx, filters.NewArgs())
	return err
}

// Volume Actions
func (d *DockerClient) CreateVolume(name string) error {
	_, err := d.cli.VolumeCreate(d.ctx, volume.CreateOptions{
		Name: name,
	})
	return err
}

func (d *DockerClient) RemoveVolume(id string) error {
	return d.cli.VolumeRemove(d.ctx, id, true)
}

func (d *DockerClient) PruneVolumes() error {
	_, err := d.cli.VolumesPrune(d.ctx, filters.NewArgs())
	return err
}

// Network Actions
func (d *DockerClient) CreateNetwork(name string) error {
	_, err := d.cli.NetworkCreate(d.ctx, name, network.CreateOptions{})
	return err
}

func (d *DockerClient) RemoveNetwork(id string) error {
	return d.cli.NetworkRemove(d.ctx, id)
}

func (d *DockerClient) PruneNetworks() error {
	_, err := d.cli.NetworksPrune(d.ctx, filters.NewArgs())
	return err
}

// Swarm Actions
func (d *DockerClient) ScaleService(id string, replicas uint64) error {
	service, _, err := d.cli.ServiceInspectWithRaw(d.ctx, id, swarm.ServiceInspectOptions{})
	if err != nil {
		return err
	}

	if service.Spec.Mode.Replicated == nil {
		return fmt.Errorf("service is not in replicated mode")
	}

	service.Spec.Mode.Replicated.Replicas = &replicas
	
	// Update
	_, err = d.cli.ServiceUpdate(d.ctx, id, service.Version, service.Spec, swarm.ServiceUpdateOptions{})
	return err
}

func (d *DockerClient) RemoveService(id string) error {
	return d.cli.ServiceRemove(d.ctx, id)
}

func (d *DockerClient) RemoveNode(id string) error {
	// Force remove
	return d.cli.NodeRemove(d.ctx, id, swarm.NodeRemoveOptions{Force: true})
}

// Helpers
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func formatTime(ts int64) string {
	t := time.Unix(ts, 0)
	return t.Format("2006-01-02 15:04")
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
