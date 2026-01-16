package ui

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
	"github.com/jr-k/d4s/internal/ui/styles"
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
				delay := view.PopRefreshDelay()
				if delay > 0 {
					go func() {
						time.Sleep(delay)
						a.RefreshCurrentView()
					}()
				} else {
					a.RefreshCurrentView()
				}
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
	view, ok := a.Views[page]

	if !ok || view.RemoveFunc == nil {
		return
	}

	action := view.RemoveFunc

	ids, err := a.getTargetIDs(view)
	if err != nil {
		return
	}

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
			return action(id, force, a)
		}
		view.HighlightIDs(ids, styles.ColorStatusRed, styles.ColorStatusRedDarkBg, time.Second)
		view.DeferRefresh(time.Second)
		a.PerformAction(simpleAction, "Deleting")
	})
	a.UpdateShortcuts()
}

func (a *App) PerformPrune() {
	page, _ := a.Pages.GetFrontPage()
	view, ok := a.Views[page]

	if !ok || view.PruneFunc == nil {
		a.Flash.SetText(fmt.Sprintf("[yellow]Prune not available for %s", page))
		return
	}

	action := view.PruneFunc
	// Capitalize page name for display (e.g. "images" -> "Images")
	name := strings.Title(page)

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
	if !ok {
		return
	}

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
		a.Flash.SetText(fmt.Sprintf("[#000000:#50fa7b:b] <copied %d rows>[-]", len(view.Data)))
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
