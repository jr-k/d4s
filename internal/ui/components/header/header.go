package header

import (
	"fmt"

	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

type HeaderComponent struct {
	View      *tview.Table
	LastStats dao.HostStats
}

func NewHeaderComponent() *HeaderComponent {
	h := tview.NewTable().SetBorders(false)
	h.SetBackgroundColor(styles.ColorBg)
	return &HeaderComponent{
		View: h,
	}
}

func (h *HeaderComponent) UpdateShortcuts(shortcuts []string) {
	h.Update(h.LastStats, shortcuts)
}

func (h *HeaderComponent) Update(stats dao.HostStats, shortcuts []string) {
	// Merge with existing stats to avoid flickering "..."
	// If new stats have "...", check if we have better old values
	if stats.CPUPercent == "..." && h.LastStats.CPUPercent != "" && h.LastStats.CPUPercent != "..." {
		stats.CPUPercent = h.LastStats.CPUPercent
	}
	if stats.MemPercent == "..." && h.LastStats.MemPercent != "" && h.LastStats.MemPercent != "..." {
		stats.MemPercent = h.LastStats.MemPercent
	}

	// Save for next time
	h.LastStats = stats

	h.View.Clear()
	h.View.SetBackgroundColor(styles.ColorBg) // Ensure no black block

	// Build CPU display with cores and percentage
	cpuDisplay := fmt.Sprintf("%s cores", stats.CPU)
	if stats.CPUPercent != "" && stats.CPUPercent != "N/A" && stats.CPUPercent != "..." {
		cpuDisplay += fmt.Sprintf(" ([blue]%s[-])", stats.CPUPercent)
	} else if stats.CPUPercent == "..." {
		cpuDisplay += " [dim](...)"
	}

	// Build Mem display with total and percentage
	memDisplay := stats.Mem
	if stats.MemPercent != "" && stats.MemPercent != "N/A" && stats.MemPercent != "..." {
		memDisplay += fmt.Sprintf(" ([blue]%s[-])", stats.MemPercent)
	} else if stats.MemPercent == "..." {
		memDisplay += " [dim](...)"
	}

	lines := []string{
		fmt.Sprintf("[#8be9fd]Host:    [white]%s", stats.Hostname),
		fmt.Sprintf("[#8be9fd]D4s Rev: [white]v%s", stats.D4SVersion),
		fmt.Sprintf("[#8be9fd]User:    [white]%s", stats.User),
		fmt.Sprintf("[#8be9fd]Engine:  [white]%s [dim](%s)", stats.Name, stats.Version),
		fmt.Sprintf("[#8be9fd]CPU:     [white]%s", cpuDisplay),
		fmt.Sprintf("[#8be9fd]Mem:     [white]%s", memDisplay),
	}

	// Layout Header
	// Col 0: Stats
	for i, line := range lines {
		// Add padding to the right of stats
		cell := tview.NewTableCell(line).
			SetBackgroundColor(styles.ColorBg).
			SetAlign(tview.AlignLeft).
			SetExpansion(0) // Fixed width
		h.View.SetCell(i, 0, cell)
	}

	// Spacer Column (between Stats and Shortcuts)
	// A fixed width column to separate them nicely (tripled size ~21 spaces)
	spacerWidth := "                     "
	for i := 0; i < 6; i++ {
		h.View.SetCell(i, 1, tview.NewTableCell(spacerWidth).SetBackgroundColor(styles.ColorBg))
	}

	// Center Columns: Shortcuts
	// Max 6 per column (matches header height)
	// Each shortcut uses 2 columns: alias (fixed width) and label
	const maxPerCol = 6
	const groupSpacer = "      " // Spacer between shortcut groups

	// Organize shortcuts into columns, respecting color changes
	var columns [][]string
	if len(shortcuts) > 0 {
		var currentCol []string
		lastColorPrefix := ""

		for _, sc := range shortcuts {
			// Extract color prefix
			colorPrefix := ""
			idx := -1
			for j := 0; j < len(sc); j++ {
				if sc[j] == '<' {
					idx = j
					break
				}
			}
			if idx > 0 {
				colorPrefix = sc[:idx]
			}

			effectiveColor := colorPrefix
			if sc == "" {
				effectiveColor = lastColorPrefix
			}

			// Determine if we need a new column
			// New column if:
			// 1. Current column is full
			// 2. Color prefix changes (and current column is not empty)
			if len(currentCol) >= maxPerCol || (len(currentCol) > 0 && lastColorPrefix != "" && effectiveColor != lastColorPrefix) {
				columns = append(columns, currentCol)
				currentCol = []string{}
			}

			currentCol = append(currentCol, sc)
			if sc != "" {
				lastColorPrefix = effectiveColor
			}
		}
		// Append last column
		if len(currentCol) > 0 {
			columns = append(columns, currentCol)
		}
	}

	colIndex := 2 // Start at 2 (0=Stats, 1=Spacer)

	// Render Columns
	for i, colShortcuts := range columns {
		// Add spacer column between groups (but not before the first one)
		if i > 0 {
			for row := 0; row < maxPerCol; row++ {
				h.View.SetCell(row, colIndex, tview.NewTableCell(groupSpacer).SetBackgroundColor(styles.ColorBg))
			}
			colIndex++
		}

		// Fill all 6 rows for this column pair
		for row := 0; row < maxPerCol; row++ {
			var aliasText, labelText string
			if row < len(colShortcuts) {
				shortcut := colShortcuts[row]

				// Parse logic (kept same as before)
				ltIdx := -1
				gtIdx := -1
				for j := 0; j < len(shortcut); j++ {
					if shortcut[j] == '<' && ltIdx == -1 {
						ltIdx = j
					} else if shortcut[j] == '>' && ltIdx != -1 {
						gtIdx = j
						break
					}
				}

				if ltIdx != -1 && gtIdx != -1 {
					colorPrefix := shortcut[:ltIdx]
					key := shortcut[ltIdx+1 : gtIdx]
					aliasText = colorPrefix + "<" + key + ">[-]  "

					labelStart := gtIdx + 1
					if labelStart < len(shortcut) && shortcut[labelStart] == '[' {
						for labelStart < len(shortcut) && shortcut[labelStart] != ']' {
							labelStart++
						}
						labelStart++ // Skip ]
					}
					for labelStart < len(shortcut) && shortcut[labelStart] == ' ' {
						labelStart++
					}
					if labelStart < len(shortcut) {
						labelText = shortcut[labelStart:]
					}
				} else {
					labelText = shortcut
				}
			}

			// Alias column
			aliasCell := tview.NewTableCell(aliasText).
				SetAlign(tview.AlignLeft).
				SetExpansion(0).
				SetBackgroundColor(styles.ColorBg)
			h.View.SetCell(row, colIndex, aliasCell)

			// Label column
			labelCell := tview.NewTableCell(labelText).
				SetAlign(tview.AlignLeft).
				SetExpansion(0).
				SetBackgroundColor(styles.ColorBg)
			h.View.SetCell(row, colIndex+1, labelCell)
		}
		colIndex += 2
	}

	// Flexible Spacer Column (pushes logo to right)
	// Use an empty cell with Expansion 1. Need to set it on at least one row.
	// Set on all rows to be safe with background
	for i := 0; i < 6; i++ {
		h.View.SetCell(i, colIndex, tview.NewTableCell("").SetExpansion(1).SetBackgroundColor(styles.ColorBg))
	}
	colIndex++

	// Right Column: Logo
	for i, line := range common.GetLogo() {
		cell := tview.NewTableCell(line).
			SetAlign(tview.AlignLeft).
			SetBackgroundColor(styles.ColorBg).
			SetExpansion(0) // Fixed width
		h.View.SetCell(i, colIndex, cell)
	}
}
