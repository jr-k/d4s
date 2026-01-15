package compose

import (
	"fmt"
	"strings"

	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/jr-k/d4s/internal/dao/common"
	"golang.org/x/net/context"
)

type Manager struct {
	cli *client.Client
	ctx context.Context
}

func NewManager(cli *client.Client, ctx context.Context) *Manager {
	return &Manager{cli: cli, ctx: ctx}
}

// ComposeProject Model
type ComposeProject struct {
	Name        string
	Status      string
	ConfigFiles string
	ConfigPaths []string
}

func (cp ComposeProject) GetID() string { return cp.Name }
func (cp ComposeProject) GetCells() []string {
	return []string{cp.Name, cp.Status, cp.ConfigFiles}
}

func (m *Manager) List() ([]common.Resource, error) {
	list, err := m.cli.ContainerList(m.ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	type projData struct {
		total       int
		running     int
		config      string
		configPaths []string
	}
	projects := make(map[string]*projData)

	for _, c := range list {
		proj := c.Labels["com.docker.compose.project"]
		if proj == "" {
			continue
		}

		if _, ok := projects[proj]; !ok {
			config := ""
			var paths []string
			if cf, ok := c.Labels["com.docker.compose.project.config_files"]; ok {
				config = common.ShortenPath(cf)
				paths = strings.Split(cf, ",")
			}
			projects[proj] = &projData{
				config:      config,
				configPaths: paths,
			}
		}

		projects[proj].total++
		if c.State == "running" {
			projects[proj].running++
		}
	}

	var res []common.Resource
	for name, data := range projects {
		var configStr string
		if data.config != "" {
			configStr = fmt.Sprintf("📄 [#6272a4]%s", data.config)
		}
		
		res = append(res, ComposeProject{
			Name:        name,
			Status:      fmt.Sprintf("Running (%d/%d)", data.running, data.total),
			ConfigFiles: configStr,
			ConfigPaths: data.configPaths,
		})
	}
	return res, nil
}

func (m *Manager) Stop(projectName string) error {
	// Find all containers with this project name
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("com.docker.compose.project=%s", projectName))
	
	containers, err := m.cli.ContainerList(m.ctx, container.ListOptions{Filters: args, All: true})
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
		if err := m.cli.ContainerStop(m.ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", c.Names[0], err))
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("errors stopping containers: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (m *Manager) Restart(projectName string) error {
	// Find all containers with this project name
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("com.docker.compose.project=%s", projectName))
	
	containers, err := m.cli.ContainerList(m.ctx, container.ListOptions{Filters: args, All: true})
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		return fmt.Errorf("no containers found for project %s", projectName)
	}

	timeout := 10
	var errs []string
	for _, c := range containers {
		if err := m.cli.ContainerRestart(m.ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", c.Names[0], err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors restarting containers: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (m *Manager) GetConfig(projectName string) (string, error) {
	// Find one container to get config path
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("com.docker.compose.project=%s", projectName))
	
	containers, err := m.cli.ContainerList(m.ctx, container.ListOptions{Filters: args, All: true, Limit: 1})
	if err != nil {
		return "", err
	}
	
	if len(containers) == 0 {
		return "", fmt.Errorf("project not found or no containers")
	}
	
	configFiles := containers[0].Labels["com.docker.compose.project.config_files"]
	if configFiles == "" {
		return "", fmt.Errorf("no config files label found")
	}
	
	// Handle multiple files (separated by comma)
	files := strings.Split(configFiles, ",")
	var sb strings.Builder
	
	for _, f := range files {
		path := strings.TrimSpace(f)
		if path == "" { continue }
		
		content, err := os.ReadFile(path)
		if err != nil {
			sb.WriteString(fmt.Sprintf("# Error reading %s: %v\n", path, err))
			continue
		}
		
		sb.WriteString(fmt.Sprintf("# File: %s\n", path))
		sb.WriteString(string(content))
		sb.WriteString("\n---\n")
	}
	
	return sb.String(), nil
}
