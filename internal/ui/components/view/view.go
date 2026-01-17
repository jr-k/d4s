package view

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

// ResourceView is the generic table view for any resource
type ResourceView struct {
	Table        *tview.Table
	App          common.AppController
	Title        string
	Data         []dao.Resource // Filtered/Sorted Data for display
	RawData      []dao.Resource // Original Data from last fetch
	Filter       string         // User Filter (via /)
	SortCol      int
	SortAsc      bool
	FocusCol     int // Focused column for navigation/copy (independent of SortCol)
	ColCount     int // To avoid out of bound when switching views
	SelectedIDs  map[string]bool
	ActionStates map[string]ActionState // ID -> Action State
	Headers      []string               // Stored for rendering
	ColumnWidths []int                  // Cache for column widths

	// Optional Overrides
	InputHandler             func(event *tcell.EventKey) *tcell.EventKey
	ShortcutsFunc            func() []string
	FetchFunc                func(app common.AppController) ([]dao.Resource, error)
	InspectFunc              func(app common.AppController, id string)
	RemoveFunc               func(id string, force bool, app common.AppController) error
	PruneFunc                func(app common.AppController) error
	highlightMu              sync.Mutex
	transientHighlights      map[string]highlightEntry
	pendingHighlightRequests []highlightRequest

	refreshDelayMu    sync.Mutex
	refreshDelayUntil time.Time
}

type ActionState struct {
	Label string
	Color tcell.Color
}

type highlightEntry struct {
	bg, fg tcell.Color
	expiry time.Time
}

type highlightRequest struct {
	match    func(dao.Resource) bool
	bg, fg   tcell.Color
	duration time.Duration
}

