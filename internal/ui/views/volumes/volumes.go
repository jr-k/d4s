package volumes

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/jessym/d4s/internal/dao"
	"github.com/jessym/d4s/internal/ui/common"
	"github.com/jessym/d4s/internal/ui/dialogs"
)

var Headers = []string{"NAME", "DRIVER", "MOUNTPOINT"}

func Fetch(app common.AppController) ([]dao.Resource, error) {
	return app.GetDocker().ListVolumes()
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
		go func() {
			err := app.GetDocker().CreateVolume(text)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashText(fmt.Sprintf("[red]Error creating volume: %v", err))
				} else {
					app.SetFlashText(fmt.Sprintf("[green]Volume %s created", text))
					app.RefreshCurrentView()
				}
			})
		}()
	})
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
	
	go func() {
		err := cmd.Run()
		app.GetTviewApp().QueueUpdateDraw(func() {
			if err != nil {
				app.SetFlashText(fmt.Sprintf("[red]Open error: %v (Path: %s)", err, path))
			} else {
				app.SetFlashText(fmt.Sprintf("[green]Opened %s", path))
			}
		})
	}()
}

