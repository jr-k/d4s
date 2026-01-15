package ui

import (
	"fmt"
	"time"

	"runtime/debug"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/command"
	"github.com/jr-k/d4s/internal/ui/components/footer"
	"github.com/jr-k/d4s/internal/ui/components/header"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/jr-k/d4s/internal/ui/views/compose"
	"github.com/jr-k/d4s/internal/ui/views/containers"
	"github.com/jr-k/d4s/internal/ui/views/images"
	"github.com/jr-k/d4s/internal/ui/views/networks"
	"github.com/jr-k/d4s/internal/ui/views/nodes"
	"github.com/jr-k/d4s/internal/ui/views/services"
	"github.com/jr-k/d4s/internal/ui/views/volumes"
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
	ActiveFilter    string
	ActiveScope     *common.Scope
	ActiveInspector common.Inspector
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
	// Containers
	vContainers := view.NewResourceView(a, styles.TitleContainers)
	vContainers.ShortcutsFunc = containers.GetShortcuts
	vContainers.FetchFunc = containers.Fetch
	vContainers.RemoveFunc = containers.Remove
	vContainers.Headers = containers.Headers
	vContainers.InputHandler = func(event *tcell.EventKey) *tcell.EventKey {
		return containers.InputHandler(vContainers, event)
	}
	a.Views[styles.TitleContainers] = vContainers

	// Images
	vImages := view.NewResourceView(a, styles.TitleImages)
	vImages.ShortcutsFunc = images.GetShortcuts
	vImages.FetchFunc = images.Fetch
	vImages.RemoveFunc = images.Remove
	vImages.PruneFunc = images.Prune
	vImages.Headers = images.Headers
	vImages.InputHandler = func(event *tcell.EventKey) *tcell.EventKey {
		return images.InputHandler(vImages, event)
	}
	a.Views[styles.TitleImages] = vImages

	// Volumes
	vVolumes := view.NewResourceView(a, styles.TitleVolumes)
	vVolumes.ShortcutsFunc = volumes.GetShortcuts
	vVolumes.FetchFunc = volumes.Fetch
	vVolumes.RemoveFunc = volumes.Remove
	vVolumes.PruneFunc = volumes.Prune
	vVolumes.Headers = volumes.Headers
	vVolumes.InputHandler = func(event *tcell.EventKey) *tcell.EventKey {
		return volumes.InputHandler(vVolumes, event)
	}
	a.Views[styles.TitleVolumes] = vVolumes

	// Networks
	vNetworks := view.NewResourceView(a, styles.TitleNetworks)
	vNetworks.ShortcutsFunc = networks.GetShortcuts
	vNetworks.FetchFunc = networks.Fetch
	vNetworks.RemoveFunc = networks.Remove
	vNetworks.PruneFunc = networks.Prune
	vNetworks.Headers = networks.Headers
	vNetworks.InputHandler = func(event *tcell.EventKey) *tcell.EventKey {
		return networks.InputHandler(vNetworks, event)
	}
	a.Views[styles.TitleNetworks] = vNetworks

	// Services
	vServices := view.NewResourceView(a, styles.TitleServices)
	vServices.ShortcutsFunc = services.GetShortcuts
	vServices.FetchFunc = services.Fetch
	vServices.RemoveFunc = services.Remove
	vServices.Headers = services.Headers
	vServices.InputHandler = func(event *tcell.EventKey) *tcell.EventKey {
		return services.InputHandler(vServices, event)
	}
	a.Views[styles.TitleServices] = vServices

	// Nodes
	vNodes := view.NewResourceView(a, styles.TitleNodes)
	vNodes.ShortcutsFunc = nodes.GetShortcuts
	vNodes.FetchFunc = nodes.Fetch
	vNodes.RemoveFunc = nodes.Remove
	vNodes.Headers = nodes.Headers
	vNodes.InputHandler = func(event *tcell.EventKey) *tcell.EventKey {
		return nodes.InputHandler(vNodes, event)
	}
	a.Views[styles.TitleNodes] = vNodes

	// Compose
	vCompose := view.NewResourceView(a, styles.TitleCompose)
	vCompose.ShortcutsFunc = compose.GetShortcuts
	vCompose.FetchFunc = compose.Fetch
	vCompose.InspectFunc = compose.Inspect
	vCompose.Headers = compose.Headers
	vCompose.InputHandler = func(event *tcell.EventKey) *tcell.EventKey {
		return compose.InputHandler(vCompose, event)
	}
	a.Views[styles.TitleCompose] = vCompose

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
		
		// Helper to route input to Active Inspector
		if a.ActiveInspector != nil {
			return a.ActiveInspector.InputHandler(event)
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

	// Delegate to Active View Input Handler
	if view, ok := a.Views[frontPage]; ok {
		if view.InputHandler != nil {
			// If handler returns nil, event was handled
			if ret := view.InputHandler(event); ret == nil {
				return nil
			}
		}
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
		case 'c': // Global Copy
			a.PerformCopy()
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

func (a *App) OpenInspector(inspector common.Inspector) {
	if a.ActiveInspector != nil {
		a.CloseInspector()
	}
	
	a.ActiveInspector = inspector
	inspector.OnMount(a)
	
	a.Pages.AddPage("inspect", inspector.GetPrimitive(), true, true)
	a.TviewApp.SetFocus(inspector.GetPrimitive())
	a.UpdateShortcuts()
}

func (a *App) CloseInspector() {
	if a.ActiveInspector != nil {
		a.ActiveInspector.OnUnmount()
		a.ActiveInspector = nil
	}
	
	if a.Pages.HasPage("inspect") {
		a.Pages.RemovePage("inspect")
	}
	
	a.RestoreFocus()
	a.UpdateShortcuts()
}
