package ui

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"runtime/debug"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/config"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/command"
	"github.com/jr-k/d4s/internal/ui/components/footer"
	"github.com/jr-k/d4s/internal/ui/components/header"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/jr-k/d4s/internal/ui/views/aliases"
	"github.com/jr-k/d4s/internal/ui/views/compose"
	"github.com/jr-k/d4s/internal/ui/views/containers"
	"github.com/jr-k/d4s/internal/ui/views/images"
	"github.com/jr-k/d4s/internal/ui/views/networks"
	"github.com/jr-k/d4s/internal/ui/views/nodes"
	"github.com/jr-k/d4s/internal/ui/views/secrets"
	"github.com/jr-k/d4s/internal/ui/views/services"
	"github.com/jr-k/d4s/internal/ui/views/volumes"
	"github.com/jr-k/d4s/internal/updater"
	"github.com/rivo/tview"
)

type App struct {
	TviewApp *tview.Application
	Screen   tcell.Screen
	Docker   *dao.DockerClient
	Cfg      *config.Config

	// Components
	Layout  *tview.Flex
	Header  *header.HeaderComponent
	Pages   *tview.Pages
	CmdLine *command.CommandComponent
	Flash   *footer.FlashComponent
	Help tview.Primitive

	// Views
	Views map[string]*view.ResourceView

	// State
	ActiveFilter    string
	ActiveScope     *common.Scope
	scopeMx         sync.RWMutex
	ActiveInspector common.Inspector
	PreviousView    string
	CurrentView     string // Track current view name before inspector
	LatestVersion   string 

	// Concurrency
	pauseMx    sync.RWMutex
	paused     bool
	stopTicker chan struct{}
	
	flashMx    sync.Mutex
	flashExpiry time.Time
	
	appendTimer *time.Timer
	appendMx    sync.Mutex
}

// Ensure App implements AppController interface
var _ common.AppController = (*App)(nil)

func NewApp(contextName string, cfg *config.Config) (*App, error) {
	// Configure global tview borders (Normal)
	tview.Borders.TopLeft = '┌'
	tview.Borders.TopRight = '┐'
	tview.Borders.BottomLeft = '└'
	tview.Borders.BottomRight = '┘'
	tview.Borders.Horizontal = '─'
	tview.Borders.Vertical = '│'
	tview.Borders.LeftT = '├'
	tview.Borders.RightT = '┤'
	tview.Borders.TopT = '┬'
	tview.Borders.BottomT = '┴'
	tview.Borders.Cross = '┼'

	// Focused borders (same style)
	tview.Borders.TopLeftFocus = '┌'
	tview.Borders.TopRightFocus = '┐'
	tview.Borders.BottomLeftFocus = '└'
	tview.Borders.BottomRightFocus = '┘'
	tview.Borders.HorizontalFocus = '─'
	tview.Borders.VerticalFocus = '│'

	// Apply skin if configured
	if skin := config.LoadSkin(cfg.D4S.UI.Skin); skin != nil {
		styles.ApplySkin(skin)
	}

	// Apply color inversion if configured
	if cfg.D4S.UI.Invert {
		styles.InvertColors()
	}

	docker, err := dao.NewDockerClient(contextName, cfg.D4S.GetAPIServerTimeout())
	if err != nil {
		return nil, fmt.Errorf("failed to init docker client: %w", err)
	}

	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, fmt.Errorf("failed to create screen: %w", err)
	}
	
	tviewApp := tview.NewApplication()
	tviewApp.SetScreen(screen)

	// Enable mouse support if configured
	if cfg.D4S.UI.EnableMouse {
		tviewApp.EnableMouse(true)
	}

	app := &App{
		TviewApp: tviewApp,
		Screen:   screen,
		Docker:   docker,
		Cfg:      cfg,
		Views:    make(map[string]*view.ResourceView),
		Pages:    tview.NewPages(),
	}

	app.initUI()
	return app, nil
}

