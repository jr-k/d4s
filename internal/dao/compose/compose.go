package compose

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao/common"
	"github.com/jr-k/d4s/internal/sshutil"
	"github.com/jr-k/d4s/internal/ui/styles"
	"golang.org/x/net/context"
)

type Manager struct {
	cli *client.Client
	ctx context.Context

	// Where docker compose CLI commands must run. For SSH contexts the
	// compose files only exist on the remote host, so commands are
	// executed there through ssh.
	targetMu    sync.RWMutex
	contextName string
	remoteHost  string // empty = run locally
}

func NewManager(cli *client.Client, ctx context.Context) *Manager {
	return &Manager{cli: cli, ctx: ctx}
}

// SetExecTarget configures where compose CLI commands run.
// remoteHost is empty for local contexts, "user@ip" for SSH contexts.
func (m *Manager) SetExecTarget(contextName, remoteHost string) {
	m.targetMu.Lock()
	m.contextName = contextName
	m.remoteHost = remoteHost
	m.targetMu.Unlock()
}

func (m *Manager) execTarget() (string, string) {
	m.targetMu.RLock()
	defer m.targetMu.RUnlock()
	return m.contextName, m.remoteHost
}

// dockerCmd builds a docker CLI invocation that runs on the host where
// the compose files live: locally (pinned to the right docker context)
// or on the remote SSH host.
func (m *Manager) dockerCmd(args []string, workDir string) *exec.Cmd {
	contextName, remoteHost := m.execTarget()

	if remoteHost != "" {
		quoted := make([]string, 0, len(args)+1)
		quoted = append(quoted, "docker")
		for _, a := range args {
			quoted = append(quoted, sshutil.ShellQuote(a))
		}
		remoteCmd := strings.Join(quoted, " ")
		if workDir != "" {
			remoteCmd = fmt.Sprintf("cd %s && %s", sshutil.ShellQuote(workDir), remoteCmd)
		}
		return sshutil.SSHCommand(contextName, remoteHost, remoteCmd)
	}

	cmd := exec.Command("docker", args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	if contextName != "" && contextName != "default" && contextName != "env" {
		cmd.Env = append(os.Environ(), "DOCKER_CONTEXT="+contextName)
	}
	return cmd
}

// readConfigFile reads a compose file from where it actually lives.
func (m *Manager) readConfigFile(path string) ([]byte, error) {
	contextName, remoteHost := m.execTarget()
	if remoteHost == "" {
		return os.ReadFile(path)
	}
	cmd := sshutil.SSHCommand(contextName, remoteHost, "cat "+sshutil.ShellQuote(path))
	return cmd.Output()
}

// ComposeProject Model
type ComposeProject struct {
	Name        string
	Status      string
	Ready       string
	ConfigFiles string
	ConfigPaths []string
}

func (cp ComposeProject) GetID() string { return cp.Name }
func (cp ComposeProject) GetCells() []string {
	return []string{cp.Name, cp.Ready, cp.Status, cp.ConfigFiles}
}

func (cp ComposeProject) GetStatusColor() (tcell.Color, tcell.Color) {
	if strings.Contains(cp.Ready, "/") {
		parts := strings.Split(cp.Ready, "/")
		if len(parts) == 2 {
			var running, desired int
			fmt.Sscanf(parts[0], "%d", &running)
			fmt.Sscanf(parts[1], "%d", &desired)

			if desired == 0 && running == 0 {
				return styles.ColorStatusGray, styles.ColorBlack
			} else if running < desired {
				return styles.ColorStatusGray, styles.ColorBlack
			} else if running > desired {
				return tcell.ColorMediumPurple, styles.ColorBlack
			} else if desired > 0 {
				return styles.ColorIdle, styles.ColorBlack
			}
		}
	}
	return styles.ColorFg, styles.ColorBlack
}

func (cp ComposeProject) GetColumnValue(column string) string {
	switch strings.ToLower(column) {
	case "project":
		return cp.Name
	case "ready":
		return cp.Ready
	case "status":
		return cp.Status
	case "config files":
		return cp.ConfigFiles
	}
	return ""
}

func (cp ComposeProject) GetDefaultColumn() string {
	return "PROJECT"
}

func (cp ComposeProject) GetDefaultSortColumn() string {
	return "PROJECT"
}

func (m *Manager) List() ([]common.Resource, error) {
	list, err := m.cli.ContainerList(m.ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	completedServices := make(map[string]map[string]struct{})
	for _, c := range list {
		project := c.Labels["com.docker.compose.project"]
		dependencies := c.Labels["com.docker.compose.depends_on"]
		if project == "" || dependencies == "" {
			continue
		}

		for dependency := range strings.SplitSeq(dependencies, ",") {
			parts := strings.Split(strings.TrimSpace(dependency), ":")
			if len(parts) < 2 || parts[1] != "service_completed_successfully" {
				continue
			}
			if completedServices[project] == nil {
				completedServices[project] = make(map[string]struct{})
			}
			completedServices[project][parts[0]] = struct{}{}
		}
	}

	type projData struct {
		total       int
		jobs        int
		running     int
		restarting  int
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

		service := c.Labels["com.docker.compose.service"]
		_, completedJob := completedServices[proj][service]
		if c.Labels["d4s.lifecycle"] == "job" || completedJob {
			projects[proj].jobs++
			continue
		}

		projects[proj].total++
		switch c.State {
		case "running":
			projects[proj].running++
		case "restarting":
			projects[proj].restarting++
		}
	}

	var res []common.Resource
	for name, data := range projects {
		status := "Ready"
		if data.running < data.total {
			status = "Degraded"
		} else if data.total == 0 {
			status = "Stopped"
		}

		res = append(res, ComposeProject{
			Name:        name,
			Status:      status,
			Ready:       fmt.Sprintf("%d/%d", data.running, data.total),
			ConfigFiles: data.config,
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
		if path == "" {
			continue
		}

		content, err := m.readConfigFile(path)
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

func (m *Manager) getConfigPaths(projectName string) ([]string, error) {
	args := filters.NewArgs()
	args.Add("label", fmt.Sprintf("com.docker.compose.project=%s", projectName))

	containers, err := m.cli.ContainerList(m.ctx, container.ListOptions{Filters: args, All: true, Limit: 1})
	if err != nil {
		return nil, err
	}

	if len(containers) == 0 {
		return nil, fmt.Errorf("project '%s' not found or has no active containers to read configuration from", projectName)
	}

	configFiles := containers[0].Labels["com.docker.compose.project.config_files"]
	if configFiles == "" {
		return nil, fmt.Errorf("no config files label found for project '%s'", projectName)
	}

	var paths []string
	for f := range strings.SplitSeq(configFiles, ",") {
		path := strings.TrimSpace(f)
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func (m *Manager) Logs(projectName string, since string, tail string, timestamps bool) (io.ReadCloser, error) {
	args := []string{"compose", "-p", projectName, "logs", "-f"}
	if tail != "" && tail != "all" {
		args = append(args, "--tail", tail)
	}
	if timestamps {
		args = append(args, "-t")
	}
	if since != "" {
		args = append(args, "--since", since)
	}

	cmd := m.dockerCmd(args, "")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	// Merge stderr into stdout
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &cmdReadCloser{pipe: stdout, cmd: cmd}, nil
}

func (m *Manager) Down(projectName string) error {
	cmd := m.dockerCmd([]string{"compose", "-p", projectName, "down"}, "")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running docker compose down: %v, output: %s", err, string(output))
	}
	return nil
}

func (m *Manager) Redeploy(projectName string) error {
	paths, err := m.getConfigPaths(projectName)
	if err != nil {
		return fmt.Errorf("failed to redeploy project: %v", err)
	}

	if err := m.Down(projectName); err != nil {
		return err
	}

	return m.up(projectName, paths, "--force-recreate")
}

func (m *Manager) Up(projectName string) error {
	paths, err := m.getConfigPaths(projectName)
	if err != nil {
		return fmt.Errorf("failed to up project: %v", err)
	}
	return m.up(projectName, paths, "--force-recreate")
}

func (m *Manager) Build(projectName string) error {
	paths, err := m.getConfigPaths(projectName)
	if err != nil {
		return fmt.Errorf("failed to build project: %v", err)
	}
	return m.up(projectName, paths, "--build")
}

func (m *Manager) up(projectName string, paths []string, extraFlag string) error {
	args := []string{"compose", "-p", projectName}
	for _, path := range paths {
		args = append(args, "-f", path)
	}
	args = append(args, "up", "-d", extraFlag)

	workDir := ""
	if len(paths) > 0 {
		workDir = filepath.Dir(paths[0])
	}

	cmd := m.dockerCmd(args, workDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running docker compose up: %v\nOutput: %s", err, string(output))
	}
	return nil
}

type cmdReadCloser struct {
	pipe io.ReadCloser
	cmd  *exec.Cmd
}

func (c *cmdReadCloser) Read(p []byte) (n int, err error) {
	return c.pipe.Read(p)
}

func (c *cmdReadCloser) Close() error {
	if c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
	return c.pipe.Close()
}
