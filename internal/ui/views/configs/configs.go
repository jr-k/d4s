package configs

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
	"github.com/jr-k/d4s/internal/ui/styles"
)

var Headers = []string{"ID", "NAME", "SERVICES", "CREATED", "UPDATED", "LABELS"}

func Fetch(app common.AppController, v *view.ResourceView) ([]dao.Resource, error) {
	return app.GetDocker().ListConfigs()
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("s", "Services"),
		common.FormatSCHeader("d", "Describe"),
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
	case 's':
		ViewServices(app, v)
		return nil
	case 'd':
		app.InspectCurrentSelection()
		return nil
	case 'a':
		Create(app)
		return nil
	}

	return event
}

func ViewServices(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	name := id
	row, _ := v.Table.GetSelection()
	if row > 0 {
		index := row - 1
		if index >= 0 && index < len(v.Data) {
			if c, ok := v.Data[index].(dao.Config); ok {
				name = c.Name
			}
		}
	}

	app.SetActiveScope(&common.Scope{
		Type:       "config",
		Value:      id,
		Label:      name,
		OriginView: styles.TitleConfigs,
	})

	app.SwitchTo(styles.TitleServices)
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
	return app.GetDocker().RemoveConfig(id)
}

func Create(app common.AppController) {
	fields := []dialogs.FormField{
		{Name: "name", Label: "Name", Type: dialogs.FieldTypeInput},
		{Name: "value", Label: "Value", Type: dialogs.FieldTypeTextArea},
	}

	dialogs.ShowFormWithDescription(app, "Create Config", "Enter config name and value", fields, func(result dialogs.FormResult) {
		name := result["name"]
		value := result["value"]

		if name == "" || value == "" {
			app.SetFlashError("name and value are required")
			return
		}

		app.AppendFlashPending(fmt.Sprintf("creating config %s...", name), 30*time.Second)

		app.RunInBackground(func() {
			err := app.GetDocker().CreateConfig(name, []byte(value))
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.AppendFlashError(fmt.Sprintf("failed to create config: %v", err))
				} else {
					app.AppendFlashSuccess(fmt.Sprintf("config %s created", name))
				}
				app.RefreshCurrentView()
			})
		})
	})
}

func Inspect(app common.AppController, id string) {
	subject := id
	if len(id) > 12 {
		subject = id[:12]
	}
	inspector := inspect.NewTextInspector("Describe config", subject, fmt.Sprintf(" [%s]Loading config...\n", styles.TagAccent), "json")
	app.OpenInspector(inspector)

	app.RunInBackground(func() {
		content, err := app.GetDocker().Inspect("config", id)
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() {
				inspector.Viewer.Update(fmt.Sprintf("Error: %v", err), "text")
			})
			return
		}

		resolvedSubject := subject
		cfgs, err := app.GetDocker().ListConfigs()
		if err == nil {
			for _, item := range cfgs {
				if item.GetID() == id {
					if c, ok := item.(dao.Config); ok {
						resolvedSubject = fmt.Sprintf("%s@%s", c.Name, resolvedSubject)
					}
					break
				}
			}
		}

		app.GetTviewApp().QueueUpdateDraw(func() {
			inspector.Subject = resolvedSubject
			inspector.Viewer.Update(content, "json")
			inspector.Viewer.View.SetTitle(inspector.GetTitle())
		})
	})
}

func resolveConfigSubject(v *view.ResourceView, id string) string {
	name := ""
	row, _ := v.Table.GetSelection()
	if row > 0 {
		index := row - 1
		if index >= 0 && index < len(v.Data) {
			if c, ok := v.Data[index].(dao.Config); ok {
				name = c.Name
			}
		}
	}

	subject := id
	if len(id) > 12 {
		subject = id[:12]
	}
	if name != "" {
		return fmt.Sprintf("%s@%s", name, subject)
	}
	return subject
}
