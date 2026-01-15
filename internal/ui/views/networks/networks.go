package networks

import (
	"fmt"

	"github.com/jessym/d4s/internal/dao"
	"github.com/jessym/d4s/internal/ui/common"
	"github.com/jessym/d4s/internal/ui/dialogs"
)

var Headers = []string{"ID", "NAME", "DRIVER", "SCOPE"}

func Fetch(app common.AppController) ([]dao.Resource, error) {
	return app.GetDocker().ListNetworks()
}

func Prune(app common.AppController) error {
	return app.GetDocker().PruneNetworks()
}

func Remove(id string, force bool, app common.AppController) error {
	return app.GetDocker().RemoveNetwork(id)
}

func Create(app common.AppController) {
	dialogs.ShowInput(app, "Create Network", "Network Name: ", "", func(text string) {
		app.SetFlashText(fmt.Sprintf("[yellow]Creating network %s...", text))
		go func() {
			err := app.GetDocker().CreateNetwork(text)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashText(fmt.Sprintf("[red]Error creating network: %v", err))
				} else {
					app.SetFlashText(fmt.Sprintf("[green]Network %s created", text))
					app.RefreshCurrentView()
				}
			})
		}()
	})
}

