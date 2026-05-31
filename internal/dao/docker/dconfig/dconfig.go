package dconfig

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao/common"
	"github.com/jr-k/d4s/internal/ui/styles"
)

type Manager struct {
	cli *client.Client
	ctx context.Context
}

func NewManager(cli *client.Client, ctx context.Context) *Manager {
	return &Manager{cli: cli, ctx: ctx}
}

type Config struct {
	ID       string
	Name     string
	Services int
	Created  string
	Updated  string
	Labels   string
}

func (c Config) GetID() string { return c.ID }
func (c Config) GetCells() []string {
	id := c.ID
	if len(id) > 12 {
		id = id[:12]
	}
	servicesStr := fmt.Sprintf("%d", c.Services)
	return []string{id, c.Name, servicesStr, c.Created, c.Updated, c.Labels}
}

func (c Config) GetStatusColor() (tcell.Color, tcell.Color) {
	return styles.ColorIdle, styles.ColorBlack
}

func (c Config) GetColumnValue(column string) string {
	switch strings.ToLower(column) {
	case "id":
		return c.ID
	case "name":
		return c.Name
	case "services":
		return fmt.Sprintf("%d", c.Services)
	case "created":
		return c.Created
	case "updated":
		return c.Updated
	case "labels":
		return c.Labels
	}
	return ""
}

func (c Config) GetDefaultColumn() string {
	return "Name"
}

func (c Config) GetDefaultSortColumn() string {
	return "Name"
}

func (m *Manager) List() ([]common.Resource, error) {
	list, err := m.cli.ConfigList(m.ctx, types.ConfigListOptions{})
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	services, err := m.cli.ServiceList(m.ctx, types.ServiceListOptions{})
	if err == nil {
		for _, svc := range services {
			if svc.Spec.TaskTemplate.ContainerSpec != nil {
				for _, cfgRef := range svc.Spec.TaskTemplate.ContainerSpec.Configs {
					counts[cfgRef.ConfigID]++
				}
			}
		}
	}

	var res []common.Resource
	for _, c := range list {
		labels := formatLabels(c.Spec.Labels)

		res = append(res, Config{
			ID:       c.ID,
			Name:     c.Spec.Name,
			Services: counts[c.ID],
			Created:  common.FormatTime(c.CreatedAt.Unix()),
			Updated:  common.FormatTime(c.UpdatedAt.Unix()),
			Labels:   labels,
		})
	}
	return res, nil
}

func (m *Manager) Remove(id string) error {
	return m.cli.ConfigRemove(m.ctx, id)
}

func (m *Manager) Create(name string, data []byte) error {
	_, err := m.cli.ConfigCreate(m.ctx, swarm.ConfigSpec{
		Annotations: swarm.Annotations{
			Name: name,
		},
		Data: data,
	})
	return err
}

func (m *Manager) Update(id string, data []byte) error {
	cfg, _, err := m.cli.ConfigInspectWithRaw(m.ctx, id)
	if err != nil {
		return err
	}

	cfg.Spec.Data = data

	return m.cli.ConfigUpdate(m.ctx, id, cfg.Version, cfg.Spec)
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "-"
	}
	var parts []string
	for k, v := range labels {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ", ")
}
