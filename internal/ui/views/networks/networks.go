package networks

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

var Headers = []string{"ID", "NAME", "DRIVER", "SCOPE", "CREATED", "INTERNAL", "SUBNET"}

func Fetch(app common.AppController) ([]dao.Resource, error) {
	scope := app.GetActiveScope()
	if scope != nil && scope.Type == "container" {
		return app.GetDocker().ListNetworksForContainer(scope.Value)
	}
	return app.GetDocker().ListNetworks()
}

func Inspect(app common.AppController, id string) {
	content, err := app.GetDocker().Inspect("network", id)
	if err != nil {
		app.SetFlashText(fmt.Sprintf("[red]Inspect Error: %v", err))
		return
	}

	name := ""
	list, _ := app.GetDocker().ListNetworks()
	for _, item := range list {
		if item.GetID() == id {
			if net, ok := item.(dao.Network); ok {
				name = net.Name
			}
			break
		}
	}

	subject := id
	if len(id) > 12 {
		subject = id[:12]
	}

	if name != "" && name != id {
		subject = fmt.Sprintf("%s@%s", name, subject)
	}

	app.OpenInspector(inspect.NewTextInspector("Describe network", subject, content, "json"))
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("a", "Add"),
		common.FormatSCHeader("p", "Prune"),
		common.FormatSCHeader("ctrl-d", "Delete"),
	}
}

func InputHandler(v *view.ResourceView, event *tcell.EventKey) *tcell.EventKey {
	app := v.App
	switch event.Rune() {
	case 'd':
		app.InspectCurrentSelection()
		return nil
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
		app.RunInBackground(func() {
			err := Prune(app)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashText(fmt.Sprintf("[red]Prune Error: %v", err))
				} else {
					app.SetFlashText("[green]Pruned Networks")
					app.RefreshCurrentView()
				}
			})
		})
	})
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
		app.PerformAction(simpleAction, "Deleting", styles.ColorStatusRed)
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
		app.RunInBackground(func() {
			err := app.GetDocker().CreateNetwork(text)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashText(fmt.Sprintf("[red]Error creating network: %v", err))
				} else {
					app.SetFlashText(fmt.Sprintf("[green]Network %s created", text))
					
					// Highlight and Select the new resource
					app.ScheduleViewHighlight(styles.TitleNetworks, func(res dao.Resource) bool {
						net, ok := res.(dao.Network)
						return ok && net.Name == text
					}, styles.ColorStatusGreen, styles.ColorStatusGreen, 2*time.Second)

					app.RefreshCurrentView()
				}
			})
		})
	})
}
