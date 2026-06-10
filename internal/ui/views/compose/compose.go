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
	"github.com/jr-k/d4s/internal/portforward"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
	"github.com/jr-k/d4s/internal/ui/styles"
)

var Headers = []string{"PROJECT", "READY", "STATUS", "CONFIG FILES"}

func Fetch(app common.AppController, v *view.ResourceView) ([]dao.Resource, error) {
	data, err := app.GetDocker().ListCompose()
	if err != nil {
		return nil, err
	}

	scope := app.GetActiveScope()
	if scope != nil {
		if scope.Type == "container" {
			var scopedData []dao.Resource
			for _, res := range data {
				if cp, ok := res.(daoCompose.ComposeProject); ok {
					if cp.Name == scope.Value {
						scopedData = append(scopedData, res)
					}
				}
			}
			return scopedData, nil
		}
	}

	return data, nil
}

func Inspect(app common.AppController, id string) {
	inspector := inspect.NewTextInspector("Describe compose", id, fmt.Sprintf(" [%s]Loading compose...\n", styles.TagAccent), "yaml")
	app.OpenInspector(inspector)

	app.RunInBackground(func() {
		content, err := app.GetDocker().GetComposeConfig(id)
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() {
				inspector.Viewer.Update(fmt.Sprintf("Error: %v", err), "text")
			})
			return
		}

		// Resolve config path for title
		resolvedSubject := id
		projects, _ := app.GetDocker().ListCompose()
		for _, p := range projects {
			if cp, ok := p.(daoCompose.ComposeProject); ok {
				if cp.Name == id && len(cp.ConfigPaths) > 0 {
					resolvedSubject = fmt.Sprintf("%s@%s", id, daoCommon.ShortenPath(cp.ConfigPaths[0]))
					break
				}
			}
		}

		app.GetTviewApp().QueueUpdateDraw(func() {
			inspector.Subject = resolvedSubject
			inspector.Viewer.Update(content, "yaml")
			inspector.Viewer.View.SetTitle(inspector.GetTitle())
		})
	})
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("enter", "Containers"),
		common.FormatSCHeader("l", "Logs"),
		common.FormatSCHeader("f", "Show PortForward"),
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("e", "Edit"),
		common.FormatSCHeader("r", "(Re)Start"),
		common.FormatSCHeader("b", "Build"),
		common.FormatSCHeader("shift-f", "Port-Forward"),
		common.FormatSCHeader("shift-r", "(Re)Deploy"),
		common.FormatSCHeader("ctrl-d", "Delete"),
		common.FormatSCHeader("ctrl-k", "Stop"),
	}
}

