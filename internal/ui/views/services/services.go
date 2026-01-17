package services

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
	"github.com/jr-k/d4s/internal/ui/styles"
)

var Headers = []string{"ID", "NAME", "IMAGE", "MODE", "REPLICAS", "PORTS", "CREATED", "UPDATED"}

func Fetch(app common.AppController) ([]dao.Resource, error) {
	services, err := app.GetDocker().ListServices()
	if err != nil {
		return nil, err
	}

	// Filter by Node Scope
	scope := app.GetActiveScope()
	if scope != nil && scope.Type == "node" {
		nodeID := scope.Value
		var filtered []dao.Resource
		
		// We need to check which services have tasks on this node
		// This requires an extra call to list tasks for this node
		tasks, err := app.GetDocker().ListTasksForNode(nodeID)
		if err == nil {
			serviceIDs := make(map[string]bool)
			for _, task := range tasks {
				serviceIDs[task.ServiceID] = true
			}
			
			for _, s := range services {
				if serviceIDs[s.GetID()] {
					filtered = append(filtered, s)
				}
			}
			return filtered, nil
		}
	}
	
	return services, nil
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("l", "Logs"),
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("s", "Scale"),
		common.FormatSCHeader("z", "No Replica"),
		common.FormatSCHeader("ctrl-d", "Delete"),
	}
}

func InputHandler(v *view.ResourceView, event *tcell.EventKey) *tcell.EventKey {
	app := v.App
	
	switch event.Rune() {
	case 's':
		ScaleAction(app, v)
		return nil
	case 'l':
		Logs(app, v)
		return nil
	case 'z':
		ScaleZero(app, v)
		return nil
	case 'd':
		app.InspectCurrentSelection()
		return nil
	}
	
	if event.Key() == tcell.KeyCtrlD {
		DeleteAction(app, v)
		return nil
	}
	
	return event
}

func Logs(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil { return }
	
	// Open Logs view
	app.OpenInspector(inspect.NewLogInspector(id, id, "service"))
}

func DeleteAction(app common.AppController, v *view.ResourceView) {
	ids, err := v.GetSelectedIDs()
	if err != nil { return }

	dialogs.ShowConfirmation(app, "DELETE", fmt.Sprintf("%d services", len(ids)), func(force bool) {
		simpleAction := func(id string) error {
			return Remove(id, force, app)
		}
		app.PerformAction(simpleAction, "Deleting", styles.ColorStatusRed)
	})
}

func ScaleAction(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil { return }
	
	currentReplicas := ""
	row, _ := v.Table.GetSelection()
	if row > 0 && row <= len(v.Data) {
		item := v.Data[row-1]
		cells := item.GetCells()
		if len(cells) > 4 {
			currentReplicas = strings.TrimSpace(cells[4])
		}
	}
	Scale(app, id, currentReplicas)
}

func ScaleZero(app common.AppController, v *view.ResourceView) {
	ids, err := v.GetSelectedIDs()
	if err != nil { return }
	
	msg := fmt.Sprintf("You are about to scale %d services to 0 replicas.\nThis will make them unavailable.\nAre you sure?", len(ids))
	
	dialogs.ShowConfirmation(app, "NO REPLICA", msg, func(force bool) {
		scaleAction := func(id string) error {
			return app.GetDocker().ScaleService(id, 0)
		}
		app.PerformAction(scaleAction, "Scaling to 0", styles.ColorStatusOrange)
	})
}

func Inspect(app common.AppController, id string) {
	content, err := app.GetDocker().Inspect("service", id)
	if err != nil {
		app.SetFlashText(fmt.Sprintf("[red]Inspect error: %v", err))
		return
	}

	subject := id
	if len(id) > 12 {
		subject = id[:12]
	}

	// Resolve Name
	services, err := app.GetDocker().ListServices()
	if err == nil {
		for _, item := range services {
			if item.GetID() == id {
				if s, ok := item.(dao.Service); ok {
					subject = fmt.Sprintf("%s@%s", s.Name, subject)
				}
				break
			}
		}
	}

	app.OpenInspector(inspect.NewTextInspector("Describe service", subject, content, "json"))
}

func Remove(id string, force bool, app common.AppController) error {
	return app.GetDocker().RemoveService(id)
}

func Scale(app common.AppController, id string, currentReplicas string) {
	if parts := strings.Split(currentReplicas, "/"); len(parts) == 2 {
		currentReplicas = parts[1]
	}

	dialogs.ShowInput(app, "Scale Service", "Replicas:", currentReplicas, func(text string) {
		replicas, err := strconv.ParseUint(text, 10, 64)
		if err != nil {
			app.SetFlashText("[red]Invalid number")
			return
		}
		
		app.SetFlashText(fmt.Sprintf("[yellow]Scaling %s to %d...", id, replicas))
		
		app.RunInBackground(func() {
			err := app.GetDocker().ScaleService(id, replicas)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashText(fmt.Sprintf("[red]Scale Error: %v", err))
				} else {
					app.SetFlashText(fmt.Sprintf("[green]Service scaled to %d", replicas))
					app.RefreshCurrentView()
				}
			})
		})
	})
}
