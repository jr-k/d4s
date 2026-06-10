package dialogs

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

func ShowResultModal(app common.AppController, action string, successCount int, errors []string) {
	text := fmt.Sprintf("\n[%s]✔ %d items processed successfully.\n\n[%s]✘ %d items failed:\n", styles.TagInfo, successCount, styles.TagError, len(errors))
	for _, err := range errors {
		text += fmt.Sprintf("\n• [%s]%s", styles.TagFg, err)
	}
	
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetText(text).
		SetTextAlign(tview.AlignLeft).
		SetScrollable(true)
	tv.SetBackgroundColor(styles.ColorBlack)
	
	tv.SetBorder(true).SetTitle(fmt.Sprintf("[%s::b]<Action Report>[-::-]", styles.TagCyan)).SetBorderColor(styles.ColorMenuKey).SetBackgroundColor(styles.ColorBlack)
	
	// Modal Layout
	modalWidth := 60
	modalHeight := 15
	
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(tv, modalHeight, 1, true).
			AddItem(nil, 0, 1, false), modalWidth, 1, true).
		AddItem(nil, 0, 1, false)
		
	// Close Handler
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc || event.Key() == tcell.KeyEnter {
			app.GetPages().RemovePage("result")
			app.RefreshCurrentView()
			// Restore focus
			app.GetTviewApp().SetFocus(app.GetPages())
			return nil
		}
		return event
	})
	
	app.GetPages().AddPage("result", flex, true, true)
	app.GetTviewApp().SetFocus(flex)
}
