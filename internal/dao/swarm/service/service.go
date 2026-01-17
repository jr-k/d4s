package service

import (
	"fmt"
	"io"
	"strings"

	dt "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao/common"
	"github.com/jr-k/d4s/internal/ui/styles"
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
	Created  string
	Updated  string
}

func (s Service) GetID() string { return s.ID }
func (s Service) GetCells() []string {
	return []string{s.ID[:12], s.Name, s.Image, s.Mode, s.Replicas, s.Ports, s.Created, s.Updated}
}

func (s Service) GetStatusColor() (tcell.Color, tcell.Color) {
	if strings.Contains(s.Replicas, "/") {
		parts := strings.Split(s.Replicas, "/")
		if len(parts) == 2 {
			var running, desired int
			fmt.Sscanf(parts[0], "%d", &running)
			fmt.Sscanf(parts[1], "%d", &desired)

			if desired == 0 && running == 0 {
				return styles.ColorStatusGray, styles.ColorBlack
			} else if running < desired {
				return styles.ColorStatusOrange, styles.ColorBlack
			} else if running > desired {
				return tcell.ColorMediumPurple, styles.ColorBlack
			} else if desired > 0 {
				return styles.ColorIdle, styles.ColorBlack
			}
		}
	}
	return styles.ColorIdle, styles.ColorBlack
}

func (s Service) GetColumnValue(column string) string {
	switch strings.ToLower(column) {
	case "id":
		return s.ID
	case "name":
		return s.Name
	case "image":
		return s.Image
	case "mode":
		return s.Mode
	case "replicas":
		return s.Replicas
	case "ports":
		return s.Ports
	case "created":
		return s.Created
	case "updated":
		return s.Updated
	}
	return ""
}

func (s Service) GetDefaultColumn() string {
	return "Name"
}

func (s Service) GetDefaultSortColumn() string {
	return "Name"
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
		
		// Style tag
		if idx := strings.LastIndex(imageName, ":"); idx != -1 {
			tag := imageName[idx:]
			if tag == ":latest" {
				imageName = imageName[:idx] + "[gray]" + tag + "[-]"
			}
		}

		res = append(res, Service{
			ID:       s.ID,
			Name:     s.Spec.Name,
			Image:    imageName,
			Mode:     mode,
			Replicas: replicas,
			Ports:    ports,
			Created:  common.FormatTime(s.CreatedAt.Unix()),
			Updated:  common.FormatTime(s.UpdatedAt.Unix()),
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

func (m *Manager) Logs(id string, since string, tail string, timestamps bool) (io.ReadCloser, error) {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Since:      since,
		Timestamps: timestamps,
	}
	// Service logs also use ContainerLogsOptions but passed to ServiceLogs
	if tail != "" {
		opts.Tail = tail
	} else if since == "" {
		opts.Tail = "200"
	} else {
		opts.Tail = "all"
	}
	return m.cli.ServiceLogs(m.ctx, id, opts)
}
