package network

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
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

// Network Model
type Network struct {
	ID         string
	Name       string
	Driver     string
	Scope      string
	Created    string
	Internal   string
	Subnet     string
	Containers int
}

func (n Network) GetID() string { return n.ID }
func (n Network) GetCells() []string {
	containersStr := fmt.Sprintf("%d", n.Containers)
	return []string{n.ID[:12], n.Name, n.Driver, n.Scope, containersStr, n.Created, n.Internal, n.Subnet}
}

func (n Network) GetStatusColor() (tcell.Color, tcell.Color) {
	return styles.ColorIdle, styles.ColorBlack
}

func (n Network) GetColumnValue(column string) string {
	switch strings.ToLower(column) {
	case "id":
		return n.ID
	case "name":
		return n.Name
	case "driver":
		return n.Driver
	case "scope":
		return n.Scope
	case "containers":
		return fmt.Sprintf("%d", n.Containers)
	case "created":
		return n.Created
	case "internal":
		return n.Internal
	case "subnet":
		return n.Subnet
	}
	return ""
}

func (n Network) GetDefaultColumn() string {
	return "Name"
}

func (n Network) GetDefaultSortColumn() string {
	return "Name"
}

func (m *Manager) List() ([]common.Resource, error) {
	list, err := m.cli.NetworkList(m.ctx, network.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Count containers per network
	counts := make(map[string]int)
	containers, err := m.cli.ContainerList(m.ctx, container.ListOptions{All: true})
	// Debug: force error check visibility
	if err != nil {
		// In a real app we might log this or handle it, but here we just ignore
	} else {
		for _, c := range containers {
			if c.NetworkSettings != nil {
				for _, net := range c.NetworkSettings.Networks {
					if net.NetworkID != "" {
						counts[net.NetworkID]++
					}
				}
			}
		}
	}

	var res []common.Resource
	for _, n := range list {
		internal := "No"
		if n.Internal {
			internal = "Yes"
		}
		var subnets []string
		for _, conf := range n.IPAM.Config {
			if conf.Subnet != "" {
				s := conf.Subnet
				if parts := strings.Split(s, "/"); len(parts) == 2 {
					s = fmt.Sprintf("%s/%s", parts[0], parts[1])
				}
				subnets = append(subnets, s)
			}
		}

		count := counts[n.ID]
		if len(n.Containers) > count {
			count = len(n.Containers)
		}

		res = append(res, Network{
			ID:         n.ID,
			Name:       n.Name,
			Driver:     n.Driver,
			Scope:      n.Scope,
			Created:    common.FormatTime(n.Created.Unix()),
			Internal:   internal,
			Subnet:     strings.Join(subnets, ","),
			Containers: count,
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

func (m *Manager) Connect(networkID, containerID string) error {
	return m.cli.NetworkConnect(m.ctx, networkID, containerID, nil)
}

func (m *Manager) Disconnect(networkID, containerID string) error {
	return m.cli.NetworkDisconnect(m.ctx, networkID, containerID, true)
}
