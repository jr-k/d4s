package ui

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowConfirmation shows a modal asking to type "Yes Please!" and allows forcing
func (a *App) ShowConfirmation(actionName, item string, onConfirm func(force bool)) {
	// Center the dialog
	dialogWidth := 60
	dialogHeight := 16 

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
	
	// Force Checkbox
	force := false
	checkbox := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[white][ ] Force (Tab to focus, Space to toggle)")
	checkbox.SetBackgroundColor(tcell.ColorBlack)

	updateCheckbox := func(focused bool) {
		prefix := "[ ]"
		if force {
			prefix = "[red][X]"
		}
		
		color := "[white]"
		if focused {
			color = "[#ffb86c]" // Orange focus
		}

		checkbox.SetText(fmt.Sprintf("%s%s Force (Tab to focus, Space to toggle)", color, prefix))
	}

	// Layout
	// We need to handle focus switching between Input and Checkbox manually or use tview built-in focus chain?
	// But `modal` is the page. `flex` contains items.
	
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(text, 0, 1, false).
		AddItem(input, 3, 1, true).
		AddItem(checkbox, 1, 1, false) // 1 line for checkbox
	
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

	// Input Handlers
	
	// We use a state variable to track focus because tview.Flex doesn't strictly enforce focus cycling the way we might want with 2 items where one is a TextView acting as Checkbox.
	// Actually, we can make the TextView interactive if we give it InputCapture.
	// But tview's SetFocus must be called.
	
	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			if input.GetText() == "Yes Please!" {
				closeModal()
				onConfirm(force)
			} else {
				a.Flash.SetText("[red]Confirmation mismatch. Action cancelled.")
				closeModal()
			}
		} else if key == tcell.KeyEsc {
			closeModal()
		} else if key == tcell.KeyTab {
			// Switch to Checkbox
			updateCheckbox(true)
			a.TviewApp.SetFocus(checkbox)
		}
	})

	checkbox.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			// Switch back to Input
			updateCheckbox(false)
			a.TviewApp.SetFocus(input)
			return nil
		}
		if event.Rune() == ' ' {
			force = !force
			updateCheckbox(true)
			return nil
		}
		if event.Key() == tcell.KeyEsc {
			closeModal()
			return nil
		}
		if event.Key() == tcell.KeyEnter {
			// Allow confirming from checkbox too if valid?
			if input.GetText() == "Yes Please!" {
				closeModal()
				onConfirm(force)
			}
			return nil
		}
		return event
	})

	a.Pages.AddPage("confirm", modal, true, true)
	a.TviewApp.SetFocus(input)
}