func (a *App) Run() error {
	defer func() {
		if r := recover(); r != nil {
			a.TviewApp.Stop()
			fmt.Printf("Application crashed: %v\nStack trace:\n%s\n", r, string(debug.Stack()))
		}
	}()

	// Start auto-refresh
	a.StartAutoRefresh()
	
	// Check for updates (unless skipped by config)
	if !a.Cfg.D4S.SkipLatestRevCheck {
		go a.checkLatestVersion()
	}

	// Pre-pull shell pod image so volume shell and secret decode are fast on first use
	shellImage := a.Cfg.D4S.ShellPod.Image
	go exec.Command("docker", "pull", shellImage).Run()

	return a.TviewApp.SetRoot(a.Layout, true).Run()
}

func (a *App) checkLatestVersion() {
	latest, err := updater.CheckLatestVersion()
	if err == nil {
		a.LatestVersion = latest
	}
}

func (a *App) StartAutoRefresh() {
	if a.stopTicker != nil {
		return
	}
	a.stopTicker = make(chan struct{})

	go func() {
		// Initial update
		// We use a small delay on first run to let UI settle if needed, but only for the first ever run
		// subsequent restarts of auto-refresh might want immediate effect or wait for next tick.
		// Let's rely on tick.
		
		ticker := time.NewTicker(a.Cfg.D4S.GetRefreshInterval())
		defer ticker.Stop()

		// Immediate update on start
		// a.RefreshCurrentView() calls a.UpdateShortcuts() which calls tview methods. 
		// Since we are in a goroutine here, we MUST queue.
		a.SafeQueueUpdateDraw(func() {
			// Actually RefreshCurrentView spawns BG task, so we shouldn't wrap the whole thing?
			// RefreshCurrentView has:
			// 1. GetFrontPage (Read UI) -> Needs Queue or Lock
			// 2. UpdateShortcuts (Write UI) -> Needs Queue
			// 3. RunInBackground -> Spawns BG
			// So calling RefreshCurrentView from BG is inherently unsafe if not wrapped.
			// But RefreshCurrentView spawns BG, so wrapping it might block UI while it spawns? No spawning is fast.
			// However, RefreshCurrentView reads "v.FetchFunc".
			a.RefreshCurrentView()
			a.updateHeader()
		})

		for {
			select {
			case <-ticker.C:
				a.SafeQueueUpdateDraw(func() {
					a.RefreshCurrentView()
					a.updateHeader()
				})
			case <-a.stopTicker:
				return
			}
		}
	}()
}

func (a *App) StopAutoRefresh() {
	if a.stopTicker != nil {
		close(a.stopTicker)
		a.stopTicker = nil
	}
}

