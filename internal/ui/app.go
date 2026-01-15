package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"runtime/debug"

	"io"
	"os"
	"os/exec"
	"runtime"

	"github.com/gdamore/tcell/v2"
	"github.com/jessym/d4s/internal/dao"
	"github.com/rivo/tview"
)

type App struct {
	TviewApp *tview.Application
	Docker   *dao.DockerClient

	// Components
	Layout  *tview.Flex
	Header  *tview.Table
	Pages   *tview.Pages
	CmdLine *tview.InputField
	Flash   *tview.TextView
	Footer  *tview.TextView
	Help    tview.Primitive

	// Views
	Views map[string]*ResourceView
	
	// State
	ActiveFilter string
}

func NewApp() *App {
	docker, err := dao.NewDockerClient()
	if err != nil {
		panic(err)
	}

	app := &App{
		TviewApp: tview.NewApplication(),
		Docker:   docker,
		Views:    make(map[string]*ResourceView),
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
	a.Header = tview.NewTable().SetBorders(false)
	a.Header.SetBackgroundColor(ColorBg)
	
	// 2. Main Content
	a.Views[TitleContainers] = NewResourceView(a, TitleContainers)
	a.Views[TitleImages] = NewResourceView(a, TitleImages)
	a.Views[TitleVolumes] = NewResourceView(a, TitleVolumes)
	a.Views[TitleNetworks] = NewResourceView(a, TitleNetworks)
	a.Views[TitleServices] = NewResourceView(a, TitleServices)
	a.Views[TitleNodes] = NewResourceView(a, TitleNodes)

	for title, view := range a.Views {
		a.Pages.AddPage(title, view.Table, true, false)
	}

	// 3. Command Line & Flash & Footer
	a.CmdLine = tview.NewInputField().
		SetFieldBackgroundColor(ColorBg).
		SetLabelColor(tcell.ColorWhite). // Use white as base, dynamic in label string
		SetFieldTextColor(ColorFg).
		SetLabel("[#ffb86c::b]VIEW> [-:-:-]")
	
	// Handle Esc/Enter in Command Line
	a.CmdLine.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			a.CmdLine.SetText("")
			a.CmdLine.SetLabel("[#ffb86c::b]VIEW> [-:-:-]")
			a.ActiveFilter = ""
			a.RefreshCurrentView()
			a.Flash.SetText("")
			
			// Restore focus
			page, _ := a.Pages.GetFrontPage()
			if view, ok := a.Views[page]; ok {
				a.TviewApp.SetFocus(view.Table)
			} else {
				a.TviewApp.SetFocus(a.Pages)
			}
			return nil
		}
		return event
	})

	a.CmdLine.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			cmd := a.CmdLine.GetText()
			// ... traitement commande ...
			if strings.HasPrefix(cmd, "/") {
				if len(cmd) > 1 {
					a.ActiveFilter = strings.TrimPrefix(cmd, "/")
					a.RefreshCurrentView()
					a.Flash.SetText(fmt.Sprintf("Filter: %s", a.ActiveFilter))
				}
			} else {
				a.ExecuteCmd(cmd)
			}
			
			a.CmdLine.SetText("")
			a.CmdLine.SetLabel("[#ffb86c::b]VIEW> [-:-:-]")
			// Restore focus
			page, _ := a.Pages.GetFrontPage()
			if view, ok := a.Views[page]; ok {
				a.TviewApp.SetFocus(view.Table)
			} else {
				a.TviewApp.SetFocus(a.Pages)
			}
		}
	})

	a.Flash = tview.NewTextView()
	a.Flash.SetTextColor(tcell.NewRGBColor(95, 135, 255)).SetBackgroundColor(ColorBg) // Royal Blueish
	
	a.Footer = tview.NewTextView()
	a.Footer.SetDynamicColors(true).SetBackgroundColor(ColorBg)

	// 4. Help View
	helpTable := tview.NewTable()
	helpTable.SetBorders(false)
	helpTable.SetBackgroundColor(tcell.ColorBlack)
	
	// Format: Col1 | Col2
	rows := [][]string{
		{"[#ffb86c::b]GLOBAL", ""},
		{"[#5f87ff]:[-]             Command", "[#5f87ff]?[-]             Help"},
		{"[#5f87ff]/[-]             Filter", "[#5f87ff]Esc[-]           Back/Clear"},
		{"", ""},
		{"[#ffb86c::b]VIEWS", ""},
		{"[#5f87ff]:c[-]            Containers", "[#5f87ff]:i[-]            Images"},
		{"[#5f87ff]:v[-]            Volumes", "[#5f87ff]:n[-]            Networks"},
		{"[#5f87ff]:s[-]            Services", "[#5f87ff]:no[-]           Nodes"},
		{"", ""},
		{"[#ffb86c::b]NAVIGATION", ""},
		{"[#5f87ff]Arrows[-], [#5f87ff]j/k[-]   Navigate", "[#5f87ff]Enter[-], [#5f87ff]d[-]       Inspect"},
		{"[#5f87ff]< >[-]           Sort Column", "[#5f87ff]+[-]             Toggle Order"},
	}

	for i, row := range rows {
		for j, text := range row {
			if text == "" { continue }
			
			cell := tview.NewTableCell(text).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignLeft).
				SetExpansion(1)
			
			// Add padding
			if j == 0 {
				cell.SetText("  " + text + "      ") // Left padding + spacer
			} else {
				cell.SetText("  " + text) // Left padding for second col
			}
			
			helpTable.SetCell(i, j, cell)
		}
	}

	helpBox := tview.NewFrame(helpTable).
		SetBorders(1, 1, 1, 1, 0, 0).
		AddText(" Help ", true, tview.AlignCenter, ColorTitle).
		AddText(" [Scroll: Arrows/PgDn] ", false, tview.AlignCenter, tcell.ColorDarkGray)
	helpBox.SetBorder(true).SetBorderColor(ColorTitle).SetBackgroundColor(tcell.ColorBlack)

	// Center Modal
	helpFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(helpBox, 20, 1, true).
			AddItem(nil, 0, 1, false), 80, 1, true).
		AddItem(nil, 0, 1, false)

	helpFlex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc || event.Rune() == 'q' {
			a.Pages.RemovePage("help")
			// Restore focus
			page, _ := a.Pages.GetFrontPage()
			if view, ok := a.Views[page]; ok {
				a.TviewApp.SetFocus(view.Table)
			}
			return nil
		}
		return event
	})
	
	a.Help = helpFlex

	// 5. Inspect View (Modal TextView)
	inspectView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetScrollable(true)
	inspectView.SetBorder(true).SetTitle(" Inspect ").SetTitleColor(ColorTitle)
	inspectView.SetBackgroundColor(ColorBg)
	
	// Close inspect on Esc
	inspectView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			a.Pages.RemovePage("inspect")
			// Restore focus is handled by global capture mostly, but safe to set default here just in case
			if view, ok := a.Views[TitleContainers]; ok {
				a.TviewApp.SetFocus(view.Table)
			}
			return nil
		}
		return event
	})

	// 6. Layout
	a.Layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.Header, 4, 1, false).
		AddItem(a.Pages, 0, 1, true).
		AddItem(a.CmdLine, 1, 1, false).
		AddItem(a.Flash, 1, 1, false).
		AddItem(a.Footer, 1, 1, false)

	// Global Shortcuts
	a.TviewApp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if a.CmdLine.HasFocus() {
			return event
		}
		
		// Helper to close modals if open
		if a.Pages.HasPage("inspect") && event.Key() == tcell.KeyEsc {
			a.Pages.RemovePage("inspect")
			// Restore focus to active view
			page, _ := a.Pages.GetFrontPage()
			if view, ok := a.Views[page]; ok {
				a.TviewApp.SetFocus(view.Table)
			}
			return nil
		}

		// Don't intercept global keys if an input modal is open
		frontPage, _ := a.Pages.GetFrontPage()
		if frontPage == "input" || frontPage == "confirm" {
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
			if page == TitleContainers {
				a.PerformLogs()
			}
			return nil
		case 's':
			page, _ := a.Pages.GetFrontPage()
			if page == TitleContainers {
				a.PerformShell()
			} else if page == TitleServices {
				a.PerformScale()
			}
			return nil
		case 'c': // Contextual Create
			page, _ := a.Pages.GetFrontPage()
			if page == TitleVolumes {
				a.PerformCreateVolume()
				return nil
			}
			if page == TitleNetworks {
				a.PerformCreateNetwork()
				return nil
			}
			return nil
		case 'o': // Open Volume
			page, _ := a.Pages.GetFrontPage()
			if page == TitleVolumes {
				a.PerformOpenVolume()
				return nil
			}
			return nil
		case 'r': // Restart
			// Only Containers
			page, _ := a.Pages.GetFrontPage()
			if page == TitleContainers {
				a.PerformAction(func(id string) error {
					return a.Docker.RestartContainer(id)
				}, "Restarting")
			}
			return nil
		case 'x': // Stop
			// Only Containers
			page, _ := a.Pages.GetFrontPage()
			if page == TitleContainers {
				a.PerformAction(func(id string) error {
					return a.Docker.StopContainer(id)
				}, "Stopping")
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
	// Don't call SwitchTo here to avoid triggering RefreshCurrentView before Run
	a.Pages.SwitchToPage(TitleContainers)
	a.updateHeader()
}

func (a *App) PerformOpenVolume() {
	page, _ := a.Pages.GetFrontPage()
	view, ok := a.Views[page]
	if !ok { return }
	
	// Get Mountpoint from selected row
	// Usually Mountpoint is the last column or index 2 in our view (NAME, DRIVER, MOUNTPOINT)
	// We should get it from the Data object to be safe.
	
	row, _ := view.Table.GetSelection()
	if row < 1 || row >= len(view.Data)+1 { return }
	
	dataIdx := row - 1
	res := view.Data[dataIdx]
	
	// Cast to Volume to get Mountpoint
	// Or we rely on GetCells() returning it at index 2?
	// The resource is interface, we can check type or just use cells if consistent.
	// But dao.Volume struct has Mount field.
	
	// Safer: Type assertion
	vol, ok := res.(dao.Volume)
	if !ok {
		a.Flash.SetText("[red]Not a volume")
		return
	}
	
	path := vol.Mount
	if path == "" {
		a.Flash.SetText("[yellow]No mountpoint found")
		return
	}

	// Check if path exists on host
	if _, err := os.Stat(path); os.IsNotExist(err) {
		a.Flash.SetText(fmt.Sprintf("[red]Path not found on Host: %s (Is it inside Docker VM?)", path))
		return
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	default: // linux, etc
		cmd = exec.Command("xdg-open", path)
	}

	a.Flash.SetText(fmt.Sprintf("[yellow]Opening %s...", path))
	
	go func() {
		err := cmd.Run()
		a.TviewApp.QueueUpdateDraw(func() {
			if err != nil {
				a.Flash.SetText(fmt.Sprintf("[red]Open error: %v (Path: %s)", err, path))
			} else {
				a.Flash.SetText(fmt.Sprintf("[green]Opened %s", path))
			}
		})
	}()
}

func (a *App) PerformCreateNetwork() {
	a.ShowInput("Create Network", "Network Name: ", "", func(text string) {
		a.Flash.SetText(fmt.Sprintf("[yellow]Creating network %s...", text))
		go func() {
			err := a.Docker.CreateNetwork(text)
			a.TviewApp.QueueUpdateDraw(func() {
				if err != nil {
					a.Flash.SetText(fmt.Sprintf("[red]Error creating network: %v", err))
				} else {
					a.Flash.SetText(fmt.Sprintf("[green]Network %s created", text))
					a.RefreshCurrentView()
				}
			})
		}()
	})
}

func (a *App) PerformCreateVolume() {
	a.ShowInput("Create Volume", "Volume Name: ", "", func(text string) {
		a.Flash.SetText(fmt.Sprintf("[yellow]Creating volume %s...", text))
		go func() {
			err := a.Docker.CreateVolume(text)
			a.TviewApp.QueueUpdateDraw(func() {
				if err != nil {
					a.Flash.SetText(fmt.Sprintf("[red]Error creating volume: %v", err))
				} else {
					a.Flash.SetText(fmt.Sprintf("[green]Volume %s created", text))
					a.RefreshCurrentView()
				}
			})
		}()
	})
}

func (a *App) PerformScale() {
	page, _ := a.Pages.GetFrontPage()
	if page != TitleServices { return }
	
	view, ok := a.Views[page]
	if !ok { return }
	
	id, err := a.getSelectedID(view)
	if err != nil { return }
    
	// Get current value from view data
	currentReplicas := ""
	row, _ := view.Table.GetSelection()
	if row > 0 && row <= len(view.Data) {
		item := view.Data[row-1]
		cells := item.GetCells()
		// Service columns: ID, NAME, IMAGE, MODE, REPLICAS, PORTS
		// Replicas is at index 4
		if len(cells) > 4 {
			currentReplicas = strings.TrimSpace(cells[4])
            // If format is "1/3", we take "3" (desired)
            if parts := strings.Split(currentReplicas, "/"); len(parts) == 2 {
                currentReplicas = parts[1]
            }
		}
	}

	a.ShowInput("Scale Service", "Replicas:", currentReplicas, func(text string) {
		replicas, err := strconv.ParseUint(text, 10, 64)
		if err != nil {
			a.Flash.SetText("[red]Invalid number")
			return
		}
		
		a.Flash.SetText(fmt.Sprintf("[yellow]Scaling %s to %d...", id, replicas))
		
		go func() {
			err := a.Docker.ScaleService(id, replicas)
			a.TviewApp.QueueUpdateDraw(func() {
				if err != nil {
					a.Flash.SetText(fmt.Sprintf("[red]Scale Error: %v", err))
				} else {
					a.Flash.SetText(fmt.Sprintf("[green]Service scaled to %d", replicas))
					a.RefreshCurrentView()
				}
			})
		}()
	})
}

func (a *App) PerformLogs() {
	page, _ := a.Pages.GetFrontPage()
	view, ok := a.Views[page]
	if !ok { return }
	id, err := a.getSelectedID(view)
	if err != nil { return }

	resourceType := "container"
	if page == TitleServices {
		resourceType = "service"
	}

	logView := NewLogView(a, id, resourceType)
	a.Pages.AddPage("logs", logView, true, true)
	a.TviewApp.SetFocus(logView)

	// Update Footer for Logs
	shortcuts := formatSC("s", "AutoScroll") + 
				 formatSC("w", "Wrap") + 
				 formatSC("t", "Time") + 
				 formatSC("c", "Copy") + 
				 formatSC("S+c", "Clear") + 
				 formatSC("Esc", "Back")
	a.Footer.SetText(shortcuts)
}

func (a *App) PerformShell() {
	page, _ := a.Pages.GetFrontPage()
	view, ok := a.Views[page]
	if !ok { return }
	id, err := a.getSelectedID(view)
	if err != nil { return }

	a.TviewApp.Suspend(func() {
		// Clear screen
		fmt.Print("\033[H\033[2J")
		fmt.Printf("Entering shell for %s (type 'exit' to return)...\n", id)
		
		cmd := exec.Command("docker", "exec", "-it", id, "/bin/sh")
		// Fallback to /bin/bash if sh fails? Docker exec doesn't easily allow fallback logic without probing
		// We try sh as it is most common.
		
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		if err := cmd.Run(); err != nil {
			fmt.Printf("Error executing shell: %v\nPress Enter to continue...", err)
			fmt.Scanln()
		}
	})
}

func (a *App) PerformAction(action func(id string) error, actionName string) {
	page, _ := a.Pages.GetFrontPage()
	view, ok := a.Views[page]
	if !ok {
		return
	}
	
	ids, err := a.getTargetIDs(view)
	if err != nil {
		return
	}

	// Visual Feedback: Mark as Actioning
	for _, id := range ids {
		view.SetActionState(id, actionName)
	}
	// Force redraw to show orange state immediately
	a.RefreshCurrentView()

	a.Flash.SetText(fmt.Sprintf("[yellow]%s %d items...", actionName, len(ids)))
	
	// Async Action
	go func() {
		var errs []string
		for _, id := range ids {
			if err := action(id); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", id, err))
			}
		}
		
		a.TviewApp.QueueUpdateDraw(func() {
			// Clear action state
			for _, id := range ids {
				view.ClearActionState(id)
			}
			
			if len(errs) > 0 {
				a.ShowResultModal(actionName, len(ids)-len(errs), errs)
			} else {
				a.Flash.SetText(fmt.Sprintf("[green]%s %d items done", actionName, len(ids)))
				// Clear selection on success?
				view.SelectedIDs = make(map[string]bool)
				a.RefreshCurrentView() // Trigger refresh
			}
		})
	}()
}

