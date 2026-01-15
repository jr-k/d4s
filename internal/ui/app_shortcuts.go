package ui

import (
	"github.com/jessym/d4s/internal/ui/common"
	"github.com/jessym/d4s/internal/ui/styles"
)

func (a *App) getCurrentShortcuts() []string {
	page, _ := a.Pages.GetFrontPage()
	var shortcuts []string
	
	switch page {
	case styles.TitleContainers:
		shortcuts = []string{
			common.FormatSCHeader("l", "Logs"),
			common.FormatSCHeader("s", "Shell"),
			common.FormatSCHeader("d", "Describe"),
			common.FormatSCHeader("e", "Env"),
			common.FormatSCHeader("t", "Stats"),
			common.FormatSCHeader("v", "Volumes"),
			common.FormatSCHeader("n", "Networks"),
			common.FormatSCHeader("r", "(Re)Start"),
			common.FormatSCHeader("x", "Stop"),
		}
	case styles.TitleImages:
		shortcuts = []string{
			common.FormatSCHeader("d", "Describe"),
			common.FormatSCHeader("p", "Prune"),
			common.FormatSCHeader("Ctrl-d", "Delete"),
		}
	case styles.TitleVolumes:
		shortcuts = []string{
			common.FormatSCHeader("d", "Describe"),
			common.FormatSCHeader("o", "Open"),
			common.FormatSCHeader("a", "Add"),
			common.FormatSCHeader("p", "Prune"),
			common.FormatSCHeader("Ctrl-d", "Delete"),
		}
	case styles.TitleNetworks:
		shortcuts = []string{
			common.FormatSCHeader("d", "Describe"),
			common.FormatSCHeader("a", "Add"),
			common.FormatSCHeader("p", "Prune"),
			common.FormatSCHeader("Ctrl-d", "Delete"),
		}
	case styles.TitleServices:
		shortcuts = []string{
			common.FormatSCHeader("d", "Describe"),
			common.FormatSCHeader("s", "Scale"),
			common.FormatSCHeader("Ctrl-d", "Delete"),
		}
	case styles.TitleNodes:
		shortcuts = []string{
			common.FormatSCHeader("d", "Describe"),
			common.FormatSCHeader("Ctrl-d", "Delete"),
		}
	case styles.TitleCompose:
		shortcuts = []string{
			common.FormatSCHeader("Enter", "Containers"),
			common.FormatSCHeader("d", "Describe"),
			common.FormatSCHeader("r", "(Re)Start"),
			common.FormatSCHeader("x", "Stop"),
		}
	case "inspect":
		return []string{
			common.FormatSCHeader("c", "Copy"),
			common.FormatSCHeader("Esc", "Close"),
		}
	case "logs":
		return []string{
			common.FormatSCHeader("s", "AutoScroll"),
			common.FormatSCHeader("w", "Wrap"),
			common.FormatSCHeader("t", "Time"),
			common.FormatSCHeader("c", "Copy"),
			common.FormatSCHeader("S+c", "Clear"),
			common.FormatSCHeader("Esc", "Back"),
		}
	default:
	}
	
	shortcuts = append(shortcuts, common.FormatSCHeader("S+Arr", "Sort"))
	shortcuts = append(shortcuts, common.FormatSCHeader("c", "Copy"))
	shortcuts = append(shortcuts, common.FormatSCHeader("?", "Help"))
	
	return shortcuts
}

func (a *App) UpdateShortcuts() {
	shortcuts := a.getCurrentShortcuts()
	a.Header.UpdateShortcuts(shortcuts)
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

