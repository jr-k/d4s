package dialogs

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

type MultiPickerItem struct {
	ID       string
	Label    string
	Selected bool
}

func ShowMultiPicker(app common.AppController, title string, items []MultiPickerItem, onConfirm func(selected []string)) {
	if len(items) == 0 {
		app.SetFlashError("no items available")
		return
	}

	dialogWidth := 50
	dialogHeight := 6 + len(items)
	if dialogHeight > 20 {
		dialogHeight = 20
	}

	pages := app.GetPages()
	tviewApp := app.GetTviewApp()

	// Track selections
	selections := make([]bool, len(items))
	for i, item := range items {
		selections[i] = item.Selected
	}

	currentIndex := 0

	// Create list
	list := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	list.SetBackgroundColor(styles.ColorBlack)

	updateList := func() {
		var content string
		for i, item := range items {
			checkbox := "[ ]"
			if selections[i] {
				checkbox = "[" + "✔" + "]"
			}

			color := "[white]"
			if i == currentIndex {
				color = "[orange]"
				checkbox = "> " + checkbox
			} else {
				checkbox = "  " + checkbox
			}

			content += fmt.Sprintf("%s%s %s[-]\n", color, checkbox, item.Label)
		}
		list.SetText(content)
	}

	updateList()

	// Help text
	helpText := tview.NewTextView().
		SetDynamicColors(true).
		SetText("[gray]↑/↓ navigate • space toggle • enter confirm • esc cancel").
		SetTextAlign(tview.AlignCenter)
	helpText.SetBackgroundColor(styles.ColorBlack)

	// Layout
	content := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(helpText, 1, 0, false)

	content.SetBorder(true).
		SetTitle(" " + title + " ").
		SetTitleColor(styles.ColorTitle).
		SetBorderColor(styles.ColorTitle).
		SetBackgroundColor(styles.ColorBlack)

	// Center on screen
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(content, dialogHeight, 1, true).
			AddItem(nil, 0, 1, false), dialogWidth, 1, true).
		AddItem(nil, 0, 1, false)

	closeModal := func() {
		pages.RemovePage("picker")
		tviewApp.SetFocus(pages)
		app.UpdateShortcuts()
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			closeModal()
			return nil
		case tcell.KeyEnter:
			var selected []string
			for i, item := range items {
				if selections[i] {
					selected = append(selected, item.ID)
				}
			}
			closeModal()
			onConfirm(selected)
			return nil
		case tcell.KeyUp:
			if currentIndex > 0 {
				currentIndex--
				updateList()
			}
			return nil
		case tcell.KeyDown:
			if currentIndex < len(items)-1 {
				currentIndex++
				updateList()
			}
			return nil
		}

		if event.Rune() == ' ' {
			selections[currentIndex] = !selections[currentIndex]
			updateList()
			return nil
		}

		return event
	})

	pages.AddPage("picker", modal, true, true)
	tviewApp.SetFocus(list)
	app.UpdateShortcuts()
}
