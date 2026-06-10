package ui

import (
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
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
	
	shortcuts = append(shortcuts, common.FormatSCHeaderGlobal("shift-o", "Context"))
	shortcuts = append(shortcuts, common.FormatSCHeaderGlobal("shift ←/→", "Sort"))
	shortcuts = append(shortcuts, common.FormatSCHeaderGlobal("shift-c", "Copy Table"))
	shortcuts = append(shortcuts, common.FormatSCHeaderGlobal("c", "Copy Cell"))
	shortcuts = append(shortcuts, common.FormatSCHeaderGlobal("u", "Unselect All"))
	shortcuts = append(shortcuts, common.FormatSCHeaderGlobal("?", "Help"))
	
	return shortcuts
}

func (a *App) UpdateShortcuts() {
	shortcuts := a.getCurrentShortcuts()
	a.Header.UpdateShortcuts(shortcuts)

	page, _ := a.Pages.GetFrontPage()
	_, isView := a.Views[page]
	modalActive := !isView && page != "inspect" && page != ""

	if v, ok := a.Views[a.CurrentView]; ok && v.Table != nil {
		if modalActive {
			v.Table.SetBorderColor(styles.ColorMenuKey)
		} else {
			v.Table.SetBorderColor(styles.ColorTableBorder)
		}
	}
}

func (a *App) updateHeader() {
	docker := a.Docker
	go func() {
		stats, err := docker.GetHostStats()
		if err != nil {
			return
		}

		a.TviewApp.QueueUpdateDraw(func() {
			if a.Docker != docker {
				return
			}
			shortcuts := a.getCurrentShortcuts()
			stats.LatestVersion = a.LatestVersion
			a.Header.Update(stats, shortcuts)
		})

		statsWithUsage, err := docker.GetHostStatsWithUsage()
		if err == nil {
			a.TviewApp.QueueUpdateDraw(func() {
				if a.Docker != docker {
					return
				}
				shortcuts := a.getCurrentShortcuts()
				statsWithUsage.LatestVersion = a.LatestVersion
				a.Header.Update(statsWithUsage, shortcuts)
			})
		}
	}()
}
