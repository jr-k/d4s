package volumes

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	daoCommon "github.com/jr-k/d4s/internal/dao/common"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
	"github.com/jr-k/d4s/internal/ui/styles"
)

var Headers = []string{"NAME", "DRIVER", "SCOPE", "MOUNTPOINT", "CREATED", "SIZE"}

func Fetch(app common.AppController) ([]dao.Resource, error) {
	scope := app.GetActiveScope()
	if scope != nil && scope.Type == "container" {
		return app.GetDocker().ListVolumesForContainer(scope.Value)
	}
	return app.GetDocker().ListVolumes()
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("o", "Open"),
		common.FormatSCHeader("a", "Add"),
		common.FormatSCHeader("p", "Prune"),
		common.FormatSCHeader("ctrl-d", "Delete"),
	}
}

func InputHandler(v *view.ResourceView, event *tcell.EventKey) *tcell.EventKey {
	app := v.App
	switch event.Rune() {
	case 'o':
		OpenAction(app, v)
		return nil
	case 'a':
		Create(app)
		return nil
	case 'p':
		PruneAction(app)
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

func PruneAction(app common.AppController) {
	dialogs.ShowConfirmation(app, "PRUNE", "Volumes", func(force bool) {
		app.SetFlashText("[yellow]Pruning Volumes...")
		app.RunInBackground(func() {
			err := Prune(app)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashText(fmt.Sprintf("[red]Prune Error: %v", err))
				} else {
					app.SetFlashText("[green]Pruned Volumes")
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
	return app.GetDocker().PruneVolumes()
}

func Remove(id string, force bool, app common.AppController) error {
	return app.GetDocker().RemoveVolume(id, force)
}

func Create(app common.AppController) {
	dialogs.ShowInput(app, "Create Volume", "Volume Name: ", "", func(text string) {
		app.SetFlashText(fmt.Sprintf("[yellow]Creating volume %s...", text))
		app.RunInBackground(func() {
			err := app.GetDocker().CreateVolume(text)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashText(fmt.Sprintf("[red]Error creating volume: %v", err))
				} else {
					app.SetFlashText(fmt.Sprintf("[green]Volume %s created", text))
					app.ScheduleViewHighlight(styles.TitleVolumes, func(res dao.Resource) bool {
						vol, ok := res.(dao.Volume)
						return ok && vol.Name == text
					}, styles.ColorStatusGreen, styles.ColorBlack, 2*time.Second)
					app.RefreshCurrentView()
				}
			})
		})
	})
}

func OpenAction(app common.AppController, v *view.ResourceView) {
	row, _ := v.Table.GetSelection()
	if row > 0 && row <= len(v.Data) {
		Open(app, v.Data[row-1])
	}
}

func Open(app common.AppController, res dao.Resource) {
	vol, ok := res.(dao.Volume)
	if !ok {
		app.SetFlashText("[red]Not a volume")
		return
	}

	path := vol.Mount
	if path == "" {
		app.SetFlashText("[yellow]No mountpoint found")
		return
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		app.SetFlashText(fmt.Sprintf("[red]Path not found on Host: %s (Is it inside Docker VM?)", path))
		return
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	default: // linux, etc
		cmd = exec.Command("xdg-open", path)
	}

	app.SetFlashText(fmt.Sprintf("[yellow]Opening %s...", path))

	app.RunInBackground(func() {
		err := cmd.Run()
		app.GetTviewApp().QueueUpdateDraw(func() {
			if err != nil {
				app.SetFlashText(fmt.Sprintf("[red]Open error: %v (Path: %s)", err, path))
			} else {
				app.SetFlashText(fmt.Sprintf("[green]Opened %s", path))
			}
		})
	})
}

func Inspect(app common.AppController, id string) {
	content, err := app.GetDocker().Inspect("volume", id)
	if err != nil {
		app.SetFlashText(fmt.Sprintf("[red]Inspect error: %v", err))
		return
	}

	subject := id
	// Resolve Mountpoint
	volumes, err := app.GetDocker().ListVolumes()
	if err == nil {
		for _, item := range volumes {
			if item.GetID() == id {
				if vol, ok := item.(dao.Volume); ok {
					if vol.Mount != "" {
						subject = fmt.Sprintf("%s@%s", id, daoCommon.ShortenPath(vol.Mount))
					}
				}
				break
			}
		}
	}

	app.OpenInspector(inspect.NewTextInspector("Describe volume", subject, content, "json"))
}
