package secret

import (
	"context"
	"strings"

	"github.com/docker/docker/api/types"
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

// Secret Model
type Secret struct {
	ID      string
	Name    string
	Created string
	Updated string
	Labels  string
}

func (s Secret) GetID() string { return s.ID }
func (s Secret) GetCells() []string {
	id := s.ID
	if len(id) > 12 {
		id = id[:12]
	}
	return []string{id, s.Name, s.Created, s.Updated, s.Labels}
}

func (s Secret) GetStatusColor() (tcell.Color, tcell.Color) {
	return styles.ColorIdle, styles.ColorBlack
}

func (s Secret) GetColumnValue(column string) string {
	switch strings.ToLower(column) {
	case "id":
		return s.ID
	case "name":
		return s.Name
	case "created":
		return s.Created
	case "updated":
		return s.Updated
	case "labels":
		return s.Labels
	}
	return ""
}

func (s Secret) GetDefaultColumn() string {
	return "Name"
}

func (s Secret) GetDefaultSortColumn() string {
	return "Name"
}

func (m *Manager) List() ([]common.Resource, error) {
	list, err := m.cli.SecretList(m.ctx, types.SecretListOptions{})
	if err != nil {
		return nil, err
	}

	var res []common.Resource
	for _, s := range list {
		labels := formatLabels(s.Spec.Labels)

		res = append(res, Secret{
			ID:      s.ID,
			Name:    s.Spec.Name,
			Created: common.FormatTime(s.CreatedAt.Unix()),
			Updated: common.FormatTime(s.UpdatedAt.Unix()),
			Labels:  labels,
		})
	}
	return res, nil
}

func (m *Manager) Remove(id string) error {
	return m.cli.SecretRemove(m.ctx, id)
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
