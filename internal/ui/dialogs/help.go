package dialogs

import (
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

func NewHelpView(app common.AppController) tview.Primitive {
	helpTable := tview.NewTable()
	helpTable.SetBorders(false)
	helpTable.SetBackgroundColor(styles.ColorBlack)

	// Format: Col1 | Col2
	rows := [][]string{
		{"[orange::b]GLOBAL", ""},
		{"[#5f87ff]:[-]         Command", "[#5f87ff]?[-]         Help"},
		{"[#5f87ff]/[-]         Filter", "[#5f87ff]esc[-]       Back/Clear"},
		{"[#5f87ff]c[-]         Copy", "[#5f87ff]u[-]         Unselect All"},
		{"[#5f87ff]:a[-]        Aliases", ""},
		{"", ""},
		{"[orange::b]DOCKER", ""},
		{"[#5f87ff]:c[-]        Containers", "[#5f87ff]:i[-]        Images"},
		{"[#5f87ff]:v[-]        Volumes", "[#5f87ff]:n[-]        Networks"},
		{"[#5f87ff]:p[-]        Compose", ""},
		{"", ""},
		{"[orange::b]SWARM", ""},
		{"[#5f87ff]:s[-]        Services", "[#5f87ff]:no[-]       Nodes"},
		{"", ""},
		{"[orange::b]NAVIGATION", ""},
		{"[#5f87ff]←/→[-], [#5f87ff]j/k[-]   Navigate", "[#5f87ff]enter[-]       Drill Down"},
		{"[#5f87ff]shift ←/→[-] Sort Column", "[#5f87ff]shift ↑/↓[-] Toggle Order"},
	}

	for i, row := range rows {
		// Column 1
		text1 := row[0]
		cell1 := tview.NewTableCell(text1).
			SetTextColor(styles.ColorWhite).
			SetAlign(tview.AlignLeft).
			SetExpansion(1)
		helpTable.SetCell(i, 0, cell1)

		// Spacer Column
		helpTable.SetCell(i, 1, tview.NewTableCell("    ").SetSelectable(false))

		// Column 2
		text2 := ""
		if len(row) > 1 {
			text2 = row[1]
		}
		cell2 := tview.NewTableCell(text2).
			SetTextColor(styles.ColorWhite).
			SetAlign(tview.AlignLeft).
			SetExpansion(1)
		helpTable.SetCell(i, 2, cell2)
	}

	helpBox := tview.NewFrame(helpTable).
		SetBorders(1, 1, 1, 1, 4, 4).
		AddText(" Help ", true, tview.AlignCenter, styles.ColorTitle)
	helpBox.SetBorder(true).SetBorderColor(styles.ColorTitle).SetBackgroundColor(styles.ColorBlack)

	// Center Modal
	helpFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(helpBox, 30, 1, true).
			AddItem(nil, 0, 1, false), 90, 1, true).
		AddItem(nil, 0, 1, false)

	helpFlex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc || event.Rune() == 'q' {
			app.GetPages().RemovePage("help")
			// Restore focus
			app.GetTviewApp().SetFocus(app.GetPages())
			app.UpdateShortcuts()
			return nil
		}
		return event
	})

	return helpFlex
}
