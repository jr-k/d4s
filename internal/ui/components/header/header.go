package header

import (
	"fmt"

	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

type HeaderComponent struct {
	View          *tview.Flex
	StatsView     *tview.Table
	ShortcutsView *tview.Table
	LogoView      *tview.Table
	LastStats     dao.HostStats
}

func NewHeaderComponent() *HeaderComponent {
	// Stats (Left)
	stats := tview.NewTable().SetBorders(false)
	stats.SetBackgroundColor(styles.ColorBg)

	// Shortcuts (Middle)
	shortcuts := tview.NewTable().SetBorders(false)
	shortcuts.SetBackgroundColor(styles.ColorBg)

	// Logo (Right)
	logo := tview.NewTable().SetBorders(false)
	logo.SetBackgroundColor(styles.ColorBg)

	// Flex Layout
	flex := tview.NewFlex().SetDirection(tview.FlexColumn)
	flex.SetBackgroundColor(styles.ColorBg)

	// Add items: Stats (fixed), Shortcuts (flex), Logo (fixed)
	// Initial sizes 0, will be updated in Update()
	flex.AddItem(stats, 0, 0, false)
	flex.AddItem(shortcuts, 0, 1, false)
	flex.AddItem(logo, 0, 0, false)

	return &HeaderComponent{
		View:          flex,
		StatsView:     stats,
		ShortcutsView: shortcuts,
		LogoView:      logo,
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

	// Clear Sub-Views
	h.StatsView.Clear()
	h.ShortcutsView.Clear()
	h.LogoView.Clear()

	// 1. Stats View
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
		fmt.Sprintf("[orange]Host:    [white]%s", stats.Hostname),
		fmt.Sprintf("[orange]D4s Rev: [white]v%s", stats.D4SVersion),
		fmt.Sprintf("[orange]User:    [white]%s", stats.User),
		fmt.Sprintf("[orange]Engine:  [white]%s [dim](%s)", stats.Name, stats.Version),
		fmt.Sprintf("[orange]CPU:     [white]%s", cpuDisplay),
		fmt.Sprintf("[orange]Mem:     [white]%s", memDisplay),
	}

	statsWidth := 0
	for i, line := range lines {
		h.StatsView.SetCell(i, 0, tview.NewTableCell(line).
			SetBackgroundColor(styles.ColorBg).
			SetAlign(tview.AlignLeft))

		w := tview.TaggedStringWidth(line)
		if w > statsWidth {
			statsWidth = w
		}
	}
	statsWidth += 4 // Padding

	// 2. Logo View
	logoWidth := 0
	for i, line := range common.GetLogo() {
		cell := tview.NewTableCell(line).
			SetAlign(tview.AlignLeft).
			SetBackgroundColor(styles.ColorBg)
		h.LogoView.SetCell(i, 0, cell)

		w := tview.TaggedStringWidth(line)
		if w > logoWidth {
			logoWidth = w
		}
	}
	logoWidth += 2 // Padding

	// 3. Shortcuts View
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

	colIndex := 0

	// Render Columns
	for i, colShortcuts := range columns {
		// Add spacer column between groups
		if i > 0 {
			for row := 0; row < maxPerCol; row++ {
				h.ShortcutsView.SetCell(row, colIndex, tview.NewTableCell(groupSpacer).SetBackgroundColor(styles.ColorBg))
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
				SetBackgroundColor(styles.ColorBg)
			h.ShortcutsView.SetCell(row, colIndex, aliasCell)

			// Label column
			labelCell := tview.NewTableCell(labelText).
				SetAlign(tview.AlignLeft).
				SetBackgroundColor(styles.ColorBg)
			h.ShortcutsView.SetCell(row, colIndex+1, labelCell)
		}
		colIndex += 2
	}

	// 4. Resize Flex Items
	h.View.ResizeItem(h.StatsView, statsWidth, 0)
	h.View.ResizeItem(h.LogoView, logoWidth, 0)
}
