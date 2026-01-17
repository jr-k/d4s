package container

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"golang.org/x/net/context"
)

type CachedStats struct {
	CPU string
	Mem string
	TS  time.Time
}

type Manager struct {
	cli *client.Client
	ctx context.Context
	
	statsCache map[string]CachedStats
	statsMutex sync.RWMutex
	updating   int32
}

func NewManager(cli *client.Client, ctx context.Context) *Manager {
	return &Manager{
		cli: cli, 
		ctx: ctx,
		statsCache: make(map[string]CachedStats),
	}
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
	IP          string
	Cmd         string
}

func (c Container) GetID() string { return c.ID }
func (c Container) GetCells() []string {
	id := c.ID
	if len(id) > 12 {
		id = id[:12]
	}
	return []string{id, c.Names, c.Image, c.IP, c.Status, c.Age, c.Ports, c.CPU, c.Mem, c.Compose, c.Cmd, c.Created}
}

func (c Container) GetStatusColor() (tcell.Color, tcell.Color) {
	lower := strings.ToLower(c.State)

	// Fallback to parsed status if State is generic
	if strings.Contains(strings.ToLower(c.Status), "starting") {
		return styles.ColorStatusBlue, styles.ColorBlack
	}

	switch lower {
	case "running":
		if strings.Contains(strings.ToLower(c.Status), "healthy") {
			//return styles.ColorStatusGreen, styles.ColorBlack
		} else if strings.Contains(strings.ToLower(c.Status), "unhealthy") {
			return styles.ColorStatusRed, styles.ColorBlack
		}
	case "paused":
		return styles.ColorStatusYellow, styles.ColorBlack
	case "restarting":
		return styles.ColorStatusOrange, styles.ColorBlack
	case "exited", "dead":
		return styles.ColorStatusGray, styles.ColorBlack
	case "created":
		return styles.ColorStatusBlue, styles.ColorBlack
	}

	return styles.ColorIdle, styles.ColorBlack
}

func (c Container) GetColumnValue(column string) string {
	switch strings.ToLower(column) {
	case "id":
		return c.ID
	case "names":
		return c.Names
	case "image":
		return c.Image
	case "ip":
		return c.IP
	case "status":
		return c.Status
	case "age":
		return c.Age
	case "ports":
		return c.Ports
	case "cpu":
		return c.CPU
	case "mem":
		return c.Mem
	case "compose":
		return c.Compose
	case "cmd":
		return c.Cmd
	case "created":
		return c.Created
	}
	return ""
}

func (c Container) GetDefaultColumn() string {
	return "Name"
}

func (c Container) GetDefaultSortColumn() string {
	return "Name"
}

func (m *Manager) updateStats(containers []types.Container) {
	if !atomic.CompareAndSwapInt32(&m.updating, 0, 1) {
		return
	}
	
	// Create a detached operation, do not block caller
	go func() {
		defer atomic.StoreInt32(&m.updating, 0)
		
		var wg sync.WaitGroup
		sem := make(chan struct{}, 5) // Limit concurrency

		for _, c := range containers {
			if c.State != "running" {
				continue
			}

			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				statsResp, err := m.cli.ContainerStats(m.ctx, id, false)
				if err != nil {
					return
				}
				
				cpuPct, mem, limit := common.CalculateContainerStats(statsResp.Body)
				
				cpuStr := fmt.Sprintf("%.1f%%", cpuPct)
				memStr := ""
				if limit > 0 {
					memStr = fmt.Sprintf("%6.1f%% (%s)", float64(mem)/float64(limit)*100.0, common.FormatBytesFixed(int64(mem)))
				} else {
					memStr = fmt.Sprintf("%6.1f%% (%s)", 0.0, common.FormatBytesFixed(int64(mem)))
				}

				m.statsMutex.Lock()
				m.statsCache[id] = CachedStats{
					CPU: cpuStr,
					Mem: memStr,
					TS:  time.Now(),
				}
				m.statsMutex.Unlock()
			}(c.ID)
		}
		wg.Wait()
	}()
}

func (m *Manager) List() ([]common.Resource, error) {
	list, err := m.cli.ContainerList(m.ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	// Trigger async update
	m.updateStats(list)

	res := make([]common.Resource, len(list))
	for i, c := range list {
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
			compose = common.ShortenPath(cf)
		} else if proj, ok := c.Labels["com.docker.compose.project"]; ok {
			compose = "📦 " + proj
		}
		
		projectName := c.Labels["com.docker.compose.project"]

		status, age := common.ParseStatus(c.Status)

		// Fetch Stats from Cache
		cpuStr := "..."
		memStr := "..."
		
		if c.State != "running" {
			cpuStr = "-"
			memStr = "-"
		} else {
			m.statsMutex.RLock()
			if s, ok := m.statsCache[c.ID]; ok {
				// Expire cache after 15 seconds if needed, but here we just use it
				cpuStr = s.CPU
				memStr = s.Mem
			}
			m.statsMutex.RUnlock()
		}

		ip := ""
		if c.NetworkSettings != nil {
			for _, n := range c.NetworkSettings.Networks {
				if n.IPAddress != "" {
					ip = n.IPAddress
					break
				}
			}
		}

		cmd := c.Command
		// if len(cmd) > 20 {
		// 	cmd = cmd[:20] + "..."
		// }
		cmd = fmt.Sprintf("%s", cmd)

		imageName := c.Image
		parts := strings.SplitN(imageName, ":", 2)
		if len(parts) == 2 {
			imageName = fmt.Sprintf("%s:%s", parts[0], parts[1])
		}

		res[i] = Container{
			ID:          c.ID,
			Names:       name,
			Image:       imageName,
			Status:      status,
			Age:         age,
			State:       c.State,
			Ports:       ports,
			Created:     common.FormatTime(c.Created),
			Compose:     compose,
			ProjectName: projectName,
			CPU:         cpuStr,
			Mem:         memStr,
			IP:          ip,
			Cmd:         cmd,
		}
	}
	return res, nil
}

func (m *Manager) Stop(id string) error {
	timeout := 10 // seconds
	return m.cli.ContainerStop(m.ctx, id, container.StopOptions{Timeout: &timeout})
}

func (m *Manager) Start(id string) error {
	return m.cli.ContainerStart(m.ctx, id, container.StartOptions{})
}

func (m *Manager) Restart(id string) error {
	timeout := 10 // seconds
	return m.cli.ContainerRestart(m.ctx, id, container.StopOptions{Timeout: &timeout})
}

func (m *Manager) Remove(id string, force bool) error {
	return m.cli.ContainerRemove(m.ctx, id, container.RemoveOptions{Force: force})
}

func (m *Manager) Prune() error {
	_, err := m.cli.ContainersPrune(m.ctx, filters.NewArgs())
	return err
}

func (m *Manager) Logs(id string, since string, tail string, timestamps bool) (io.ReadCloser, error) {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Since:      since,
		Timestamps: timestamps,
	}
	if tail != "" {
		opts.Tail = tail
	} else if since == "" {
		opts.Tail = "200"
	} else {
		opts.Tail = "all"
	}
	return m.cli.ContainerLogs(m.ctx, id, opts)
}

func (m *Manager) GetEnv(id string) ([]string, error) {
	c, err := m.cli.ContainerInspect(m.ctx, id)
	if err != nil {
		return nil, err
	}
	return c.Config.Env, nil
}
