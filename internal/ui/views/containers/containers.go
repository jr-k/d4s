package containers

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
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

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("l", "Logs"),
		common.FormatSCHeader("s", "Shell"),
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("e", "Env"),
		common.FormatSCHeader("t", "Stats"),
		common.FormatSCHeader("m", "Monitor"),
		common.FormatSCHeader("v", "Volumes"),
		common.FormatSCHeader("n", "Networks"),
		common.FormatSCHeader("r", "(Re)Start"),
		common.FormatSCHeader("x", "Stop"),
	}
}

func InputHandler(v *view.ResourceView, event *tcell.EventKey) *tcell.EventKey {
	app := v.App
	
	switch event.Rune() {
	case 'e':
		Env(app, v)
		return nil
	case 't':
		Stats(app, v)
		return nil
	case 'm':
		Monitor(app, v)
		return nil
	case 'v':
		Volumes(app, v)
		return nil
	case 'n':
		Networks(app, v)
		return nil
	case 'l':
		Logs(app, v)
		return nil
	case 's':
		// Shell
		id, err := v.GetSelectedID()
		if err == nil {
			Shell(app, id)
		}
		return nil
	case 'd':
		Describe(app, v)
		return nil
	case 'r':
		RestartOrStart(app, v)
		return nil
	case 'x':
		StopAction(app, v)
		return nil
	}
	
	if event.Key() == tcell.KeyCtrlD {
		DeleteAction(app, v)
		return nil
	}
	
	return event
}

func DeleteAction(app common.AppController, v *view.ResourceView) {
	ids, err := v.GetSelectedIDs()
	if err != nil { return }

	label := ids[0]
	if len(ids) == 1 {
		row, _ := v.Table.GetSelection()
		if row > 0 && row <= len(v.Data) {
			item := v.Data[row-1]
			if item.GetID() == ids[0] {
				cells := item.GetCells()
				if len(cells) > 1 {
					label = fmt.Sprintf("%s ([#8be9fd]%s[yellow])", label, cells[1])
				}
			}
		}
	} else {
		label = fmt.Sprintf("%d items", len(ids))
	}

	dialogs.ShowConfirmation(app, "DELETE", label, func(force bool) {
		simpleAction := func(id string) error {
			return Remove(id, force, app)
		}
		app.PerformAction(simpleAction, "Deleting")
	})
}

func Env(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil { return }

	env, err := app.GetDocker().GetContainerEnv(id)
	if err != nil {
		app.SetFlashText(fmt.Sprintf("[red]Env Error: %v", err))
		return
	}

	subject := resolveContainerSubject(v, id)

	var lines []string
	for _, line := range env {
		lines = append(lines, line)
	}
	
	app.OpenInspector(inspect.NewTextInspector("Environment", subject, strings.Join(lines, "\n"), "env"))
}

func Stats(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil { return }

	// Resolve Name
	name := ""
	row, _ := v.Table.GetSelection()
	if row > 0 {
		index := row - 1
		if index >= 0 && index < len(v.Data) {
			if c, ok := v.Data[index].(dao.Container); ok {
				name = c.Names
			}
		}
	}

	app.OpenInspector(inspect.NewStatsInspector(id, name))
}

func Monitor(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil { return }

	// Resolve Name
	name := ""
	row, _ := v.Table.GetSelection()
	if row > 0 {
		index := row - 1
		if index >= 0 && index < len(v.Data) {
			if c, ok := v.Data[index].(dao.Container); ok {
				name = c.Names
			}
		}
	}

	app.OpenInspector(inspect.NewMonitorInspector(id, name))
}


func Volumes(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil { return }
	subject := resolveContainerSubject(v, id)

	content, err := app.GetDocker().Inspect("container", id)
	if err != nil {
		app.SetFlashText(fmt.Sprintf("[red]Inspect Error: %v", err))
		return
	}
	app.OpenInspector(inspect.NewTextInspector("Volumes", subject, content, "json"))
}

func Networks(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil { return }
	subject := resolveContainerSubject(v, id)

	content, err := app.GetDocker().Inspect("container", id)
	if err != nil {
		app.SetFlashText(fmt.Sprintf("[red]Inspect Error: %v", err))
		return
	}
	app.OpenInspector(inspect.NewTextInspector("Networks", subject, content, "json"))
}

func Logs(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil { return }
	subject := resolveContainerSubject(v, id)

	app.OpenInspector(inspect.NewLogInspector(id, subject, "container"))
}

func Describe(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil { return }

	content, err := app.GetDocker().Inspect("container", id)
	if err != nil {
		app.SetFlashText(fmt.Sprintf("[red]Inspect error: %v", err))
		return
	}

	name := ""
	row, _ := v.Table.GetSelection()
	if row > 0 {
		index := row - 1
		if index >= 0 && index < len(v.Data) {
			if c, ok := v.Data[index].(dao.Container); ok {
				name = strings.TrimPrefix(c.Names, "/")
			}
		}
	}
	
	subject := id
	if len(id) > 12 {
		subject = id[:12]
	}
	if name != "" {
		subject = fmt.Sprintf("%s@%s", name, subject)
	}

	app.OpenInspector(inspect.NewTextInspector("Describe Container", subject, content, "json"))
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

func RestartOrStart(app common.AppController, v *view.ResourceView) {
	row, _ := v.Table.GetSelection()
	if row > 0 && row <= len(v.Data) {
		item := v.Data[row-1]
		if c, ok := item.(dao.Container); ok {
			lowerStatus := strings.ToLower(c.Status)
			if strings.Contains(lowerStatus, "exited") || strings.Contains(lowerStatus, "created") {
				app.PerformAction(func(id string) error {
					return app.GetDocker().StartContainer(id)
				}, "Starting")
				return
			}
		}
	}
	
	// Default to Restart
	app.PerformAction(func(id string) error {
		return app.GetDocker().RestartContainer(id)
	}, "Restarting")
}

func StopAction(app common.AppController, v *view.ResourceView) {
	app.PerformAction(func(id string) error {
		return app.GetDocker().StopContainer(id)
	}, "Stopping")
}

func Remove(id string, force bool, app common.AppController) error {
	return app.GetDocker().RemoveContainer(id, force)
}

func resolveContainerSubject(v *view.ResourceView, id string) string {
	name := ""
	row, _ := v.Table.GetSelection()
	if row > 0 {
		index := row - 1
		if index >= 0 && index < len(v.Data) {
			if c, ok := v.Data[index].(dao.Container); ok {
				name = strings.TrimPrefix(c.Names, "/")
			}
		}
	}
	
	subject := id
	if len(id) > 12 {
		subject = id[:12]
	}
	if name != "" {
		return fmt.Sprintf("%s@%s", name, subject)
	}
	return subject
}
