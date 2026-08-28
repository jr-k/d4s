package task

import (
	"context"
	"fmt"
	"strings"

	dt "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
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

type Task struct {
	ID           string
	Name         string
	Image        string
	Node         string
	DesiredState string
	CurrentState string
	Error        string
	ContainerID  string
	ServiceID    string
}

func (t Task) GetID() string { return t.ID }
func (t Task) GetCells() []string {
	id := t.ID
	if len(id) > 12 {
		id = id[:12]
	}
	containerID := t.ContainerID
	if len(containerID) > 12 {
		containerID = containerID[:12]
	}
	return []string{id, t.Name, t.Image, t.Node, t.DesiredState, t.CurrentState, t.Error, containerID}
}

func (t Task) GetStatusColor() (tcell.Color, tcell.Color) {
	switch strings.ToLower(t.CurrentState) {
	case "running":
		return styles.ColorIdle, styles.ColorBlack
	case "complete":
		return styles.ColorStatusGray, styles.ColorBlack
	case "failed", "rejected", "orphaned":
		return styles.ColorStatusRed, styles.ColorBlack
	case "preparing", "starting", "assigned", "accepted", "ready":
		return styles.ColorStatusBlue, styles.ColorBlack
	case "pending", "new":
		return styles.ColorStatusOrange, styles.ColorBlack
	case "shutdown", "remove":
		return styles.ColorStatusGray, styles.ColorBlack
	}
	return styles.ColorFg, styles.ColorBlack
}

func ucfirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (t Task) GetColumnValue(column string) string {
	switch strings.ToLower(column) {
	case "id":
		return t.ID
	case "name":
		return t.Name
	case "image":
		return t.Image
	case "node":
		return t.Node
	case "desired state":
		return t.DesiredState
	case "current state":
		return t.CurrentState
	case "error":
		return t.Error
	case "container":
		return t.ContainerID
	}
	return ""
}

func (t Task) GetDefaultColumn() string {
	return "Name"
}

func (t Task) GetDefaultSortColumn() string {
	return "Name"
}

func (m *Manager) List() ([]common.Resource, error) {
	tasks, err := m.cli.TaskList(m.ctx, dt.TaskListOptions{})
	if err != nil {
		return nil, err
	}

	nodes := m.resolveNodes()
	services := m.resolveServices()

	return m.toResources(tasks, nodes, services), nil
}

func (m *Manager) ListForService(serviceID string) ([]common.Resource, error) {
	filter := filters.NewArgs()
	filter.Add("service", serviceID)

	tasks, err := m.cli.TaskList(m.ctx, dt.TaskListOptions{Filters: filter})
	if err != nil {
		return nil, err
	}

	nodes := m.resolveNodes()
	services := m.resolveServices()

	return m.toResources(tasks, nodes, services), nil
}

func (m *Manager) ListForNode(nodeID string) ([]common.Resource, error) {
	filter := filters.NewArgs()
	filter.Add("node", nodeID)

	tasks, err := m.cli.TaskList(m.ctx, dt.TaskListOptions{Filters: filter})
	if err != nil {
		return nil, err
	}

	nodes := m.resolveNodes()
	services := m.resolveServices()

	return m.toResources(tasks, nodes, services), nil
}

func (m *Manager) ListForServiceAndNode(serviceID, nodeID string) ([]common.Resource, error) {
	filter := filters.NewArgs()
	filter.Add("service", serviceID)
	filter.Add("node", nodeID)

	tasks, err := m.cli.TaskList(m.ctx, dt.TaskListOptions{Filters: filter})
	if err != nil {
		return nil, err
	}

	nodes := m.resolveNodes()
	services := m.resolveServices()

	return m.toResources(tasks, nodes, services), nil
}

func (m *Manager) toResources(tasks []swarm.Task, nodes map[string]string, services map[string]string) []common.Resource {
	var res []common.Resource
	for _, t := range tasks {
		imageName := ""
		if t.Spec.ContainerSpec != nil {
			imageName = t.Spec.ContainerSpec.Image
			if idx := strings.LastIndex(imageName, "@"); idx != -1 {
				imageName = imageName[:idx]
			}
		}

		nodeName := t.NodeID
		if name, ok := nodes[t.NodeID]; ok {
			nodeName = name
		}

		serviceName := t.ServiceID
		if name, ok := services[t.ServiceID]; ok {
			serviceName = name
		}

		name := fmt.Sprintf("%s.%d", serviceName, t.Slot)

		containerID := ""
		if t.Status.ContainerStatus != nil {
			containerID = t.Status.ContainerStatus.ContainerID
		}

		currentState := ucfirst(string(t.Status.State))

		errMsg := t.Status.Err
		if len(errMsg) > 50 {
			errMsg = errMsg[:50] + "…"
		}

		res = append(res, Task{
			ID:           t.ID,
			Name:         name,
			Image:        imageName,
			Node:         nodeName,
			DesiredState: ucfirst(string(t.DesiredState)),
			CurrentState: currentState,
			Error:        errMsg,
			ContainerID:  containerID,
			ServiceID:    t.ServiceID,
		})
	}
	return res
}

func (m *Manager) resolveNodes() map[string]string {
	nodes := make(map[string]string)
	list, err := m.cli.NodeList(m.ctx, dt.NodeListOptions{})
	if err != nil {
		return nodes
	}
	for _, n := range list {
		nodes[n.ID] = n.Description.Hostname
	}
	return nodes
}

func (m *Manager) resolveServices() map[string]string {
	services := make(map[string]string)
	list, err := m.cli.ServiceList(m.ctx, dt.ServiceListOptions{})
	if err != nil {
		return services
	}
	for _, s := range list {
		services[s.ID] = s.Spec.Name
	}
	return services
}
