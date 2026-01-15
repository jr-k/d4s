package node

import (
	dt "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
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

// Node Model
type Node struct {
	ID       string
	Hostname string
	Status   string
	Avail    string
	Role     string
	Version  string
}

func (n Node) GetID() string { return n.ID }
func (n Node) GetCells() []string {
	return []string{n.ID[:12], n.Hostname, n.Status, n.Avail, n.Role, n.Version}
}

func (m *Manager) List() ([]common.Resource, error) {
	list, err := m.cli.NodeList(m.ctx, dt.NodeListOptions{})
	if err != nil {
		return nil, err
	}

	var res []common.Resource
	for _, n := range list {
		res = append(res, Node{
			ID:       n.ID,
			Hostname: n.Description.Hostname,
			Status:   string(n.Status.State),
			Avail:    string(n.Spec.Availability),
			Role:     string(n.Spec.Role),
			Version:  n.Description.Engine.EngineVersion,
		})
	}
	return res, nil
}

func (m *Manager) Remove(id string, force bool) error {
	// Force remove
	return m.cli.NodeRemove(m.ctx, id, swarm.NodeRemoveOptions{Force: force})
}

func (m *Manager) ListTasks(nodeID string) ([]swarm.Task, error) {
	filter := filters.NewArgs()
	filter.Add("node", nodeID)
	filter.Add("desired-state", "running")
	
	return m.cli.TaskList(m.ctx, dt.TaskListOptions{Filters: filter})
}
