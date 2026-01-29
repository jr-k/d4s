package ui

import (
	"fmt"
	"strings"

	"github.com/jr-k/d4s/internal/ui/styles"
)

func (a *App) ActivateCmd(initial string) {
	a.SetCmdLineVisible(true)
	a.CmdLine.Activate(initial)
}

func (a *App) ExecuteCmd(cmd string) {
	cmd = strings.TrimPrefix(cmd, ":")

	switchToRoot := func(title string) {
		a.SafeSetScope(nil)
		a.SwitchTo(title)
	}

	switch cmd {
	case "q", "quit":
		a.TviewApp.Stop()
	case "c", "co", "con", "container", "containers":
		switchToRoot(styles.TitleContainers)
	case "i", "im", "img", "image", "images":
		switchToRoot(styles.TitleImages)
	case "v", "vo", "vol", "volume", "volumes":
		switchToRoot(styles.TitleVolumes)
	case "n", "ne", "net", "network", "networks":
		switchToRoot(styles.TitleNetworks)
	case "s", "se", "svc", "service", "services":
		switchToRoot(styles.TitleServices)
	case "no", "node", "nodes":
		switchToRoot(styles.TitleNodes)
	case "p", "cp", "compose", "project", "projects":
		switchToRoot(styles.TitleCompose)
	case "a", "al", "alias", "aliases":
		switchToRoot(styles.TitleAliases)
	case "x", "sec", "secret", "secrets":
		switchToRoot(styles.TitleSecrets)
	case "h", "help", "?":
		a.Pages.AddPage("help", a.Help, true, true)
	default:
		a.Flash.SetText(fmt.Sprintf("[red]Unknown command: %s", cmd))
	}
}

func (a *App) SwitchTo(viewName string) {
	a.SwitchToWithSelection(viewName, true)
}

func (a *App) SwitchToWithSelection(viewName string, reset bool) {
	if viewName == "containers" && a.Views["containers"] == nil {
		// Initialize containers view if missing
	}

	if v, ok := a.Views[viewName]; ok {
		// Record previous view
		current, _ := a.Pages.GetFrontPage()
		
		// Avoid stacking the same view as previous repeatedly
		if current != "" && current != viewName && current != "inspect" {
			// Only stack if it's a drill-down (new scope) or a distinct view switch
			
			// Simple deduplication for PreviousView
			if a.PreviousView != current {
				a.PreviousView = current
			}
		}
		
		// Always update CurrentView
		a.CurrentView = viewName
		
		// Flush/Clear manual selection on view switch
		v.SelectedIDs = make(map[string]bool)

		// Reset Selection to top when EXPLICITLY requested (default behavior for navigation)
		if reset && v.Table.GetRowCount() > 1 {
			v.Table.Select(1, 0)
			v.Table.ScrollToBeginning()
		}

		a.Pages.SwitchToPage(viewName)
		a.ActiveFilter = "" // Reset filter on view switch

		// Update Command Line (Reset)
		a.CmdLine.Reset()

		// Show loading if the scope has changed (e.g. drilling down to container volumes)
		shouldClear := false
		if a.ActiveScope != nil {
			if v.CurrentScope == nil || v.CurrentScope.Value != a.ActiveScope.Value || v.CurrentScope.Type != a.ActiveScope.Type {
				shouldClear = true
			}
		} else if v.CurrentScope != nil {
			shouldClear = true
		}

		if shouldClear || len(v.Data) == 0 {
			v.SetLoading(true)
		}

		// Don't spawn a goroutine here! 
		// RefreshCurrentView accesses UI state (ActiveScope, FrontPage) and calls UpdateShortcuts (UI).
		// It internally spawns a background task for the heavy lifting.
		// Running it in 'go' causes race conditions with UI drawing.
		a.RefreshCurrentView()
		a.updateHeader()
		a.TviewApp.SetFocus(a.Pages) // Usually focus page, but actually table

		a.TviewApp.SetFocus(v.Table)
	} else {
		a.Flash.SetText(fmt.Sprintf("[red]Unknown view: %s", viewName))
	}
}