func (a *App) ShowResultModal(action string, successCount int, errors []string) {
	text := fmt.Sprintf("\n[green]✔ %d items processed successfully.\n\n[red]✘ %d items failed:\n", successCount, len(errors))
	for _, err := range errors {
		text += fmt.Sprintf("\n• [white]%s", err)
	}
	
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetText(text).
		SetTextAlign(tview.AlignLeft).
		SetScrollable(true)
	tv.SetBackgroundColor(tcell.ColorBlack)
	
	tv.SetBorder(true).SetTitle(" Action Report ").SetTitleColor(ColorError).SetBackgroundColor(tcell.ColorBlack)
	
	// Modal Layout
	modalWidth := 60
	modalHeight := 15
	
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(tv, modalHeight, 1, true).
			AddItem(nil, 0, 1, false), modalWidth, 1, true).
		AddItem(nil, 0, 1, false)
		
	// Close Handler
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc || event.Key() == tcell.KeyEnter {
			a.Pages.RemovePage("result")
			a.RefreshCurrentView()
			// Restore focus
			page, _ := a.Pages.GetFrontPage()
			if view, ok := a.Views[page]; ok {
				a.TviewApp.SetFocus(view.Table)
			}
			return nil
		}
		return event
	})
	
	a.Pages.AddPage("result", flex, true, true)
	a.TviewApp.SetFocus(flex)
}

