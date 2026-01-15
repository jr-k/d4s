package dialogs

import (
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

// ShowInput shows a modal with an input field
func ShowInput(app common.AppController, title, label, initialText string, onDone func(text string)) {
	// Center the dialog
	dialogWidth := 50
	dialogHeight := 7

	pages := app.GetPages()
	tviewApp := app.GetTviewApp()

	input := tview.NewInputField().
		SetFieldBackgroundColor(styles.ColorSelectBg).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabel(" " + label). // Padding label
		SetLabelColor(tcell.ColorWhite).
		SetText(initialText)
	input.SetBackgroundColor(tcell.ColorBlack)
	
	// Layout
	content := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 1, 1, false). // Top padding
		AddItem(input, 1, 1, true). // Input line
		AddItem(nil, 1, 1, false)  // Bottom padding
	
	content.SetBackgroundColor(tcell.ColorBlack)

	// Main Frame with Border
	frame := tview.NewFrame(content).
		SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true).
		SetTitle(" " + title + " ").
		SetTitleColor(styles.ColorTitle).
		SetBorderColor(styles.ColorTitle).
		SetBackgroundColor(tcell.ColorBlack)

	// Center on screen
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(frame, dialogHeight, 1, true).
			AddItem(nil, 0, 1, false), dialogWidth, 1, true).
		AddItem(nil, 0, 1, false)

	// Restore focus helper
	closeModal := func() {
		pages.RemovePage("input")
		tviewApp.SetFocus(pages)
	}

	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			text := input.GetText()
			closeModal()
			if text != "" {
				onDone(text)
			}
		} else if key == tcell.KeyEsc {
			closeModal()
		}
	})

	input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			closeModal()
			return nil
		}
		return event
	})

	pages.AddPage("input", modal, true, true)
	tviewApp.SetFocus(input)
}
