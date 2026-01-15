package image

import (
	"strings"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
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

// Image Model
type Image struct {
	ID      string
	Tags    string
	Size    string
	Created string
}

func (i Image) GetID() string { return i.ID }
func (i Image) GetCells() []string {
	return []string{i.ID[:12], i.Tags, i.Size, i.Created}
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
			tags = i.RepoTags[0]
		}
		res = append(res, Image{
			ID:      strings.TrimPrefix(i.ID, "sha256:"),
			Tags:    tags,
			Size:    common.FormatBytes(i.Size),
			Created: common.FormatTime(i.Created),
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
