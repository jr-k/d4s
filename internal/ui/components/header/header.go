package header

import (
	"fmt"

	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

type HeaderComponent struct {
	View *tview.Table
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
	
	logo := []string{
		" [#ffb86c]██████╗   ██╗  ██╗   █████╗ ",
		" [#ffb86c]██╔══██╗  ██║  ██║  ██╔═══╝ ",
		" [#ffb86c]██║  ██║  ███████║  █████╗ ",
		" [#ffb86c]██║  ██║       ██║       ██╗",
		" [#ffb86c]██████╔╝       ██║  ██████╔╝",
		" [#ffb86c]╚═════╝        ╚═╝  ╚═════╝ ",
	}
	
	// Build CPU display with cores and percentage
	cpuDisplay := fmt.Sprintf("%s cores", stats.CPU)
	if stats.CPUPercent != "" && stats.CPUPercent != "N/A" && stats.CPUPercent != "..." {
		cpuDisplay += fmt.Sprintf(" ([grey]%s[-])", stats.CPUPercent)
	} else if stats.CPUPercent == "..." {
		cpuDisplay += " [dim](...)"
	}
	
	// Build Mem display with total and percentage
	memDisplay := stats.Mem
	if stats.MemPercent != "" && stats.MemPercent != "N/A" && stats.MemPercent != "..." {
		memDisplay += fmt.Sprintf(" ([grey]%s[-]L)", stats.MemPercent)
	} else if stats.MemPercent == "..." {
		memDisplay += " [dim](...)"
	}
	
	lines := []string{
		fmt.Sprintf("[#8be9fd]Host:    [white]%s", stats.Hostname),
		fmt.Sprintf("[#8be9fd]D4s Rev: [white]v%s", stats.D4SVersion),
		fmt.Sprintf("[#8be9fd]User:    [white]%s", stats.User),
		fmt.Sprintf("[#8be9fd]Engine:  [white]%s [dim](v%s)", stats.Name, stats.Version),
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
	
	colIndex := 2 // Start at 2 (0=Stats, 1=Spacer)
	for i := 0; i < len(shortcuts); i += maxPerCol {
		end := i + maxPerCol
		if end > len(shortcuts) {
			end = len(shortcuts)
		}
		
		chunk := shortcuts[i:end]
		
		// Add spacer column between groups (but not before the first one, handled by existing spacer)
		if i > 0 {
			for row := 0; row < maxPerCol; row++ {
				h.View.SetCell(row, colIndex, tview.NewTableCell(groupSpacer).SetBackgroundColor(styles.ColorBg))
			}
			colIndex++
		}
		
		// Fill all 6 rows for this column pair to ensure background color
		for row := 0; row < maxPerCol; row++ {
			var aliasText, labelText string
			if row < len(chunk) {
				// Parse the shortcut format: [#5f87ff]<key>[-]   label
				shortcut := chunk[row]
				
				// Find the start of <key>
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
					// Extract everything before < as color prefix
					colorPrefix := shortcut[:ltIdx]
					// Extract key between < and >
					key := shortcut[ltIdx+1 : gtIdx]
					// Build alias: colorPrefix + <key> + [-] + 2 spaces padding
					// This lets tview auto-size the column to the widest alias + 2 spaces,
					// ensuring labels are aligned and close to the aliases.
					aliasText = colorPrefix + "<" + key + ">[-]  "
					
					// Find label: everything after >[-] and spaces
					labelStart := gtIdx + 1
					// Skip [-]
					if labelStart < len(shortcut) && shortcut[labelStart] == '[' {
						for labelStart < len(shortcut) && shortcut[labelStart] != ']' {
							labelStart++
						}
						labelStart++ // Skip ]
					}
					// Skip spaces
					for labelStart < len(shortcut) && shortcut[labelStart] == ' ' {
						labelStart++
					}
					// Extract label
					if labelStart < len(shortcut) {
						labelText = shortcut[labelStart:]
					}
				} else {
					// Fallback: use whole text as label if parsing fails
					labelText = shortcut
				}
			}
			
			// Alias column (fixed width, left aligned)
			aliasCell := tview.NewTableCell(aliasText).
				SetAlign(tview.AlignLeft).
				SetExpansion(0).
				SetBackgroundColor(styles.ColorBg)
			h.View.SetCell(row, colIndex, aliasCell)
			
			// Label column (flexible, left aligned)
			labelCell := tview.NewTableCell(labelText).
				SetAlign(tview.AlignLeft).
				SetExpansion(0).
				SetBackgroundColor(styles.ColorBg)
			h.View.SetCell(row, colIndex+1, labelCell)
		}
		colIndex += 2 // Move to next shortcut column pair
	}
	
	// Flexible Spacer Column (pushes logo to right)
	// Use an empty cell with Expansion 1. Need to set it on at least one row.
	// Set on all rows to be safe with background
	for i := 0; i < 6; i++ {
		h.View.SetCell(i, colIndex, tview.NewTableCell("").SetExpansion(1).SetBackgroundColor(styles.ColorBg))
	}
	colIndex++

	// Right Column: Logo
	for i, line := range logo {
		cell := tview.NewTableCell(line).
			SetAlign(tview.AlignLeft).
			SetBackgroundColor(styles.ColorBg).
			SetExpansion(0) // Fixed width
		h.View.SetCell(i, colIndex, cell)
	}
}
