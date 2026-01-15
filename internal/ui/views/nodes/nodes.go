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

var Headers = []string{"ID", "HOSTNAME", "STATUS", "AVAIL", "ROLE", "VERSION"}

func Fetch(app common.AppController) ([]dao.Resource, error) {
	return app.GetDocker().ListNodes()
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("ctrl-d", "Delete"),
	}
}

func Inspect(app common.AppController, id string) {
	content, err := app.GetDocker().Inspect("node", id)
	if err != nil {
		app.SetFlashText(fmt.Sprintf("[red]Inspect error: %v", err))
		return
	}

	subject := id
	if len(id) > 12 {
		subject = id[:12]
	}

	// Resolve Hostname
	nodes, err := app.GetDocker().ListNodes()
	if err == nil {
		for _, item := range nodes {
			if item.GetID() == id {
				if n, ok := item.(dao.Node); ok {
					subject = fmt.Sprintf("%s@%s", n.Hostname, subject)
				}
				break
			}
		}
	}

	app.OpenInspector(inspect.NewTextInspector("Describe Node", subject, content, "json"))
}

func InputHandler(v *view.ResourceView, event *tcell.EventKey) *tcell.EventKey {
	app := v.App
	
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

	if event.Key() == tcell.KeyCtrlD {
		DeleteAction(app, v)
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

func Remove(id string, force bool, app common.AppController) error {
	return app.GetDocker().RemoveNode(id, force)
}
