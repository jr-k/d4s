package view

import (
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jessym/d4s/internal/dao"
	"github.com/jessym/d4s/internal/ui/common"
	"github.com/jessym/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

// ResourceView is the generic table view for any resource
type ResourceView struct {
	Table    *tview.Table
	App      common.AppController
	Title    string
	Data     []dao.Resource
	Filter   string // User Filter (via /)
	SortCol  int
	SortAsc  bool
	ColCount int // To avoid out of bound when switching views
	SelectedIDs map[string]bool
	ActionStates map[string]string // ID -> Action Name (e.g. "Stopping")
	Headers  []string // Stored for rendering
}

func NewResourceView(app common.AppController, title string) *ResourceView {
	tv := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 1).
		// No vertical borders for cleaner look
		SetSeparator(' ')
	
	tv.SetBorder(false)
	tv.SetBackgroundColor(styles.ColorBg)
	// Disable default selected style to handle overlay manually
	tv.SetSelectedStyle(tcell.StyleDefault)

	v := &ResourceView{
		Table:       tv,
		App:         app,
		Title:       title,
		SortAsc:     true, // Default ASC
		SortCol:     0,    // Default first column
		SelectedIDs: make(map[string]bool),
		ActionStates: make(map[string]string),
	}
	
	// Handle Selection Change for custom highlighting (Optimized)
	tv.SetSelectionChangedFunc(func(row, col int) {
		v.updateCursorStyle(row)
	})

	// Navigation shortcuts
	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Sorting Shortcuts
		if event.Modifiers()&tcell.ModShift != 0 {
			switch event.Key() {
			case tcell.KeyRight:
				v.SortCol = (v.SortCol + 1) % v.ColCount
				app.RefreshCurrentView()
				return nil
			case tcell.KeyLeft:
				v.SortCol--
				if v.SortCol < 0 {
					v.SortCol = v.ColCount - 1
				}
				app.RefreshCurrentView()
				return nil
			case tcell.KeyUp, tcell.KeyDown: // Toggle Sort Order
				v.SortAsc = !v.SortAsc
				app.RefreshCurrentView()
				return nil
			}
		}

		// Pass through commands to App
		switch event.Rune() {
		case ' ': // Multi-select
			row, _ := tv.GetSelection()
			if row > 0 && row <= len(v.Data) {
				item := v.Data[row-1]
				id := item.GetID()
				if v.SelectedIDs[id] {
					delete(v.SelectedIDs, id)
				} else {
					v.SelectedIDs[id] = true
				}
				// Optimized update
				v.updateRowStyle(row, item)
				v.updateCursorStyle(row)
			}
			return nil
		case '+': // Toggle Sort Order
			v.SortAsc = !v.SortAsc
			app.RefreshCurrentView()
			return nil
		case '/':
			app.ActivateCmd("/")
			return nil
		case ':':
			app.ActivateCmd(":")
			return nil
		case 'g': // Top
			tv.ScrollToBeginning()
			return nil
		case 'G': // Bottom
			tv.ScrollToEnd()
			return nil
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		
		// Context specific shortcuts
		if v.Title == styles.TitleCompose && event.Key() == tcell.KeyEnter {
			row, _ := tv.GetSelection()
			if row > 0 && row <= len(v.Data) {
				res := v.Data[row-1]
				projName := res.GetID()
				
				// Try to get config file path
				label := projName
				if cp, ok := res.(dao.ComposeProject); ok {
					if cp.ConfigFiles != "" {
						label = cp.ConfigFiles
					}
				}

				// Set Scope
				v.App.SetActiveScope(&common.Scope{
					Type:       "compose",
					Value:      projName,
					Label:      label,
					OriginView: styles.TitleCompose,
				})
				
				// Switch to Containers
				v.App.SwitchTo(styles.TitleContainers)
				return nil
			}
		}

		if v.Title == styles.TitleContainers {
			switch event.Rune() {
			case 'e':
				v.App.PerformEnv()
				return nil
			case 't':
				v.App.PerformStats()
				return nil
			case 'v':
				v.App.PerformContainerVolumes()
				return nil
			case 'n':
				v.App.PerformContainerNetworks()
				return nil
			}
		}

		// Map Ctrl-D/U to PageDown/PageUp
		switch event.Key() {
		case tcell.KeyCtrlD:
			return tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone)
		case tcell.KeyCtrlU:
			return tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone)
		case tcell.KeyEsc:
			if len(v.SelectedIDs) > 0 {
				v.SelectedIDs = make(map[string]bool)
				app.RefreshCurrentView()
				return nil
			}
			return event // Let App handle Esc (e.g. Back/Quit)
		}

		return event
	})

	return v
}

