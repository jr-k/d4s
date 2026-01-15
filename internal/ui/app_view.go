package ui

import (
	"fmt"
	"strings"

	"github.com/jessym/d4s/internal/dao"
	"github.com/jessym/d4s/internal/ui/dialogs"
	"github.com/jessym/d4s/internal/ui/styles"
	"github.com/jessym/d4s/internal/ui/views/compose"
	"github.com/jessym/d4s/internal/ui/views/containers"
	"github.com/jessym/d4s/internal/ui/views/images"
	"github.com/jessym/d4s/internal/ui/views/networks"
	"github.com/jessym/d4s/internal/ui/views/nodes"
	"github.com/jessym/d4s/internal/ui/views/services"
	"github.com/jessym/d4s/internal/ui/views/volumes"
)

func (a *App) RefreshCurrentView() {
	page, _ := a.Pages.GetFrontPage()
	// Modal check logic needs specific naming convention or check
	if page == "help" || page == "inspect" || page == "logs" || page == "confirm" || page == "result" || page == "input" || page == "textview" {
		return
	}
	
	view, ok := a.Views[page]
	if !ok || view == nil {
		return
	}
	
	filter := a.ActiveFilter

	// 1. Immediate Updates (Optimistic UI)
	a.UpdateShortcuts()
	
	viewName := strings.ToLower(page)
	title := fmt.Sprintf(" [#8be9fd]%s [#8be9fd][[white]...[#8be9fd]] ", viewName)
	
	if a.ActiveScope != nil {
		parentView := strings.ToLower(a.ActiveScope.OriginView)
		title = fmt.Sprintf(" [#8be9fd]%s [dim](%s) > [#bd93f9]%s [#8be9fd][[white]...[#8be9fd]] ", 
			parentView, 
			a.ActiveScope.Label,
			viewName)
	}
	
	if filter != "" {
		title += fmt.Sprintf(" [#8be9fd][[white]Filter: %s[#8be9fd]] ", filter)
	}
	
	view.Table.SetTitle(title)
	view.Table.SetTitleColor(styles.ColorTitle)
	view.Table.SetBorder(true)
	view.Table.SetBorderColor(styles.ColorTableBorder)

	go func() {
		var err error
		var data []dao.Resource
		var headers []string

		switch page {
		case styles.TitleContainers:
			headers = containers.Headers
			data, err = containers.Fetch(a)
		case styles.TitleCompose:
			headers = compose.Headers
			data, err = compose.Fetch(a)
		case styles.TitleImages:
			headers = images.Headers
			data, err = images.Fetch(a)
		case styles.TitleVolumes:
			headers = volumes.Headers
			data, err = volumes.Fetch(a)
		case styles.TitleNetworks:
			headers = networks.Headers
			data, err = networks.Fetch(a)
		case styles.TitleServices:
			headers = services.Headers
			data, err = services.Fetch(a)
		case styles.TitleNodes:
			headers = nodes.Headers
			data, err = nodes.Fetch(a)
		}

		a.TviewApp.QueueUpdateDraw(func() {
			view.SetFilter(filter)

			if err != nil {
				a.Flash.SetText(fmt.Sprintf("[red]Error: %v", err))
			} else {
				viewName := strings.ToLower(page)
				title := fmt.Sprintf(" [#8be9fd]%s [#8be9fd][[white]%d[#8be9fd]] ", viewName, len(view.Data))
				
				if a.ActiveScope != nil {
					parentView := strings.ToLower(a.ActiveScope.OriginView)
					title = fmt.Sprintf(" [#8be9fd]%s [dim](%s) > [#bd93f9]%s [#8be9fd][[white]%d[#8be9fd]] ", 
						parentView, 
						a.ActiveScope.Label,
						viewName,
						len(view.Data))
				}
				
				if filter != "" {
					title += fmt.Sprintf(" [#8be9fd][[white]Filter: %s[#8be9fd]] ", filter)
				}
				view.Table.SetTitle(title)
				view.Table.SetTitleColor(styles.ColorTitle)
				view.Table.SetBorder(true)
				view.Table.SetBorderColor(styles.ColorTableBorder)
				
				view.Update(headers, data)
				
				status := fmt.Sprintf("Viewing %s", page)
				if filter != "" {
					status += fmt.Sprintf(" [orange]Filter: %s", filter)
				}
				a.Flash.SetText(status)
			}
		})
	}()
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
		// Special case for Compose
		content, err := a.Docker.GetComposeConfig(id)
		if err != nil {
			a.Flash.SetText(fmt.Sprintf("[red]Inspect error: %v", err))
			return
		}
		dialogs.ShowInspectModal(a, id, content)
		a.UpdateShortcuts()
		return
	}

	content, err := a.Docker.Inspect(resourceType, id)
	if err != nil {
		a.Flash.SetText(fmt.Sprintf("[red]Inspect error: %v", err))
		return
	}

	dialogs.ShowInspectModal(a, id, content)
	a.UpdateShortcuts()
}
