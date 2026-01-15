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
	CPU        string
	CPUPercent string
	Mem        string
	MemPercent string
	Name       string
	Version    string
	Context    string
	User       string
	Hostname   string
	D4SVersion string
}

// Container Model
type Container struct {
	ID          string
	Names       string
	Image       string
	Status      string
	State       string
	Age         string
	Ports       string
	Created     string
	Compose     string
	ProjectName string
	CPU         string
	Mem         string
}

func (c Container) GetID() string { return c.ID }
func (c Container) GetCells() []string {
	id := c.ID
	if len(id) > 12 {
		id = id[:12]
	}
	return []string{id, c.Names, c.Image, c.Status, c.Age, c.Ports, c.CPU, c.Mem, c.Compose, c.Created}
}

// ComposeProject Model
type ComposeProject struct {
	Name        string
	Status      string
	ConfigFiles string
}

func (cp ComposeProject) GetID() string { return cp.Name }
func (cp ComposeProject) GetCells() []string {
	return []string{cp.Name, cp.Status, cp.ConfigFiles}
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
	
	memTotal := formatBytes(info.MemTotal)
	
	// Get current user
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME") // Windows fallback
	}
	if user == "" {
		user = "unknown"
	}
	
	// Get hostname
	hostname, _ := os.Hostname()

	return HostStats{
		CPU:        fmt.Sprintf("%d", info.NCPU),
		CPUPercent: "...", // Placeholder
		Mem:        memTotal,
		MemPercent: "...", // Placeholder
		Name:       info.Name,
		Version:    info.ServerVersion,
		Context:    "default",
		User:       user,
		Hostname:   hostname,
		D4SVersion: "1.0.0",
	}, nil
}

