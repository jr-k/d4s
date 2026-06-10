package dialogs

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

// ShowInput shows a modal with an input field
func ShowInput(app common.AppController, title, label, initialText string, onDone func(text string)) {
	dialogWidth := 50
	dialogHeight := 7

	pages := app.GetPages()
	tviewApp := app.GetTviewApp()

	input := tview.NewInputField().
		SetFieldBackgroundColor(styles.ColorBlack).
		SetFieldTextColor(styles.ColorWhite).
		SetLabel(" " + label + " ").
		SetLabelColor(styles.ColorWhite).
		SetText(initialText)
	input.SetBackgroundColor(styles.ColorBlack)

	confirmBtn := tview.NewButton("Confirm")
	confirmBtn.SetStyle(tcell.StyleDefault.Foreground(styles.ColorFg).Background(styles.ColorBlack)).
		SetActivatedStyle(tcell.StyleDefault.Foreground(styles.ColorBlack).Background(styles.ColorMenuKey))
	confirmBtn.SetBackgroundColor(styles.ColorBlack)

	btnRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).
		AddItem(confirmBtn, 11, 0, false).
		AddItem(nil, 0, 1, false)
	btnRow.SetBackgroundColor(styles.ColorBlack)

	// Layout
	content := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 1, 0, false).
		AddItem(input, 1, 0, true).
		AddItem(nil, 1, 0, false).
		AddItem(btnRow, 1, 0, false)
	content.SetBackgroundColor(styles.ColorBlack)

	// Main Frame with Border
	frame := tview.NewFrame(content).
		SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true).
		SetTitle(fmt.Sprintf("[%s::b]<%s>[-::-]", styles.TagCyan, title)).
		SetTitleColor(styles.ColorTitle).
		SetBorderColor(styles.ColorMenuKey).
		SetBackgroundColor(styles.ColorBlack)

	// Center on screen
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(frame, dialogHeight, 1, true).
			AddItem(nil, 0, 1, false), dialogWidth, 1, true).
		AddItem(nil, 0, 1, false)

	closeModal := func() {
		pages.RemovePage("input")
		tviewApp.SetFocus(pages)
		app.UpdateShortcuts()
	}

	submit := func() {
		text := input.GetText()
		closeModal()
		if text != "" {
			onDone(text)
		}
	}

	focusOnBtn := false

	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			focusOnBtn = true
			tviewApp.SetFocus(confirmBtn)
		} else if key == tcell.KeyEsc {
			closeModal()
		}
	})

	input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			closeModal()
			return nil
		}
		if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyDown {
			focusOnBtn = true
			tviewApp.SetFocus(confirmBtn)
			return nil
		}
		return event
	})

	confirmBtn.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			closeModal()
			return nil
		case tcell.KeyEnter:
			submit()
			return nil
		case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyUp:
			focusOnBtn = false
			tviewApp.SetFocus(input)
			return nil
		}
		return event
	})

	_ = focusOnBtn

	pages.AddPage("input", modal, true, true)
	tviewApp.SetFocus(input)
	app.UpdateShortcuts()
}
