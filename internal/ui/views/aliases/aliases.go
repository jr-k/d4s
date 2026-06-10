package aliases

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/styles"
)

var Headers = []string{"RESOURCE", "GROUP", "SHORTCUTS"}

type Alias struct {
	Title     string   // The view title/key (page name)
	Resource  string   // Display name
	Group     string   // Group name
	Shortcuts []string // Command-line shortcuts (without the leading ':')
}

// Ensure Alias implements dao.Resource
var _ dao.Resource = Alias{}

func (a Alias) GetID() string {
	return a.Title
}

func (a Alias) GetCells() []string {
	return []string{a.Resource, a.Group, strings.Join(a.Shortcuts, ", ")}
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
	case "SHORTCUTS":
		return strings.Join(a.Shortcuts, ", ")
	}
	return ""
}

func (a Alias) GetDefaultColumn() string {
	return "RESOURCE"
}

func (a Alias) GetDefaultSortColumn() string {
	return "RESOURCE"
}

func Fetch(app common.AppController, v *view.ResourceView) ([]dao.Resource, error) {
	// Static list of aliases. Shortcuts mirror the cases handled in
	// (*App).ExecuteCmd; keep them in sync when adding new commands.
	aliases := []Alias{
		{Title: styles.TitleAliases, Resource: "aliases", Group: "internal", Shortcuts: []string{"a", "al", "alias", "aliases"}},
		{Title: styles.TitleContainers, Resource: "containers", Group: "docker", Shortcuts: []string{"c", "co", "con", "container", "containers"}},
		{Title: styles.TitleImages, Resource: "images", Group: "docker", Shortcuts: []string{"i", "im", "img", "image", "images"}},
		{Title: styles.TitleVolumes, Resource: "volumes", Group: "docker", Shortcuts: []string{"v", "vo", "vol", "volume", "volumes"}},
		{Title: styles.TitleNetworks, Resource: "networks", Group: "docker", Shortcuts: []string{"n", "ne", "net", "network", "networks"}},
		{Title: styles.TitleServices, Resource: "services", Group: "swarm", Shortcuts: []string{"s", "se", "svc", "service", "services"}},
		{Title: styles.TitleNodes, Resource: "nodes", Group: "swarm", Shortcuts: []string{"d", "no", "node", "nodes"}},
		{Title: styles.TitleSecrets, Resource: "secrets", Group: "swarm", Shortcuts: []string{"x", "sec", "secret", "secrets"}},
		{Title: styles.TitleConfigs, Resource: "configmaps", Group: "swarm", Shortcuts: []string{"m", "cm", "configmap", "configmaps"}},
		{Title: styles.TitleStacks, Resource: "stacks", Group: "swarm", Shortcuts: []string{"k", "st", "stack", "stacks"}},
		{Title: styles.TitleTasks, Resource: "tasks", Group: "swarm", Shortcuts: []string{"t", "task", "tasks"}},
		{Title: styles.TitleContexts, Resource: "contexts", Group: "docker", Shortcuts: []string{"o", "ctx", "context", "contexts"}},
		{Title: styles.TitlePlugins, Resource: "plugins", Group: "docker", Shortcuts: []string{"g", "pl", "plugin", "plugins"}},
		{Title: styles.TitleCompose, Resource: "compose", Group: "compose", Shortcuts: []string{"p", "cp", "compose", "project", "projects"}},
		{Title: styles.TitlePortForwards, Resource: "portforwards", Group: "internal", Shortcuts: []string{"w", "pf", "portforward", "portforwards"}},
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
	switch event.Key() {
	case tcell.KeyEnter:
		id, err := v.GetSelectedID()
		if err == nil {
			v.App.SwitchTo(id)
			return nil
		}
	}
	return event
}