func (a *App) initUI() {
	// 1. Header
	a.Header = header.NewHeaderComponent(a.Cfg.D4S.UI.Logoless)

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
	vImages.InspectFunc = images.Inspect
	vImages.RemoveFunc = images.Remove
	vImages.PruneFunc = images.Prune
	vImages.Headers = images.Headers
	
	// Default Sort: Containers (Index 3) DESC
	vImages.SortCol = 3 
	vImages.SortAsc = false

	vImages.InputHandler = func(event *tcell.EventKey) *tcell.EventKey {
		return images.InputHandler(vImages, event)
	}
	a.Views[styles.TitleImages] = vImages

	// Volumes
	vVolumes := view.NewResourceView(a, styles.TitleVolumes)
	vVolumes.ShortcutsFunc = volumes.GetShortcuts
	vVolumes.FetchFunc = volumes.Fetch
	vVolumes.InspectFunc = volumes.Inspect
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
	vNetworks.InspectFunc = networks.Inspect
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
	vServices.InspectFunc = services.Inspect
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
	vNodes.InspectFunc = nodes.Inspect
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

	// Aliases
	vAliases := view.NewResourceView(a, styles.TitleAliases)
	vAliases.Headers = aliases.Headers
	vAliases.FetchFunc = aliases.Fetch
	vAliases.ShortcutsFunc = aliases.GetShortcuts
	vAliases.InputHandler = func(event *tcell.EventKey) *tcell.EventKey {
		return aliases.InputHandler(vAliases, event)
	}
	a.Views[styles.TitleAliases] = vAliases

	// Secrets
	vSecrets := view.NewResourceView(a, styles.TitleSecrets)
	vSecrets.ShortcutsFunc = secrets.GetShortcuts
	vSecrets.FetchFunc = secrets.Fetch
	vSecrets.InspectFunc = secrets.Inspect
	vSecrets.RemoveFunc = secrets.Remove
	vSecrets.Headers = secrets.Headers
	vSecrets.InputHandler = func(event *tcell.EventKey) *tcell.EventKey {
		return secrets.InputHandler(vSecrets, event)
	}
	a.Views[styles.TitleSecrets] = vSecrets

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

	if !a.Cfg.D4S.UI.Headless {
		a.Layout.AddItem(a.Header.View, 7, 1, false)
	}
	a.Layout.AddItem(a.CmdLine.View, 0, 0, false). // Hidden by default (size 0, proportion 0)
		AddItem(a.Pages, 0, 1, true)

	if !a.Cfg.D4S.UI.Crumbsless {
		a.Layout.AddItem(a.Flash.View, 2, 1, false)
	}

	// Global Shortcuts
	a.TviewApp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		
		// Priority 0: Global Exit on Ctrl+C (unless noExitOnCtrlC is set)
		if event.Key() == tcell.KeyCtrlC {
			if !a.Cfg.D4S.NoExitOnCtrlC {
				a.TviewApp.Stop()
				return nil
			}
			// When noExitOnCtrlC is true, Ctrl+C is ignored — use :quit instead
			return nil
		}

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
				a.SetActiveFilter("")
				a.CmdLine.Reset()
				a.RefreshCurrentView() // Still trigger full refresh to be safe/consistent
				a.Flash.SetText("")
				return nil
			}

			// Priority 2: Exit scope if active (return to origin view)
			scope := a.GetActiveScope()
			if scope != nil {
				origin := scope.OriginView
				parent := scope.Parent
				
				a.SafeSetScope(parent) // Pop breadcrumb
				
				// Navigate back: either to parent's active view (if we can infer it?)
				// Or use OriginView. OriginView is the view we were in when we drilled down.
				// This matches "Back" behavior perfectly.
				a.SwitchToWithSelection(origin, false)
				return nil
			}

			return event
		}

		// logMsg := fmt.Sprintf("Key: %v (rune: %q) | Modifiers: %v\n", event.Key(), event.Rune(), event.Modifiers())
		// f, err := os.OpenFile("d4s.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		// if err == nil {
		// 	_, _ = f.WriteString(logMsg)
		// 	_ = f.Close()
		// }

		if event.Key() == tcell.KeyCtrlE && event.Modifiers() == tcell.ModCtrl {
			return nil
		}

		if event.Key() == tcell.KeyHome || (event.Key() == tcell.KeyCtrlA && event.Modifiers() == tcell.ModCtrl) {
			current, _ := a.Pages.GetFrontPage()
			if current == styles.TitleAliases {
				// If already on aliases, go back to previous view
				if a.PreviousView != "" {
					a.SwitchToWithSelection(a.PreviousView, false)
				} else {
					// Fallback to containers if no previous view
					a.SwitchToWithSelection(styles.TitleContainers, false)
				}
			} else {
				// Go to aliases
				// We use SwitchToWithSelection to populate PreviousView and handle focus
				a.SwitchToWithSelection(styles.TitleAliases, false) 
			}
			return nil
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
			a.UpdateShortcuts()
			return nil
		case 'c': // Global Copy
			a.PerformCopy()
			return nil
		}

		return event
	})

	// Initial State: use defaultView from config, fallback to Containers
	initialView := a.resolveDefaultView()
	a.Pages.SwitchToPage(initialView)
	a.CurrentView = initialView
	a.updateHeader()
}

