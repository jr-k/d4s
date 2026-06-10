package portforward

import (
	"fmt"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao/common"
	"github.com/jr-k/d4s/internal/ui/styles"
)

type Status int

const (
	StatusRunning Status = iota
	StatusStopped
)

type PortForward struct {
	ID            string
	ContextName   string
	SSHHost       string
	ContainerID   string
	ContainerName string
	ContainerIP   string
	ContainerPort uint16
	HostPort      uint16
	LocalPort     uint16
	Status        Status
	CreatedAt     time.Time

	tunnel *Tunnel
}

func (pf PortForward) GetID() string { return pf.ID }

func (pf *PortForward) remoteTarget() (string, uint16) {
	if pf.HostPort > 0 {
		return "127.0.0.1", pf.HostPort
	}
	return pf.ContainerIP, pf.ContainerPort
}

func (pf PortForward) GetCells() []string {
	status := "●"
	if pf.Status == StatusStopped {
		status = "○"
	}
	remoteIP, remotePort := pf.remoteTarget()
	return []string{
		status,
		pf.ContextName,
		pf.ContainerName,
		fmt.Sprintf("localhost:%d", pf.LocalPort),
		fmt.Sprintf("%s:%d", remoteIP, remotePort),
		formatAge(pf.CreatedAt),
	}
}

func (pf PortForward) GetStatusColor() (tcell.Color, tcell.Color) {
	if pf.Status == StatusRunning {
		return styles.ColorInfo, styles.ColorBlack
	}
	return styles.ColorStatusGray, styles.ColorBlack
}

func (pf PortForward) GetColumnValue(column string) string {
	switch column {
	case "status":
		if pf.Status == StatusRunning {
			return "running"
		}
		return "stopped"
	case "context":
		return pf.ContextName
	case "container":
		return pf.ContainerName
	case "local":
		return fmt.Sprintf("localhost:%d", pf.LocalPort)
	case "remote":
		remoteIP, remotePort := pf.remoteTarget()
		return fmt.Sprintf("%s:%d", remoteIP, remotePort)
	case "age":
		return formatAge(pf.CreatedAt)
	}
	return ""
}

func (pf PortForward) GetDefaultColumn() string      { return "container" }
func (pf PortForward) GetDefaultSortColumn() string  { return "container" }

var _ common.Resource = PortForward{}

type Manager struct {
	mu       sync.RWMutex
	forwards map[string]*PortForward
}

func NewManager() *Manager {
	return &Manager{
		forwards: make(map[string]*PortForward),
	}
}

func (m *Manager) Add(pf *PortForward) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, err := NewTunnel(pf.SSHHost, pf.LocalPort, pf.ContainerID, pf.ContainerPort, pf.HostPort)
	if err != nil {
		return fmt.Errorf("tunnel creation failed: %w", err)
	}

	remoteIP, remotePort := pf.remoteTarget()

	pf.tunnel = tunnel
	pf.Status = StatusRunning
	pf.CreatedAt = time.Now()
	pf.ID = fmt.Sprintf("%s:%d->%s:%d", pf.ContextName, pf.LocalPort, remoteIP, remotePort)
	m.forwards[pf.ID] = pf

	return nil
}

func (m *Manager) Stop(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pf, ok := m.forwards[id]; ok && pf.tunnel != nil {
		pf.tunnel.Close()
		pf.Status = StatusStopped
	}
}

func (m *Manager) Start(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pf, ok := m.forwards[id]
	if !ok {
		return fmt.Errorf("port-forward %s not found", id)
	}

	tunnel, err := NewTunnel(pf.SSHHost, pf.LocalPort, pf.ContainerID, pf.ContainerPort, pf.HostPort)
	if err != nil {
		return fmt.Errorf("tunnel creation failed: %w", err)
	}

	pf.tunnel = tunnel
	pf.Status = StatusRunning
	return nil
}

func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pf, ok := m.forwards[id]; ok {
		if pf.tunnel != nil {
			pf.tunnel.Close()
		}
		delete(m.forwards, id)
	}
}

func (m *Manager) List() []common.Resource {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]common.Resource, 0, len(m.forwards))
	for _, pf := range m.forwards {
		result = append(result, *pf)
	}
	return result
}

func (m *Manager) GetForContainer(containerID string) *PortForward {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, pf := range m.forwards {
		if pf.ContainerID == containerID && pf.Status == StatusRunning {
			return pf
		}
	}
	return nil
}

func (m *Manager) HasActiveForwards() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.forwards) > 0
}

func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, pf := range m.forwards {
		if pf.tunnel != nil {
			pf.tunnel.Close()
		}
	}
	m.forwards = make(map[string]*PortForward)
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
