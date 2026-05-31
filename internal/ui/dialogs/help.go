package dialogs

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

func NewHelpView(app common.AppController) tview.Primitive {
	helpTable := tview.NewTable()
	helpTable.SetBorders(false)
	helpTable.SetBackgroundColor(styles.ColorBlack)

	k := styles.TagSCKey // shortcut key color
	a := styles.TagAccent // accent/section header color

	// Format: Col1 | Col2
	rows := [][]string{
		{fmt.Sprintf("[%s::b]GLOBAL", a), ""},
		{fmt.Sprintf("[%s]:[-]         Command", k), fmt.Sprintf("[%s]?[-]         Help", k)},
		{fmt.Sprintf("[%s]/[-]         Filter", k), fmt.Sprintf("[%s]esc[-]       Back/Clear", k)},
		{fmt.Sprintf("[%s]c[-]         Copy", k), fmt.Sprintf("[%s]u[-]         Unselect All", k)},
		{fmt.Sprintf("[%s]:a[-]        Aliases", k), ""},
		{"", ""},
		{fmt.Sprintf("[%s::b]DOCKER", a), ""},
		{fmt.Sprintf("[%s]:c[-]        Containers", k), fmt.Sprintf("[%s]:i[-]        Images", k)},
		{fmt.Sprintf("[%s]:v[-]        Volumes", k), fmt.Sprintf("[%s]:n[-]        Networks", k)},
		{fmt.Sprintf("[%s]:p[-]        Compose", k), fmt.Sprintf("[%s]:o[-]        Contexts", k)},
		{fmt.Sprintf("[%s]:g[-]        Plugins", k), ""},
		{"", ""},
		{fmt.Sprintf("[%s::b]SWARM", a), ""},
		{fmt.Sprintf("[%s]:d[-]        Nodes", k), fmt.Sprintf("[%s]:t[-]        Tasks", k)},
		{fmt.Sprintf("[%s]:k[-]        Stacks", k), fmt.Sprintf("[%s]:f[-]        Configs", k)},
		{fmt.Sprintf("[%s]:s[-]        Services", k), fmt.Sprintf("[%s]:x[-]        Secrets", k)},
		{"", ""},
		{fmt.Sprintf("[%s::b]NAVIGATION", a), ""},
		{fmt.Sprintf("[%s]←/→[-], [%s]j/k[-]   Navigate", k, k), fmt.Sprintf("[%s]enter[-]       Drill Down", k)},
		{fmt.Sprintf("[%s]shift ←/→[-] Sort Column", k), fmt.Sprintf("[%s]shift ↑/↓[-] Toggle Order", k)},
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