// Helper to get target IDs (Multi or Single)
func (a *App) getTargetIDs(view *ResourceView) ([]string, error) {
	if len(view.SelectedIDs) > 0 {
		var ids []string
		for id := range view.SelectedIDs {
			ids = append(ids, id)
		}
		return ids, nil
	}
	// Fallback to single selection
	id, err := a.getSelectedID(view)
	if err != nil {
		return nil, err
	}
	return []string{id}, nil
}

func (a *App) PerformDelete() {
	page, _ := a.Pages.GetFrontPage()
	var action func(id string) error
	
	switch page {
	case TitleContainers:
		action = a.Docker.RemoveContainer
	case TitleImages:
		action = a.Docker.RemoveImage
	case TitleVolumes:
		action = a.Docker.RemoveVolume
	case TitleNetworks:
		action = a.Docker.RemoveNetwork
	case TitleServices:
		action = a.Docker.RemoveService
	case TitleNodes:
		action = a.Docker.RemoveNode
	default:
		return
	}
	
	view, ok := a.Views[page]
	if !ok { return }
	
	ids, err := a.getTargetIDs(view)
	if err != nil { return }

	label := ids[0]
	if len(ids) == 1 {
		// Try to get Name for better context
		// We need to find the item in view.Data that matches this ID
		// Since we might have filtered/sorted, we iterate to be safe or use current selection if it matches
		// Optimization: if not multi-select, it IS the current row.
		row, _ := view.Table.GetSelection()
		if row > 0 && row <= len(view.Data) {
			item := view.Data[row-1]
			if item.GetID() == ids[0] {
				cells := item.GetCells()
				if len(cells) > 1 {
					// Use 2nd column as Name (Container Name, Image Tag, etc)
					label = fmt.Sprintf("%s ([#8be9fd]%s[yellow])", label, cells[1])
				}
			}
		}
	} else if len(ids) > 1 {
		label = fmt.Sprintf("%d items", len(ids))
	}

	a.ShowConfirmation("DELETE", label, func() {
		a.PerformAction(action, "Deleting")
	})
}

