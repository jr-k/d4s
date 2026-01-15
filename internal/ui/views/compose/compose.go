package compose

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/styles"
)

var Headers = []string{"PROJECT", "STATUS", "CONFIG FILES"}

func Fetch(app common.AppController) ([]dao.Resource, error) {
	return app.GetDocker().ListCompose()
}

func Inspect(app common.AppController, id string) {
	content, err := app.GetDocker().GetComposeConfig(id)
	if err != nil {
		app.SetFlashText(fmt.Sprintf("[red]Inspect error: %v", err))
		return
	}
	app.OpenInspector(inspect.NewTextInspector("Inspect", id, content, "yaml"))
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("Enter", "Containers"),
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("r", "(Re)Start"),
		common.FormatSCHeader("x", "Stop"),
	}
}

func InputHandler(v *view.ResourceView, event *tcell.EventKey) *tcell.EventKey {
	app := v.App
	switch event.Rune() {
	case 'd':
		app.InspectCurrentSelection()
		return nil
	case 'r':
		RestartAction(app, v)
		return nil
	case 'x':
		StopAction(app, v)
		return nil
	}
	
	if event.Key() == tcell.KeyEnter {
		NavigateToContainers(app, v)
		return nil
	}
	return event
}

func NavigateToContainers(app common.AppController, v *view.ResourceView) {
	row, _ := v.Table.GetSelection()
	if row > 0 && row <= len(v.Data) {
		res := v.Data[row-1]
		projName := res.GetID()
		
		// Try to get config file path
		label := projName
		if cp, ok := res.(dao.ComposeProject); ok {
			if cp.ConfigFiles != "" {
				label = cp.ConfigFiles
			}
		}

		// Set Scope
		app.SetActiveScope(&common.Scope{
			Type:       "compose",
			Value:      projName,
			Label:      label,
			OriginView: styles.TitleCompose,
		})
		
		// Switch to Containers
		app.SwitchTo(styles.TitleContainers)
	}
}

func RestartAction(app common.AppController, v *view.ResourceView) {
	app.PerformAction(func(id string) error {
		return app.GetDocker().RestartComposeProject(id)
	}, "Restarting Project")
}

func StopAction(app common.AppController, v *view.ResourceView) {
	app.PerformAction(func(id string) error {
		return app.GetDocker().StopComposeProject(id)
	}, "Stopping Project")
}

func Restart(app common.AppController, id string) error {
	return app.GetDocker().RestartComposeProject(id)
}

func Stop(app common.AppController, id string) error {
	return app.GetDocker().StopComposeProject(id)
}
