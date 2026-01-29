package secrets

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
	return app.GetDocker().ListSecrets()
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

	// Get secret name for label
	name := id
	row, _ := v.Table.GetSelection()
	if row > 0 {
		index := row - 1
		if index >= 0 && index < len(v.Data) {
			if s, ok := v.Data[index].(dao.Secret); ok {
				name = s.Name
			}
		}
	}

	app.SetActiveScope(&common.Scope{
		Type:       "secret",
		Value:      id,
		Label:      name,
		OriginView: styles.TitleSecrets,
	})

	app.SwitchTo(styles.TitleServices)
}

func DeleteAction(app common.AppController, v *view.ResourceView) {
	ids, err := v.GetSelectedIDs()
	if err != nil {
		return
	}

	label := ids[0]
	if len(ids) > 1 {
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
	return app.GetDocker().RemoveSecret(id)
}

func Create(app common.AppController) {
	fields := []dialogs.FormField{
		{Name: "name", Label: "Name", Type: dialogs.FieldTypeInput},
		{Name: "value", Label: "Value", Type: dialogs.FieldTypeInput},
	}

	dialogs.ShowFormWithDescription(app, "Create Secret", "Enter secret name and value", fields, func(result dialogs.FormResult) {
		name := result["name"]
		value := result["value"]

		if name == "" || value == "" {
			app.SetFlashError("name and value are required")
			return
		}

		app.SetFlashPending(fmt.Sprintf("creating secret %s...", name))
		app.RunInBackground(func() {
			err := app.GetDocker().CreateSecret(name, []byte(value))
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashError(fmt.Sprintf("%v", err))
				} else {
					app.SetFlashSuccess(fmt.Sprintf("secret %s created", name))
					app.ScheduleViewHighlight(styles.TitleSecrets, func(res dao.Resource) bool {
						sec, ok := res.(dao.Secret)
						return ok && sec.Name == name
					}, styles.ColorStatusGreen, styles.ColorBlack, 2*time.Second)
					app.RefreshCurrentView()
				}
			})
		})
	})
}

func Inspect(app common.AppController, id string) {
	content, err := app.GetDocker().Inspect("secret", id)
	if err != nil {
		app.SetFlashError(fmt.Sprintf("%v", err))
		return
	}

	subject := id
	if len(id) > 12 {
		subject = id[:12]
	}

	// Resolve Name
	secrets, err := app.GetDocker().ListSecrets()
	if err == nil {
		for _, item := range secrets {
			if item.GetID() == id {
				if s, ok := item.(dao.Secret); ok {
					subject = fmt.Sprintf("%s@%s", s.Name, subject)
				}
				break
			}
		}
	}

	app.OpenInspector(inspect.NewTextInspector("Describe secret", subject, content, "json"))
}
