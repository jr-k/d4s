package ui

import (
	"fmt"
	"strings"
	"time"

	"runtime/debug"

	"github.com/gdamore/tcell/v2"
	"github.com/jessym/d4s/internal/dao"
	"github.com/jessym/d4s/internal/ui/common"
	"github.com/jessym/d4s/internal/ui/components/command"
	"github.com/jessym/d4s/internal/ui/components/footer"
	"github.com/jessym/d4s/internal/ui/components/header"
	"github.com/jessym/d4s/internal/ui/components/view"
	"github.com/jessym/d4s/internal/ui/dialogs"
	"github.com/jessym/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

type App struct {
	TviewApp *tview.Application
	Docker   *dao.DockerClient

	// Components
	Layout  *tview.Flex
	Header  *header.HeaderComponent
	Pages   *tview.Pages
	CmdLine *command.CommandComponent
	Flash   *footer.FlashComponent
	// Footer  *footer.FooterComponent // Legacy?
	Help    tview.Primitive

	// Views
	Views map[string]*view.ResourceView
	
	// State
	ActiveFilter  string
	ActiveScope   *common.Scope
}

// Ensure App implements AppController interface
var _ common.AppController = (*App)(nil)

func NewApp() *App {
	docker, err := dao.NewDockerClient()
	if err != nil {
		panic(err)
	}

	app := &App{
		TviewApp: tview.NewApplication(),
		Docker:   docker,
		Views:    make(map[string]*view.ResourceView),
		Pages:    tview.NewPages(),
	}

	app.initUI()
	return app
}

func (a *App) Run() error {
	defer func() {
		if r := recover(); r != nil {
			a.TviewApp.Stop()
			fmt.Printf("Application crashed: %v\nStack trace:\n%s\n", r, string(debug.Stack()))
		}
	}()

	go func() {
		// Initial Delay for UI setup
		time.Sleep(100 * time.Millisecond)
		a.RefreshCurrentView()
		a.updateHeader()
		
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			a.RefreshCurrentView()
			a.updateHeader()
		}
	}()

	return a.TviewApp.SetRoot(a.Layout, true).Run()
}

