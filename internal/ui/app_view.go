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
		if a.ActiveInspector != nil {
			// Construct breadcrumb manually for inspector
			actionName := "inspect"

			switch v := a.ActiveInspector.(type) {
			case *inspect.LogInspector:
				actionName = "logs"
			case *inspect.TextInspector:
				actionName = strings.ToLower(v.Action)
				// Simplify "Describe container" to "describe" in breadcrumb
				if strings.Contains(actionName, " ") {
					parts := strings.Split(actionName, " ")
					if len(parts) > 0 {
						actionName = parts[0]
					}
				}
			case *inspect.StatsInspector:
				actionName = "stats"
			}

			status := ""

			// Start with base context
			// Use CurrentView if available, or ActiveScope logic
			baseView := a.CurrentView
			if baseView == "" {
				baseView = "containers" // Default fallback
			}
			
			scope := a.GetActiveScope()
			// If we have ActiveScope, it means we are in drilled down mode
			if scope != nil {
				// E.g. <compose> <containers> <logs>
				var breadcrumbs []string
				curr := scope
				for curr != nil {
					if curr.OriginView != "" {
						breadcrumbs = append([]string{curr.OriginView}, breadcrumbs...)
					}
					curr = curr.Parent
				}
				breadcrumbs = append(breadcrumbs, baseView)
				breadcrumbs = append(breadcrumbs, actionName)
				
				status += " "
				for i, s := range breadcrumbs {
					color := styles.TagCyan
					if i == len(breadcrumbs)-1 {
						color = styles.TagAccent
					}
					firstChar := ""
					if i > 0 {
						firstChar = " "
					}
					status += fmt.Sprintf("%s[%s:%s] <%s> ", firstChar, styles.TagBg, color, strings.ToLower(s))
					if i < len(breadcrumbs)-1 {
						status += fmt.Sprintf("[%s:%s]", styles.TagBg, styles.TagBg)
					}
				}
				status += "[-:-:-]"
			} else {
				// E.g. <containers> <logs>
				scopes := []string{baseView, actionName}
				
				status += " "
				for i, s := range scopes {
					color := styles.TagCyan
					if i == len(scopes)-1 {
						color = styles.TagAccent
					}
					firstChar := ""
					if i > 0 {
						firstChar = " "
					}
					status += fmt.Sprintf("%s[%s:%s] <%s> ", firstChar, styles.TagBg, color, strings.ToLower(s))
					if i < len(scopes)-1 {
						status += fmt.Sprintf("[%s:%s]", styles.TagBg, styles.TagBg)
					}
				}
				status += "[-:-:-]"
			}

			if !a.IsFlashLocked() && !a.Cfg.D4S.UI.Crumbsless {
				a.Flash.SetText(status)
			}
		}
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
	
	// 1. Immediate Updates (Optimistic UI)
	// UpdateShortcuts modifies the UI. Must be called from main thread.
	// Since RefreshCurrentView is sometimes called from valid UI context (SwitchTo) 
	// and sometimes from BG (Ticker -> QueueUpdateDraw), we assume we are in Main Thread here IF caller respected rules.
	// BUT, we previously wrapped Ticker calls in QueueUpdateDraw.
	// So UpdateShortcuts is safe here.
	a.UpdateShortcuts()
	
	a.RunInBackground(func() {
		if a.IsPaused() {
			return
		}

		var err error
		var data []dao.Resource
		var headers []string

		if v.FetchFunc != nil {
			data, err = v.FetchFunc(a, v)
			headers = v.Headers
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

			// Read filter at callback time (not before the fetch) to avoid
			// overwriting a filter the user set while the fetch was in flight.
			filter := a.ActiveFilter

			v.SetFilter(filter)
			v.CurrentScope = a.GetActiveScope()

			if err != nil {
				a.Flash.SetText(fmt.Sprintf("[%s]Error: %v", styles.TagError, err))
			} else {
				// Show actual title
				title := a.formatViewTitle(page, fmt.Sprintf("%d", len(v.Data)), filter)
				a.updateViewTitle(v, title)
				
				v.Update(headers, data)

				// Only update flash if not error
				status := ""
				scope := a.GetActiveScope()
				if scope != nil {
					// Dynamic breadcrumb trail coloring: last is always orange, others cyan
					// Traverse up to get full history
					var breadcrumbs []string
					curr := scope
					for curr != nil {
						if curr.OriginView != "" {
							breadcrumbs = append([]string{curr.OriginView}, breadcrumbs...)
						}
						curr = curr.Parent
					}
					// Add current page
					breadcrumbs = append(breadcrumbs, page)

					status += " "
					for i, s := range breadcrumbs {
						color := styles.TagCyan
						if i == len(breadcrumbs)-1 {
							color = styles.TagAccent
						}
						firstChar := ""
						if i > 0 {
							firstChar = " "
						}
						status += fmt.Sprintf("%s[%s:%s] <%s> ", firstChar, styles.TagBg, color, strings.ToLower(s))
						if i < len(breadcrumbs)-1 {
							status += fmt.Sprintf("[%s:%s]", styles.TagBg, styles.TagBg)
						}
					}
					status += "[-:-:-]"
				} else {
					status = fmt.Sprintf(" [%s:%s] <%s> [-:-]", styles.TagBg, styles.TagAccent, strings.ToLower(page))
				}

				if filter != "" {
					status += fmt.Sprintf(` [%s:%s] <filter: %s> [-:-]`, styles.TagBg, styles.TagFilter, filter)
				}
				
				// Only update flash if not locked by temporary message and crumbs are enabled
				if !a.IsFlashLocked() && !a.Cfg.D4S.UI.Crumbsless {
					a.Flash.SetText(status)
				}
			}
		})
	})
}

