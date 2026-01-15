package ui

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowConfirmation shows a modal asking to type "Yes Please!"
func (a *App) ShowConfirmation(actionName, item string, onConfirm func()) {
	// Center the dialog
	dialogWidth := 60
	dialogHeight := 14

	text := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf("\n[red::b] DANGER ZONE \n\n[white::-]You are about to %s:\n[yellow]%s[white]\n\nType exactly: [red::b]Yes Please![white::-]", actionName, item))
	text.SetBackgroundColor(tcell.ColorBlack)
	
	input := tview.NewInputField().
		SetFieldBackgroundColor(ColorSelectBg).
		SetFieldTextColor(tcell.ColorRed).
		SetLabel("Confirmation: ").
		SetLabelColor(tcell.ColorWhite)
	input.SetBackgroundColor(tcell.ColorBlack)
	
	// Layout
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(text, 0, 1, false).
		AddItem(input, 3, 1, true)
	
	flex.SetBorder(true).
		SetTitle(" Are you sure? ").
		SetTitleColor(tcell.ColorRed).
		SetBorderColor(tcell.ColorRed).
		SetBackgroundColor(tcell.ColorBlack)

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
		a.Pages.RemovePage("confirm")
		page, _ := a.Pages.GetFrontPage()
		if view, ok := a.Views[page]; ok {
			a.TviewApp.SetFocus(view.Table)
		}
	}

	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			if input.GetText() == "Yes Please!" {
				closeModal()
				onConfirm()
			} else {
				a.Flash.SetText("[red]Confirmation mismatch. Action cancelled.")
				closeModal()
			}
		} else if key == tcell.KeyEsc {
			closeModal()
		}
	})

	// Also catch Esc on the input field input capture to be sure
	input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			closeModal()
			return nil
		}
		return event
	})

	a.Pages.AddPage("confirm", modal, true, true)
	a.TviewApp.SetFocus(input)
}

