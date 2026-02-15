package image

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"golang.org/x/net/context"
)

type Manager struct {
	cli *client.Client
	ctx context.Context
	
	pullStatuses map[string]string
	statusMu     sync.RWMutex
}

func NewManager(cli *client.Client, ctx context.Context) *Manager {
	return &Manager{
		cli:          cli,
		ctx:          ctx,
		pullStatuses: make(map[string]string),
	}
}

// Image Model
type Image struct {
	ID         string
	RepoTag    string
	Tags       string
	Size       string
	Created    string
	Containers int64
}

func (i Image) GetID() string { return i.ID }
func (i Image) GetCells() []string {
	containersStr := fmt.Sprintf("%d", i.Containers)
	if i.Containers <= 0 {
		containersStr = ""
	}
	return []string{i.ID[:12], i.Tags, i.Size, containersStr, i.Created}
}

func (i Image) GetStatusColor() (tcell.Color, tcell.Color) {
	if strings.Contains(i.Tags, "Pulling") {
		return styles.ColorStatusOrange, styles.ColorBlack
	}
	return styles.ColorIdle, styles.ColorBlack
}

func (i Image) GetColumnValue(column string) string {
	switch strings.ToLower(column) {
	case "id":
		return i.ID
	case "tags":
		return i.Tags
	case "size":
		return i.Size
	case "containers":
		return fmt.Sprintf("%d", i.Containers)
	case "created":
		return i.Created
	}
	return ""
}

func (i Image) GetDefaultColumn() string {
	return "Tags"
}

func (i Image) GetDefaultSortColumn() string {
	return "Tags" // Most recent first usually
}

func (m *Manager) SetPullStatus(tag string, status string) {
	if m == nil {
		return
	}
	m.statusMu.Lock()
	defer m.statusMu.Unlock()
	
	if m.pullStatuses == nil {
		m.pullStatuses = make(map[string]string)
	}

	if status == "" {
		delete(m.pullStatuses, tag)
	} else {
		m.pullStatuses[tag] = status
	}
}

func (m *Manager) GetPullStatus(tag string) string {
	if m == nil {
		return ""
	}
	m.statusMu.RLock()
	defer m.statusMu.RUnlock()
	
	if m.pullStatuses == nil {
		return ""
	}
	
	return m.pullStatuses[tag]
}

func (m *Manager) Pull(tag string) error {
	if m == nil || m.cli == nil {
		return fmt.Errorf("image manager not initialized")
	}

	m.SetPullStatus(tag, fmt.Sprintf(" [%s]⟳ Pulling...[-]", styles.ColorStatusOrange.String()))
	
	reader, err := m.cli.ImagePull(m.ctx, tag, image.PullOptions{})
	if err != nil {
		m.SetPullStatus(tag, fmt.Sprintf(" [%s]✘ Error[-]", styles.TagError))
		go func() {
			time.Sleep(5 * time.Second)
			m.SetPullStatus(tag, "")
		}()
		return err
	}
	defer reader.Close()

	// Consume output to wait for finish
	_, _ = io.Copy(io.Discard, reader)

	m.SetPullStatus(tag, fmt.Sprintf(" [%s]✔ Done[-]", styles.ColorSelect.String()))
	go func() {
		time.Sleep(5 * time.Second)
		m.SetPullStatus(tag, "")
	}()

	return nil
}

func (m *Manager) List() ([]common.Resource, error) {
	if m == nil || m.cli == nil {
		return nil, fmt.Errorf("image manager not initialized")
	}

	list, err := m.cli.ImageList(m.ctx, image.ListOptions{})
	if err != nil {
		return nil, err
	}

	var res []common.Resource
	for _, i := range list {
		tags := "<none>"
		rawTag := ""
		
		if len(i.RepoTags) > 0 {
			rawTag = i.RepoTags[0]
			parts := strings.SplitN(rawTag, ":", 2)
			if len(parts) == 2 {
				// Image Name: [cyan]name[-]:[white]tag[-]
				tags = fmt.Sprintf("%s:%s", parts[0], parts[1])
			} else {
				tags = rawTag
			}

			// Check Pull Status
			if status := m.GetPullStatus(rawTag); status != "" {
				tags += status
			}
		}
		res = append(res, Image{
			ID:         strings.TrimPrefix(i.ID, "sha256:"),
			RepoTag:    rawTag,
			Tags:       tags,
			Size:       common.FormatBytes(i.Size),
			Created:    common.FormatTime(i.Created),
			Containers: i.Containers,
		})
	}
	return res, nil
}

func (m *Manager) Remove(id string, force bool) error {
	_, err := m.cli.ImageRemove(m.ctx, id, image.RemoveOptions{Force: force, PruneChildren: true})
	return err
}

func (m *Manager) Prune() error {
	_, err := m.cli.ImagesPrune(m.ctx, filters.NewArgs())
	return err
}