func (a *App) PerformPrune() {
	page, _ := a.Pages.GetFrontPage()
	var action func() error
	var name string

	switch page {
	case TitleImages:
		action = a.Docker.PruneImages
		name = "Images"
	case TitleVolumes:
		action = a.Docker.PruneVolumes
		name = "Volumes"
	case TitleNetworks:
		action = a.Docker.PruneNetworks
		name = "Networks"
	default:
		a.Flash.SetText(fmt.Sprintf("[yellow]Prune not available for %s", page))
		return
	}

	a.ShowConfirmation("PRUNE", name, func() {
		a.Flash.SetText(fmt.Sprintf("[yellow]Pruning %s...", name))
		go func() {
			err := action()
			a.TviewApp.QueueUpdateDraw(func() {
				if err != nil {
					a.Flash.SetText(fmt.Sprintf("[red]Prune Error: %v", err))
				} else {
					a.Flash.SetText(fmt.Sprintf("[green]Pruned %s", name))
					a.RefreshCurrentView()
				}
			})
		}()
	})
}

// Helper to get selected ID safely
func (a *App) getSelectedID(view *ResourceView) (string, error) {
	row, _ := view.Table.GetSelection()
	if row < 1 || row >= view.Table.GetRowCount() {
		return "", fmt.Errorf("no selection")
	}

	dataIndex := row - 1
	if dataIndex < 0 || dataIndex >= len(view.Data) {
		return "", fmt.Errorf("invalid index")
	}
	
	return view.Data[dataIndex].GetID(), nil
}