// AppController Implementation

func (a *App) GetPages() *tview.Pages {
	return a.Pages
}

func (a *App) GetTviewApp() *tview.Application {
	return a.TviewApp
}

func (a *App) GetScreen() tcell.Screen {
	return a.Screen
}

func (a *App) GetDocker() *dao.DockerClient {
	return a.Docker
}

func (a *App) GetConfig() *config.Config {
	return a.Cfg
}

func (a *App) IsReadOnly() bool {
	return a.Cfg.D4S.ReadOnly
}

// resolveDefaultView maps the config defaultView string to a valid view title.
func (a *App) resolveDefaultView() string {
	v := strings.ToLower(strings.TrimSpace(a.Cfg.D4S.DefaultView))
	switch v {
	case "containers", "container":
		return styles.TitleContainers
	case "images", "image":
		return styles.TitleImages
	case "volumes", "volume":
		return styles.TitleVolumes
	case "networks", "network":
		return styles.TitleNetworks
	case "services", "service":
		return styles.TitleServices
	case "nodes", "node":
		return styles.TitleNodes
	case "compose", "project":
		return styles.TitleCompose
	case "aliases", "alias":
		return styles.TitleAliases
	case "secrets", "secret":
		return styles.TitleSecrets
	default:
		return styles.TitleContainers
	}
}

func (a *App) SetActiveScope(scope *common.Scope) {
	a.scopeMx.Lock()
	defer a.scopeMx.Unlock()

	// If different from current, stack it
	if a.ActiveScope != nil {
		// Only stack if it's a new drill-down (different value or type)
		// Prevent stacking identical scopes if called repeatedly? 
		// Actually typical usage is creating a NEW struct instance, so we check content.
		if a.ActiveScope.Value != scope.Value || a.ActiveScope.Type != scope.Type {
			scope.Parent = a.ActiveScope
		}
	}
	a.ActiveScope = scope
}

func (a *App) GetActiveScope() *common.Scope {
	a.scopeMx.RLock()
	defer a.scopeMx.RUnlock()
	return a.ActiveScope
}

func (a *App) SafeSetScope(scope *common.Scope) {
	a.scopeMx.Lock()
	defer a.scopeMx.Unlock()
	a.ActiveScope = scope
}

func (a *App) SetFilter(filter string) {
	a.ActiveFilter = filter
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
	// If inspector is active, route search to it directly
	// Do NOT update global ActiveFilter (which belongs to Table Views)
	if a.ActiveInspector != nil {
		a.ActiveInspector.ApplyFilter(filter)
		return
	}
	a.ActiveFilter = filter

	// Immediate Feedback: Refilter current view using cached data
	// We use async update to avoid blocking/crashing if Tview is busy
	go func() {
		a.TviewApp.QueueUpdateDraw(func() {
			page, _ := a.Pages.GetFrontPage()
			if v, ok := a.Views[page]; ok && v != nil {
				v.SetFilter(filter)
				v.Refilter()

				// Optimistically update the title count
				count := len(v.Data)
				title := a.formatViewTitle(page, fmt.Sprintf("%d", count), filter)
				a.updateViewTitle(v, title)
			}
		})
	}()
}

func (a *App) SetCmdLineVisible(visible bool) {
	size := 0
	if visible {
		size = 3
	}
	// Important: Set proportion to 0 when hidden, otherwise it takes relative space
	a.Layout.ResizeItem(a.CmdLine.View, size, 0)
}

func (a *App) ScheduleViewHighlight(viewName string, match func(dao.Resource) bool, bg, fg tcell.Color, duration time.Duration) {
	if match == nil || duration <= 0 {
		return
	}
	if v, ok := a.Views[viewName]; ok && v != nil {
		v.ScheduleHighlight(match, bg, fg, duration)
	}
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
	
	// Force immediate update of breadcrumb/UI
	a.RefreshCurrentView()
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
	
	// Force immediate update of breadcrumb/UI
	a.RefreshCurrentView()
}

