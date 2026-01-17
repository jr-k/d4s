package aliases

import (
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/styles"
)

var Headers = []string{"RESOURCE", "GROUP"}

type Alias struct {
	Title    string // The view title/key (page name)
	Resource string // Display name
	Group    string // Group name
}

// Ensure Alias implements dao.Resource
var _ dao.Resource = Alias{}

func (a Alias) GetID() string {
	return a.Title
}

func (a Alias) GetCells() []string {
	return []string{a.Resource, a.Group}
}

func (a Alias) GetStatusColor() (tcell.Color, tcell.Color) {
	return styles.ColorIdle, styles.ColorBlack
}

func (a Alias) GetColumnValue(columnName string) string {
	switch columnName {
	case "RESOURCE":
		return a.Resource
	case "GROUP":
		return a.Group
	}
	return ""
}

func (a Alias) GetDefaultColumn() string {
	return "RESOURCE"
}

func (a Alias) GetDefaultSortColumn() string {
	return "RESOURCE"
}

func Fetch(app common.AppController) ([]dao.Resource, error) {
	// Static list of aliases
	aliases := []Alias{
		{Title: styles.TitleContainers, Resource: "aliases", Group: "internal"},
		{Title: styles.TitleContainers, Resource: "containers", Group: "docker"},
		{Title: styles.TitleImages, Resource: "images", Group: "docker"},
		{Title: styles.TitleVolumes, Resource: "volumes", Group: "docker"},
		{Title: styles.TitleNetworks, Resource: "networks", Group: "docker"},
		{Title: styles.TitleServices, Resource: "services", Group: "swarm"},
		{Title: styles.TitleNodes, Resource: "nodes", Group: "swarm"},
		{Title: styles.TitleCompose, Resource: "compose", Group: "compose"},
	}

	var resources []dao.Resource
	for _, a := range aliases {
		resources = append(resources, a)
	}
	return resources, nil
}

func GetShortcuts() []string {
	return []string{}
}

func InputHandler(v *view.ResourceView, event *tcell.EventKey) *tcell.EventKey {
	return event
}
