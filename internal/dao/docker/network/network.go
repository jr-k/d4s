package network

import (
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
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

func (m *Manager) List() ([]common.Resource, error) {
	list, err := m.cli.NetworkList(m.ctx, network.ListOptions{})
	if err != nil {
		return nil, err
	}

	var res []common.Resource
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

func (m *Manager) Create(name string) error {
	_, err := m.cli.NetworkCreate(m.ctx, name, network.CreateOptions{})
	return err
}

func (m *Manager) Remove(id string) error {
	return m.cli.NetworkRemove(m.ctx, id)
}

func (m *Manager) Prune() error {
	_, err := m.cli.NetworksPrune(m.ctx, filters.NewArgs())
	return err
}