func (v *ResourceView) Update(headers []string, data []dao.Resource) {
	v.Headers = headers
	v.ColCount = len(headers)
	if v.SortCol >= v.ColCount {
		v.SortCol = 0
	}

	// 1. Filter Data First
	var filtered []dao.Resource
	
	for _, item := range data {
		match := true
		
		cells := item.GetCells()

		// User Filter
		if v.Filter != "" {
			userMatch := false
			for _, cell := range cells {
				if strings.Contains(strings.ToLower(cell), strings.ToLower(v.Filter)) {
					userMatch = true
					break
				}
			}
			if !userMatch {
				match = false
			}
		}

		if match {
			filtered = append(filtered, item)
		}
	}

	// 2. Sort Data
	sort.SliceStable(filtered, func(i, j int) bool {
		rowI := filtered[i].GetCells()
		rowJ := filtered[j].GetCells()
		
		if v.SortCol >= len(rowI) || v.SortCol >= len(rowJ) {
			return i < j
		}

		valI := rowI[v.SortCol]
		valJ := rowJ[v.SortCol]

		// Try numeric/size sort
		less := common.CompareValues(valI, valJ)
		
		if v.SortAsc {
			return less
		}
		return !less
	})

	v.Data = filtered // Update view data with sorted/filtered list
	v.renderAll()
}

func (v *ResourceView) renderAll() {
	v.Table.Clear()

	// 3. Set Headers with Indicators
	for i, h := range v.Headers {
		title := h
		if i == v.SortCol {
			if v.SortAsc {
				title += " ↑"
			} else {
				title += " ↓"
			}
		}

		cell := tview.NewTableCell(" " + title + " ").
			SetTextColor(tcell.ColorAqua).
			SetBackgroundColor(styles.ColorBg).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold)
		
		// Highlight sorted column header
		if i == v.SortCol {
			cell.SetTextColor(tcell.ColorMediumPurple)
		}

		v.Table.SetCell(0, i, cell)
	}

	// 4. Set Data
	for i, item := range v.Data {
		cells := item.GetCells()
		rowIndex := i + 1
		
		for j, text := range cells {
			// Basic Cell creation - styles applied in refreshStyles
			cell := tview.NewTableCell(" " + text + " ")
			v.Table.SetCell(rowIndex, j, cell)
		}
	}
	
	// Scroll/Selection Logic
	rowCount := v.Table.GetRowCount()
	if rowCount > 1 {
		row, _ := v.Table.GetSelection()
		if row <= 0 || row >= rowCount {
			v.Table.Select(1, 0)
		}
	} else {
		v.Table.Select(0, 0)
	}
	
	v.refreshStyles()
}


func (v *ResourceView) SetActionState(id, action string) {
	v.ActionStates[id] = action
}

func (v *ResourceView) ClearActionState(id string) {
	delete(v.ActionStates, id)
}

func (v *ResourceView) SetFilter(filter string) {
	v.Filter = filter
}

// updateCursorStyle updates the global selection style based on the current row
func (v *ResourceView) updateCursorStyle(cursorRow int) {
	dataIdx := cursorRow - 1
	isCursorSelected := false
	isCursorAction := false
	
	if dataIdx >= 0 && dataIdx < len(v.Data) {
		id := v.Data[dataIdx].GetID()
		isCursorSelected = v.SelectedIDs[id]
		isCursorAction = v.ActionStates[id] != ""
	}

	if isCursorAction {
		// Cursor + Action (Orange Light BG, Orange Text)
		v.Table.SetSelectedStyle(tcell.StyleDefault.Background(tcell.NewRGBColor(80, 50, 30)).Foreground(styles.ColorLogo))
	} else if isCursorSelected {
		// Cursor + Selected (Pink Light BG, White Text)
		v.Table.SetSelectedStyle(tcell.StyleDefault.Background(styles.ColorMultiSelectBg).Foreground(tcell.ColorWhite))
	} else {
		// Normal Cursor (Hover) -> Bg White Transparent, Texte Blanc
		v.Table.SetSelectedStyle(tcell.StyleDefault.Background(styles.ColorSelectBg).Foreground(styles.ColorSelectFg))
	}
}

