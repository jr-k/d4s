package ui

import (
	"sort"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jessym/d4s/internal/dao"
	"github.com/rivo/tview"
)

// ResourceView is the generic table view for any resource
type ResourceView struct {
	Table    *tview.Table
	App      *App
	Title    string
	Data     []dao.Resource
	Filter   string
	SortCol  int
	SortAsc  bool
	ColCount int // To avoid out of bound when switching views
	SelectedIDs map[string]bool
	ActionStates map[string]string // ID -> Action Name (e.g. "Stopping")
	Headers  []string // Stored for rendering
}

func NewResourceView(app *App, title string) *ResourceView {
	tv := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 1).
		// No vertical borders for cleaner look
		SetSeparator(' ')
	
	tv.SetBorder(false)
	tv.SetBackgroundColor(ColorBg)
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
	
	// Handle Selection Change for custom highlighting
	tv.SetSelectionChangedFunc(func(row, col int) {
		v.refreshStyles()
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
				// Force redraw to update selection style
				v.refreshStyles()
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
	if v.Filter != "" {
		for _, item := range data {
			match := false
			for _, cell := range item.GetCells() {
				if strings.Contains(strings.ToLower(cell), strings.ToLower(v.Filter)) {
					match = true
					break
				}
			}
			if match {
				filtered = append(filtered, item)
			}
		}
	} else {
		filtered = make([]dao.Resource, len(data))
		copy(filtered, data)
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
		less := compareValues(valI, valJ)
		
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
				title += " ▲"
			} else {
				title += " ▼"
			}
		}

		cell := tview.NewTableCell(title).
			SetTextColor(ColorHeaderFg).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold)
		
		// Highlight sorted column header
		if i == v.SortCol {
			cell.SetTextColor(ColorTitle)
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

// Helper for smart comparison
func compareValues(a, b string) bool {
	// 1. Percentage (e.g. "20.5%")
	if strings.HasSuffix(a, "%") && strings.HasSuffix(b, "%") {
		fa, errA := strconv.ParseFloat(strings.TrimSuffix(a, "%"), 64)
		fb, errB := strconv.ParseFloat(strings.TrimSuffix(b, "%"), 64)
		if errA == nil && errB == nil {
			return fa < fb
		}
	}

	// 2. Size (e.g. "10MB", "1GB") - Simple approximation
	if isSize(a) && isSize(b) {
		return parseBytes(a) < parseBytes(b)
	}

	// 3. Default String Compare
	return strings.ToLower(a) < strings.ToLower(b)
}

func isSize(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	// Must start with digit
	if s[0] < '0' || s[0] > '9' {
		return false
	}
	s = strings.ToUpper(s)
	return strings.HasSuffix(s, "B")
}

func parseBytes(s string) float64 {
	s = strings.ToUpper(s)
	unit := 1.0
	if strings.HasSuffix(s, "KB") || strings.HasSuffix(s, "K") {
		unit = 1024
	} else if strings.HasSuffix(s, "MB") || strings.HasSuffix(s, "M") {
		unit = 1024 * 1024
	} else if strings.HasSuffix(s, "GB") || strings.HasSuffix(s, "G") {
		unit = 1024 * 1024 * 1024
	} else if strings.HasSuffix(s, "TB") || strings.HasSuffix(s, "T") {
		unit = 1024 * 1024 * 1024 * 1024
	}
	
	valStr := strings.TrimRight(s, "KMGTB") // Trim units
	valStr = strings.TrimSpace(valStr)
	
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return 0
	}
	return val * unit
}

func (v *ResourceView) refreshStyles() {
	cursorRow, _ := v.Table.GetSelection()
	
	// Update Global Selection Style based on current row state
	dataIdx := cursorRow - 1
	isCursorSelected := false
	isCursorAction := false
	
	if dataIdx >= 0 && dataIdx < len(v.Data) {
		id := v.Data[dataIdx].GetID()
		isCursorSelected = v.SelectedIDs[id]
		isCursorAction = v.ActionStates[id] != ""
	}

	if isCursorAction {
		// Cursor + Action = Lighter Orange
		v.Table.SetSelectedStyle(tcell.StyleDefault.Background(tcell.NewRGBColor(140, 80, 20)).Foreground(tcell.ColorWhite))
	} else if isCursorSelected {
		// Cursor + Selected = Lighter Pink
		v.Table.SetSelectedStyle(tcell.StyleDefault.Background(tcell.NewRGBColor(140, 60, 100)).Foreground(tcell.ColorWhite))
	} else {
		// Normal Cursor
		v.Table.SetSelectedStyle(tcell.StyleDefault.Background(ColorSelectBg).Foreground(ColorSelectFg))
	}

	for i, item := range v.Data {
		rowIndex := i + 1
		id := item.GetID()
		
		isSelected := v.SelectedIDs[id]
		actionState := v.ActionStates[id]
		isAction := actionState != ""
		
		// Determine Base Colors
		var bgColor tcell.Color
		var fgColor tcell.Color
		
		// Priority: Action > Selected > Normal
		if isAction {
			bgColor = tcell.NewRGBColor(100, 60, 20) // Orange Dark
			fgColor = ColorLogo // Orange
		} else if isSelected {
			bgColor = tcell.NewRGBColor(80, 40, 60) // Pink Dark
			fgColor = ColorAccent // Pink
		} else {
			bgColor = ColorBg
			fgColor = ColorFg
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
			// If isSelected/Action, we enforce the theme color (Pink/Orange)
			forceTheme := isSelected || isAction

			// 1. ID Column
			if headerName == "ID" {
				if !forceTheme { colColor = ColorDim }
			}

			// 2. Status Column
			if headerName == "STATUS" {
				if isAction {
					displayText = "🟠 " + actionState + "..."
				} else {
					lowerStatus := strings.ToLower(text)
					if strings.Contains(lowerStatus, "up") || strings.Contains(lowerStatus, "running") || strings.Contains(lowerStatus, "healthy") {
						if !forceTheme { colColor = ColorStatusGreen }
						if !strings.Contains(text, "Up") {
							displayText = "🟢 " + text
						} else {
							displayText = strings.Replace(text, "Up", "🟢 Up", 1)
						}
					} else if strings.Contains(lowerStatus, "exited") || strings.Contains(lowerStatus, "stop") {
						if !forceTheme { colColor = ColorStatusGray }
						displayText = "⚫ " + text
					} else if strings.Contains(lowerStatus, "created") {
						if !forceTheme { colColor = ColorStatusYellow }
						displayText = "🟡 " + text
					} else if strings.Contains(lowerStatus, "dead") || strings.Contains(lowerStatus, "error") {
						if !forceTheme { colColor = ColorStatusRed }
						displayText = "🔴 " + text
					} else if strings.Contains(lowerStatus, "pause") {
						if !forceTheme { colColor = ColorStatusYellow }
						displayText = "⏸️ " + text
					}
				}
			}
			
			// 3. Size / Ports
			if headerName == "SIZE" || headerName == "PORTS" {
				if !forceTheme { colColor = ColorTitle }
			}

			// 3b. Mountpoint / Compose
			if headerName == "MOUNTPOINT" || headerName == "COMPOSE" {
				if !forceTheme { colColor = ColorDim }
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
}
