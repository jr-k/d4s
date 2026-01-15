package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowInput shows a modal with an input field
func (a *App) ShowInput(title, label string, onDone func(text string)) {
	// Center the dialog
	dialogWidth := 50
	dialogHeight := 7

	input := tview.NewInputField().
		SetFieldBackgroundColor(ColorSelectBg).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabel(label).
		SetLabelColor(tcell.ColorWhite)
	input.SetBackgroundColor(ColorBg)
	
	// Layout
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(input, 3, 1, true)
	
	flex.SetBorder(true).
		SetTitle(" " + title + " ").
		SetTitleColor(ColorTitle).
		SetBorderColor(ColorTitle).
		SetBackgroundColor(ColorBg)

	// Center on screen
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(flex, dialogHeight, 1, true).
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