// updateRowStyle updates the style for a specific row
func (v *ResourceView) updateRowStyle(rowIndex int, item dao.Resource) {
	id := item.GetID()
	
	isSelected := v.SelectedIDs[id]
	actionState := v.ActionStates[id]
	isAction := actionState != ""
	isExiting := false
	isStarting := false
	
	// Check for Exiting/Starting status in Data (Backend status)
	if container, ok := item.(dao.Container); ok {
		lowerStatus := strings.ToLower(container.Status)
		if strings.Contains(lowerStatus, "exiting") {
			isExiting = true
		}
		if strings.Contains(lowerStatus, "starting") {
			isStarting = true
		}
	}

	// Determine Base Colors
	var bgColor tcell.Color
	var fgColor tcell.Color
	
	// Priority: Starting/Exiting > Action > Selected > Normal
	if isStarting {
		bgColor = styles.ColorActionStartingBg // Blue Background
		fgColor = tcell.ColorWhite      // White Text
	} else if isExiting {
		bgColor = styles.ColorExitingBg        // Red Background
		fgColor = tcell.ColorWhite      // White Text
	} else if isAction {
		if strings.Contains(actionState, "Stopping") {
			bgColor = styles.ColorActionStopping
			fgColor = tcell.ColorWhite
		} else if strings.Contains(actionState, "Starting") && !strings.Contains(actionState, "Restarting") {
			bgColor = styles.ColorActionStartingBg // Blue
			fgColor = tcell.ColorWhite
		} else {
			// Restarting or others -> Orange
			bgColor = tcell.NewRGBColor(80, 50, 30) 
			fgColor = styles.ColorLogo
		}
	} else if isSelected {
		bgColor = styles.ColorMultiSelectBg // Pink Light/Dark BG
		fgColor = styles.ColorAccent // Pink Strong Text
	} else {
		bgColor = styles.ColorBg
		fgColor = styles.ColorFg
	}
	
	// Apply to all cells in row
	cells := item.GetCells()
	for j, text := range cells {
		cell := v.Table.GetCell(rowIndex, j)
		if cell == nil { continue }
		
		displayText := text
		
		// Specific Column Logic (Status, Name, etc)
		headerName := ""
		if j < len(v.Headers) {
			headerName = strings.ToUpper(v.Headers[j])
		}

		colColor := fgColor // Default to determined FG
		
		// Override FG based on column type if NOT selected/action
		forceTheme := isSelected || isAction || isStarting || isExiting

		// 1. ID Column
		if headerName == "ID" {
			if !forceTheme { colColor = styles.ColorDim }
		}

		// 2. Status Column
		if headerName == "STATUS" {
			if isAction {
				if strings.Contains(actionState, "Stopping") {
					displayText = "🔴 " + actionState + "..."
				} else if strings.Contains(actionState, "Starting") && !strings.Contains(actionState, "Restarting") {
					displayText = "🔵 " + actionState + "..."
				} else {
					displayText = "🟠 " + actionState + "..."
				}
			} else {
				lowerStatus := strings.ToLower(text)
				if strings.Contains(lowerStatus, "exiting") {
					displayText = "🔴 " + text
				} else if strings.Contains(lowerStatus, "starting") {
					displayText = "🔵 " + text
				} else if strings.Contains(lowerStatus, "up") || strings.Contains(lowerStatus, "running") || strings.Contains(lowerStatus, "healthy") {
					if !forceTheme { colColor = styles.ColorStatusGreen }
					if !strings.Contains(text, "Up") {
						displayText = "🟢 " + text
					} else {
						displayText = strings.Replace(text, "Up", "🟢 Up", 1)
					}
				} else if strings.Contains(lowerStatus, "exited") || strings.Contains(lowerStatus, "stop") {
					if !forceTheme { colColor = styles.ColorStatusGray }
					displayText = "⚫ " + text
				} else if strings.Contains(lowerStatus, "created") {
					if !forceTheme { colColor = styles.ColorStatusYellow }
					displayText = "🟡 " + text
				} else if strings.Contains(lowerStatus, "dead") || strings.Contains(lowerStatus, "error") {
					if !forceTheme { colColor = styles.ColorStatusRed }
					displayText = "🔴 " + text
				} else if strings.Contains(lowerStatus, "pause") {
					if !forceTheme { colColor = styles.ColorStatusYellow }
					displayText = "⏸️ " + text
				}
			}
		}
		
		// 3. Size / Ports
		if headerName == "SIZE" || headerName == "PORTS" {
			if !forceTheme { colColor = styles.ColorTitle }
		}

		// 3b. Mountpoint / Compose
		if headerName == "MOUNTPOINT" || headerName == "COMPOSE" {
			if !forceTheme { colColor = styles.ColorDim }
		}
		
		// 4. Name
		if headerName == "NAME" {
			if !forceTheme { colColor = tcell.ColorWhite }
			cell.SetAttributes(tcell.AttrBold)
		} else {
			cell.SetAttributes(tcell.AttrNone)
		}

		cell.SetText(" " + displayText + " ")
		cell.SetBackgroundColor(bgColor)
		cell.SetTextColor(colColor)
	}
}

func (v *ResourceView) refreshStyles() {
	row, _ := v.Table.GetSelection()
	v.updateCursorStyle(row)
	
	for i, item := range v.Data {
		v.updateRowStyle(i+1, item)
	}
}
