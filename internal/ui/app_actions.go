package ui

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/jessym/d4s/internal/ui/common"
	"github.com/jessym/d4s/internal/ui/components/view"
	"github.com/jessym/d4s/internal/ui/dialogs"
	"github.com/jessym/d4s/internal/ui/styles"
	"github.com/jessym/d4s/internal/ui/views/containers"
	"github.com/jessym/d4s/internal/ui/views/images"
	"github.com/jessym/d4s/internal/ui/views/networks"
	"github.com/jessym/d4s/internal/ui/views/nodes"
	"github.com/jessym/d4s/internal/ui/views/services"
	"github.com/jessym/d4s/internal/ui/views/volumes"
)

func (a *App) PerformAction(action func(id string) error, actionName string) {
	page, _ := a.Pages.GetFrontPage()
	view, ok := a.Views[page]
	if !ok {
		return
	}
	
	ids, err := a.getTargetIDs(view)
	if err != nil {
		return
	}

	for _, id := range ids {
		view.SetActionState(id, actionName)
	}
	a.RefreshCurrentView()

	a.Flash.SetText(fmt.Sprintf("[yellow]%s %d items...", actionName, len(ids)))
	
	go func() {
		var errs []string
		for _, id := range ids {
			if err := action(id); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", id, err))
			}
		}
		
		a.TviewApp.QueueUpdateDraw(func() {
			for _, id := range ids {
				view.ClearActionState(id)
			}
			
			if len(errs) > 0 {
				dialogs.ShowResultModal(a, actionName, len(ids)-len(errs), errs)
			} else {
				a.Flash.SetText(fmt.Sprintf("[green]%s %d items done", actionName, len(ids)))
				// Clear selection on success?
				view.SelectedIDs = make(map[string]bool)
				a.RefreshCurrentView() 
			}
			a.UpdateShortcuts()
		})
	}()
}

// Helper to get target IDs (Multi or Single)
func (a *App) getTargetIDs(v *view.ResourceView) ([]string, error) {
	if len(v.SelectedIDs) > 0 {
		var ids []string
		for id := range v.SelectedIDs {
			ids = append(ids, id)
		}
		return ids, nil
	}
	// Fallback to single selection
	id, err := a.getSelectedID(v)
	if err != nil {
		return nil, err
	}
	return []string{id}, nil
}

func (a *App) getSelectedID(v *view.ResourceView) (string, error) {
	row, _ := v.Table.GetSelection()
	if row < 1 || row >= v.Table.GetRowCount() {
		return "", fmt.Errorf("no selection")
	}

	dataIndex := row - 1
	if dataIndex < 0 || dataIndex >= len(v.Data) {
		return "", fmt.Errorf("invalid index")
	}
	
	return v.Data[dataIndex].GetID(), nil
}


func (a *App) PerformDelete() {
	page, _ := a.Pages.GetFrontPage()
	var action func(id string, force bool) error
	
	switch page {
	case styles.TitleContainers:
		action = func(id string, force bool) error {
			return containers.Remove(id, force, a)
		}
	case styles.TitleImages:
		action = func(id string, force bool) error {
			return images.Remove(id, force, a)
		}
	case styles.TitleVolumes:
		action = func(id string, force bool) error {
			return volumes.Remove(id, force, a)
		}
	case styles.TitleNetworks:
		action = func(id string, force bool) error {
			return networks.Remove(id, force, a)
		}
	case styles.TitleServices:
		action = func(id string, force bool) error {
			return services.Remove(id, force, a)
		}
	case styles.TitleNodes:
		action = func(id string, force bool) error {
			return nodes.Remove(id, force, a)
		}
	default:
		return
	}
	
	view, ok := a.Views[page]
	if !ok { return }
	
	ids, err := a.getTargetIDs(view)
	if err != nil { return }

	label := ids[0]
	if len(ids) == 1 {
		row, _ := view.Table.GetSelection()
		if row > 0 && row <= len(view.Data) {
			item := view.Data[row-1]
			if item.GetID() == ids[0] {
				cells := item.GetCells()
				if len(cells) > 1 {
					label = fmt.Sprintf("%s ([#8be9fd]%s[yellow])", label, cells[1])
				}
			}
		}
	} else if len(ids) > 1 {
		label = fmt.Sprintf("%d items", len(ids))
	}

	dialogs.ShowConfirmation(a, "DELETE", label, func(force bool) {
		simpleAction := func(id string) error {
			return action(id, force)
		}
		a.PerformAction(simpleAction, "Deleting")
	})
	a.UpdateShortcuts()
}

