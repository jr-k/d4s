package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

// ResourceView is the generic table view for any resource
type ResourceView struct {
	Table    *tview.Table
	App      common.AppController
	Title    string
	Data     []dao.Resource // Filtered/Sorted Data for display
	RawData  []dao.Resource // Original Data from last fetch
	Filter   string // User Filter (via /)
	SortCol  int
	SortAsc  bool
	ColCount int // To avoid out of bound when switching views
	SelectedIDs map[string]bool
	ActionStates map[string]string // ID -> Action Name (e.g. "Stopping")
	Headers  []string // Stored for rendering
	
	// Optional Overrides
	InputHandler func(event *tcell.EventKey) *tcell.EventKey
	ShortcutsFunc func() []string
	FetchFunc func(app common.AppController) ([]dao.Resource, error)
	InspectFunc func(app common.AppController, id string)
	RemoveFunc func(id string, force bool, app common.AppController) error
	PruneFunc func(app common.AppController) error
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

		if v.Title == styles.TitleNodes && event.Key() == tcell.KeyEnter {
			row, _ := tv.GetSelection()
			if row > 0 && row <= len(v.Data) {
				res := v.Data[row-1]
				nodeID := res.GetID()
				
				// Get Node Hostname for Label
				label := nodeID
				if cells := res.GetCells(); len(cells) > 1 {
					label = cells[1] // Assuming Name/Hostname is 2nd column
				}

				// Set Scope
				v.App.SetActiveScope(&common.Scope{
					Type:       "node",
					Value:      nodeID,
					Label:      label,
					OriginView: styles.TitleNodes,
				})
				
				// Switch to Services
				v.App.SwitchTo(styles.TitleServices)
				return nil
			}
		}

		// Delegate to view specific input handler
		if v.InputHandler != nil {
			result := v.InputHandler(event)
			if result == nil {
				return nil
			}
			event = result
		}

		// Map Ctrl-D/U to PageDown/PageUp
		switch event.Key() {
		case tcell.KeyCtrlD:
			return tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone)
		case tcell.KeyCtrlU:
			return tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone)
		case tcell.KeyEsc:
			return event // Let App handle Esc (e.g. Back/Quit)
		}

		return event
	})

	return v
}

// Update triggers a refresh with new data
func (v *ResourceView) Update(headers []string, data []dao.Resource) {
	v.Headers = headers
	v.ColCount = len(headers)
	v.RawData = data
	v.Refilter()
}

