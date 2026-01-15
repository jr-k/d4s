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
	"github.com/docker/docker/client"
	"github.com/jr-k/d4s/internal/dao/common"
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
}

func (c Container) GetID() string { return c.ID }
func (c Container) GetCells() []string {
	id := c.ID
	if len(id) > 12 {
		id = id[:12]
	}
	return []string{id, c.Names, c.Image, c.Status, c.Age, c.Ports, c.CPU, c.Mem, c.Compose, c.Created}
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
					memStr = fmt.Sprintf("%.1f%% ([#6272a4]%s[-])", float64(mem)/float64(limit)*100.0, common.FormatBytes(int64(mem)))
				} else {
					memStr = fmt.Sprintf("0%% ([#6272a4]%s[-])", common.FormatBytes(int64(mem)))
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
			compose = "📄 " + common.ShortenPath(cf)
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

		res[i] = Container{
			ID:          c.ID,
			Names:       name,
			Image:       c.Image,
			Status:      status,
			Age:         age,
			State:       c.State,
			Ports:       ports,
			Created:     common.FormatTime(c.Created),
			Compose:     compose,
			ProjectName: projectName,
			CPU:         cpuStr,
			Mem:         memStr,
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