func (a *App) PerformPrune() {
	page, _ := a.Pages.GetFrontPage()
	var action func(common.AppController) error
	var name string

	switch page {
	case styles.TitleImages:
		action = images.Prune
		name = "Images"
	case styles.TitleVolumes:
		action = volumes.Prune
		name = "Volumes"
	case styles.TitleNetworks:
		action = networks.Prune
		name = "Networks"
	default:
		a.Flash.SetText(fmt.Sprintf("[yellow]Prune not available for %s", page))
		return
	}

	dialogs.ShowConfirmation(a, "PRUNE", name, func(force bool) {
		a.Flash.SetText(fmt.Sprintf("[yellow]Pruning %s...", name))
		go func() {
			err := action(a)
			a.TviewApp.QueueUpdateDraw(func() {
				if err != nil {
					a.Flash.SetText(fmt.Sprintf("[red]Prune Error: %v", err))
				} else {
					a.Flash.SetText(fmt.Sprintf("[green]Pruned %s", name))
					a.RefreshCurrentView()
				}
			})
		}()
	})
	a.UpdateShortcuts()
}

func (a *App) PerformCopy() {
	page, _ := a.Pages.GetFrontPage()
	view, ok := a.Views[page]
	if !ok { return }

	var buffer bytes.Buffer
	
	// Headers
	var headers []string
	for i := 0; i < view.Table.GetColumnCount(); i++ {
		cell := view.Table.GetCell(0, i)
		headers = append(headers, common.StripColorTags(strings.TrimSpace(cell.Text)))
	}
	buffer.WriteString(strings.Join(headers, "\t") + "\n")

	// Data Rows (view.Data is already filtered/sorted)
	for _, item := range view.Data {
		cells := item.GetCells()
		var line []string
		for _, cell := range cells {
			line = append(line, common.StripColorTags(cell))
		}
		buffer.WriteString(strings.Join(line, "\t") + "\n")
	}
	
	if err := a.CopyToClipboard(buffer.String()); err != nil {
		a.Flash.SetText(fmt.Sprintf("[red]Copy error: %v", err))
	} else {
		a.Flash.SetText(fmt.Sprintf("[green]Copied %d rows", len(view.Data)))
	}
	a.UpdateShortcuts()
}

func (a *App) CopyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try xclip first, then xsel
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else {
			return fmt.Errorf("no clipboard tool found (install xclip or xsel)")
		}
	case "windows":
		cmd = exec.Command("clip")
	default:
		return fmt.Errorf("unsupported OS")
	}

	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func (a *App) PerformEnv() {
	page, _ := a.Pages.GetFrontPage()
	view, ok := a.Views[page]
	if !ok { return }
	id, err := a.getSelectedID(view)
	if err != nil { return }

	env, err := a.Docker.GetContainerEnv(id)
	if err != nil {
		a.Flash.SetText(fmt.Sprintf("[red]Env Error: %v", err))
		return
	}

	var colored []string
	for _, line := range env {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			colored = append(colored, fmt.Sprintf("[#8be9fd]%s[white]=[#50fa7b]%s", parts[0], parts[1]))
		} else {
			colored = append(colored, line)
		}
	}
	
	dialogs.ShowTextView(a, " Environment ", strings.Join(colored, "\n"))
	a.UpdateShortcuts()
}

