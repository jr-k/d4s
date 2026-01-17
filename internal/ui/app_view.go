package ui

import (
	"fmt"
	"strings"

	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/styles"
)

func (a *App) RefreshCurrentView() {
	page, _ := a.Pages.GetFrontPage()
	
	// Handle Inspector Filtering
	if page == "inspect" {
		return
	}

	// Modal check logic needs specific naming convention or check
	if page == "help" || page == "logs" || page == "confirm" || page == "result" || page == "input" || page == "textview" {
		return
	}
	
	v, ok := a.Views[page]
	if !ok || v == nil {
		return
	}
	
	filter := a.ActiveFilter

	// 1. Immediate Updates (Optimistic UI) - Inside Queue not needed if Ticker calls this?
	// Ticker runs in BG. `UpdateShortcuts` accesses UI. Should be queued.
	// But let's revert to "working but racy" to fix the "broken views" regression first.
	// a.UpdateShortcuts() -> Moved inside SafeQueueUpdateDraw if needed, but keeping as is for now unless asked.
	// NOTE: UpdateShortcuts() calls existing UI methods which might not be thread safe from ticker.
	a.UpdateShortcuts()
	
	a.RunInBackground(func() {
		if a.IsPaused() {
			return
		}

		var err error
		var data []dao.Resource
		var headers []string

		if v.FetchFunc != nil {
			headers = v.Headers
			data, err = v.FetchFunc(a)
		}

		// Check pause again after fetching (fetching can take time)
		if a.IsPaused() {
			return
		}

		a.SafeQueueUpdateDraw(func() {
			// Check if page changed while fetching?
			currentPage, _ := a.Pages.GetFrontPage()
			if currentPage != page {
				return
			}
			
			v.SetFilter(filter)

			if err != nil {
				a.Flash.SetText(fmt.Sprintf("[red]Error: %v", err))
			} else {
				// Show actual title
				title := a.formatViewTitle(page, fmt.Sprintf("%d", len(v.Data)), filter)
				a.updateViewTitle(v, title)
				
				v.Update(headers, data)

				// Only update flash if not error
				status := ""
				if a.ActiveScope != nil {
					// Dynamic breadcrumb trail coloring: last is always orange, others cyan
					scopes := []string{a.ActiveScope.OriginView, page}
					status += " "
					for i, s := range scopes {
						color := "#00ffff" // cyan
						if i == len(scopes)-1 {
							color = "orange"
						}
						firstChar := ""
						if i > 0 {
							firstChar = " "
						}
						// l'espace entre les éléments doit toujours être noir sur noir
						status += fmt.Sprintf("%s[black:%s] <%s> ", firstChar, color, strings.ToLower(s))
						if i < len(scopes)-1 {
							status += "[black:black]" // black fg, black bg for space
						}
					}
					status += "[-:-:-]"
				} else {
					status = fmt.Sprintf(" [black:orange] <%s> [-:-]", strings.ToLower(page))
				}

				if filter != "" {
					status += fmt.Sprintf(` [black:#bd93f9] <filter: %s> [-]`, filter)
				}
				a.Flash.SetText(status)
			}
		})
	})
}

func (a *App) formatViewTitle(viewName string, countStr string, filter string) string {
	viewName = strings.ToLower(viewName)
	// Show the view name and the number of items
	title := fmt.Sprintf(" [#00ffff::b]%s[#00ffff][[white]%s[#00ffff]] ", viewName, countStr)
	
	// Show the parent view name and the active scope (subview) label
	if a.ActiveScope != nil {
		parentView := strings.ToLower(a.ActiveScope.OriginView)
		cleanLabel := strings.ReplaceAll(a.ActiveScope.Label, "@", "[white] @ [#ff00ff]")
		title = fmt.Sprintf(" [#00ffff::b]%s([-][#ff00ff]%s[#00ffff]) > [#00ffff]%s[#00ffff][[white]%s[#00ffff]] ", 
			parentView, 
			cleanLabel,
			viewName,
			countStr)
	}
	
	if filter != "" {
		title += fmt.Sprintf(" [#00ffff][[white]Filter: [::b]%s[::-][#00ffff]] ", filter)
	}
	return title
}

func (a *App) updateViewTitle(v *view.ResourceView, title string) {
	v.Table.SetTitle(title)
	v.Table.SetTitleColor(styles.ColorTitle)
	v.Table.SetBorder(true)
	v.Table.SetBorderColor(styles.ColorTableBorder)
}

func (a *App) InspectCurrentSelection() {
	page, _ := a.Pages.GetFrontPage()
	view, ok := a.Views[page]
	if !ok {
		return
	}

	row, _ := view.Table.GetSelection()
	if row < 1 || row >= view.Table.GetRowCount() {
		return
	}

	dataIndex := row - 1
	if dataIndex < 0 || dataIndex >= len(view.Data) {
		return
	}
	
	resource := view.Data[dataIndex]
	id := resource.GetID()
	
	if view.InspectFunc != nil {
		view.InspectFunc(a, id)
		a.UpdateShortcuts()
		return
	}
	
	resourceType := "container"
	switch page {
		case styles.TitleImages:
			resourceType = "image"
		case styles.TitleVolumes:
			resourceType = "volume"
		case styles.TitleNetworks:
			resourceType = "network"
		case styles.TitleServices:
			resourceType = "service"
		case styles.TitleNodes:
			resourceType = "node"
		case styles.TitleCompose:
			resourceType = "compose"
	}

	content, err := a.Docker.Inspect(resourceType, id)
	if err != nil {
		a.Flash.SetText(fmt.Sprintf("[red]Inspect error: %v", err))
		return
	}

	a.OpenInspector(inspect.NewTextInspector("Inspect", id, content, "json"))
}
