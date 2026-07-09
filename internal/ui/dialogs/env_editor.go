package dialogs

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

type EnvItem struct {
	Key      string
	Value    string
	Selected bool
}

func ShowEnvEditor(app common.AppController, subject string, items []EnvItem, onConfirm func(envVars []string)) {
	dialogWidth := 70
	dialogHeight := 12 + len(items)
	if dialogHeight > 30 {
		dialogHeight = 30
	}
	if dialogHeight < 14 {
		dialogHeight = 14
	}

	pages := app.GetPages()
	tviewApp := app.GetTviewApp()

	// Sort items by key
	sort.Slice(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})

	// Track selections
	selections := make([]bool, len(items))
	for i, item := range items {
		selections[i] = item.Selected
	}

	currentIndex := 0

	// --- Add new env form ---
	placeholderStyle := tcell.StyleDefault.
		Foreground(styles.ColorDim).
		Background(styles.ColorBlack)

	nameInput := tview.NewInputField().
		SetFieldBackgroundColor(styles.ColorBlack).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabel(" Name: ").
		SetLabelColor(styles.ColorWhite).
		SetPlaceholder("MY_VAR").
		SetPlaceholderStyle(placeholderStyle)
	nameInput.SetBackgroundColor(styles.ColorBlack)

	valueInput := tview.NewInputField().
		SetFieldBackgroundColor(styles.ColorBlack).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabel(" Value: ").
		SetLabelColor(styles.ColorWhite).
		SetPlaceholder("my_value").
		SetPlaceholderStyle(placeholderStyle)
	valueInput.SetBackgroundColor(styles.ColorBlack)

	addButton := tview.NewButton("  Add  ")
	addButton.SetStyle(tcell.StyleDefault.Foreground(styles.ColorFg).Background(styles.ColorBlack)).
		SetActivatedStyle(tcell.StyleDefault.Foreground(styles.ColorBlack).Background(styles.ColorMenuKey))
	addButton.SetBackgroundColor(styles.ColorBlack)

	spacer := tview.NewBox().SetBackgroundColor(styles.ColorBlack)

	addFormRow := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(nameInput, 0, 1, true).
		AddItem(valueInput, 0, 1, false).
		AddItem(spacer, 2, 0, false).
		AddItem(addButton, 10, 0, false)

	// Subject header
	subjectView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf("[%s::b]%s[-::-]", styles.TagPink, subject))
	subjectView.SetBackgroundColor(styles.ColorBlack)

	// --- Env list with checkboxes ---
	list := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	list.SetBackgroundColor(styles.ColorBlack)

	updateList := func() {
		if len(items) == 0 {
			list.SetText(fmt.Sprintf("[%s]  No environment variables[-]", styles.TagDim))
			return
		}
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

			display := fmt.Sprintf("%s=%s", item.Key, item.Value)
			if len(display) > 60 {
				display = display[:57] + "..."
			}

			content += fmt.Sprintf("%s%s %s[-]\n", color, checkbox, display)
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

	confirmRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(tview.NewBox().SetBackgroundColor(styles.ColorBlack), 0, 1, false).
		AddItem(confirmBtn, 11, 0, false).
		AddItem(tview.NewBox().SetBackgroundColor(styles.ColorBlack), 0, 1, false)
	confirmRow.SetBackgroundColor(styles.ColorBlack)

	// Separator
	separator := tview.NewTextView().
		SetDynamicColors(true).
		SetText("[" + styles.TagDim + "]" + strings.Repeat("─", 50) + "[-]").
		SetTextAlign(tview.AlignCenter)
	separator.SetBackgroundColor(styles.ColorBlack)

	// Layout
	spacer1 := tview.NewBox().SetBackgroundColor(styles.ColorBlack)
	spacer2 := tview.NewBox().SetBackgroundColor(styles.ColorBlack)
	bottomSpacer := tview.NewBox().SetBackgroundColor(styles.ColorBlack)
	content := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(spacer1, 1, 0, false).
		AddItem(subjectView, 1, 0, false).
		AddItem(spacer2, 1, 0, false).
		AddItem(addFormRow, 1, 0, true).
		AddItem(separator, 1, 0, false).
		AddItem(list, 0, 1, false).
		AddItem(confirmRow, 1, 0, false).
		AddItem(bottomSpacer, 1, 0, false)

	content.SetBorder(true).
		SetTitle(fmt.Sprintf("[%s::b]<Edit Env>[-::-]", styles.TagCyan)).
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
		pages.RemovePage("env_editor")
		tviewApp.SetFocus(pages)
		app.UpdateShortcuts()
	}

	collectResults := func() {
		var envVars []string
		for i, item := range items {
			if selections[i] {
				envVars = append(envVars, fmt.Sprintf("%s=%s", item.Key, item.Value))
			}
		}
		closeModal()
		onConfirm(envVars)
	}

	addEnvVar := func() {
		name := strings.TrimSpace(nameInput.GetText())
		value := valueInput.GetText()
		if name == "" {
			return
		}

		// Check if key already exists, if so update it
		found := false
		for i, item := range items {
			if item.Key == name {
				items[i].Value = value
				selections[i] = true
				found = true
				break
			}
		}

		if !found {
			items = append(items, EnvItem{Key: name, Value: value, Selected: true})
			selections = append(selections, true)
		}

		// Re-sort
		type indexedItem struct {
			item     EnvItem
			selected bool
		}
		indexed := make([]indexedItem, len(items))
		for i := range items {
			indexed[i] = indexedItem{items[i], selections[i]}
		}
		sort.Slice(indexed, func(i, j int) bool {
			return indexed[i].item.Key < indexed[j].item.Key
		})
		for i := range indexed {
			items[i] = indexed[i].item
			selections[i] = indexed[i].selected
		}

		nameInput.SetText("")
		valueInput.SetText("")
		updateList()

		// Recalculate dialog height
		newHeight := 10 + len(items)
		if newHeight > 28 {
			newHeight = 28
		}
		if newHeight < 14 {
			newHeight = 14
		}
		_ = newHeight

		tviewApp.SetFocus(nameInput)
	}

	// Focus management: 0=nameInput, 1=valueInput, 2=addButton, 3=list, 4=confirmBtn
	focusTarget := 0

	setFocus := func(target int) {
		focusTarget = target
		switch target {
		case 0:
			tviewApp.SetFocus(nameInput)
		case 1:
			tviewApp.SetFocus(valueInput)
		case 2:
			tviewApp.SetFocus(addButton)
		case 3:
			tviewApp.SetFocus(list)
		case 4:
			tviewApp.SetFocus(confirmBtn)
		}
	}

	addOrConfirm := func() {
		name := strings.TrimSpace(nameInput.GetText())
		if name == "" {
			collectResults()
		} else {
			addEnvVar()
		}
	}

	// Input handlers
	nameInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			collectResults()
			return nil
		}
		if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyDown {
			setFocus(1)
			return nil
		}
		if event.Key() == tcell.KeyBacktab || event.Key() == tcell.KeyUp {
			if len(items) > 0 {
				setFocus(3)
			}
			return nil
		}
		if event.Key() == tcell.KeyEnter {
			addOrConfirm()
			return nil
		}
		return event
	})

	valueInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			collectResults()
			return nil
		}
		if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyDown {
			setFocus(2)
			return nil
		}
		if event.Key() == tcell.KeyBacktab || event.Key() == tcell.KeyUp {
			setFocus(0)
			return nil
		}
		if event.Key() == tcell.KeyEnter {
			addOrConfirm()
			return nil
		}
		return event
	})

	addButton.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			collectResults()
			return nil
		}
		if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyDown {
			if len(items) > 0 {
				setFocus(3)
			} else {
				setFocus(0)
			}
			return nil
		}
		if event.Key() == tcell.KeyBacktab || event.Key() == tcell.KeyUp {
			setFocus(1)
			return nil
		}
		if event.Key() == tcell.KeyEnter {
			addOrConfirm()
			return nil
		}
		return event
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			collectResults()
			return nil
		case tcell.KeyEnter:
			setFocus(4)
			return nil
		case tcell.KeyTab:
			setFocus(4)
			return nil
		case tcell.KeyBacktab:
			setFocus(2)
			return nil
		case tcell.KeyUp:
			if currentIndex > 0 {
				currentIndex--
				updateList()
			} else {
				setFocus(2)
			}
			return nil
		case tcell.KeyDown:
			if currentIndex < len(items)-1 {
				currentIndex++
				updateList()
			} else {
				setFocus(4)
			}
			return nil
		}

		if event.Rune() == ' ' {
			if len(items) > 0 {
				selections[currentIndex] = !selections[currentIndex]
				updateList()
			}
			return nil
		}

		return event
	})

	confirmBtn.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			collectResults()
			return nil
		case tcell.KeyEnter:
			collectResults()
			return nil
		case tcell.KeyTab:
			setFocus(0)
			return nil
		case tcell.KeyBacktab:
			setFocus(3)
			return nil
		case tcell.KeyUp:
			setFocus(3)
			return nil
		}
		return event
	})

	pages.AddPage("env_editor", modal, true, true)
	setFocus(0)
	_ = focusTarget
	app.UpdateShortcuts()
}