// preloadViews fetches data for all views in background so navigation is instant.
// The initial view is skipped since it's already being refreshed by auto-refresh.
func (a *App) preloadViews() {
	initialView := a.resolveDefaultView()

	for title, v := range a.Views {
		if title == initialView {
			continue // Already being refreshed by StartAutoRefresh
		}
		if v.FetchFunc == nil {
			continue
		}

		go func(title string, v *view.ResourceView) {
			data, err := v.FetchFunc(a, v)
			if err != nil {
				return // Silently ignore preload errors
			}

			headers := v.Headers // Capture after fetch (FetchFunc may update headers)

			a.SafeQueueUpdateDraw(func() {
				// Don't overwrite if the user already navigated here
				currentPage, _ := a.Pages.GetFrontPage()
				if currentPage == title {
					return
				}

				v.CurrentScope = nil
				v.Update(headers, data)

				countStr := fmt.Sprintf("%d", len(v.Data))
				viewTitle := a.formatViewTitle(title, countStr, "")
				a.updateViewTitle(v, viewTitle)
			})
		}(title, v)
	}
}

func (a *App) formatViewTitle(viewName string, countStr string, filter string) string {
	viewName = strings.ToLower(viewName)
	
	// Default simple title
	title := fmt.Sprintf(" [%s::b]%s[%s][[%s]%s[%s]] ", styles.TagCyan, viewName, styles.TagCyan, styles.TagFg, countStr, styles.TagCyan)
	
	// Dynamic recursive breadcrumb
	scope := a.GetActiveScope()
	if scope != nil {
		var parts []string
		
		// Walk up the stack
		curr := scope
		for curr != nil {
			cleanLabel := strings.ReplaceAll(curr.Label, "@", fmt.Sprintf("[%s] @ [%s]", styles.TagFg, styles.TagPink))
			origin := strings.ToLower(curr.OriginView)
			
			// Format: "origin(label)"
			part := fmt.Sprintf("[%s::b]%s([-][%s]%s[%s])", styles.TagCyan, origin, styles.TagPink, cleanLabel, styles.TagCyan)
			// Prepend to list (since we walk backwards)
			parts = append([]string{part}, parts...)
			
			curr = curr.Parent
		}
		
		// Append current view name
		parts = append(parts, fmt.Sprintf("[%s]%s[%s][[%s]%s[%s]]", styles.TagCyan, viewName, styles.TagCyan, styles.TagFg, countStr, styles.TagCyan))
		
		title = " " + strings.Join(parts, " > ") + " "
	}
	
	if filter != "" {
		title += fmt.Sprintf(" [%s][[%s]Filter: [::b]%s[::-][%s]] ", styles.TagCyan, styles.TagFg, filter, styles.TagCyan)
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
		// InspectFunc typically opens a new modal or changes view.
		// We should refresh shortcuts to reflect the new state immediately.
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

	inspector := inspect.NewTextInspector("Inspect", id, fmt.Sprintf(" [%s]Loading %s...\n", styles.TagAccent, resourceType), "json")
	a.OpenInspector(inspector)

	a.RunInBackground(func() {
		content, err := a.Docker.Inspect(resourceType, id)
		a.GetTviewApp().QueueUpdateDraw(func() {
			if err != nil {
				inspector.Viewer.Update(fmt.Sprintf("Error: %v", err), "text")
			} else {
				inspector.Viewer.Update(content, "json")
				inspector.Viewer.View.SetTitle(inspector.GetTitle())
			}
		})
	})
}