func (a *App) ActionPause() {
	a.SetPaused(true)
}

func (a *App) ActionResume() {
	a.SetPaused(false)
	a.TviewApp.Draw() // Force redraw
}

func (a *App) SetPaused(paused bool) {
	a.pauseMx.Lock()
	defer a.pauseMx.Unlock()
	a.paused = paused
}

func (a *App) IsPaused() bool {
	a.pauseMx.RLock()
	defer a.pauseMx.RUnlock()
	return a.paused
}


func (a *App) SetFlashText(text string) {
	a.flashMx.Lock()
	a.flashExpiry = time.Now().Add(100 * time.Millisecond) // Short lock to prevent immediate overwrite
	a.flashMx.Unlock()
	a.Flash.SetText(text)
}

func (a *App) AppendFlash(text string) {
	// No need to lock main flash, as we use a separate slot in FlashComponent.
	a.appendMx.Lock()
	defer a.appendMx.Unlock()

	if a.appendTimer != nil {
		a.appendTimer.Stop()
	}

	a.Flash.Append(text)

	a.appendTimer = time.AfterFunc(2*time.Second, func() {
		a.appendMx.Lock()
		defer a.appendMx.Unlock()
		a.Flash.ClearAppend()
		// SafeQueueUpdateDraw handles thread safety for the draw
		a.SafeQueueUpdateDraw(func() {})
	})
}

func  (a *App) AppendFlashError(text string) {
	a.AppendFlash(fmt.Sprintf("[black:red] <error: %s> [-:-]", text))
}

func (a *App) AppendFlashPending(text string) {
	a.AppendFlash(fmt.Sprintf("[black:%s] <pending: %s> [-:-]", styles.ColorIdle, text))
}

func (a *App) AppendFlashSuccess(text string) {
	a.AppendFlash(fmt.Sprintf("[black:#50fa7b] <success: %s> [-:-]", text))
}

func (a *App) SetFlashMessage(text string, duration time.Duration) {
	a.flashMx.Lock()
	a.flashExpiry = time.Now().Add(duration)
	a.flashMx.Unlock()
	
	a.Flash.SetText(text)
}

func (a *App) SetFlashError(text string) {
	a.AppendFlash(fmt.Sprintf("[black:red] <error: %s> [-:-]", text))
}

func (a *App) SetFlashPending(text string) {
	a.AppendFlash(fmt.Sprintf("[black:%s] <pending: %s> [-:-]", styles.ColorIdle, text))
}

func (a *App) SetFlashSuccess(text string) {
	a.AppendFlash(fmt.Sprintf("[black:#50fa7b] <success: %s> [-:-]", text))
}

func (a *App) IsFlashLocked() bool {
	a.flashMx.Lock()
	defer a.flashMx.Unlock()
	return time.Now().Before(a.flashExpiry)
}

func (a *App) SafeQueueUpdateDraw(f func()) {
	a.pauseMx.RLock()
	isPaused := a.paused
	a.pauseMx.RUnlock()

	if isPaused {
		return
	}

	// ALWAYS run QueueUpdateDraw in a goroutine to avoid deadlocks if called from within a callback 
	// that holds internal tview locks (though QueueUpdateDraw is supposed to be safe, sometimes it blocks).
	// Actually, QueueUpdateDraw puts it in a channel.
	go a.TviewApp.QueueUpdateDraw(func() {
		a.pauseMx.RLock()
		isPausedNow := a.paused
		a.pauseMx.RUnlock()

		if isPausedNow {
			return
		}
		f()
	})
}

func (a *App) RunInBackground(task func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				a.TviewApp.QueueUpdateDraw(func() {
					a.Flash.SetText(fmt.Sprintf("[%s]Background task panic: %v", styles.TagError, r))
					// Also print to stdout for debugging if app is still running or logs are captured
					fmt.Printf("Background task panic: %v\nStack trace:\n%s\n", r, string(debug.Stack()))
				})
			}
		}()
		task()
	}()
}
