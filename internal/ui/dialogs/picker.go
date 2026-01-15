package dialogs

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/jessym/d4s/internal/ui/common"
	"github.com/jessym/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

type PickerItem struct {
	Description string
	Label    string
	Value    string
	Shortcut rune
}

func ShowPicker(app common.AppController, title string, items []PickerItem, onSelect func(value string)) {
	// Center the dialog
	dialogWidth := 50
	dialogHeight := len(items) + 4 // Title + borders + padding

	pages := app.GetPages()
	tviewApp := app.GetTviewApp()

	list := tview.NewList().
		SetMainTextColor(tcell.ColorWhite).
		SetSelectedTextColor(tcell.ColorWhite).
		SetSelectedBackgroundColor(styles.ColorSelectBg).
		SetHighlightFullLine(true)

	list.SetBackgroundColor(tcell.ColorBlack)

	// Helper to close
	closeModal := func() {
		pages.RemovePage("picker")
		tviewApp.SetFocus(pages)
	}

	for _, item := range items {
		// Capture variable for closure
		val := item.Value
		
		// Format label with shortcut
		// (a) Label
		// Use explicit color tags for shortcut
		label := fmt.Sprintf("[white] %s", item.Label)
		description := fmt.Sprintf("[#44475a](%s)", item.Description)
		
		list.AddItem(label, description, item.Shortcut, func() {
			closeModal()
			onSelect(val)
		})
	}
	
	list.SetDoneFunc(func() {
		closeModal()
	})
	
	// Handle Esc manually to be safe, though SetDoneFunc should handle it for List
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			closeModal()
			return nil
		}
		return event
	})

	// Wrap in frame
	frame := tview.NewFrame(list).
		SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true).
		SetTitle(fmt.Sprintf(" %s ", title)).
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

	pages.AddPage("picker", modal, true, true)
	tviewApp.SetFocus(list)
}

