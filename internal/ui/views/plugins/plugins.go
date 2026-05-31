package plugins

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
	"github.com/jr-k/d4s/internal/ui/styles"
)

var Headers = []string{"ID", "NAME", "ENABLED", "DESCRIPTION"}

type Plugin struct {
	ID          string
	Name        string
	Enabled     bool
	Description string
}

func (p Plugin) GetID() string { return p.ID }
func (p Plugin) GetCells() []string {
	enabled := "False"
	if p.Enabled {
		enabled = "True"
	}
	return []string{p.ID, p.Name, enabled, p.Description}
}

func (p Plugin) GetStatusColor() (tcell.Color, tcell.Color) {
	if p.Enabled {
		return styles.ColorIdle, styles.ColorBlack
	}
	return styles.ColorStatusGray, styles.ColorBlack
}

func (p Plugin) GetColumnValue(column string) string {
	switch strings.ToLower(column) {
	case "id":
		return p.ID
	case "name":
		return p.Name
	case "enabled":
		if p.Enabled {
			return "True"
		}
		return "False"
	case "description":
		return p.Description
	}
	return ""
}

func (p Plugin) GetDefaultColumn() string {
	return "Name"
}

func (p Plugin) GetDefaultSortColumn() string {
	return "Name"
}

func Fetch(app common.AppController, v *view.ResourceView) ([]dao.Resource, error) {
	list, err := app.GetDocker().ListPlugins()
	if err != nil {
		return nil, err
	}

	var res []dao.Resource
	for _, p := range list {
		res = append(res, Plugin{
			ID:          p.ID,
			Name:        p.Name,
			Enabled:     p.Enabled,
			Description: p.Description,
		})
	}
	return res, nil
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("e", "Enable/Disable"),
		common.FormatSCHeader("a", "Add"),
		common.FormatSCHeader("ctrl-d", "Delete"),
	}
}

func InputHandler(v *view.ResourceView, event *tcell.EventKey) *tcell.EventKey {
	app := v.App

	if event.Key() == tcell.KeyCtrlD {
		DeleteAction(app, v)
		return nil
	}

	switch event.Rune() {
	case 'd':
		app.InspectCurrentSelection()
		return nil
	case 'e':
		ToggleAction(app, v)
		return nil
	case 'a':
		Install(app)
		return nil
	}

	return event
}

func DeleteAction(app common.AppController, v *view.ResourceView) {
	ids, err := v.GetSelectedIDs()
	if err != nil {
		return
	}

	label := ids[0]
	if len(ids) == 1 {
		row, _ := v.Table.GetSelection()
		if row > 0 && row <= len(v.Data) {
			item := v.Data[row-1]
			if item.GetID() == ids[0] {
				cells := item.GetCells()
				if len(cells) > 1 {
					label = fmt.Sprintf("%s ([%s]%s[yellow])", label, styles.TagCyan, cells[1])
				}
			}
		}
	} else {
		label = fmt.Sprintf("%d items", len(ids))
	}

	dialogs.ShowConfirmation(app, "DELETE", label, func(force bool) {
		simpleAction := func(id string) error {
			return Remove(id, force, app)
		}
		app.PerformAction(simpleAction, "deleting", styles.ColorStatusRed)
	})
}

func Remove(id string, force bool, app common.AppController) error {
	return app.GetDocker().RemovePlugin(id, force)
}

func ToggleAction(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	row, _ := v.Table.GetSelection()
	if row <= 0 || row > len(v.Data) {
		return
	}

	p, ok := v.Data[row-1].(Plugin)
	if !ok {
		return
	}

	if p.Enabled {
		app.AppendFlashPending(fmt.Sprintf("disabling plugin %s...", p.Name))
		app.RunInBackground(func() {
			err := app.GetDocker().DisablePlugin(id)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.AppendFlashError(fmt.Sprintf("failed to disable plugin: %v", err))
				} else {
					app.AppendFlashSuccess(fmt.Sprintf("plugin %s disabled", p.Name))
				}
				app.RefreshCurrentView()
			})
		})
	} else {
		app.AppendFlashPending(fmt.Sprintf("enabling plugin %s...", p.Name))
		app.RunInBackground(func() {
			err := app.GetDocker().EnablePlugin(id)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.AppendFlashError(fmt.Sprintf("failed to enable plugin: %v", err))
				} else {
					app.AppendFlashSuccess(fmt.Sprintf("plugin %s enabled", p.Name))
				}
				app.RefreshCurrentView()
			})
		})
	}
}

func Install(app common.AppController) {
	fields := []dialogs.FormField{
		{Name: "name", Label: "Plugin name", Type: dialogs.FieldTypeInput},
	}

	dialogs.ShowFormWithDescription(app, "Install Plugin", "Enter plugin name (e.g. vieux/sshfs)", fields, func(result dialogs.FormResult) {
		name := result["name"]

		if name == "" {
			app.SetFlashError("plugin name is required")
			return
		}

		app.AppendFlashPending(fmt.Sprintf("installing plugin %s...", name))

		app.RunInBackground(func() {
			err := app.GetDocker().InstallPlugin(name)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.AppendFlashError(fmt.Sprintf("failed to install plugin: %v", err))
				} else {
					app.AppendFlashSuccess(fmt.Sprintf("plugin %s installed", name))
				}
				app.RefreshCurrentView()
			})
		})
	})
}

func Inspect(app common.AppController, id string) {
	inspector := inspect.NewTextInspector("Describe plugin", id, fmt.Sprintf(" [%s]Loading plugin...\n", styles.TagAccent), "json")
	app.OpenInspector(inspector)

	app.RunInBackground(func() {
		content, err := app.GetDocker().InspectPlugin(id)
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() {
				inspector.Viewer.Update(fmt.Sprintf("Error: %v", err), "text")
			})
			return
		}

		app.GetTviewApp().QueueUpdateDraw(func() {
			inspector.Viewer.Update(content, "json")
		})
	})
}