func (a *App) initUI() {
	// 1. Header
	a.Header = header.NewHeaderComponent()
	
	// 2. Main Content
	a.Views[styles.TitleContainers] = view.NewResourceView(a, styles.TitleContainers)
	a.Views[styles.TitleImages] = view.NewResourceView(a, styles.TitleImages)
	a.Views[styles.TitleVolumes] = view.NewResourceView(a, styles.TitleVolumes)
	a.Views[styles.TitleNetworks] = view.NewResourceView(a, styles.TitleNetworks)
	a.Views[styles.TitleServices] = view.NewResourceView(a, styles.TitleServices)
	a.Views[styles.TitleNodes] = view.NewResourceView(a, styles.TitleNodes)
	a.Views[styles.TitleCompose] = view.NewResourceView(a, styles.TitleCompose)

	for title, view := range a.Views {
		a.Pages.AddPage(title, view.Table, true, false)
	}

	// 3. Command Line & Flash & Footer
	a.CmdLine = command.NewCommandComponent(a)
	
	a.Flash = footer.NewFlashComponent()
	// a.Footer = footer.NewFooterComponent()

	// 4. Help View
	a.Help = dialogs.NewHelpView(a)

	// 6. Layout
	a.Layout = tview.NewFlex().SetDirection(tview.FlexRow)
	a.Layout.SetBackgroundColor(styles.ColorBg)

	a.Layout.AddItem(a.Header.View, 6, 1, false).
		AddItem(a.CmdLine.View, 0, 0, false). // Hidden by default (size 0, proportion 0)
		AddItem(a.Pages, 0, 1, true).
		AddItem(a.Flash.View, 1, 1, false)
		// AddItem(a.Footer.View, 1, 1, false)

	// Global Shortcuts
	a.TviewApp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if a.CmdLine.HasFocus() {
			return event
		}
		
		// Helper to close modals if open
		if a.Pages.HasPage("inspect") {
			if event.Key() == tcell.KeyEsc {
				a.Pages.RemovePage("inspect")
				// Restore focus to active view
				page, _ := a.Pages.GetFrontPage()
				if view, ok := a.Views[page]; ok {
					a.TviewApp.SetFocus(view.Table)
				}
				a.UpdateShortcuts() // Update shortcuts immediately
				return nil
			}
			// Pass 'c' through to modal
			if event.Rune() == 'c' {
				return event
			}
		}

	// Don't intercept global keys if an input modal is open
	frontPage, _ := a.Pages.GetFrontPage()
	if frontPage == "input" || frontPage == "confirm" {
		return event
	}

	// Handle Esc to clear filter and exit scope
	if event.Key() == tcell.KeyEsc {
		// Priority 1: Clear active filter if any
		if a.ActiveFilter != "" {
			a.ActiveFilter = ""
			a.CmdLine.Reset()
			a.RefreshCurrentView()
			a.Flash.SetText("")
			return nil
		}
		
		// Priority 2: Exit scope if active (return to origin view)
		if a.ActiveScope != nil {
			origin := a.ActiveScope.OriginView
			a.ActiveScope = nil
			a.SwitchTo(origin)
			return nil
		}
		
		return event
	}

	switch event.Rune() {
		case ':':
			a.ActivateCmd(":")
			return nil
		case '/':
			a.ActivateCmd("/")
			return nil
		case '?':
			a.Pages.AddPage("help", a.Help, true, true)
			return nil
		case 'd':
			a.InspectCurrentSelection()
			return nil
		case 'l':
			page, _ := a.Pages.GetFrontPage()
			if page == styles.TitleContainers {
				a.PerformLogs()
			}
			return nil
		case 's':
			page, _ := a.Pages.GetFrontPage()
			if page == styles.TitleContainers {
				a.PerformShell()
			} else if page == styles.TitleServices {
				a.PerformScale()
			}
			return nil
		case 'a': // Contextual Add (Create)
			page, _ := a.Pages.GetFrontPage()
			if page == styles.TitleVolumes {
				a.PerformCreateVolume()
				return nil
			}
			if page == styles.TitleNetworks {
				a.PerformCreateNetwork()
				return nil
			}
			return nil
		case 'c': // Global Copy
			a.PerformCopy()
			return nil
		case 'o': // Open Volume
			page, _ := a.Pages.GetFrontPage()
			if page == styles.TitleVolumes {
				a.PerformOpenVolume()
				return nil
			}
			return nil
		case 'r': // Restart / Start
			// Only Containers
			page, _ := a.Pages.GetFrontPage()
			if page == styles.TitleContainers {
				// Check status to decide Start or Restart
				view, ok := a.Views[page]
				if ok {
					// Check status from data
					row, _ := view.Table.GetSelection()
					if row > 0 && row <= len(view.Data) {
						item := view.Data[row-1]
						if c, ok := item.(dao.Container); ok {
							// If Exited or Created -> Start
							lowerStatus := strings.ToLower(c.Status)
							if strings.Contains(lowerStatus, "exited") || strings.Contains(lowerStatus, "created") {
								a.PerformAction(func(id string) error {
									return a.Docker.StartContainer(id)
								}, "Starting")
								return nil
							}
						}
					}
				}
				
				// Default to Restart
				a.PerformAction(func(id string) error {
					return a.Docker.RestartContainer(id)
				}, "Restarting")
			} else if page == styles.TitleCompose {
				a.PerformAction(func(id string) error {
					return a.Docker.RestartComposeProject(id)
				}, "Restarting Project")
			}
			return nil
		case 'x': // Stop
			// Only Containers
			page, _ := a.Pages.GetFrontPage()
			if page == styles.TitleContainers {
				a.PerformAction(func(id string) error {
					return a.Docker.StopContainer(id)
				}, "Stopping")
			} else if page == styles.TitleCompose {
				a.PerformAction(func(id string) error {
					return a.Docker.StopComposeProject(id)
				}, "Stopping Project")
			}
			return nil
		case 'p': // Prune
			a.PerformPrune()
			return nil
		}
		
		// Ctrl+D for Delete
		if event.Key() == tcell.KeyCtrlD {
			a.PerformDelete()
			return nil
		}

		return event
	})

	// Initial State
	a.Pages.SwitchToPage(styles.TitleContainers)
	a.updateHeader()
}

// AppController Implementation

func (a *App) GetPages() *tview.Pages {
	return a.Pages
}

func (a *App) GetTviewApp() *tview.Application {
	return a.TviewApp
}

func (a *App) GetDocker() *dao.DockerClient {
	return a.Docker
}

func (a *App) SetActiveScope(scope *common.Scope) {
	a.ActiveScope = scope
}

func (a *App) GetActiveScope() *common.Scope {
	return a.ActiveScope
}

func (a *App) SetFilter(filter string) {
	a.ActiveFilter = filter
}

func (a *App) SetFlashText(text string) {
	a.Flash.SetText(text)
}

func (a *App) RestoreFocus() {
	page, _ := a.Pages.GetFrontPage()
	if view, ok := a.Views[page]; ok {
		a.TviewApp.SetFocus(view.Table)
	} else {
		a.TviewApp.SetFocus(a.Pages)
	}
}

func (a *App) GetActiveFilter() string {
	return a.ActiveFilter
}

func (a *App) SetActiveFilter(filter string) {
	a.ActiveFilter = filter
}

func (a *App) SetCmdLineVisible(visible bool) {
	size := 0
	if visible {
		size = 3
	}
	// Important: Set proportion to 0 when hidden, otherwise it takes relative space
	a.Layout.ResizeItem(a.CmdLine.View, size, 0)
}
