package volume

import (
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
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

func (m *Manager) List() ([]common.Resource, error) {
	list, err := m.cli.VolumeList(m.ctx, volume.ListOptions{})
	if err != nil {
		return nil, err
	}

	var res []common.Resource
	for _, v := range list.Volumes {
		res = append(res, Volume{
			Name:   v.Name,
			Driver: v.Driver,
			Mount:  common.ShortenPath(v.Mountpoint),
		})
	}
	return res, nil
}

func (m *Manager) Create(name string) error {
	_, err := m.cli.VolumeCreate(m.ctx, volume.CreateOptions{
		Name: name,
	})
	return err
}

func (m *Manager) Remove(id string, force bool) error {
	return m.cli.VolumeRemove(m.ctx, id, force)
}

func (m *Manager) Prune() error {
	_, err := m.cli.VolumesPrune(m.ctx, filters.NewArgs())
	return err
}
