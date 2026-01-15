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
	// Modal check logic needs specific naming convention or check
	if page == "help" || page == "inspect" || page == "logs" || page == "confirm" || page == "result" || page == "input" || page == "textview" {
		return
	}
	
	v, ok := a.Views[page]
	if !ok || v == nil {
		return
	}
	
	filter := a.ActiveFilter

	// 1. Immediate Updates (Optimistic UI)
	a.UpdateShortcuts()
	
	// Show optimistic title
	title := a.formatViewTitle(page, "...", filter)
	a.updateViewTitle(v, title)

	go func() {
		var err error
		var data []dao.Resource
		var headers []string

		if v.FetchFunc != nil {
			headers = v.Headers
			data, err = v.FetchFunc(a)
		}

		a.TviewApp.QueueUpdateDraw(func() {
			v.SetFilter(filter)

			if err != nil {
				a.Flash.SetText(fmt.Sprintf("[red]Error: %v", err))
			} else {
				// Show actual title
				title := a.formatViewTitle(page, fmt.Sprintf("%d", len(v.Data)), filter)
				a.updateViewTitle(v, title)
				
				v.Update(headers, data)
				
				status := fmt.Sprintf("Viewing %s", page)
				if filter != "" {
					status += fmt.Sprintf(" [orange]Filter: %s", filter)
				}
				a.Flash.SetText(status)
			}
		})
	}()
}

func (a *App) formatViewTitle(viewName string, countStr string, filter string) string {
	viewName = strings.ToLower(viewName)
	// Show the view name and the number of items
	title := fmt.Sprintf(" [#8be9fd]%s[#8be9fd][[white]%s[#8be9fd]] ", viewName, countStr)
	
	// Show the parent view name and the active scope (subview) label
	if a.ActiveScope != nil {
		parentView := strings.ToLower(a.ActiveScope.OriginView)
		title = fmt.Sprintf(" [#8be9fd]%s[dim](%s) > [#bd93f9]%s[#8be9fd][[white]%s[#8be9fd]] ", 
			parentView, 
			a.ActiveScope.Label,
			viewName,
			countStr)
	}
	
	if filter != "" {
		title += fmt.Sprintf(" [#8be9fd][[white]Filter: %s[#8be9fd]] ", filter)
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

	a.OpenInspector(inspect.NewTextInspector(id, content, "json"))
}
