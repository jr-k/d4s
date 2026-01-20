package compose

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	daoCommon "github.com/jr-k/d4s/internal/dao/common"
	daoCompose "github.com/jr-k/d4s/internal/dao/compose"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/styles"
)

var Headers = []string{"PROJECT", "READY", "STATUS", "CONFIG FILES"}

func Fetch(app common.AppController) ([]dao.Resource, error) {
	return app.GetDocker().ListCompose()
}

func Inspect(app common.AppController, id string) {
	content, err := app.GetDocker().GetComposeConfig(id)
	if err != nil {
		app.SetFlashError(fmt.Sprintf("%v", err))
		return
	}

	// Resolve config path for title
	path := ""
	projects, _ := app.GetDocker().ListCompose()
	for _, p := range projects {
		if cp, ok := p.(daoCompose.ComposeProject); ok {
			if cp.Name == id && len(cp.ConfigPaths) > 0 {
				path = cp.ConfigPaths[0]
				break
			}
		}
	}

	subject := id
	if path != "" {
		subject = fmt.Sprintf("%s@%s", id, daoCommon.ShortenPath(path))
	}

	app.OpenInspector(inspect.NewTextInspector("Describe compose", subject, content, "yaml"))
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("enter", "Containers"),
		common.FormatSCHeader("l", "Logs"),
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("e", "Edit"),
		common.FormatSCHeader("r", "(Re)Start"),
		common.FormatSCHeader("x", "Stop"),
	}
}

func InputHandler(v *view.ResourceView, event *tcell.EventKey) *tcell.EventKey {
	app := v.App
	switch event.Rune() {
	case 'l':
		Logs(app, v)
		return nil
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

func Logs(app common.AppController, v *view.ResourceView) {
	row, _ := v.Table.GetSelection()
	if row > 0 && row <= len(v.Data) {
		res := v.Data[row-1]
		projName := res.GetID()

		app.OpenInspector(inspect.NewLogInspector(projName, projName, "compose"))
	}
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
	}, "restarting", styles.ColorStatusOrange)
}

func StopAction(app common.AppController, v *view.ResourceView) {
	app.PerformAction(func(id string) error {
		return app.GetDocker().StopComposeProject(id)
	}, "stopping", styles.ColorStatusRed)
}

func EditAction(app common.AppController, v *view.ResourceView) {
	row, _ := v.Table.GetSelection()
	if row > 0 && row <= len(v.Data) {
		res := v.Data[row-1]
		if cp, ok := res.(daoCompose.ComposeProject); ok {
			if len(cp.ConfigPaths) == 0 {
				app.SetFlashError("no config file for this project")
				return
			}

			// Use the first config file found
			fileToEdit := strings.TrimSpace(cp.ConfigPaths[0])
			if fileToEdit == "" {
				app.SetFlashError("empty config file path")
				return
			}

		// Stop any background refresh to prevent UI updates interfering with the editor
		app.StopAutoRefresh()
		// Still set paused flag as double safety for any lingering goroutines
		app.SetPaused(true)

		defer func() {
			app.SetPaused(false)
			app.StartAutoRefresh()
		}()

		app.GetTviewApp().Suspend(func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Editor panic: %v\n", r)
				}
			}()

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi" // Fallback
			}

			fmt.Print("\033[H\033[2J") // Clear screen before editor

			cmd := exec.Command(editor, fileToEdit)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error running editor: %v\nPress Enter to continue...", err)
				fmt.Scanln()
			}
			
			fmt.Print("\033[H\033[2J") // Clear screen after editor
		})
		
		// Fix race conditions/glitches where screen isn't fully restored
		if app.GetScreen() != nil {
			app.GetScreen().Sync()
		}
		}
	}
}

func Restart(app common.AppController, id string) error {
	return app.GetDocker().RestartComposeProject(id)
}

func Stop(app common.AppController, id string) error {
	return app.GetDocker().StopComposeProject(id)
}
