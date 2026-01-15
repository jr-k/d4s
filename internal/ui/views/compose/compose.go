package compose

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	daoCompose "github.com/jr-k/d4s/internal/dao/compose"
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
		common.FormatSCHeader("e", "Edit"),
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
	case 'e':
		EditAction(app, v)
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
		if cp, ok := res.(daoCompose.ComposeProject); ok {
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

func EditAction(app common.AppController, v *view.ResourceView) {
	row, _ := v.Table.GetSelection()
	if row > 0 && row <= len(v.Data) {
		res := v.Data[row-1]
		if cp, ok := res.(daoCompose.ComposeProject); ok {
			if len(cp.ConfigPaths) == 0 {
				app.SetFlashText("[red]No config file found for this project")
				return
			}

			// Use the first config file found
			fileToEdit := strings.TrimSpace(cp.ConfigPaths[0])
			if fileToEdit == "" {
				app.SetFlashText("[red]Empty config file path")
				return
			}

			app.GetTviewApp().Suspend(func() {
				editor := os.Getenv("EDITOR")
				if editor == "" {
					editor = "vi" // Fallback
				}

				cmd := exec.Command(editor, fileToEdit)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				
				if err := cmd.Run(); err != nil {
					fmt.Fprintf(os.Stderr, "Error running editor: %v\nPress Enter to continue...", err)
					fmt.Scanln()
				}
			})
		}
	}
}

func Restart(app common.AppController, id string) error {
	return app.GetDocker().RestartComposeProject(id)
}

func Stop(app common.AppController, id string) error {
	return app.GetDocker().StopComposeProject(id)
}