func (a *App) PerformStats() {
	page, _ := a.Pages.GetFrontPage()
	view, ok := a.Views[page]
	if !ok { return }
	id, err := a.getSelectedID(view)
	if err != nil { return }

	a.Flash.SetText(fmt.Sprintf("[yellow]Fetching stats for %s...", id))
	go func() {
		stats, err := a.Docker.GetContainerStats(id)
		a.TviewApp.QueueUpdateDraw(func() {
			if err != nil {
				a.Flash.SetText(fmt.Sprintf("[red]Stats Error: %v", err))
			} else {
				a.Flash.SetText("")
				colored := strings.ReplaceAll(stats, "\"", "[#f1fa8c]\"")
				colored = strings.ReplaceAll(colored, ": ", ": [#50fa7b]")
				dialogs.ShowTextView(a, " Stats ", colored)
				a.UpdateShortcuts()
			}
		})
	}()
}

func (a *App) PerformContainerVolumes() {
	page, _ := a.Pages.GetFrontPage()
	view, ok := a.Views[page]
	if !ok { return }
	id, err := a.getSelectedID(view)
	if err != nil { return }

	content, err := a.Docker.Inspect("container", id)
	if err != nil {
		a.Flash.SetText(fmt.Sprintf("[red]Inspect Error: %v", err))
		return
	}
	dialogs.ShowInspectModal(a, "Volumes (JSON)", content)
	a.UpdateShortcuts()
}

func (a *App) PerformContainerNetworks() {
	page, _ := a.Pages.GetFrontPage()
	view, ok := a.Views[page]
	if !ok { return }
	id, err := a.getSelectedID(view)
	if err != nil { return }

	content, err := a.Docker.Inspect("container", id)
	if err != nil {
		a.Flash.SetText(fmt.Sprintf("[red]Inspect Error: %v", err))
		return
	}
	dialogs.ShowInspectModal(a, "Networks (JSON)", content)
	a.UpdateShortcuts()
}

func (a *App) PerformCreateVolume() {
	volumes.Create(a)
	a.UpdateShortcuts()
}

func (a *App) PerformCreateNetwork() {
	networks.Create(a)
	a.UpdateShortcuts()
}

func (a *App) PerformOpenVolume() {
	page, _ := a.Pages.GetFrontPage()
	view, ok := a.Views[page]
	if !ok { return }
	
	row, _ := view.Table.GetSelection()
	if row < 1 || row >= len(view.Data)+1 { return }
	
	dataIdx := row - 1
	res := view.Data[dataIdx]
	
	volumes.Open(a, res)
	a.UpdateShortcuts()
}

func (a *App) PerformScale() {
	page, _ := a.Pages.GetFrontPage()
	if page != styles.TitleServices { return }
	
	view, ok := a.Views[page]
	if !ok { return }
	
	id, err := a.getSelectedID(view)
	if err != nil { return }
    
	currentReplicas := ""
	row, _ := view.Table.GetSelection()
	if row > 0 && row <= len(view.Data) {
		item := view.Data[row-1]
		cells := item.GetCells()
		if len(cells) > 4 {
			currentReplicas = strings.TrimSpace(cells[4])
		}
	}

	services.Scale(a, id, currentReplicas)
	a.UpdateShortcuts()
}

func (a *App) PerformShell() {
	page, _ := a.Pages.GetFrontPage()
	view, ok := a.Views[page]
	if !ok { return }
	id, err := a.getSelectedID(view)
	if err != nil { return }

	if page == styles.TitleContainers {
		containers.Shell(a, id)
	}
	a.UpdateShortcuts()
}

func (a *App) PerformLogs() {
	page, _ := a.Pages.GetFrontPage()
	resourceView, ok := a.Views[page]
	if !ok { return }
	id, err := a.getSelectedID(resourceView)
	if err != nil { return }

	resourceType := "container"
	if page == styles.TitleServices {
		resourceType = "service"
	}

	logView := view.NewLogView(a, id, resourceType)
	a.Pages.AddPage("logs", logView, true, true)
	a.TviewApp.SetFocus(logView)

	a.UpdateShortcuts()
}

