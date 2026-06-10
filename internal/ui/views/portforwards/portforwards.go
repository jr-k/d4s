package portforwards

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/portforward"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/styles"
)

var Headers = []string{"STATUS", "CONTEXT", "CONTAINER", "LOCAL", "REMOTE", "AGE"}

func Fetch(app common.AppController, v *view.ResourceView) ([]dao.Resource, error) {
	all := app.GetPortForwardManager().List()

	scope := app.GetActiveScope()
	if scope == nil {
		return all, nil
	}

	switch scope.Type {
	case "container":
		var filtered []dao.Resource
		for _, r := range all {
			if pf, ok := r.(portforward.PortForward); ok {
				if pf.ContainerID == scope.Value {
					filtered = append(filtered, r)
				}
			}
		}
		return filtered, nil
	case "compose":
		containerIDs := getComposeContainerIDs(app, scope.Value)
		var filtered []dao.Resource
		for _, r := range all {
			if pf, ok := r.(portforward.PortForward); ok {
				if containerIDs[pf.ContainerID] {
					filtered = append(filtered, r)
				}
			}
		}
		return filtered, nil
	case "service":
		containerIDs := getServiceContainerIDs(app, scope.Value)
		var filtered []dao.Resource
		for _, r := range all {
			if pf, ok := r.(portforward.PortForward); ok {
				if containerIDs[pf.ContainerID] {
					filtered = append(filtered, r)
				}
			}
		}
		return filtered, nil
	}

	return all, nil
}

func getComposeContainerIDs(app common.AppController, projectName string) map[string]bool {
	ids := make(map[string]bool)
	containers, err := app.GetDocker().ListContainers()
	if err != nil {
		return ids
	}
	for _, r := range containers {
		if c, ok := r.(dao.Container); ok {
			if c.ProjectName == projectName {
				ids[c.ID] = true
			}
		}
	}
	return ids
}

func getServiceContainerIDs(app common.AppController, serviceName string) map[string]bool {
	ids := make(map[string]bool)
	containers, err := app.GetDocker().ListContainers()
	if err != nil {
		return ids
	}
	for _, r := range containers {
		if c, ok := r.(dao.Container); ok {
			if c.ServiceName == serviceName {
				ids[c.ID] = true
			}
		}
	}
	return ids
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("r", "Run/Stop"),
		common.FormatSCHeader("enter", "Open"),
		common.FormatSCHeader("ctrl-d", "Delete"),
	}
}

func InputHandler(v *view.ResourceView, event *tcell.EventKey) *tcell.EventKey {
	app := v.App

	if event.Key() == tcell.KeyCtrlD {
		RemoveAction(app, v)
		return nil
	}

	if event.Key() == tcell.KeyEnter {
		OpenInBrowser(app, v)
		return nil
	}

	switch event.Rune() {
	case 'r':
		ToggleAction(app, v)
		return nil
	}

	return event
}

func ToggleAction(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	mgr := app.GetPortForwardManager()

	row, _ := v.Table.GetSelection()
	if row > 0 && row <= len(v.Data) {
		if pf, ok := v.Data[row-1].(portforward.PortForward); ok {
			if pf.Status == portforward.StatusRunning {
				mgr.Stop(id)
				app.AppendFlashSuccess(fmt.Sprintf("stopped port-forward %s", id))
			} else {
				if err := mgr.Start(id); err != nil {
					app.AppendFlashError(fmt.Sprintf("failed to start: %v", err))
				} else {
					app.AppendFlashSuccess(fmt.Sprintf("started port-forward %s", id))
				}
			}
			app.RefreshCurrentView()
		}
	}
}

func RemoveAction(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	app.GetPortForwardManager().Remove(id)
	app.AppendFlashSuccess(fmt.Sprintf("removed port-forward %s", id))
	app.RefreshCurrentView()
}

func OpenInBrowser(app common.AppController, v *view.ResourceView) {
	row, _ := v.Table.GetSelection()
	if row <= 0 || row > len(v.Data) {
		return
	}

	pf, ok := v.Data[row-1].(portforward.PortForward)
	if !ok {
		return
	}

	if pf.Status != portforward.StatusRunning {
		app.AppendFlashError("port-forward is not running")
		return
	}

	url := fmt.Sprintf("http://localhost:%d", pf.LocalPort)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		app.AppendFlash(fmt.Sprintf("[%s]%s[-]", styles.TagIdle, url))
		return
	}

	if err := cmd.Start(); err != nil {
		app.AppendFlashError(fmt.Sprintf("failed to open browser: %v", err))
	} else {
		app.AppendFlashSuccess(fmt.Sprintf("opened %s", url))
	}
}
