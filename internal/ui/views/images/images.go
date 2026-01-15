package images

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
)

var Headers = []string{"ID", "TAGS", "SIZE", "CREATED"}

func Fetch(app common.AppController) ([]dao.Resource, error) {
	return app.GetDocker().ListImages()
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("p", "Prune"),
		common.FormatSCHeader("Ctrl-d", "Delete"),
	}
}

func InputHandler(v *view.ResourceView, event *tcell.EventKey) *tcell.EventKey {
	app := v.App
	switch event.Rune() {
	case 'p':
		PruneAction(app)
		return nil
	case 'd':
		Describe(app, v)
		return nil
	}
	if event.Key() == tcell.KeyCtrlD {
		DeleteAction(app, v)
		return nil
	}
	return event
}

func PruneAction(app common.AppController) {
	dialogs.ShowConfirmation(app, "PRUNE", "Images", func(force bool) {
		app.SetFlashText("[yellow]Pruning Images...")
		go func() {
			err := Prune(app)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashText(fmt.Sprintf("[red]Prune Error: %v", err))
				} else {
					app.SetFlashText("[green]Pruned Images")
					app.RefreshCurrentView()
				}
			})
		}()
	})
}

func Describe(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil { return }

	content, err := app.GetDocker().Inspect("image", id)
	if err != nil {
		app.SetFlashText(fmt.Sprintf("[red]Inspect error: %v", err))
		return
	}
	app.OpenInspector(inspect.NewTextInspector("Describe", id, content, "json"))
}

func DeleteAction(app common.AppController, v *view.ResourceView) {
	ids, err := v.GetSelectedIDs()
	if err != nil { return }

	label := ids[0]
	if len(ids) == 1 {
		row, _ := v.Table.GetSelection()
		if row > 0 && row <= len(v.Data) {
			item := v.Data[row-1]
			if item.GetID() == ids[0] {
				cells := item.GetCells()
				if len(cells) > 1 {
					label = fmt.Sprintf("%s ([#8be9fd]%s[yellow])", label, cells[1])
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
		app.PerformAction(simpleAction, "Deleting")
	})
}

func Prune(app common.AppController) error {
	return app.GetDocker().PruneImages()
}

func Remove(id string, force bool, app common.AppController) error {
	return app.GetDocker().RemoveImage(id, force)
}
