package image

import (
	"fmt"
	"strings"

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
}

func NewManager(cli *client.Client, ctx context.Context) *Manager {
	return &Manager{cli: cli, ctx: ctx}
}

// Image Model
type Image struct {
	ID         string
	Tags       string
	Size       string
	Created    string
	Containers int64
}

func (i Image) GetID() string { return i.ID }
func (i Image) GetCells() []string {
	containersStr := fmt.Sprintf("%d", i.Containers)
	if i.Containers == 0 {
		containersStr = ""
	}
	return []string{i.ID[:12], i.Tags, i.Size, containersStr, i.Created}
}

func (i Image) GetStatusColor() (tcell.Color, tcell.Color) {
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

func (m *Manager) List() ([]common.Resource, error) {
	list, err := m.cli.ImageList(m.ctx, image.ListOptions{})
	if err != nil {
		return nil, err
	}

	var res []common.Resource
	for _, i := range list {
		tags := "<none>"
		if len(i.RepoTags) > 0 {
			t := i.RepoTags[0]
			parts := strings.SplitN(t, ":", 2)
			if len(parts) == 2 {
				// Image Name: [cyan]name[-]:[white]tag[-]
				// But tview formatting...
				tags = fmt.Sprintf("%s:%s", parts[0], parts[1])
			} else {
				tags = t
			}
		}
		res = append(res, Image{
			ID:         strings.TrimPrefix(i.ID, "sha256:"),
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
