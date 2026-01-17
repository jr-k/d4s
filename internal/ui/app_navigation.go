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
		a.ActiveScope = nil
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
	if v, ok := a.Views[viewName]; ok {
		// Reset Selection to top when EXPLICITLY requested (default behavior for navigation)
		if reset && v.Table.GetRowCount() > 1 {
			v.Table.Select(1, 0)
			v.Table.ScrollToBeginning()
		}

		a.Pages.SwitchToPage(viewName)
		a.ActiveFilter = "" // Reset filter on view switch

		// Update Command Line (Reset)
		a.CmdLine.Reset()

		go a.RefreshCurrentView()
		a.updateHeader()
		a.TviewApp.SetFocus(a.Pages) // Usually focus page, but actually table

		a.TviewApp.SetFocus(v.Table)
	} else {
		a.Flash.SetText(fmt.Sprintf("[red]Unknown view: %s", viewName))
	}
}
