package containers

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/jessym/d4s/internal/dao"
	"github.com/jessym/d4s/internal/ui/common"
	"github.com/jessym/d4s/internal/ui/dialogs"
)

var Headers = []string{"ID", "NAME", "IMAGE", "STATUS", "AGE", "PORTS", "CPU", "MEM", "COMPOSE", "CREATED"}

func Fetch(app common.AppController) ([]dao.Resource, error) {
	data, err := app.GetDocker().ListContainers()
	if err != nil {
		return nil, err
	}

	scope := app.GetActiveScope()
	if scope != nil && scope.Type == "compose" {
		var scopedData []dao.Resource
		for _, res := range data {
			if c, ok := res.(dao.Container); ok {
				if c.ProjectName == scope.Value {
					scopedData = append(scopedData, res)
				}
			}
		}
		return scopedData, nil
	}
	
	return data, nil
}

func Shell(app common.AppController, id string) {
	items := []dialogs.PickerItem{
		{Description: "/bin/bash", Label: "Bash", Value: "/bin/bash", Shortcut: '1'},
		{Description: "/bin/sh", Label: "Sh", Value: "/bin/sh", Shortcut: '2'},
	}

	dialogs.ShowPicker(app, "Shell Picker", items, func(shell string) {
		app.GetTviewApp().Suspend(func() {
			fmt.Print("\033[H\033[2J")
			fmt.Printf("Entering shell %s for %s (type 'exit' to return)...\n", shell, id)
			
			cmd := exec.Command("docker", "exec", "-it", id, shell)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			
			if err := cmd.Run(); err != nil {
				fmt.Printf("Error executing shell: %v\nPress Enter to continue...", err)
				fmt.Scanln()
			}
		})
	})
}

func Restart(app common.AppController, id string) error {
	return app.GetDocker().RestartContainer(id)
}

func Start(app common.AppController, id string) error {
	return app.GetDocker().StartContainer(id)
}

func Stop(app common.AppController, id string) error {
	return app.GetDocker().StopContainer(id)
}

func Remove(id string, force bool, app common.AppController) error {
	return app.GetDocker().RemoveContainer(id, force)
}

