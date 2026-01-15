package dialogs

import (
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

func ShowTextView(app common.AppController, title, content string) {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetScrollable(true).
		SetText(content)
	
	tv.SetBorder(true).SetTitle(title).SetTitleColor(styles.ColorTitle)
	tv.SetBackgroundColor(styles.ColorBg)
	
	pages := app.GetPages()
	tviewApp := app.GetTviewApp()
	
	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			pages.RemovePage("textview")
			tviewApp.SetFocus(pages)
			return nil
		}
		return event
	})
	
	pages.AddPage("textview", tv, true, true)
	tviewApp.SetFocus(tv)
}