// Refilter re-applies filter and sort to cached RawData
func (v *ResourceView) Refilter() {
	if v.SortCol >= v.ColCount {
		v.SortCol = 0
	}

	// 1. Filter Data First
	var filtered []dao.Resource
	
	// Use cached RawData
	for _, item := range v.RawData {
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

		// Align Right for numeric columns
		headerName := strings.ToUpper(h)
		if headerName == "SIZE" || headerName == "REPLICAS" || headerName == "CPU" || headerName == "MEM" {
			cell.SetAlign(tview.AlignRight)
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
		// Cursor + Action
		//v.Table.SetSelectedStyle(tcell.StyleDefault.Background(styles.ColorBrown).Foreground(styles.ColorAccentAlt))
	} else if isCursorSelected {
		// Cursor + Selected
		v.Table.SetSelectedStyle(tcell.StyleDefault.Background(styles.ColorMultiSelectBg).Foreground(styles.ColorBg))
	} else {
		// Normal Cursor (Hover)
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

	// Service Replicas Status (Active/Desired)
	var serviceStatusColor tcell.Color
	hasServiceStatus := false
	
	if service, ok := item.(dao.Service); ok {
		if strings.Contains(service.Replicas, "/") {
			parts := strings.Split(service.Replicas, "/")
			if len(parts) == 2 {
				var running, desired int
				fmt.Sscanf(parts[0], "%d", &running)
				fmt.Sscanf(parts[1], "%d", &desired)
				
				if desired == 0 && running == 0 {
					// 0/0 -> (Stopped but intentional)
					serviceStatusColor = styles.ColorStatusOrange // Orange
					hasServiceStatus = true
				} else if running < desired {
					// X < Y -> (Not enough replicas)
					serviceStatusColor = styles.ColorStatusRed
					hasServiceStatus = true
				} else if running > desired {
					// X > Y -> (Too many replicas - scaling down or issue)
					serviceStatusColor = tcell.ColorMediumPurple
					hasServiceStatus = true
				}
			}
		}
	}

	// Determine Base Colors
	var bgColor tcell.Color
	var fgColor tcell.Color
	
	// Priority: Starting/Exiting > Action > Selected > Normal
	if isStarting {
		bgColor = styles.ColorStatusBlue
		fgColor = styles.ColorStatusBlueDarkBg
	} else if isExiting {
		bgColor = styles.ColorStatusRed
		fgColor = styles.ColorStatusRedDarkBg
	} else if isAction {
		actionStateLower := strings.ToLower(actionState)
		if strings.Contains(actionStateLower, "stopping") && !strings.Contains(actionStateLower, "restarting") {
			bgColor = styles.ColorStatusRedDarkBg
			fgColor = styles.ColorStatusRed
		} else if strings.Contains(actionStateLower, "starting") && !strings.Contains(actionStateLower, "restarting") {
			bgColor = styles.ColorStatusBlueDarkBg
			fgColor = styles.ColorStatusBlue
		} else {
			// Restarting or others -> Orange
			bgColor = styles.ColorStatusOrangeDarkBg
			fgColor = styles.ColorStatusOrange
		}
	} else if isSelected {
		bgColor = styles.ColorBg
		fgColor = styles.ColorMultiSelectFg
	} else if hasServiceStatus {
		// Apply Service Status Color to the whole row text if not selected/action
		bgColor = styles.ColorBg
		fgColor = serviceStatusColor
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
		forceTheme := isSelected || isAction || isStarting || isExiting || hasServiceStatus

		// 1. ID Column
		if headerName == "ID" {
			if !forceTheme { colColor = styles.ColorDim }
		}

		// 2. Status Column
		if headerName == "STATUS" {
			if isAction {
				actionState := strings.ToLower(actionState)
				if strings.Contains(actionState, "stopping") && !strings.Contains(actionState, "restarting") {
					if !forceTheme { colColor = styles.ColorStatusRed } // Red
					displayText = "🔴 " + strings.ToUpper(actionState[:1]) + actionState[1:] + "..."
				} else if strings.Contains(actionState, "starting") && !strings.Contains(actionState, "restarting") {
					if !forceTheme { colColor = styles.ColorStatusBlue } // Blue
					displayText = "🔵 " + strings.ToUpper(actionState[:1]) + actionState[1:] + "..."
				} else {
					if !forceTheme { colColor = styles.ColorStatusOrange } // Orange
					displayText = "🟠 " + strings.ToUpper(actionState[:1]) + actionState[1:] + "..."
				}
			} else {
				lowerStatus := strings.ToLower(text)
				// Node Status
				if strings.Contains(lowerStatus, "ready") || strings.Contains(lowerStatus, "active") {
					if !forceTheme { colColor = styles.ColorStatusGreen }
					// Deprecated: use of strings.Title; using ucfirst.
					displayText = "🟢 " + strings.ToUpper(text[:1]) + text[1:]
				} else if strings.Contains(lowerStatus, "down") || strings.Contains(lowerStatus, "drain") || strings.Contains(lowerStatus, "disconnected") {
					if !forceTheme { colColor = styles.ColorStatusRed }
					displayText = "🔴 " + strings.ToUpper(text[:1]) + text[1:]
				} else if strings.Contains(lowerStatus, "unknown") {
					if !forceTheme { colColor = styles.ColorStatusOrange } // Orange
					displayText = "🟠 " + strings.ToUpper(text[:1]) + text[1:]
				} else if strings.Contains(lowerStatus, "exiting") {
					if !forceTheme { colColor = styles.ColorStatusRed } // Red
					displayText = "🔴 " + strings.ToUpper(text[:1]) + text[1:]
				} else if strings.Contains(lowerStatus, "starting") {
					if !forceTheme { colColor = styles.ColorStatusBlue } // Blue
					displayText = "🔵 " + strings.ToUpper(text[:1]) + text[1:]
				} else if strings.Contains(lowerStatus, "up") || strings.Contains(lowerStatus, "running") || strings.Contains(lowerStatus, "healthy") {
					if !forceTheme { colColor = styles.ColorStatusGreen }
					if !strings.Contains(text, "Up") {
						displayText = "🟢 " + strings.ToUpper(text[:1]) + text[1:]
					} else {
						displayText = strings.Replace(text, "Up", "🟢 Up", 1)
					}
				} else if strings.Contains(lowerStatus, "exited") || strings.Contains(lowerStatus, "stop") {
					if !forceTheme { colColor = styles.ColorStatusGray }
					displayText = "⚫ " + strings.ToUpper(text[:1]) + text[1:]
				} else if strings.Contains(lowerStatus, "created") {
					if !forceTheme { colColor = styles.ColorStatusBlue } // Blue
					displayText = "🔵 " + strings.ToUpper(text[:1]) + text[1:]
				} else if strings.Contains(lowerStatus, "dead") || strings.Contains(lowerStatus, "error") {
					if !forceTheme { colColor = styles.ColorStatusRed }
					displayText = "🔴 " + strings.ToUpper(text[:1]) + text[1:]
				} else if strings.Contains(lowerStatus, "pause") {
					if !forceTheme { colColor = styles.ColorStatusYellow }
					displayText = "⏸️ " + strings.ToUpper(text[:1]) + text[1:]
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
		
		// Align Right for numeric columns
		if headerName == "SIZE" || headerName == "REPLICAS" || headerName == "CPU" || headerName == "MEM" {
			cell.SetAlign(tview.AlignRight)
		}
	}
}

func (v *ResourceView) refreshStyles() {
	row, _ := v.Table.GetSelection()
	v.updateCursorStyle(row)
	
	for i, item := range v.Data {
		v.updateRowStyle(i+1, item)
	}
}

func (v *ResourceView) GetSelectedID() (string, error) {
	row, _ := v.Table.GetSelection()
	if row < 1 || row >= v.Table.GetRowCount() {
		return "", fmt.Errorf("no selection")
	}

	dataIndex := row - 1
	if dataIndex < 0 || dataIndex >= len(v.Data) {
		return "", fmt.Errorf("invalid index")
	}
	
	return v.Data[dataIndex].GetID(), nil
}

func (v *ResourceView) GetSelectedIDs() ([]string, error) {
	if len(v.SelectedIDs) > 0 {
		var ids []string
		for id := range v.SelectedIDs {
			ids = append(ids, id)
		}
		return ids, nil
	}
	// Fallback to single selection
	id, err := v.GetSelectedID()
	if err != nil {
		return nil, err
	}
	return []string{id}, nil
}
