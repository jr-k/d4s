package nodes

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

var Headers = []string{"ID", "HOSTNAME", "STATUS", "AVAIL", "ROLE", "VERSION", "CREATED"}

func Fetch(app common.AppController, v *view.ResourceView) ([]dao.Resource, error) {
	return app.GetDocker().ListNodes()
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("ctrl-d", "Delete"),
	}
}

func Inspect(app common.AppController, id string) {
	subject := id
	if len(id) > 12 {
		subject = id[:12]
	}
	inspector := inspect.NewTextInspector("Describe node", subject, fmt.Sprintf(" [%s]Loading node...\n", styles.TagAccent), "json")
	app.OpenInspector(inspector)

	app.RunInBackground(func() {
		content, err := app.GetDocker().Inspect("node", id)
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() {
				inspector.Viewer.Update(fmt.Sprintf("Error: %v", err), "text")
			})
			return
		}

		// Resolve Hostname
		resolvedSubject := subject
		nodes, err := app.GetDocker().ListNodes()
		if err == nil {
			for _, item := range nodes {
				if item.GetID() == id {
					if n, ok := item.(dao.Node); ok {
						resolvedSubject = fmt.Sprintf("%s@%s", n.Hostname, resolvedSubject)
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
	}
	
	// Navigation to Services
	if event.Key() == tcell.KeyEnter {
		NavigateToServices(app, v)
		return nil
	}

	return event
}

func NavigateToServices(app common.AppController, v *view.ResourceView) {
	row, _ := v.Table.GetSelection()
	if row > 0 && row <= len(v.Data) {
		res := v.Data[row-1]
		nodeID := res.GetID()
		
		// Get Node Hostname for Label
		label := nodeID
		if cells := res.GetCells(); len(cells) > 1 {
			label = cells[1] // Assuming Name/Hostname is 2nd column
		}

		// Set Scope
		app.SetActiveScope(&common.Scope{
			Type:       "node",
			Value:      nodeID,
			Label:      label,
			OriginView: styles.TitleNodes,
		})
		
		// Switch to Services
		app.SwitchTo(styles.TitleServices)
	}
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
	return app.GetDocker().RemoveNode(id, force)
}