func InputHandler(v *view.ResourceView, event *tcell.EventKey) *tcell.EventKey {
	app := v.App
	if event.Key() == tcell.KeyCtrlK {
		StopAction(app, v)
		return nil
	}
	if event.Key() == tcell.KeyCtrlD {
		DeleteAction(app, v)
		return nil
	}
	switch event.Rune() {
	case 'l':
		Logs(app, v)
		return nil
	case 'f':
		ShowPortForwards(app, v)
		return nil
	case 'F':
		PortForwardAction(app, v)
		return nil
	case 'd':
		app.InspectCurrentSelection()
		return nil
	case 'e':
		EditAction(app, v)
		return nil
	case 'r':
		UpAction(app, v)
		return nil
	case 'R':
		RedeployAction(app, v)
		return nil
	case 'b':
		BuildAction(app, v)
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

		app.OpenInspector(inspect.NewLogInspectorWithConfig(projName, projName, "compose", app.GetConfig().D4S.Logger))
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

func UpAction(app common.AppController, v *view.ResourceView) {
	app.PerformAction(func(id string) error {
		return app.GetDocker().UpComposeProject(id)
	}, "restarting", styles.ColorStatusOrange)
}

func RedeployAction(app common.AppController, v *view.ResourceView) {
	app.PerformAction(func(id string) error {
		return app.GetDocker().RedeployComposeProject(id)
	}, "redeploying", styles.ColorStatusMagenta)
}

func BuildAction(app common.AppController, v *view.ResourceView) {
	app.PerformAction(func(id string) error {
		return app.GetDocker().BuildComposeProject(id)
	}, "building", styles.ColorStatusMagenta)
}

func DeleteAction(app common.AppController, v *view.ResourceView) {
	ids, err := v.GetSelectedIDs()
	if err != nil || len(ids) == 0 {
		return
	}

	label := ids[0]
	if len(ids) > 1 {
		label = fmt.Sprintf("%d items", len(ids))
	}

	// Stop background refresh to prevent UI flickering during confirmation/deletion
	app.StopAutoRefresh()
	app.SetPaused(true)

	dialogs.ShowConfirmation(app, "DELETE", label, func(force bool) {
		// Ensure we always resume UI refresh cycles when the action completes or cancels
		defer func() {
			app.SetPaused(false)
			app.StartAutoRefresh()
			
			// Force UI sync to clean up any leftover artifacts from the dialog
			if app.GetScreen() != nil {
				app.GetScreen().Sync()
			}
		}()

		// Define the multi-item deletion task
		batchDeleteAction := func(_ string) error {
			for _, id := range ids {
				if err := Remove(id, force, app); err != nil {
					return err // Halts and displays error if one fails
				}
			}
			return nil
		}

		// Perform the action with the consistent styling
		app.PerformAction(batchDeleteAction, "deleting", styles.ColorStatusRed)
	})
}

func Stop(app common.AppController, id string) error {
	return app.GetDocker().StopComposeProject(id)
}

func Remove(id string, force bool, app common.AppController) error {
	return app.GetDocker().DownComposeProject(id)
}

func ShowPortForwards(app common.AppController, v *view.ResourceView) {
	if !app.GetDocker().IsSSHContext() {
		app.AppendFlashError("port-forward is only available on SSH contexts")
		return
	}

	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	app.SetActiveScope(&common.Scope{
		Type:       "compose",
		Value:      id,
		Label:      id,
		OriginView: styles.TitleCompose,
	})
	app.SwitchTo(styles.TitlePortForwards)
}

func PortForwardAction(app common.AppController, v *view.ResourceView) {
	if !app.GetDocker().IsSSHContext() {
		app.AppendFlashError("port-forward is only available on SSH contexts")
		return
	}

	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	pickerTitle := fmt.Sprintf("Port-Forward: %s", id)
	dialogs.ShowPickerLoading(app, pickerTitle)

	app.RunInBackground(func() {
		containers, err := app.GetDocker().ListContainers()
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() {
				app.GetPages().RemovePage("picker")
				app.AppendFlashError(fmt.Sprintf("failed to list containers: %v", err))
			})
			return
		}

		var items []dialogs.PickerItem
		for _, r := range containers {
			if c, ok := r.(dao.Container); ok {
				if c.ProjectName == id && c.Ports != "" {
					items = append(items, dialogs.PickerItem{
						Label:       c.Names,
						Description: c.Ports,
						Value:       c.ID,
					})
				}
			}
		}

		app.GetTviewApp().QueueUpdateDraw(func() {
			if len(items) == 0 {
				app.GetPages().RemovePage("picker")
				app.AppendFlashError("no containers with exposed ports in this project")
				return
			}

			dialogs.ShowPicker(app, pickerTitle, items, func(containerID string) {
				var name, portsStr string
				for _, r := range containers {
					if c, ok := r.(dao.Container); ok {
						if c.ID == containerID {
							name = c.Names
							portsStr = c.Ports
							break
						}
					}
				}

				portInfos := parsePortsString(portsStr)
				if len(portInfos) == 0 {
					app.AppendFlashError("no ports found")
					return
				}

				dialogs.ShowPortForwardDialog(app, containerID, name, portInfos, func(result dialogs.PortForwardResult) {
					app.SetFlashPending(fmt.Sprintf("forwarding %s:%d...", result.Address, result.LocalPort))
					app.RunInBackground(func() {
						containerIP, err := app.GetDocker().GetContainerIP(containerID)
						if err != nil {
							app.GetTviewApp().QueueUpdateDraw(func() {
								app.AppendFlashError(fmt.Sprintf("failed to get container IP: %v", err))
							})
							return
						}

						pf := &portforward.PortForward{
							ContextName:   app.GetDocker().ContextName,
							SSHHost:       app.GetDocker().GetSSHHost(),
							ContainerID:   containerID,
							ContainerName: name,
							ContainerIP:   containerIP,
							ContainerPort: result.ContainerPort,
							HostPort:      result.HostPort,
							LocalPort:     result.LocalPort,
						}

						err = app.GetPortForwardManager().Add(pf)
						app.GetTviewApp().QueueUpdateDraw(func() {
							if err != nil {
								app.AppendFlashError(fmt.Sprintf("port-forward failed: %v", err))
							} else {
								app.AppendFlashSuccess(fmt.Sprintf("forwarding %s:%d -> container:%d", result.Address, result.LocalPort, result.ContainerPort))
								app.RefreshCurrentView()
							}
						})
					})
				})
			})
		})
	})
}

func parsePortsString(ports string) []dialogs.PortInfo {
	var result []dialogs.PortInfo
	seen := make(map[uint16]bool)
	for _, part := range strings.Split(ports, ", ") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parts := strings.SplitN(part, "->", 2)
		if len(parts) != 2 {
			continue
		}
		var cp int
		fmt.Sscanf(parts[1], "%d", &cp)
		if cp <= 0 || seen[uint16(cp)] {
			continue
		}
		seen[uint16(cp)] = true
		var hp int
		hostPart := parts[0]
		if idx := strings.LastIndex(hostPart, ":"); idx >= 0 {
			fmt.Sscanf(hostPart[idx+1:], "%d", &hp)
		}
		result = append(result, dialogs.PortInfo{
			ContainerPort: uint16(cp),
			HostPort:      uint16(hp),
			Protocol:      "tcp",
		})
	}
	return result
}
