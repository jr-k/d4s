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

	// Use temporary flash message that locks manual updates from RefreshCurrentView
	// Also stop AutoRefresh to prevent list update from clearing the red selection
	a.StopAutoRefresh()
	plural := "s"
	if len(ids) == 1 {
		plural = ""
	}
	a.AppendFlashPending(fmt.Sprintf("%s %d item%s...", actionName, len(ids), plural))

	a.RunInBackground(func() {
		var errs []string
		for _, id := range ids {
			if err := action(id); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", id, err))
			}
		}

		time.Sleep(1 * time.Second)

		a.TviewApp.QueueUpdateDraw(func() {
			for _, id := range ids {
				view.ClearActionState(id)
			}
			
			// Resume AutoRefresh first
			a.StartAutoRefresh()

			if len(errs) > 0 {
				a.AppendFlashError(fmt.Sprintf("%s completed with errors", actionName))
				dialogs.ShowResultModal(a, actionName, len(ids)-len(errs), errs)
			} else {
				plural := "s"
				if len(ids) == 1 {
					plural = ""
				}
				// Show success message for 3 seconds
				a.AppendFlashSuccess(fmt.Sprintf("%s %d item%s done", actionName, len(ids), plural))
				
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
		a.AppendFlashError(fmt.Sprintf("%v", err))
	} else {
		preview := value
		if len(preview) > 60 {
			preview = preview[:60] + "..."
		}
		a.AppendFlashSuccess(fmt.Sprintf("copied %s", preview))
	}
	a.UpdateShortcuts()
}

func (a *App) CopyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}

func (a *App) GetActionState(viewName string, id string) (string, tcell.Color, bool) {
	if v, ok := a.Views[viewName]; ok {
		if state, ok := v.ActionStates[id]; ok {
			return state.Label, state.Color, true
		}
	}
	return "", tcell.ColorDefault, false
}