func (a *App) SwitchTo(viewName string) {
	if _, ok := a.Views[viewName]; ok {
		a.Pages.SwitchToPage(viewName)
		a.ActiveFilter = "" // Reset filter on view switch
		go a.RefreshCurrentView()
		a.updateHeader()
		a.TviewApp.SetFocus(a.Pages)
	} else {
		a.Flash.SetText(fmt.Sprintf("[red]Unknown view: %s", viewName))
	}
}

func (a *App) ActivateCmd(initial string) {
	label := "[#ffb86c::b]CMD> [-:-:-]" // Orange for Command
	if strings.HasPrefix(initial, "/") {
		label = "[#ffb86c::b]FILTER> [-:-:-]" // Orange for Filter
	}
	a.CmdLine.SetLabel(label)
	a.CmdLine.SetText(initial)
	a.TviewApp.SetFocus(a.CmdLine)
}

func (a *App) ExecuteCmd(cmd string) {
	cmd = strings.TrimPrefix(cmd, ":")
	
	switch cmd {
	case "q", "quit":
		a.TviewApp.Stop()
	case "c", "co", "con", "containers":
		a.SwitchTo(TitleContainers)
	case "i", "im", "img", "images":
		a.SwitchTo(TitleImages)
	case "v", "vo", "vol", "volumes":
		a.SwitchTo(TitleVolumes)
	case "n", "ne", "net", "networks":
		a.SwitchTo(TitleNetworks)
	case "s", "se", "svc", "services":
		a.SwitchTo(TitleServices)
	case "no", "node", "nodes":
		a.SwitchTo(TitleNodes)
	case "h", "help", "?":
		a.Pages.AddPage("help", a.Help, true, true)
	default:
		a.Flash.SetText(fmt.Sprintf("[red]Unknown command: %s", cmd))
	}
}

