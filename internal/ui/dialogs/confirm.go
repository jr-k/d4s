package dialogs

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

// ShowConfirmation shows a modal asking to type "Yes Please!" and allows forcing
func ShowConfirmation(app common.AppController, actionName, item string, onConfirm func(force bool)) {
	// Center the dialog
	dialogWidth := 60
	dialogHeight := 16 
	
	pages := app.GetPages()
	tviewApp := app.GetTviewApp()

	text := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf("\n[red::b] DANGER ZONE \n\n[white::-]You are about to %s:\n[yellow]%s[white]\n\nType exactly: [red::b]Yes Please![white::-]", actionName, item))
	text.SetBackgroundColor(tcell.ColorBlack)
	
	// Force Checkbox
	force := false
	checkboxLabel := tview.NewTextView().
		SetDynamicColors(true).
		SetText("Force:").
		SetTextAlign(tview.AlignLeft)
	checkboxLabel.SetBackgroundColor(tcell.ColorBlack)
	
	checkbox := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetText("[ ]")
	checkbox.SetBackgroundColor(tcell.ColorBlack)

	updateCheckbox := func(focused bool) {
		text := "No"
		if force {
			text = "Yes"
		}
		
		color := "[white]"
		if focused {
			color = "[#ffb86c]" // Orange focus
			text = fmt.Sprintf("> %s", text)
		}

		checkbox.SetText(fmt.Sprintf("%s%s", color, text))
	}
	
	updateCheckbox(false)

	// Input field
	inputLabel := tview.NewTextView().
		SetDynamicColors(true).
		SetText("Confirmation:").
		SetTextAlign(tview.AlignLeft)
	inputLabel.SetBackgroundColor(tcell.ColorBlack)
	
	input := tview.NewInputField().
		SetFieldBackgroundColor(styles.ColorSelectBg).
		SetFieldTextColor(tcell.ColorRed)
	input.SetBackgroundColor(tcell.ColorBlack)

	// Form layout: 2 columns (label | widget) with fixed label width and padding
	
	// Helper for empty box
	empty := func(w int) tview.Primitive {
		return tview.NewBox().SetBackgroundColor(tcell.ColorBlack)
	}

	// Row 1: Checkbox
	checkboxRow := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(empty(2), 2, 0, false). // Left Padding
		AddItem(checkboxLabel, 15, 0, false). // Fixed Label Width
		AddItem(checkbox, 0, 1, false). // Widget
		AddItem(empty(2), 2, 0, false) // Right Padding
	
	// Row 2: Input
	inputRow := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(empty(2), 2, 0, false).
		AddItem(inputLabel, 15, 0, false).
		AddItem(input, 0, 1, false).
		AddItem(empty(2), 2, 0, false)
	
	// Form container
	form := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(checkboxRow, 1, 0, false).
		AddItem(inputRow, 3, 0, false)

	// Main content
	content := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(text, 0, 1, false).
		AddItem(form, 4, 0, true)
	
	content.SetBorder(true).
		SetTitle(" Are you sure? ").
		SetTitleColor(tcell.ColorRed).
		SetBorderColor(tcell.ColorRed).
		SetBackgroundColor(tcell.ColorBlack)

	// Center on screen
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(content, dialogHeight, 1, true).
			AddItem(nil, 0, 1, false), dialogWidth, 1, true).
		AddItem(nil, 0, 1, false)

	// Track current focus (0 = checkbox, 1 = input)
	currentFocus := 0

	// Restore focus helper
	closeModal := func() {
		pages.RemovePage("confirm")
		// We assume we want to focus back on the table or pages
		tviewApp.SetFocus(pages) 
	}

	// Navigation helper
	moveFocus := func(direction int) {
		currentFocus += direction
		if currentFocus < 0 {
			currentFocus = 1
		} else if currentFocus > 1 {
			currentFocus = 0
		}
		
		if currentFocus == 0 {
			updateCheckbox(true)
			tviewApp.SetFocus(checkbox)
		} else {
			updateCheckbox(false)
			tviewApp.SetFocus(input)
		}
	}

	checkbox.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyDown {
			moveFocus(1)
			return nil
		}
		if event.Key() == tcell.KeyUp {
			moveFocus(-1)
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
			if input.GetText() == "Yes Please!" {
				closeModal()
				onConfirm(force)
			}
			return nil
		}
		return event
	})

	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			if input.GetText() == "Yes Please!" {
				closeModal()
				onConfirm(force)
			} else {
				app.SetFlashText("[red]Confirmation mismatch. Action cancelled.")
				closeModal()
			}
		} else if key == tcell.KeyEsc {
			closeModal()
		}
	})

	input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyDown {
			moveFocus(1)
			return nil
		}
		if event.Key() == tcell.KeyUp {
			moveFocus(-1)
			return nil
		}
		return event
	})

	pages.AddPage("confirm", modal, true, true)
	// Start with checkbox focused
	currentFocus = 0
	updateCheckbox(true)
	tviewApp.SetFocus(checkbox)
}
