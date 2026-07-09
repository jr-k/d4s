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

type MountAttachItem struct {
	ID        string
	Source    string
	MountType string
	Target    string
	Selected  bool
}

func ShowMountEditor(app common.AppController, subject string, items []MountAttachItem, availableVolumes map[string]bool, onConfirm func([]MountAttachItem)) {
	dialogWidth := 80
	dialogHeight := 10 + len(items)
	if dialogHeight > 30 {
		dialogHeight = 30
	}
	if dialogHeight < 14 {
		dialogHeight = 14
	}

	pages := app.GetPages()
	tviewApp := app.GetTviewApp()

	sortMountItems(items)

	selections := make([]bool, len(items))
	for i, item := range items {
		selections[i] = item.Selected
	}

	currentIndex := 0
	placeholderStyle := tcell.StyleDefault.
		Foreground(styles.ColorDim).
		Background(styles.ColorBlack)

	sourceInput := tview.NewInputField().
		SetFieldBackgroundColor(styles.ColorBlack).
		SetFieldTextColor(tcell.ColorWhite).
		SetPlaceholder("volume_name or /host/path").
		SetPlaceholderStyle(placeholderStyle)
	sourceInput.SetBackgroundColor(styles.ColorBlack)

	mountTypes := []string{"volume", "bind", "tmpfs", "npipe", "cluster"}
	selectedMountType := "volume"
	typeInput := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	typeInput.SetBackgroundColor(styles.ColorBlack)
	updateTypeInput := func(focused bool) {
		color := styles.TagFg
		if focused {
			color = styles.TagAccent
		}
		typeInput.SetText(fmt.Sprintf("[%s]%s[-]", color, selectedMountType))
	}
	updateTypeInput(false)

	targetInput := tview.NewInputField().
		SetFieldBackgroundColor(styles.ColorBlack).
		SetFieldTextColor(tcell.ColorWhite).
		SetPlaceholder("/path/in/container").
		SetPlaceholderStyle(placeholderStyle)
	targetInput.SetBackgroundColor(styles.ColorBlack)

	addButton := tview.NewButton("  Add  ")
	addButton.SetStyle(tcell.StyleDefault.Foreground(styles.ColorFg).Background(styles.ColorBlack)).
		SetActivatedStyle(tcell.StyleDefault.Foreground(styles.ColorBlack).Background(styles.ColorMenuKey))
	addButton.SetBackgroundColor(styles.ColorBlack)

	labelWidth := len("Source:") + 1
	fieldRow := func(label string, input tview.Primitive) *tview.Flex {
		labelView := tview.NewTextView().
			SetDynamicColors(true).
			SetText(label + ":").
			SetTextAlign(tview.AlignLeft)
		labelView.SetBackgroundColor(styles.ColorBlack)

		return tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(tview.NewBox().SetBackgroundColor(styles.ColorBlack), 1, 0, false).
			AddItem(labelView, labelWidth+1, 0, false).
			AddItem(input, 0, 1, false).
			AddItem(tview.NewBox().SetBackgroundColor(styles.ColorBlack), 1, 0, false)
	}

	sourceTargetRow := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(fieldRow("Source", sourceInput), 0, 1, true).
		AddItem(fieldRow("Target", targetInput), 0, 1, false).
		AddItem(addButton, 10, 0, false)

	addFormRow := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(fieldRow("Type", typeInput), 1, 0, false).
		AddItem(sourceTargetRow, 1, 0, true)

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
			list.SetText(fmt.Sprintf("[%s]  No mounts attached[-]", styles.TagDim))
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

			display := fmt.Sprintf("%s:%s -> %s", item.MountType, item.Source, item.Target)
			if len(display) > 70 {
				display = display[:67] + "..."
			}

			content += fmt.Sprintf("%s%s %s[-]\n", color, checkbox, display)
		}
		list.SetText(content)
		list.ScrollTo(currentIndex, 0)
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
		SetText("[" + styles.TagDim + "]" + strings.Repeat("─", 60) + "[-]").
		SetTextAlign(tview.AlignCenter)
	separator.SetBackgroundColor(styles.ColorBlack)

	content := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewBox().SetBackgroundColor(styles.ColorBlack), 1, 0, false).
		AddItem(subjectView, 1, 0, false).
		AddItem(tview.NewBox().SetBackgroundColor(styles.ColorBlack), 1, 0, false).
		AddItem(addFormRow, 2, 0, true).
		AddItem(separator, 1, 0, false).
		AddItem(list, 0, 1, false).
		AddItem(confirmRow, 1, 0, false).
		AddItem(tview.NewBox().SetBackgroundColor(styles.ColorBlack), 1, 0, false)

	content.SetBorder(true).
		SetTitle(fmt.Sprintf("[%s::b]<Edit Mounts>[-::-]", styles.TagCyan)).
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
		pages.RemovePage("mount_editor")
		tviewApp.SetFocus(pages)
		app.UpdateShortcuts()
	}

	collectResults := func() {
		var selected []MountAttachItem
		for i, item := range items {
			if selections[i] {
				selected = append(selected, item)
			}
		}
		closeModal()
		onConfirm(selected)
	}

	addMount := func() {
		source := strings.TrimSpace(sourceInput.GetText())
		mountType := strings.ToLower(strings.TrimSpace(selectedMountType))
		target := strings.TrimSpace(targetInput.GetText())
		if mountType == "" {
			mountType = "volume"
		}
		if !isSupportedMountType(mountType) {
			app.SetFlashError(fmt.Sprintf("unsupported mount type: %s", mountType))
			return
		}
		if source == "" && mountType != "tmpfs" {
			return
		}
		if mountType == "volume" && !availableVolumes[source] {
			app.SetFlashError(fmt.Sprintf("volume not found: %s", source))
			return
		}
		if target == "" && source != "" {
			target = "/" + source
		}
		if target == "" {
			return
		}

		id := mountItemKey(mountType, source, target)
		for i, item := range items {
			if item.ID == id || (item.Source == source && item.MountType == mountType) {
				items[i].Target = target
				items[i].ID = id
				selections[i] = true
				sortMountItemsWithSelections(items, selections)
				sourceInput.SetText("")
				selectedMountType = "volume"
				updateTypeInput(true)
				targetInput.SetText("")
				updateList()
				tviewApp.SetFocus(typeInput)
				return
			}
		}

		items = append(items, MountAttachItem{
			ID:        id,
			Source:    source,
			MountType: mountType,
			Target:    target,
			Selected:  true,
		})
		selections = append(selections, true)
		sortMountItemsWithSelections(items, selections)
		sourceInput.SetText("")
		selectedMountType = "volume"
		updateTypeInput(true)
		targetInput.SetText("")
		updateList()
		tviewApp.SetFocus(typeInput)
	}

	focusTarget := 0
	setFocus := func(target int) {
		focusTarget = target
		updateTypeInput(target == 0)
		switch target {
		case 0:
			tviewApp.SetFocus(typeInput)
		case 1:
			tviewApp.SetFocus(sourceInput)
		case 2:
			tviewApp.SetFocus(targetInput)
		case 3:
			tviewApp.SetFocus(addButton)
		case 4:
			tviewApp.SetFocus(list)
		case 5:
			tviewApp.SetFocus(confirmBtn)
		}
	}

	showTypePicker := func() {
		selectedIndex := 0
		for i, mountType := range mountTypes {
			if mountType == selectedMountType {
				selectedIndex = i
				break
			}
		}

		pickerList := tview.NewTextView().
			SetDynamicColors(true).
			SetScrollable(true)
		pickerList.SetBackgroundColor(styles.ColorBlack)

		updatePicker := func() {
			var content string
			for i, mountType := range mountTypes {
				prefix := "  "
				color := styles.TagFg
				if i == selectedIndex {
					prefix = "> "
					color = styles.TagAccent
				}
				content += fmt.Sprintf("[%s]%s%s[-]\n", color, prefix, mountType)
			}
			pickerList.SetText(content)
			pickerList.ScrollTo(selectedIndex, 0)
		}

		closePicker := func() {
			pages.RemovePage("mount_type_picker")
			tviewApp.SetFocus(typeInput)
			updateTypeInput(true)
			app.UpdateShortcuts()
		}

		selectType := func() {
			selectedMountType = mountTypes[selectedIndex]
			closePicker()
		}

		pickerList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			switch event.Key() {
			case tcell.KeyEsc:
				closePicker()
				return nil
			case tcell.KeyEnter:
				selectType()
				return nil
			case tcell.KeyUp:
				if selectedIndex > 0 {
					selectedIndex--
					updatePicker()
				}
				return nil
			case tcell.KeyDown:
				if selectedIndex < len(mountTypes)-1 {
					selectedIndex++
					updatePicker()
				}
				return nil
			}

			if event.Rune() == ' ' {
				selectType()
				return nil
			}

			return event
		})

		updatePicker()

		pickerContent := tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(tview.NewBox().SetBackgroundColor(styles.ColorBlack), 1, 0, false).
			AddItem(pickerList, 0, 1, true).
			AddItem(tview.NewBox().SetBackgroundColor(styles.ColorBlack), 1, 0, false)
		pickerContent.SetBorder(true).
			SetTitle(fmt.Sprintf("[%s::b]<Mount Type>[-::-]", styles.TagCyan)).
			SetTitleColor(styles.ColorTitle).
			SetBorderColor(styles.ColorMenuKey).
			SetBackgroundColor(styles.ColorBlack)

		pickerModal := tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(pickerContent, 8, 1, true).
				AddItem(nil, 0, 1, false), 30, 1, true).
			AddItem(nil, 0, 1, false)

		pages.AddPage("mount_type_picker", pickerModal, true, true)
		tviewApp.SetFocus(pickerList)
		app.UpdateShortcuts()
	}

	addOrConfirm := func() {
		if strings.TrimSpace(sourceInput.GetText()) == "" && strings.TrimSpace(targetInput.GetText()) == "" {
			collectResults()
			return
		}
		addMount()
	}

	typeInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			collectResults()
			return nil
		}
		if event.Key() == tcell.KeyTab {
			setFocus(1)
			return nil
		}
		if event.Key() == tcell.KeyDown {
			setFocus(1)
			return nil
		}
		if event.Key() == tcell.KeyBacktab {
			if len(items) > 0 {
				setFocus(4)
			}
			return nil
		}
		if event.Key() == tcell.KeyUp {
			if len(items) > 0 {
				setFocus(4)
			}
			return nil
		}
		if event.Key() == tcell.KeyEnter {
			showTypePicker()
			return nil
		}
		if event.Rune() == ' ' {
			showTypePicker()
			return nil
		}
		return event
	})

	sourceInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
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

	targetInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			collectResults()
			return nil
		}
		if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyDown {
			setFocus(3)
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

	addButton.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			collectResults()
			return nil
		}
		if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyDown {
			if len(items) > 0 {
				setFocus(4)
			} else {
				setFocus(0)
			}
			return nil
		}
		if event.Key() == tcell.KeyBacktab || event.Key() == tcell.KeyUp {
			setFocus(2)
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
			setFocus(5)
			return nil
		case tcell.KeyBacktab:
			setFocus(3)
			return nil
		case tcell.KeyUp:
			if currentIndex > 0 {
				currentIndex--
				updateList()
			} else {
				setFocus(3)
			}
			return nil
		case tcell.KeyDown:
			if currentIndex < len(items)-1 {
				currentIndex++
				updateList()
			} else {
				setFocus(5)
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
			setFocus(4)
			return nil
		case tcell.KeyUp:
			setFocus(4)
			return nil
		}
		return event
	})

	pages.AddPage("mount_editor", modal, true, true)
	setFocus(0)
	_ = focusTarget
	app.UpdateShortcuts()
}

func sortMountItems(items []MountAttachItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].MountType == items[j].MountType {
			return items[i].Source < items[j].Source
		}
		return items[i].MountType < items[j].MountType
	})
}

func sortMountItemsWithSelections(items []MountAttachItem, selections []bool) {
	type indexedItem struct {
		item     MountAttachItem
		selected bool
	}

	indexed := make([]indexedItem, len(items))
	for i := range items {
		indexed[i] = indexedItem{item: items[i], selected: selections[i]}
	}

	sort.Slice(indexed, func(i, j int) bool {
		if indexed[i].item.MountType == indexed[j].item.MountType {
			return indexed[i].item.Source < indexed[j].item.Source
		}
		return indexed[i].item.MountType < indexed[j].item.MountType
	})

	for i := range indexed {
		items[i] = indexed[i].item
		selections[i] = indexed[i].selected
	}
}

func mountItemKey(mountType, source, target string) string {
	return fmt.Sprintf("%s:%s:%s", mountType, source, target)
}

func isSupportedMountType(mountType string) bool {
	switch mountType {
	case "bind", "volume", "tmpfs", "npipe", "cluster":
		return true
	default:
		return false
	}
}