func (a *App) InspectCurrentSelection() {
	page, _ := a.Pages.GetFrontPage()
	view, ok := a.Views[page]
	if !ok {
		return
	}

	row, _ := view.Table.GetSelection()
	if row < 1 || row >= view.Table.GetRowCount() {
		return // Header or invalid
	}

	// Get ID from the first column (hidden or not, we assume it's ID)
	// But in view.go Update, we set ID as ID.
	// Actually we need the real ID which might be truncated in display.
	// We stored dao.Resource in View.Data.
	// The View Data index matches row-1 (header is 0).
	dataIndex := row - 1
	if dataIndex < 0 || dataIndex >= len(view.Data) {
		return
	}
	
	resource := view.Data[dataIndex]
	id := resource.GetID()
	
	resourceType := "container"
	switch page {
	case TitleImages:
		resourceType = "image"
	case TitleVolumes:
		resourceType = "volume"
	case TitleNetworks:
		resourceType = "network"
	case TitleServices:
		resourceType = "service"
	case TitleNodes:
		resourceType = "node"
	}

	content, err := a.Docker.Inspect(resourceType, id)
	if err != nil {
		a.Flash.SetText(fmt.Sprintf("[red]Inspect error: %v", err))
		return
	}

	// Show in Modal
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetText(fmt.Sprintf("[green]%s", content)).
		SetScrollable(true)
	
	tv.SetBorder(true).SetTitle(fmt.Sprintf(" Inspect %s ", id)).SetTitleColor(ColorTitle)
	tv.SetBackgroundColor(ColorBg)
	
	// Navigation for Inspect
	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			a.Pages.RemovePage("inspect")
			a.TviewApp.SetFocus(view.Table)
			return nil
		}
		if event.Rune() == 'c' {
			// Copy to clipboard (Cross-platform)
			var cmd *exec.Cmd
			switch runtime.GOOS {
			case "darwin":
				cmd = exec.Command("pbcopy")
			case "windows":
				cmd = exec.Command("clip")
			default: // linux
				// Try xclip, fallback to xsel? Just xclip for now
				cmd = exec.Command("xclip", "-selection", "clipboard")
			}

			if cmd == nil {
				a.Flash.SetText("[red]Clipboard not supported on this OS")
				return nil
			}

			stdin, err := cmd.StdinPipe()
			if err != nil {
				a.Flash.SetText("[red]Copy error: stdin pipe")
				return nil
			}
			go func() {
				defer stdin.Close()
				io.WriteString(stdin, content)
			}()
			if err := cmd.Run(); err != nil {
				a.Flash.SetText(fmt.Sprintf("[red]Copy error: %v (install xclip/pbcopy?)", err))
			} else {
				a.Flash.SetText("[green]Copied to clipboard!")
			}
			return nil
		}
		return event
	})

	a.Pages.AddPage("inspect", tv, true, true)
	a.TviewApp.SetFocus(tv)
}

