package volumes

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
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

var Headers = []string{"NAME", "DRIVER", "SCOPE", "USED BY", "MOUNTPOINT", "CREATED", "SIZE", "ANON"}

func Fetch(app common.AppController, v *view.ResourceView) ([]dao.Resource, error) {
	scope := app.GetActiveScope()
	if scope != nil && scope.Type == "container" {
		// Switch headers for Container Scope
		v.Headers = []string{"NAME", "TYPE", "DRIVER", "SCOPE", "DESTINATION", "MOUNTPOINT", "CREATED", "SIZE", "ANON"}
		return app.GetDocker().ListVolumesForContainer(scope.Value)
	}
	
	// Reset headers for Global Scope
	v.Headers = Headers
	return app.GetDocker().ListVolumes()
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("s", "Shell"),
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("o", "Open"),
		common.FormatSCHeader("a", "Add"),
		common.FormatSCHeader("shift-p", "Prune"),
		common.FormatSCHeader("ctrl-d", "Delete"),
	}
}

func InputHandler(v *view.ResourceView, event *tcell.EventKey) *tcell.EventKey {
	app := v.App
	if event.Key() == tcell.KeyCtrlD {
		DeleteAction(app, v)
		return nil
	}
	switch event.Rune() {
	case 's':
		// Shell
		id, err := v.GetSelectedID()
		if err == nil {
			Shell(app, id)
		}
		return nil
	case 'o':
		OpenAction(app, v)
		return nil
	case 'a':
		Create(app)
		return nil
	case 'P':
		PruneAction(app)
		return nil
	case 'd':
		app.InspectCurrentSelection()
		return nil
	}
	return event
}

func PruneAction(app common.AppController) {
	dialogs.ShowConfirmation(app, "PRUNE", "Volumes", func(force bool) {
		app.SetFlashPending("pruning volumes...")
		app.RunInBackground(func() {
			err := Prune(app)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashError(fmt.Sprintf("%v", err))
				} else {
					app.SetFlashSuccess("pruned volumes")
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
		app.PerformAction(simpleAction, "deleting", styles.ColorStatusRed)
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
		app.SetFlashPending(fmt.Sprintf("creating volume %s...", text))
		app.RunInBackground(func() {
			err := app.GetDocker().CreateVolume(text)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashError(fmt.Sprintf("%v", err))
				} else {
					app.SetFlashSuccess(fmt.Sprintf("volume %s created", text))
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
		app.SetFlashError("not a volume")
		return
	}

	path := vol.Mount
	if path == "" {
		app.SetFlashError("no mountpoint found")
		return
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		app.SetFlashError(fmt.Sprintf("path not found on host: %s", path))
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

	app.SetFlashPending(fmt.Sprintf("opening %s...", path))

	app.RunInBackground(func() {
		err := cmd.Run()
		app.GetTviewApp().QueueUpdateDraw(func() {
			if err != nil {
				app.SetFlashError(fmt.Sprintf("%v (Path: %s)", err, path))
			} else {
				app.SetFlashSuccess(fmt.Sprintf("opened %s", path))
			}
		})
	})
}

func Inspect(app common.AppController, id string) {
	subject := id
	inspector := inspect.NewTextInspector("Describe volume", subject, fmt.Sprintf(" [%s]Loading volume...\n", styles.TagAccent), "json")
	app.OpenInspector(inspector)

	app.RunInBackground(func() {
		content, err := app.GetDocker().Inspect("volume", id)
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() {
				inspector.Viewer.Update(fmt.Sprintf("Error: %v", err), "text")
			})
			return
		}

		// Resolve Mountpoint
		resolvedSubject := id
		volumes, err := app.GetDocker().ListVolumes()
		if err == nil {
			for _, item := range volumes {
				if item.GetID() == id {
					if vol, ok := item.(dao.Volume); ok {
						if vol.Mount != "" {
							resolvedSubject = fmt.Sprintf("%s@%s", id, daoCommon.ShortenPath(vol.Mount))
						}
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

func Shell(app common.AppController, id string) {
	app.StopAutoRefresh()
	app.SetPaused(true)

	defer func() {
		app.SetPaused(false)
		app.StartAutoRefresh()
	}()

	app.GetTviewApp().Suspend(func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Shell panic: %v\n", r)
			}
		}()

		signal.Reset(os.Interrupt, syscall.SIGTERM)

		fmt.Print("\033[H\033[2J")
		fmt.Printf("Mounting volume %s in temporary alpine container...\n", id)

		containerName := fmt.Sprintf("d4s-vol-shell-%d", time.Now().UnixNano())
		shellImage := app.GetConfig().D4S.ShellPod.Image
		cmd := exec.Command("docker", "run", "--pull", "always", "--rm", "--name", containerName, "-it", "-v", id+":/data", "-p", "33000-33100:33000-33100", "-w", "/data", shellImage, "sh")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		defer func() {
			exec.Command("docker", "rm", "-f", containerName).Run()
		}()

		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code := exitErr.ExitCode()
				if code == 130 || code == 137 || code == 0 {
					return
				}
			}
			fmt.Printf("\nError executing shell: %v\n", err)
			fmt.Println("Ensure shell image is available or that you have permission to run containers.")
			fmt.Println("Press Enter to continue...")
			fmt.Scanln()
		}
	})

	if app.GetScreen() != nil {
		app.GetScreen().Sync()
	}
}
