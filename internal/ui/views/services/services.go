package services

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jessym/d4s/internal/dao"
	"github.com/jessym/d4s/internal/ui/common"
	"github.com/jessym/d4s/internal/ui/dialogs"
)

var Headers = []string{"ID", "NAME", "IMAGE", "MODE", "REPLICAS", "PORTS"}

func Fetch(app common.AppController) ([]dao.Resource, error) {
	return app.GetDocker().ListServices()
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
		
		go func() {
			err := app.GetDocker().ScaleService(id, replicas)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashText(fmt.Sprintf("[red]Scale Error: %v", err))
				} else {
					app.SetFlashText(fmt.Sprintf("[green]Service scaled to %d", replicas))
					app.RefreshCurrentView()
				}
			})
		}()
	})
}

