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

type SecretAttachItem struct {
	ID         string
	SecretName string
	Target     string
	Selected   bool
}

func ShowSecretEditor(app common.AppController, subject string, items []SecretAttachItem, onConfirm func([]SecretAttachItem)) {
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

	sortSecretItems(items)

	selections := make([]bool, len(items))
	for i, item := range items {
		selections[i] = item.Selected
	}

	currentIndex := 0
	placeholderStyle := tcell.StyleDefault.
		Foreground(styles.ColorDim).
		Background(styles.ColorBlack)

	secretInput := tview.NewInputField().
		SetFieldBackgroundColor(styles.ColorBlack).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabel(" Secret: ").
		SetLabelColor(styles.ColorWhite).
		SetPlaceholder("secret_name").
		SetPlaceholderStyle(placeholderStyle)
	secretInput.SetBackgroundColor(styles.ColorBlack)

	targetInput := tview.NewInputField().
		SetFieldBackgroundColor(styles.ColorBlack).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabel(" Target: ").
		SetLabelColor(styles.ColorWhite).
		SetPlaceholder("/run/secrets/secret_file").
		SetPlaceholderStyle(placeholderStyle)
	targetInput.SetBackgroundColor(styles.ColorBlack)

	addButton := tview.NewButton("  Add  ")
	addButton.SetStyle(tcell.StyleDefault.Foreground(styles.ColorFg).Background(styles.ColorBlack)).
		SetActivatedStyle(tcell.StyleDefault.Foreground(styles.ColorBlack).Background(styles.ColorMenuKey))
	addButton.SetBackgroundColor(styles.ColorBlack)

	addFormRow := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(secretInput, 0, 1, true).
		AddItem(targetInput, 0, 1, false).
		AddItem(tview.NewBox().SetBackgroundColor(styles.ColorBlack), 2, 0, false).
		AddItem(addButton, 10, 0, false)

	subjectView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf("[%s::b]%s[-::-]", styles.TagPink, subject))
	subjectView.SetBackgroundColor(styles.ColorBlack)

	list := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	list.SetBackgroundColor(styles.ColorBlack)

	updateList := func() {
		if len(items) == 0 {
			list.SetText(fmt.Sprintf("[%s]  No secrets available[-]", styles.TagDim))
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

			display := fmt.Sprintf("%s -> %s", item.SecretName, item.Target)
			if len(display) > 60 {
				display = display[:57] + "..."
			}

			content += fmt.Sprintf("%s%s %s[-]\n", color, checkbox, display)
		}
		list.SetText(content)
	}
	updateList()

	confirmBtn := tview.NewButton("Confirm")
	confirmBtn.SetStyle(tcell.StyleDefault.Foreground(styles.ColorFg).Background(styles.ColorBlack)).
		SetActivatedStyle(tcell.StyleDefault.Foreground(styles.ColorBlack).Background(styles.ColorMenuKey))
	confirmBtn.SetBackgroundColor(styles.ColorBlack)

	confirmRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(tview.NewBox().SetBackgroundColor(styles.ColorBlack), 0, 1, false).
		AddItem(confirmBtn, 11, 0, false).
		AddItem(tview.NewBox().SetBackgroundColor(styles.ColorBlack), 0, 1, false)
	confirmRow.SetBackgroundColor(styles.ColorBlack)

	separator := tview.NewTextView().
		SetDynamicColors(true).
		SetText("[" + styles.TagDim + "]" + strings.Repeat("─", 66) + "[-]").
		SetTextAlign(tview.AlignCenter)
	separator.SetBackgroundColor(styles.ColorBlack)

	content := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewBox().SetBackgroundColor(styles.ColorBlack), 1, 0, false).
		AddItem(subjectView, 1, 0, false).
		AddItem(tview.NewBox().SetBackgroundColor(styles.ColorBlack), 1, 0, false).
		AddItem(addFormRow, 1, 0, true).
		AddItem(separator, 1, 0, false).
		AddItem(list, 0, 1, false).
		AddItem(confirmRow, 1, 0, false).
		AddItem(tview.NewBox().SetBackgroundColor(styles.ColorBlack), 1, 0, false)

	content.SetBorder(true).
		SetTitle(fmt.Sprintf("[%s::b]<Attach Secrets>[-::-]", styles.TagCyan)).
		SetTitleColor(styles.ColorTitle).
		SetBorderColor(styles.ColorMenuKey).
		SetBackgroundColor(styles.ColorBlack)

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(content, dialogHeight, 1, true).
			AddItem(nil, 0, 1, false), dialogWidth, 1, true).
		AddItem(nil, 0, 1, false)

	closeModal := func() {
		pages.RemovePage("secret_editor")
		tviewApp.SetFocus(pages)
		app.UpdateShortcuts()
	}

	collectResults := func() {
		var selected []SecretAttachItem
		for i, item := range items {
			if selections[i] {
				selected = append(selected, item)
			}
		}
		closeModal()
		onConfirm(selected)
	}

	addSecret := func() {
		secretName := strings.TrimSpace(secretInput.GetText())
		target := strings.TrimSpace(targetInput.GetText())
		if secretName == "" {
			return
		}
		if target == "" {
			target = secretName
		}

		for i, item := range items {
			if item.SecretName == secretName || item.ID == secretName {
				items[i].Target = target
				selections[i] = true
				sortSecretItemsWithSelections(items, selections)
				secretInput.SetText("")
				targetInput.SetText("")
				updateList()
				tviewApp.SetFocus(secretInput)
				return
			}
		}
	}

	focusTarget := 0
	setFocus := func(target int) {
		focusTarget = target
		switch target {
		case 0:
			tviewApp.SetFocus(secretInput)
		case 1:
			tviewApp.SetFocus(targetInput)
		case 2:
			tviewApp.SetFocus(addButton)
		case 3:
			tviewApp.SetFocus(list)
		case 4:
			tviewApp.SetFocus(confirmBtn)
		}
	}

	addOrConfirm := func() {
		if strings.TrimSpace(secretInput.GetText()) == "" {
			collectResults()
			return
		}
		addSecret()
	}

	secretInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			collectResults()
			return nil
		}
		if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyDown {
			setFocus(1)
			return nil
		}
		if event.Key() == tcell.KeyBacktab {
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

	targetInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			collectResults()
			return nil
		}
		if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyDown {
			setFocus(2)
			return nil
		}
		if event.Key() == tcell.KeyBacktab {
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
		if event.Key() == tcell.KeyBacktab {
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
		case tcell.KeyEnter, tcell.KeyTab:
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
		case tcell.KeyEsc, tcell.KeyEnter:
			collectResults()
			return nil
		case tcell.KeyTab:
			setFocus(0)
			return nil
		case tcell.KeyBacktab:
			setFocus(3)
			return nil
		}
		return event
	})

	pages.AddPage("secret_editor", modal, true, true)
	setFocus(0)
	_ = focusTarget
	app.UpdateShortcuts()
}

func sortSecretItems(items []SecretAttachItem) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].SecretName < items[j].SecretName
	})
}

func sortSecretItemsWithSelections(items []SecretAttachItem, selections []bool) {
	type indexedItem struct {
		item     SecretAttachItem
		selected bool
	}

	indexed := make([]indexedItem, len(items))
	for i := range items {
		indexed[i] = indexedItem{item: items[i], selected: selections[i]}
	}

	sort.Slice(indexed, func(i, j int) bool {
		return indexed[i].item.SecretName < indexed[j].item.SecretName
	})

	for i := range indexed {
		items[i] = indexed[i].item
		selections[i] = indexed[i].selected
	}
}
