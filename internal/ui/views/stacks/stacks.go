package stacks

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
	"github.com/jr-k/d4s/internal/ui/styles"
)

var Headers = []string{"NAME", "READY", "STATUS"}

func Fetch(app common.AppController, v *view.ResourceView) ([]dao.Resource, error) {
	return app.GetDocker().ListStacks()
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("enter", "Services"),
		common.FormatSCHeader("p", "Ps"),
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
	case 'p':
		app.InspectCurrentSelection()
		return nil
	}

	if event.Key() == tcell.KeyEnter {
		NavigateToServices(app, v)
		return nil
	}

	return event
}

func NavigateToServices(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	app.SetActiveScope(&common.Scope{
		Type:       "stack",
		Value:      id,
		Label:      id,
		OriginView: styles.TitleStacks,
	})

	app.SwitchTo(styles.TitleServices)
}

func DeleteAction(app common.AppController, v *view.ResourceView) {
	ids, err := v.GetSelectedIDs()
	if err != nil || len(ids) == 0 {
		return
	}

	label := ids[0]
	if len(ids) > 1 {
		label = fmt.Sprintf("%d items", len(ids))
	}

	dialogs.ShowConfirmation(app, "DELETE", label, func(force bool) {
		simpleAction := func(id string) error {
			return app.GetDocker().RemoveStack(id)
		}
		app.PerformAction(simpleAction, "deleting", styles.ColorStatusRed)
	})
}

func Inspect(app common.AppController, id string) {
	inspector := inspect.NewTextInspector("PS stack", id, fmt.Sprintf(" [%s]Loading stack tasks...\n", styles.TagAccent), "text")
	app.OpenInspector(inspector)

	app.RunInBackground(func() {
		content, err := app.GetDocker().StackPS(id)
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() {
				inspector.Viewer.Update(fmt.Sprintf("Error: %v", err), "text")
			})
			return
		}

		app.GetTviewApp().QueueUpdateDraw(func() {
			inspector.Viewer.Update(content, "text")
		})
	})
}
