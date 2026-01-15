package dialogs

import (
	"github.com/gdamore/tcell/v2"
	"github.com/jessym/d4s/internal/ui/common"
	"github.com/jessym/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

func NewHelpView(app common.AppController) tview.Primitive {
	helpTable := tview.NewTable()
	helpTable.SetBorders(false)
	helpTable.SetBackgroundColor(tcell.ColorBlack)
	
	// Format: Col1 | Col2
	rows := [][]string{
		{"[#ffb86c::b]GLOBAL", ""},
		{"[#5f87ff]:[-]             Command", "[#5f87ff]?[-]             Help"},
		{"[#5f87ff]/[-]             Filter", "[#5f87ff]Esc[-]           Back/Clear"},
		{"[#5f87ff]c[-]             Copy", ""},
		{"", ""},
		{"[#ffb86c::b]DOCKER", ""},
		{"[#5f87ff]:c[-]            Containers", "[#5f87ff]:i[-]            Images"},
		{"[#5f87ff]:v[-]            Volumes", "[#5f87ff]:n[-]            Networks"},
		{"[#5f87ff]:p[-]            Compose", ""},
		{"", ""},
		{"[#ffb86c::b]SWARM", ""},
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
		AddText(" Help ", true, tview.AlignCenter, styles.ColorTitle)
	helpBox.SetBorder(true).SetBorderColor(styles.ColorTitle).SetBackgroundColor(tcell.ColorBlack)

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
			return nil
		}
		return event
	})
	
	return helpFlex
}
