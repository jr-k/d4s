package dialogs

import (
	"fmt"
	"sort"

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

func ShowMultiPicker(app common.AppController, title string, subject string, items []MultiPickerItem, onConfirm func(selected []string)) {
	if len(items) == 0 {
		app.SetFlashError("no items available")
		return
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Label < items[j].Label
	})

	dialogWidth := 50
	dialogHeight := 9 + len(items)
	if dialogHeight > 23 {
		dialogHeight = 23
	}

	pages := app.GetPages()
	tviewApp := app.GetTviewApp()

	// Track selections
	selections := make([]bool, len(items))
	for i, item := range items {
		selections[i] = item.Selected
	}

	currentIndex := 0

	// Subject header
	subjectView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf("[%s::b]%s[-::-]", styles.TagPink, subject))
	subjectView.SetBackgroundColor(styles.ColorBlack)

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

			color := fmt.Sprintf("[%s]", styles.TagFg)
			if i == currentIndex {
				color = fmt.Sprintf("[%s]", styles.TagAccent)
				checkbox = "> " + checkbox
			} else {
				checkbox = "  " + checkbox
			}

			content += fmt.Sprintf("%s%s %s[-]\n", color, checkbox, item.Label)
		}
		list.SetText(content)
		list.ScrollTo(currentIndex, 0)
	}

	updateList()

	// Confirm button
	confirmBtn := tview.NewButton("Confirm")
	confirmBtn.SetStyle(tcell.StyleDefault.Foreground(styles.ColorFg).Background(styles.ColorBlack)).
		SetActivatedStyle(tcell.StyleDefault.Foreground(styles.ColorBlack).Background(styles.ColorMenuKey))
	confirmBtn.SetBackgroundColor(styles.ColorBlack)

	emptyBox := func() *tview.Box { return tview.NewBox().SetBackgroundColor(styles.ColorBlack) }
	btnRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(emptyBox(), 0, 1, false).
		AddItem(confirmBtn, 11, 0, false).
		AddItem(emptyBox(), 0, 1, false)
	btnRow.SetBackgroundColor(styles.ColorBlack)

	// Layout
	bottomSpacer := tview.NewBox().SetBackgroundColor(styles.ColorBlack)
	spacer1 := tview.NewBox().SetBackgroundColor(styles.ColorBlack)
	spacer2 := tview.NewBox().SetBackgroundColor(styles.ColorBlack)
	content := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(spacer1, 1, 0, false).
		AddItem(subjectView, 1, 0, false).
		AddItem(spacer2, 1, 0, false).
		AddItem(list, 0, 1, true).
		AddItem(btnRow, 1, 0, false).
		AddItem(bottomSpacer, 1, 0, false)

	content.SetBorder(true).
		SetTitle(fmt.Sprintf("[%s::b]<%s>[-::-]", styles.TagCyan, title)).
		SetTitleColor(styles.ColorTitle).
		SetBorderColor(styles.ColorMenuKey).
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

	submit := func() {
		var selected []string
		for i, item := range items {
			if selections[i] {
				selected = append(selected, item.ID)
			}
		}
		closeModal()
		onConfirm(selected)
	}

	focusOnBtn := false

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			closeModal()
			return nil
		case tcell.KeyEnter:
			submit()
			return nil
		case tcell.KeyTab:
			focusOnBtn = true
			tviewApp.SetFocus(confirmBtn)
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
			tviewApp.SetFocus(list)
			return nil
		}
		return event
	})

	_ = focusOnBtn

	pages.AddPage("picker", modal, true, true)
	tviewApp.SetFocus(list)
	app.UpdateShortcuts()
}
