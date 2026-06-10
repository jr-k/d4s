package dialogs

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

type PickerItem struct {
	Description string
	Label    string
	Value    string
	Shortcut rune
}

func ShowPickerLoading(app common.AppController, title string) {
	displayTitle := title
	if parts := strings.SplitN(title, ": ", 2); len(parts) == 2 {
		displayTitle = parts[0]
	}

	pages := app.GetPages()
	dialogWidth := 60
	dialogHeight := 5

	loadingView := tview.NewTextView().
		SetText(fmt.Sprintf("[%s]Loading...[-]", styles.TagDim)).
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	loadingView.SetBackgroundColor(styles.ColorBlack)

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	content.SetBackgroundColor(styles.ColorBlack)
	content.AddItem(tview.NewBox().SetBackgroundColor(styles.ColorBlack), 1, 0, false)
	content.AddItem(loadingView, 1, 0, false)

	frame := tview.NewFrame(content).
		SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true).
		SetTitle(fmt.Sprintf("[%s::b]<%s>[-::-]", styles.TagCyan, displayTitle)).
		SetTitleColor(styles.ColorTitle).
		SetBorderColor(styles.ColorMenuKey).
		SetBackgroundColor(styles.ColorBlack)

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(frame, dialogHeight, 1, true).
			AddItem(nil, 0, 1, false), dialogWidth, 1, true).
		AddItem(nil, 0, 1, false)

	frame.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			pages.RemovePage("picker")
			app.GetTviewApp().SetFocus(pages)
			return nil
		}
		return event
	})

	pages.AddPage("picker", modal, true, true)
	app.GetTviewApp().SetFocus(frame)
}

func ShowPicker(app common.AppController, title string, items []PickerItem, onSelect func(value string), onClose ...func()) {
	displayTitle := title
	subject := ""
	if parts := strings.SplitN(title, ": ", 2); len(parts) == 2 {
		displayTitle = parts[0]
		subject = parts[1]
	}

	dialogWidth := 60
	dialogHeight := len(items)*2 + 4
	if subject != "" {
		dialogHeight += 3
	}

	pages := app.GetPages()
	tviewApp := app.GetTviewApp()

	list := tview.NewList().
		SetMainTextColor(styles.ColorWhite).
		SetSelectedTextColor(styles.ColorWhite).
		SetSelectedBackgroundColor(styles.ColorSelectBg).
		SetHighlightFullLine(true)

	list.SetBackgroundColor(styles.ColorBlack)

	closeModal := func() {
		pages.RemovePage("picker")
		tviewApp.SetFocus(pages)
		for _, fn := range onClose {
			fn()
		}
	}

	shortcuts := assignShortcuts(items)

	for idx, item := range items {
		val := item.Value
		shortcut := shortcuts[idx]

		label := formatLabelWithShortcut(item.Label, shortcut)
		description := fmt.Sprintf("[%s](%s)", styles.TagDim, item.Description)

		list.AddItem(label, description, 0, func() {
			closeModal()
			onSelect(val)
		})
	}

	list.SetDoneFunc(func() {
		closeModal()
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			closeModal()
			return nil
		}
		if event.Key() == tcell.KeyRune {
			ch := unicode.ToLower(event.Rune())
			for idx, sc := range shortcuts {
				if sc != 0 && unicode.ToLower(sc) == ch {
					closeModal()
					onSelect(items[idx].Value)
					return nil
				}
			}
		}
		return event
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	content.SetBackgroundColor(styles.ColorBlack)

	if subject != "" {
		topSpacer := tview.NewBox()
		topSpacer.SetBackgroundColor(styles.ColorBlack)
		content.AddItem(topSpacer, 1, 0, false)

		subjectView := tview.NewTextView().
			SetText(fmt.Sprintf("[%s]%s[-]", styles.TagPink, subject)).
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter)
		subjectView.SetBackgroundColor(styles.ColorBlack)
		content.AddItem(subjectView, 1, 0, false)

		midSpacer := tview.NewBox()
		midSpacer.SetBackgroundColor(styles.ColorBlack)
		content.AddItem(midSpacer, 1, 0, false)
	}

	content.AddItem(list, 0, 1, true)

	frame := tview.NewFrame(content).
		SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true).
		SetTitle(fmt.Sprintf("[%s::b]<%s>[-::-]", styles.TagCyan, displayTitle)).
		SetTitleColor(styles.ColorTitle).
		SetBorderColor(styles.ColorMenuKey).
		SetBackgroundColor(styles.ColorBlack)

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(frame, dialogHeight, 1, true).
			AddItem(nil, 0, 1, false), dialogWidth, 1, true).
		AddItem(nil, 0, 1, false)

	// Remove loading picker if present, then add the real one
	pages.RemovePage("picker")
	pages.AddPage("picker", modal, true, true)
	tviewApp.SetFocus(list)
}

func assignShortcuts(items []PickerItem) []rune {
	used := make(map[rune]bool)
	result := make([]rune, len(items))

	for i, item := range items {
		if item.Shortcut != 0 {
			used[unicode.ToLower(item.Shortcut)] = true
			result[i] = item.Shortcut
			continue
		}

		found := false
		for _, ch := range item.Label {
			if !unicode.IsLetter(ch) {
				continue
			}
			lower := unicode.ToLower(ch)
			if !used[lower] {
				used[lower] = true
				result[i] = lower
				found = true
				break
			}
		}
		if !found {
			result[i] = 0
		}
	}

	return result
}

func formatLabelWithShortcut(label string, shortcut rune) string {
	if shortcut == 0 {
		return fmt.Sprintf("[%s] %s", styles.TagFg, label)
	}

	lower := unicode.ToLower(shortcut)
	idx := -1
	for i, ch := range label {
		if unicode.ToLower(ch) == lower {
			idx = i
			break
		}
	}

	if idx < 0 {
		return fmt.Sprintf("[%s] %s", styles.TagFg, label)
	}

	before := label[:idx]
	char := string([]rune(label)[runeIndex(label, idx)])
	after := label[idx+len(char):]

	return fmt.Sprintf("[%s] %s[%s::u]%s[-::-]%s", styles.TagFg, before, styles.TagInfo, char, after)
}

func runeIndex(s string, byteIdx int) int {
	return len([]rune(s[:byteIdx]))
}

