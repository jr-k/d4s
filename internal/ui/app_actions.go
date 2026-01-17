package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
	"github.com/jr-k/d4s/internal/ui/styles"
)

func (a *App) PerformAction(action func(id string) error, actionName string, color tcell.Color) {
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
		view.SetActionState(id, actionName, color)
	}
	a.RefreshCurrentView()

	a.Flash.SetText(fmt.Sprintf("[yellow]%s %d items...", actionName, len(ids)))

	a.RunInBackground(func() {
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
	})
}

// Helper to get target IDs (Multi or Single)
func (a *App) getTargetIDs(v *view.ResourceView) ([]string, error) {
	if len(v.SelectedIDs) > 0 {
		var ids []string
		for id := range v.SelectedIDs {
			ids = append(ids, id)
		}
		if len(ids) > 0 {
			return ids, nil
		}
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
					label = fmt.Sprintf("%s ([#00ffff]%s[yellow])", label, cells[1])
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
		// view.HighlightIDs(ids, styles.ColorStatusRed, styles.ColorStatusRedDarkBg, time.Second)
		// view.DeferRefresh(time.Second)
		a.PerformAction(simpleAction, "Deleting", styles.ColorStatusRed)
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
		a.RunInBackground(func() {
			err := action(a)
			a.TviewApp.QueueUpdateDraw(func() {
				if err != nil {
					a.Flash.SetText(fmt.Sprintf("[red]Prune Error: %v", err))
				} else {
					a.Flash.SetText(fmt.Sprintf("[green]Pruned %s", name))
					a.RefreshCurrentView()
				}
			})
		})
	})
	a.UpdateShortcuts()
}

func (a *App) PerformCopy() {
	page, _ := a.Pages.GetFrontPage()
	view, ok := a.Views[page]
	if !ok {
		return
	}

	// 1. Check if we have a selection
	row, _ := view.Table.GetSelection()
	if row < 1 || row >= view.Table.GetRowCount() {
		// Nothing selected? maybe header?
		return
	}

	// 2. Identify the resource
	dataIndex := row - 1
	if dataIndex < 0 || dataIndex >= len(view.Data) {
		return
	}
	item := view.Data[dataIndex]

	// 3. Identify the column
	headerName := ""

	// Use currently FOCUSED column if available
	focusedCol := view.GetCurrentColumnFocused()
	if focusedCol != "" {
		headerName = focusedCol
	} else {
		// Fallback to currently sorted column
		sortedCol := view.GetCurrentColumnSorted()
		if sortedCol != "" {
			headerName = sortedCol
		} else {
			// Fallback to default column defined by the resource
			headerName = item.GetDefaultSortColumn()
		}
	}

	// 4. Get the specific value via the abstraction
	value := item.GetColumnValue(headerName)
	if value == "" {
		// Fallback to cell text if abstraction returns empty (or not implemented for that column)
		// Since we want the value of the SORTED column, we must find its index
		colIndex := -1
		for i, h := range view.Headers {
			if strings.EqualFold(h, headerName) {
				colIndex = i
				break
			}
		}

		if colIndex >= 0 {
			cell := view.Table.GetCell(row, colIndex)
			value = common.StripColorTags(strings.TrimSpace(cell.Text))
		}
	} else {
		// Strip tags just in case raw value has them
		value = common.StripColorTags(value)
	}

	// 5. Copy
	if err := a.CopyToClipboard(value); err != nil {
		a.AppendFlash(fmt.Sprintf("[red]Copy error: %v", err))
	} else {
		preview := value
		if len(preview) > 60 {
			preview = preview[:60] + "..."
		}
		a.AppendFlash(fmt.Sprintf("[black:#50fa7b] <copied: %s>[-]", preview))
	}
	a.UpdateShortcuts()
}

func (a *App) CopyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}
