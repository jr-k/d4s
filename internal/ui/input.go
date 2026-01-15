package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowInput shows a modal with an input field
func (a *App) ShowInput(title, label, initialText string, onDone func(text string)) {
	// Center the dialog
	dialogWidth := 50
	dialogHeight := 7

	input := tview.NewInputField().
		SetFieldBackgroundColor(ColorSelectBg).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabel(" " + label). // Padding label
		SetLabelColor(tcell.ColorWhite).
		SetText(initialText)
	input.SetBackgroundColor(tcell.ColorBlack)
	
	// Layout
	// Use a container flex to center input vertically inside the border
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
		SetTitleColor(ColorTitle).
		SetBorderColor(ColorTitle).
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
		a.Pages.RemovePage("input")
		page, _ := a.Pages.GetFrontPage()
		if view, ok := a.Views[page]; ok {
			a.TviewApp.SetFocus(view.Table)
		}
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

	a.Pages.AddPage("input", modal, true, true)
	a.TviewApp.SetFocus(input)
}

