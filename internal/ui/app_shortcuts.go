package ui

import (
	"github.com/jr-k/d4s/internal/ui/common"
)

func (a *App) getCurrentShortcuts() []string {
	page, _ := a.Pages.GetFrontPage()
	var shortcuts []string
	
	// Handle special pages (modals, logs) manually for now, or could attach view logic too.
	if page == "inspect" {
		if a.ActiveInspector != nil {
			return a.ActiveInspector.GetShortcuts() // Use Inspector's shortcuts
		}
	}

	// Try to get view specific shortcuts
	if view, ok := a.Views[page]; ok && view.ShortcutsFunc != nil {
		shortcuts = view.ShortcutsFunc()
	}
	
	shortcuts = append(shortcuts, common.FormatSCHeader("shift ←/→", "Sort"))
	shortcuts = append(shortcuts, common.FormatSCHeader("c", "Copy"))
	shortcuts = append(shortcuts, common.FormatSCHeader("?", "Help"))
	
	return shortcuts
}

func (a *App) UpdateShortcuts() {
	shortcuts := a.getCurrentShortcuts()
	a.Header.UpdateShortcuts(shortcuts)
	a.TviewApp.ForceDraw()
}

func (a *App) updateHeader() {
	go func() {
		stats, err := a.Docker.GetHostStats()
		if err != nil {
			return 
		}
		
		a.TviewApp.QueueUpdateDraw(func() {
			shortcuts := a.getCurrentShortcuts()
			a.Header.Update(stats, shortcuts)
		})
		
		statsWithUsage, err := a.Docker.GetHostStatsWithUsage()
		if err == nil {
			a.TviewApp.QueueUpdateDraw(func() {
				shortcuts := a.getCurrentShortcuts()
				a.Header.Update(statsWithUsage, shortcuts)
			})
		}
	}()
}

