package service

import (
	"fmt"
	"io"
	"strings"

	dt "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
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

func (m *Manager) List() ([]common.Resource, error) {
	list, err := m.cli.ServiceList(m.ctx, dt.ServiceListOptions{Status: true})
	if err != nil {
		return nil, err
	}

	var res []common.Resource
	for _, s := range list {
		mode := ""
		replicas := ""
		if s.Spec.Mode.Replicated != nil {
			mode = "Replicated"
			desired := uint64(0)
			if s.Spec.Mode.Replicated.Replicas != nil {
				desired = *s.Spec.Mode.Replicated.Replicas
			}
			running := uint64(0)
			if s.ServiceStatus != nil {
				running = s.ServiceStatus.RunningTasks
			}
			replicas = fmt.Sprintf("%d/%d", running, desired)
		} else if s.Spec.Mode.Global != nil {
			mode = "Global"
			running := uint64(0)
			if s.ServiceStatus != nil {
				running = s.ServiceStatus.RunningTasks
			}
			replicas = fmt.Sprintf("%d", running)
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

func (m *Manager) Scale(id string, replicas uint64) error {
	service, _, err := m.cli.ServiceInspectWithRaw(m.ctx, id, swarm.ServiceInspectOptions{})
	if err != nil {
		return err
	}

	if service.Spec.Mode.Replicated == nil {
		return fmt.Errorf("service is not in replicated mode")
	}

	service.Spec.Mode.Replicated.Replicas = &replicas
	
	// Update
	_, err = m.cli.ServiceUpdate(m.ctx, id, service.Version, service.Spec, swarm.ServiceUpdateOptions{})
	return err
}

func (m *Manager) Remove(id string) error {
	return m.cli.ServiceRemove(m.ctx, id)
}

func (m *Manager) Logs(id string, timestamps bool) (io.ReadCloser, error) {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "200",
		Timestamps: timestamps,
	}
	return m.cli.ServiceLogs(m.ctx, id, opts)
}