func (a *App) RefreshCurrentView() {
	// Read state safely
	page, _ := a.Pages.GetFrontPage()
	if page == "help" || page == "inspect" || page == "logs" { // Don't refresh if modal is top
		return
	}
	
	view, ok := a.Views[page]
	if !ok || view == nil {
		return
	}
	
	filter := a.ActiveFilter

	// Execute fetch in a goroutine to avoid blocking UI
	go func() {
		var err error
		var data []dao.Resource
		var headers []string
		var shortcuts string

		switch page {
		case TitleContainers:
			headers = []string{"ID", "NAME", "IMAGE", "STATUS", "PORTS", "CPU", "MEM", "COMPOSE", "CREATED"}
			data, err = a.Docker.ListContainers()
			shortcuts = fmt.Sprintf("%s %s %s %s %s %s",
				formatSC("l", "Logs"),
				formatSC("s", "Shell"),
				formatSC("S", "Stats"),
				formatSC("d", "Inspect"),
				formatSC("r", "Restart"),
				formatSC("x", "Stop"))
		case TitleImages:
			headers = []string{"ID", "TAGS", "SIZE", "CREATED"}
			data, err = a.Docker.ListImages()
			shortcuts = formatSC("Ctrl-d", "Delete") + formatSC("p", "Prune")
		case TitleVolumes:
			headers = []string{"NAME", "DRIVER", "MOUNTPOINT"}
			data, err = a.Docker.ListVolumes()
			shortcuts = formatSC("Ctrl-d", "Delete") + formatSC("p", "Prune") + formatSC("c", "Create") + formatSC("d", "Inspect") + formatSC("o", "Open")
		case TitleNetworks:
			headers = []string{"ID", "NAME", "DRIVER", "SCOPE"}
			data, err = a.Docker.ListNetworks()
			shortcuts = formatSC("Ctrl-d", "Delete") + formatSC("p", "Prune") + formatSC("d", "Inspect") + formatSC("c", "Create")
		case TitleServices:
			headers = []string{"ID", "NAME", "IMAGE", "MODE", "REPLICAS", "PORTS"}
			data, err = a.Docker.ListServices()
			shortcuts = formatSC("Ctrl-d", "Delete") + formatSC("d", "Inspect") + formatSC("s", "Scale")
		case TitleNodes:
			headers = []string{"ID", "HOSTNAME", "STATUS", "AVAIL", "ROLE", "VERSION"}
			data, err = a.Docker.ListNodes()
			shortcuts = formatSC("Ctrl-d", "Delete") + formatSC("d", "Inspect")
		}

		// Append common shortcuts
		shortcuts += commonShortcuts()


		// Update UI on main thread
		a.TviewApp.QueueUpdateDraw(func() {
			// Check if we are still on the same page? 
			// Ideally yes, but refreshing the view should be fine.
			
			// Pass active filter to view (UI op)
			view.SetFilter(filter)

			if err != nil {
				a.Flash.SetText(fmt.Sprintf("[red]Error: %v", err))
			} else {
				// Update Table Title
				title := fmt.Sprintf(" %s [white][%d] ", page, len(data))
				if filter != "" {
					title += fmt.Sprintf(" [Filter: %s] ", filter)
				}
				view.Table.SetTitle(title)
				view.Table.SetTitleColor(ColorTitle)
				view.Table.SetBorder(true)
				view.Table.SetBorderColor(ColorTitle) // Visible border matching title color
				
				view.Update(headers, data)
				
				// Update Footer
				a.Footer.SetText(shortcuts)
				
				// Status update
				status := fmt.Sprintf("Viewing %s", page)
				if filter != "" {
					status += fmt.Sprintf(" [orange]Filter: %s", filter)
				}
				a.Flash.SetText(status)
			}
		})
	}()
}