func NewResourceView(app common.AppController, title string) *ResourceView {
	tv := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		// No vertical borders for cleaner look
		SetSeparator(' ')

	// Initial Loading State
	tv.SetBorder(true)
	tv.SetTitle(fmt.Sprintf(" [#00ffff::b]%s[-::-] ~ [white]loading...[-] ", strings.ToLower(title)))
	tv.SetTitleColor(styles.ColorTitle)
	tv.SetBorderColor(styles.ColorTableBorder)
	tv.SetBackgroundColor(styles.ColorBg)

	// Add centered Loading message
	loadingCell := tview.NewTableCell("Freshly squeezing data 🍊").
		SetAlign(tview.AlignCenter).
		SetTextColor(styles.ColorAccent).
		SetExpansion(1).
		SetSelectable(false)

	tv.SetCell(2, 0, loadingCell)

	// Disable default selected style to handle overlay manually
	tv.SetSelectedStyle(tcell.StyleDefault)

	v := &ResourceView{
		Table:               tv,
		App:                 app,
		Title:               title,
		SortAsc:             true, // Default ASC
		SortCol:             -1,   // Default undefined, will be resolved in Update
		FocusCol:            0,    // Start focused on first column
		SelectedIDs:         make(map[string]bool),
		ActionStates:        make(map[string]ActionState),
		transientHighlights: make(map[string]highlightEntry),
	}

	// Handle Selection Change for custom highlighting (Optimized)
	tv.SetSelectionChangedFunc(func(row, col int) {
		// Update internal FocusCol when user clicks or moves via other means
		if col >= 0 {
			v.FocusCol = col
		}
		v.updateCursorStyle(row)
	})

	// Navigation shortcuts
	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Sorting & Focus Shortcuts: SHIFT + ARROWS
		if event.Modifiers()&tcell.ModShift != 0 {
			switch event.Key() {
			case tcell.KeyUp:
				// Sort Ascending by Focused Column
				v.SortCol = v.FocusCol
				v.SortAsc = true
				app.RefreshCurrentView()
				return nil
			case tcell.KeyDown:
				// Sort Descending by Focused Column
				v.SortCol = v.FocusCol
				v.SortAsc = false
				app.RefreshCurrentView()
				return nil
			case tcell.KeyRight:
				// Move FocusCol Right
				if v.ColCount > 0 {
					if v.FocusCol < v.ColCount-1 {
						v.FocusCol++
						v.renderAll()
						
						// Smart Scroll Right:
						// Calculate if the new FocusCol is likely off-screen to the right.
						_, _, width, _ := tv.GetInnerRect()
						_, cOffset := tv.GetOffset()

						// Estimate used width from cOffset to FocusCol
						usedWidth := 0
						// We iterate up to FocusCol to see where its right edge lands
						for i := cOffset; i <= v.FocusCol; i++ {
							// Use calculated widths
							colW := 15 // Fallback min width
							if i < len(v.ColumnWidths) {
								colW = v.ColumnWidths[i]
							}
							// Add padding (2) + separator (1) = 3
							usedWidth += colW + 3
						}

						// If the right edge of the focused column is outside the view width
						// We need to scroll right until it fits.
						if usedWidth >= width {
							// Find new offset that makes it fit
							// We remove columns from the left (incrementing offset) until the focused column fits
							currentUsed := usedWidth
							newOffset := cOffset
							
							for k := cOffset; k < v.FocusCol; k++ {
								w := 15
								if k < len(v.ColumnWidths) {
									w = v.ColumnWidths[k] + 3
								}
								currentUsed -= w
								newOffset++
								// If we fit within the width (with a small margin for right border)
								if currentUsed < width-2 {
									break
								}
							}
							
							if newOffset >= v.ColCount {
								newOffset = v.ColCount - 1
							}
							tv.SetOffset(0, newOffset)
						}
					}
				}
				return nil
			case tcell.KeyLeft:
				// Move FocusCol Left
				if v.ColCount > 0 {
					if v.FocusCol > 0 {
						v.FocusCol--
						v.renderAll()
						
						// Ensure visibility (Left Edge is easy)
						// If we move focus left, and it becomes < cOffset, we MUST scroll left to see it.
						_, cOffset := tv.GetOffset()
						if v.FocusCol < cOffset {
							tv.SetOffset(0, v.FocusCol)
						}
					}
				}
				return nil
			}
		}

		// Horizontal Scrolling (Viewport)
		if event.Key() == tcell.KeyRight {
			r, c := tv.GetOffset()
			// Allow scrolling up to the last column
			if v.ColCount > 0 && c < v.ColCount-1 {
				tv.SetOffset(r, c+1)
			}
			return nil
		}
		if event.Key() == tcell.KeyLeft {
			r, c := tv.GetOffset()
			if c > 0 {
				tv.SetOffset(r, c-1)
			}
			return nil
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
		case 'u': // Unselect All
			v.SelectedIDs = make(map[string]bool)
			// Refresh styles for all rows to remove highlights and markers
			v.renderAll()
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

	// Resolve default sort column if not set
	if v.SortCol == -1 && len(data) > 0 {
		defaultCol := data[0].GetDefaultSortColumn()
		v.SortCol = 0 // Fallback
		
		for i, h := range headers {
			if strings.EqualFold(h, defaultCol) {
				v.SortCol = i
				break
			}
		}
		
		// Optional: Smart sort direction based on column?
		// e.g. Created -> Desc
		if strings.EqualFold(defaultCol, "Created") || strings.EqualFold(defaultCol, "Age") {
			v.SortAsc = false
		}

		// Also set FocusCol to default column if not manually set/modified yet
		// We assume initial FocusCol is 0, so if we are at init, we check this.
		// However, user might have navigated. But since this block runs only when SortCol is -1 (init), it's safe.
		defaultFocus := data[0].GetDefaultColumn()
		for i, h := range headers {
			if strings.EqualFold(h, defaultFocus) {
				v.FocusCol = i
				break
			}
		}
	}

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
	v.recalculateColumnWidths()
	v.renderAll()
}

func (v *ResourceView) recalculateColumnWidths() {
	if len(v.Headers) == 0 {
		return
	}
	v.ColumnWidths = make([]int, len(v.Headers))
	// Init with Header widths
	for i, h := range v.Headers {
		v.ColumnWidths[i] = len(h)
	}

	// Sample Data to find max width
	limit := len(v.Data)
	if limit > 100 {
		limit = 100
	}

	for i := 0; i < limit; i++ {
		cells := v.Data[i].GetCells()
		for j, text := range cells {
			if j < len(v.ColumnWidths) {
				// Strip tags for accurate length
				l := len(stripFormattingTags(text))
				if l > v.ColumnWidths[j] {
					v.ColumnWidths[j] = l
				}
			}
		}
	}
}

// GetCurrentColumnSorted returns the name of the column currently used for sorting
func (v *ResourceView) GetCurrentColumnSorted() string {
	// Debug print or check headers
	if v.SortCol >= 0 && v.SortCol < len(v.Headers) {
		// Strip any styling or indicators from header name just in case, though headers usually clean strings in this list
		return v.Headers[v.SortCol]
	}
	return ""
}

// GetCurrentColumnFocused returns the name of the column currently focused by the cursor
func (v *ResourceView) GetCurrentColumnFocused() string {
	if v.FocusCol >= 0 && v.FocusCol < len(v.Headers) {
		return v.Headers[v.FocusCol]
	}
	return ""
}

func (v *ResourceView) renderAll() {
	v.Table.Clear()

	v.processHighlightRequests()

	// 3. Set Headers with Indicators
	for i, h := range v.Headers {
		title := h
		if i == v.SortCol {
			if v.SortAsc {
				title += "[orange::b]↑[-::-]"
			} else {
				title += "[orange::b]↓[-::-]"
			}
		}

		cell := tview.NewTableCell(" " + title + " ").
			SetBackgroundColor(styles.ColorBg).
			SetSelectable(false).
			SetExpansion(1)

		// Color logic: Focus = Blue, Others = White
		if i == v.FocusCol {
			cell.SetTextColor(styles.ColorHeaderFocus) // Blue
		} else {
			cell.SetTextColor(styles.ColorHeader) // White
		}

		// Align Right for numeric columns
		if isNumericColumn(h) {
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

			// Align right for numeric columns (data)
			if j < len(v.Headers) {
				if isNumericColumn(v.Headers[j]) {
					cell.SetAlign(tview.AlignRight)
				}
			}

			// Safe index check
			if j < v.Table.GetColumnCount() {
				v.Table.SetCell(rowIndex, j, cell)
			}
		}
	}

	// Scroll/Selection Logic
	rowCount := v.Table.GetRowCount()
	if rowCount > 1 {
		// Only reset selection to top if we have rows AND we are invalidly positioned
		// OR current selection is 0 (header) which shouldn't happen for resource view
		row, _ := v.Table.GetSelection()
		if row <= 0 || row >= rowCount {
			v.Table.Select(1, v.FocusCol) // Use FocusCol
		} else {
			// Ensure selection column matches FocusCol
			v.Table.Select(row, v.FocusCol)
		}
	} else {
		// No data rows
		v.Table.Select(0, v.FocusCol)
	}

	v.refreshStyles()
}

func isNumericColumn(name string) bool {
	n := strings.ToUpper(name)
	return n == "SIZE" || n == "REPLICAS" || n == "CPU" || n == "MEM" || n == "CONTAINERS"
}

func (v *ResourceView) SetActionState(id, action string, color tcell.Color) {
	v.ActionStates[id] = ActionState{
		Label: action,
		Color: color,
	}
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
	statusColor := styles.ColorFg

	if dataIdx >= 0 && dataIdx < len(v.Data) {
		item := v.Data[dataIdx]
		statusColor, _ = item.GetStatusColor()
		id := item.GetID()

		// 1. Check Transient Highlight (Precedence over Status and Action)
		if entry, ok := v.getTransientHighlight(id); ok {
			statusColor = entry.fg
		} else {
			// 2. Action Override
			if actionState, ok := v.ActionStates[id]; ok {
				statusColor = actionState.Color
			}

			// 3. Selection Override (Multi-select)
			if v.SelectedIDs[id] {
				statusColor = styles.ColorSelect
			}
		}
	}

	// Hover Effect: BG=StatusColor, FG=HoverFg
	v.Table.SetSelectedStyle(tcell.StyleDefault.Foreground(statusColor).Reverse(true).Bold(true))
}

// updateRowStyle updates the style for a specific row
func (v *ResourceView) updateRowStyle(rowIndex int, item dao.Resource) {
	statusColor, _ := item.GetStatusColor()
	id := item.GetID()
	isSelected := v.SelectedIDs[id]
	
	
	if entry, ok := v.getTransientHighlight(id); ok {
		statusColor = entry.fg
	} else {
		// 2. Action Override
		if actionState, ok := v.ActionStates[id]; ok {
			statusColor = actionState.Color
		}
	}

	cells := item.GetCells()
		for j, text := range cells {
			// Check bounds first to avoid panic in GetCell
			if j >= v.Table.GetColumnCount() {
				break
			}
			
			cell := v.Table.GetCell(rowIndex, j)
			if cell == nil {
				// Try to re-create cell if missing (rare but possible in race)
				if j < v.Table.GetColumnCount() {
					cell = tview.NewTableCell(" " + text + " ")
					v.Table.SetCell(rowIndex, j, cell)
				} else {
					continue
				}
			}

		displayText := text

		// Multi-Select Indicator
		if j == 0 && isSelected {
			displayText = "*" + displayText
		}
		
		headerName := ""
		if j < len(v.Headers) {
			headerName = strings.ToUpper(v.Headers[j])
		}

		// Optional: Replace Status with Action Status
		if headerName == "STATUS" {
			if actionState, ok := v.ActionStates[id]; ok {
				displayText = actionState.Label
			}
		}

		cell.SetText(" " + displayText + " ")
		
		// Style Application
		cell.SetTextColor(statusColor)

		
		cell.SetBackgroundColor(styles.ColorBg)
	

		if isSelected {
			cell.SetBackgroundColor(styles.ColorBg)
			cell.SetTextColor(styles.ColorSelect)
		}

		// Align Right for numeric columns
		if headerName == "SIZE" || headerName == "REPLICAS" || headerName == "CPU" || headerName == "MEM" || headerName == "CONTAINERS" {
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

func (v *ResourceView) HighlightIDs(ids []string, bg, fg tcell.Color, duration time.Duration) {
	if len(ids) == 0 || duration <= 0 {
		return
	}

	for _, id := range ids {
		if id == "" {
			continue
		}
		v.addTransientHighlight(id, bg, fg, duration)
	}

	if v.App != nil {
		v.App.GetTviewApp().QueueUpdateDraw(func() {
			v.refreshStyles()
		})
	}
}

func (v *ResourceView) ScheduleHighlight(match func(dao.Resource) bool, bg, fg tcell.Color, duration time.Duration) {
	if match == nil || duration <= 0 {
		return
	}

	v.highlightMu.Lock()
	v.pendingHighlightRequests = append(v.pendingHighlightRequests, highlightRequest{
		match:    match,
		bg:       bg,
		fg:       fg,
		duration: duration,
	})
	v.highlightMu.Unlock()
}

func (v *ResourceView) processHighlightRequests() {
	v.highlightMu.Lock()
	requests := v.pendingHighlightRequests
	v.pendingHighlightRequests = nil
	v.highlightMu.Unlock()

	if len(requests) == 0 {
		return
	}

	for _, req := range requests {
		if req.match == nil {
			continue
		}
		for i, res := range v.Data {
			if req.match(res) {
				v.addTransientHighlight(res.GetID(), req.bg, req.fg, req.duration)
				
				// Focus the item (select row)
				v.Table.Select(i+1, 0)
			}
		}
	}
}

func (v *ResourceView) addTransientHighlight(id string, bg, fg tcell.Color, duration time.Duration) {
	if id == "" || duration <= 0 {
		return
	}

	expiry := time.Now().Add(duration)
	v.highlightMu.Lock()
	v.transientHighlights[id] = highlightEntry{bg: bg, fg: fg, expiry: expiry}
	v.highlightMu.Unlock()

	go func(target string, until time.Time) {
		sleep := time.Until(until)
		if sleep > 0 {
			time.Sleep(sleep)
		}
		v.highlightMu.Lock()
		entry, ok := v.transientHighlights[target]
		if ok && entry.expiry == until {
			delete(v.transientHighlights, target)
		}
		v.highlightMu.Unlock()
		if v.App != nil {
			v.App.GetTviewApp().QueueUpdateDraw(func() {
				v.refreshStyles()
			})
		}
	}(id, expiry)
}

func (v *ResourceView) getTransientHighlight(id string) (highlightEntry, bool) {
	v.highlightMu.Lock()
	defer v.highlightMu.Unlock()
	entry, ok := v.transientHighlights[id]
	return entry, ok
}

func (v *ResourceView) DeferRefresh(duration time.Duration) {
	if duration <= 0 {
		return
	}

	v.refreshDelayMu.Lock()
	defer v.refreshDelayMu.Unlock()
	until := time.Now().Add(duration)
	if until.After(v.refreshDelayUntil) {
		v.refreshDelayUntil = until
	}
}

func (v *ResourceView) PopRefreshDelay() time.Duration {
	v.refreshDelayMu.Lock()
	defer v.refreshDelayMu.Unlock()
	if v.refreshDelayUntil.IsZero() {
		return 0
	}

	now := time.Now()
	if now.Before(v.refreshDelayUntil) {
		delay := v.refreshDelayUntil.Sub(now)
		v.refreshDelayUntil = time.Time{}
		return delay
	}

	v.refreshDelayUntil = time.Time{}
	return 0
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
		if len(ids) > 0 {
			return ids, nil
		}
	}
	// Fallback to single selection
	id, err := v.GetSelectedID()
	if err != nil {
		return nil, err
	}
	return []string{id}, nil
}

func stripFormattingTags(text string) string {
	return common.StripColorTags(text)
}
