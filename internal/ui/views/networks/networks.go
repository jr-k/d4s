package networks

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
)

var Headers = []string{"ID", "NAME", "DRIVER", "SCOPE"}

func Fetch(app common.AppController) ([]dao.Resource, error) {
	return app.GetDocker().ListNetworks()
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("a", "Add"),
		common.FormatSCHeader("p", "Prune"),
		common.FormatSCHeader("Ctrl-d", "Delete"),
	}
}

func InputHandler(v *view.ResourceView, event *tcell.EventKey) *tcell.EventKey {
	app := v.App
	switch event.Rune() {
	case 'a':
		Create(app)
		return nil
	case 'p':
		PruneAction(app)
		return nil
	}
	if event.Key() == tcell.KeyCtrlD {
		DeleteAction(app, v)
		return nil
	}
	return event
}

func PruneAction(app common.AppController) {
	dialogs.ShowConfirmation(app, "PRUNE", "Networks", func(force bool) {
		app.SetFlashText("[yellow]Pruning Networks...")
		go func() {
			err := Prune(app)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashText(fmt.Sprintf("[red]Prune Error: %v", err))
				} else {
					app.SetFlashText("[green]Pruned Networks")
					app.RefreshCurrentView()
				}
			})
		}()
	})
}

func DeleteAction(app common.AppController, v *view.ResourceView) {
	ids, err := v.GetSelectedIDs()
	if err != nil { return }

	label := ids[0]
	if len(ids) > 1 {
		label = fmt.Sprintf("%d items", len(ids))
	}

	dialogs.ShowConfirmation(app, "DELETE", label, func(force bool) {
		simpleAction := func(id string) error {
			return Remove(id, force, app)
		}
		app.PerformAction(simpleAction, "Deleting")
	})
}

func Prune(app common.AppController) error {
	return app.GetDocker().PruneNetworks()
}

func Remove(id string, force bool, app common.AppController) error {
	return app.GetDocker().RemoveNetwork(id)
}

func Create(app common.AppController) {
	dialogs.ShowInput(app, "Create Network", "Network Name: ", "", func(text string) {
		app.SetFlashText(fmt.Sprintf("[yellow]Creating network %s...", text))
		go func() {
			err := app.GetDocker().CreateNetwork(text)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashText(fmt.Sprintf("[red]Error creating network: %v", err))
				} else {
					app.SetFlashText(fmt.Sprintf("[green]Network %s created", text))
					app.RefreshCurrentView()
				}
			})
		}()
	})
}