// Helper for footer shortcuts
func formatSC(key, action string) string {
	return fmt.Sprintf("[#5f87ff::b]<%s>[#f8f8f2:-] %s ", key, action)
}

func commonShortcuts() string {
	return formatSC("S+Arrows", "Sort Col") + formatSC("/", "Filter")
}

func (a *App) updateHeader() {
	// Execute fetch in background
	go func() {
		stats, err := a.Docker.GetHostStats()
		if err != nil {
			return 
		}

		a.TviewApp.QueueUpdateDraw(func() {
			a.Header.Clear()
			a.Header.SetBackgroundColor(ColorBg) // Ensure no black block
			
			logo := []string{
				"[#ffb86c]    ____  __ __ ____",
				"[#ffb86c]   / __ \\/ // // __/",
				"[#ffb86c]  / /_/ / // /_\\ \\ ",
				"[#ffb86c] /_____/_//_/____/ ",
			}
			
			lines := []string{
				fmt.Sprintf("[#8be9fd]Context: [white]%s", stats.Context),
				fmt.Sprintf("[#8be9fd]Cluster: [white]%s (v%s)", stats.Name, stats.Version),
				fmt.Sprintf("[#8be9fd]CPU:     [white]%s", stats.CPU),
				fmt.Sprintf("[#8be9fd]Mem:     [white]%s", stats.Mem),
			}

			// Layout Header
			// Col 0: Stats
			for i, line := range lines {
				a.Header.SetCell(i, 0, tview.NewTableCell(line).SetBackgroundColor(ColorBg))
			}
			
			// Col 1: View Info (Center)
			page, _ := a.Pages.GetFrontPage()
			title := fmt.Sprintf("[#f1fa8c::b]%s", strings.ToUpper(page))
			if page == "" { title = "D4S" }
			now := time.Now().Format("15:04:05")
			
			a.Header.SetCell(0, 1, tview.NewTableCell(title).SetAlign(tview.AlignCenter).SetExpansion(1).SetBackgroundColor(ColorBg))
			a.Header.SetCell(1, 1, tview.NewTableCell(fmt.Sprintf("[dim]%s", now)).SetAlign(tview.AlignCenter).SetExpansion(1).SetBackgroundColor(ColorBg))
			a.Header.SetCell(2, 1, tview.NewTableCell("").SetExpansion(1).SetBackgroundColor(ColorBg))
			a.Header.SetCell(3, 1, tview.NewTableCell("").SetExpansion(1).SetBackgroundColor(ColorBg))

			// Col 2: Logo (Right)
			for i, line := range logo {
				a.Header.SetCell(i, 2, tview.NewTableCell(line).SetAlign(tview.AlignRight).SetBackgroundColor(ColorBg))
			}
		})
	}()
}