// GetHostStatsWithUsage returns host stats including CPU and Memory usage percentages
// This is a more expensive call and should be called asynchronously
func (d *DockerClient) GetHostStatsWithUsage() (HostStats, error) {
	// First get basic stats
	stats, err := d.GetHostStats()
	if err != nil {
		return stats, err
	}
	
	// Then calculate usage stats asynchronously
	info, _ := d.cli.Info(d.ctx)
	containers, err := d.cli.ContainerList(d.ctx, container.ListOptions{All: false})
	if err != nil || len(containers) == 0 {
		return stats, nil
	}
	
	var totalCPU float64
	var totalMem uint64
	validStats := 0
	
	// Collect stats from first few containers (to avoid blocking too long)
	maxContainers := len(containers)
	if maxContainers > 10 {
		maxContainers = 10 // Limit to 10 containers for performance
	}
	
	for i := 0; i < maxContainers; i++ {
		c := containers[i]
		statsResp, err := d.cli.ContainerStats(d.ctx, c.ID, false)
		if err != nil {
			continue
		}
		
		var v map[string]interface{}
		if err := json.NewDecoder(statsResp.Body).Decode(&v); err != nil {
			statsResp.Body.Close()
			continue
		}
		statsResp.Body.Close()
		
		// Extract CPU stats
		if cpuStats, ok := v["cpu_stats"].(map[string]interface{}); ok {
			if preCPUStats, ok := v["precpu_stats"].(map[string]interface{}); ok {
				if cpuUsage, ok := cpuStats["cpu_usage"].(map[string]interface{}); ok {
					if preCPUUsage, ok := preCPUStats["cpu_usage"].(map[string]interface{}); ok {
						if totalUsage, ok := cpuUsage["total_usage"].(float64); ok {
							if preTotalUsage, ok := preCPUUsage["total_usage"].(float64); ok {
								if systemUsage, ok := cpuStats["system_cpu_usage"].(float64); ok {
									if preSystemUsage, ok := preCPUStats["system_cpu_usage"].(float64); ok {
										cpuDelta := totalUsage - preTotalUsage
										systemDelta := systemUsage - preSystemUsage
										if systemDelta > 0 && cpuDelta > 0 {
											if percpu, ok := cpuUsage["percpu_usage"].([]interface{}); ok {
												cpuPct := (cpuDelta / systemDelta) * float64(len(percpu)) * 100.0
												totalCPU += cpuPct
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
		
		// Extract memory stats
		if memStats, ok := v["memory_stats"].(map[string]interface{}); ok {
			if usage, ok := memStats["usage"].(float64); ok {
				totalMem += uint64(usage)
				validStats++
			}
		}
	}
	
	// Format CPU percentage
	if validStats > 0 && totalCPU > 0 {
		stats.CPUPercent = fmt.Sprintf("%.1f%%", totalCPU)
	} else {
		stats.CPUPercent = "0%"
	}
	
	// Format Memory percentage
	if info.MemTotal > 0 && totalMem > 0 {
		memPct := float64(totalMem) / float64(info.MemTotal) * 100.0
		stats.MemPercent = fmt.Sprintf("%.1f%%", memPct)
	} else {
		stats.MemPercent = "0%"
	}
	
	return stats, nil
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
		
		projectName := c.Labels["com.docker.compose.project"]

		status, age := parseStatus(c.Status)

		res = append(res, Container{
			ID:          c.ID,
			Names:       name,
			Image:       c.Image,
			Status:      status,
			Age:         age,
			State:       c.State,
			Ports:       ports,
			Created:     formatTime(c.Created),
			Compose:     compose,
			ProjectName: projectName,
			CPU:         "0%", // Mock until async stats implemented
			Mem:         "0% ([#6272a4]0 B[-])", // Mock
		})
	}
	return res, nil
}

func (d *DockerClient) ListComposeProjects() ([]Resource, error) {
	list, err := d.cli.ContainerList(d.ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	type projData struct {
		total   int
		running int
		config  string
	}
	projects := make(map[string]*projData)

	for _, c := range list {
		proj := c.Labels["com.docker.compose.project"]
		if proj == "" {
			continue
		}

		if _, ok := projects[proj]; !ok {
			config := ""
			if cf, ok := c.Labels["com.docker.compose.project.config_files"]; ok {
				config = shortenPath(cf)
			}
			projects[proj] = &projData{
				config: config,
			}
		}

		projects[proj].total++
		if c.State == "running" {
			projects[proj].running++
		}
	}

	var res []Resource
	for name, data := range projects {
		res = append(res, ComposeProject{
			Name:        name,
			Status:      fmt.Sprintf("Running (%d/%d)", data.running, data.total),
			ConfigFiles: data.config,
		})
	}
	return res, nil
}

// Compose Actions
func (d *DockerClient) StopComposeProject(projectName string) error {
	// Find all containers with this project name
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("com.docker.compose.project=%s", projectName))
	
	containers, err := d.cli.ContainerList(d.ctx, container.ListOptions{Filters: args, All: true})
	if err != nil {
		return err
	}
	
	if len(containers) == 0 {
		return fmt.Errorf("no containers found for project %s", projectName)
	}

	// Stop them all (sequentially for now, or parallel if needed)
	timeout := 10
	var errs []string
	for _, c := range containers {
		if err := d.cli.ContainerStop(d.ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", c.Names[0], err))
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("errors stopping containers: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (d *DockerClient) RestartComposeProject(projectName string) error {
	// Find all containers with this project name
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("com.docker.compose.project=%s", projectName))
	
	containers, err := d.cli.ContainerList(d.ctx, container.ListOptions{Filters: args, All: true})
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		return fmt.Errorf("no containers found for project %s", projectName)
	}

	timeout := 10
	var errs []string
	for _, c := range containers {
		if err := d.cli.ContainerRestart(d.ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", c.Names[0], err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors restarting containers: %s", strings.Join(errs, "; "))
	}
	return nil
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

func (d *DockerClient) StartContainer(id string) error {
	return d.cli.ContainerStart(d.ctx, id, container.StartOptions{})
}

func (d *DockerClient) RestartContainer(id string) error {
	timeout := 10 // seconds
	return d.cli.ContainerRestart(d.ctx, id, container.StopOptions{Timeout: &timeout})
}

func (d *DockerClient) RemoveContainer(id string, force bool) error {
	return d.cli.ContainerRemove(d.ctx, id, container.RemoveOptions{Force: force})
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

func (d *DockerClient) GetContainerEnv(id string) ([]string, error) {
	c, err := d.cli.ContainerInspect(d.ctx, id)
	if err != nil {
		return nil, err
	}
	return c.Config.Env, nil
}

func (d *DockerClient) GetContainerStats(id string) (string, error) {
	resp, err := d.cli.ContainerStats(d.ctx, id, false)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var v interface{}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", err
	}
	
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (d *DockerClient) HasTTY(id string) (bool, error) {
	c, err := d.cli.ContainerInspect(d.ctx, id)
	if err != nil {
		return false, err
	}
	return c.Config.Tty, nil
}

// Image Actions
func (d *DockerClient) RemoveImage(id string, force bool) error {
	_, err := d.cli.ImageRemove(d.ctx, id, image.RemoveOptions{Force: force, PruneChildren: true})
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

func (d *DockerClient) RemoveVolume(id string, force bool) error {
	return d.cli.VolumeRemove(d.ctx, id, force)
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

func (d *DockerClient) RemoveNode(id string, force bool) error {
	// Force remove
	return d.cli.NodeRemove(d.ctx, id, swarm.NodeRemoveOptions{Force: force})
}

// Helpers
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func parseStatus(s string) (status, age string) {
	s = strings.TrimSpace(s)
	
	if strings.HasPrefix(s, "Up") {
		status = "Up"
		rest := strings.TrimPrefix(s, "Up ")
		age = shortenDuration(rest)
	} else if strings.HasPrefix(s, "Exited") {
		// "Exited (0) 5 minutes ago"
		parts := strings.SplitN(s, ") ", 2)
		if len(parts) == 2 {
			status = parts[0] + ")"
			age = shortenDuration(strings.TrimSuffix(parts[1], " ago"))
		} else {
			status = "Exited"
			age = s
		}
	} else if strings.HasPrefix(s, "Created") {
		status = "Created"
		age = "-"
	} else if strings.HasPrefix(s, "Paused") {
		// "Up 2 hours (Paused)"
		if strings.Contains(s, "(Paused)") {
			status = "Paused"
			rest := strings.TrimPrefix(s, "Up ")
			rest = strings.TrimSuffix(rest, " (Paused)")
			age = shortenDuration(rest)
			return
		}
		status = "Paused"
		age = "-"
	} else if strings.HasPrefix(s, "Exiting") { // Explicitly handle Exiting
		status = "Exiting"
		age = "-"
	} else if strings.Contains(strings.ToLower(s), "starting") {
		status = "Starting"
		age = "-"
	} else {
		status = s
		age = "-"
	}
	return
}

func shortenDuration(d string) string {
	d = strings.ToLower(d)
	if strings.Contains(d, "less than") {
		return "1s"
	}
	
	// Clean up verbose words
	d = strings.ReplaceAll(d, "about ", "")
	d = strings.ReplaceAll(d, "an ", "1 ")
	d = strings.ReplaceAll(d, "a ", "1 ")
	d = strings.TrimSuffix(d, " ago")
	
	parts := strings.Fields(d)
	if len(parts) >= 2 {
		val := parts[0]
		unit := parts[1]
		
		if val == "0" && strings.HasPrefix(unit, "second") {
			return "1s"
		}

		if strings.HasPrefix(unit, "second") { return val + "s" }
		if strings.HasPrefix(unit, "minute") { return val + "m" }
		if strings.HasPrefix(unit, "hour") { return val + "h" }
		if strings.HasPrefix(unit, "day") { return val + "d" }
		if strings.HasPrefix(unit, "week") { return val + "w" }
		if strings.HasPrefix(unit, "month") { return val + "mo" }
		if strings.HasPrefix(unit, "year") { return val + "y" }
	}
	return d
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
