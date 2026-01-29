package dialogs

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

type FieldType int

const (
	FieldTypeInput FieldType = iota
	FieldTypeCheckbox
)

type FormField struct {
	Name        string
	Label       string
	Type        FieldType
	Default     string
	Placeholder string
}

type FormResult map[string]string

func ShowForm(app common.AppController, title string, fields []FormField, onSubmit func(result FormResult)) {
	ShowFormWithDescription(app, title, "", fields, onSubmit)
}

func ShowFormWithDescription(app common.AppController, title, description string, fields []FormField, onSubmit func(result FormResult)) {
	if len(fields) == 0 {
		return
	}

	dialogWidth := 50
	dialogHeight := 4 + len(fields)*2

	pages := app.GetPages()
	tviewApp := app.GetTviewApp()

	// Store widgets and values
	type fieldWidget struct {
		field     FormField
		label     *tview.TextView
		widget    tview.Primitive
		getValue  func() string
		setFocus  func(focused bool)
		boolValue *bool
	}

	widgets := make([]fieldWidget, len(fields))

	// Helper for empty box
	empty := func() tview.Primitive {
		return tview.NewBox().SetBackgroundColor(styles.ColorBlack)
	}

	// Create form rows
	formRows := tview.NewFlex().SetDirection(tview.FlexRow)

	// Add top spacing
	formRows.AddItem(empty(), 1, 0, false)

	for i, f := range fields {
		// Label
		label := tview.NewTextView().
			SetDynamicColors(true).
			SetText(f.Label + ":").
			SetTextAlign(tview.AlignRight)
		label.SetBackgroundColor(styles.ColorBlack)

		fw := fieldWidget{
			field: f,
			label: label,
		}

		switch f.Type {
		case FieldTypeInput:
			input := tview.NewInputField().
				SetFieldBackgroundColor(styles.ColorSelectBg).
				SetFieldTextColor(tcell.ColorWhite).
				SetText(f.Default).
				SetPlaceholder(f.Placeholder)
			input.SetBackgroundColor(styles.ColorBlack)

			fw.widget = input
			fw.getValue = func() string {
				return input.GetText()
			}
			fw.setFocus = func(focused bool) {}

		case FieldTypeCheckbox:
			boolVal := f.Default == "true"
			fw.boolValue = &boolVal

			checkbox := tview.NewTextView().
				SetDynamicColors(true).
				SetTextAlign(tview.AlignLeft)
			checkbox.SetBackgroundColor(styles.ColorBlack)

			updateCheckbox := func(focused bool) {
				text := "No"
				if *fw.boolValue {
					text = "Yes"
				}
				color := "[white]"
				if focused {
					color = "[orange]"
					text = "> " + text
				}
				checkbox.SetText(fmt.Sprintf("%s%s", color, text))
			}
			updateCheckbox(false)

			fw.widget = checkbox
			fw.getValue = func() string {
				if *fw.boolValue {
					return "true"
				}
				return "false"
			}
			fw.setFocus = updateCheckbox
		}

		widgets[i] = fw

		// Create row with proper alignment
		row := tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(empty(), 1, 0, false).
			AddItem(label, 12, 0, false).
			AddItem(empty(), 1, 0, false).
			AddItem(fw.widget, 0, 1, false).
			AddItem(empty(), 1, 0, false)

		formRows.AddItem(row, 1, 0, false)
		// Add spacing between fields
		if i < len(fields)-1 {
			formRows.AddItem(empty(), 1, 0, false)
		}
	}

	// Main content
	content := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(formRows, 0, 1, true)

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

	currentFocus := 0

	closeModal := func() {
		pages.RemovePage("form")
		tviewApp.SetFocus(pages)
		app.UpdateShortcuts()
	}

	// Collect results
	getResults := func() FormResult {
		result := make(FormResult)
		for _, fw := range widgets {
			result[fw.field.Name] = fw.getValue()
		}
		return result
	}

	// Focus management
	setFocusTo := func(index int) {
		for i, fw := range widgets {
			fw.setFocus(i == index)
		}
		tviewApp.SetFocus(widgets[index].widget)
	}

	moveFocus := func(direction int) {
		currentFocus += direction
		if currentFocus < 0 {
			currentFocus = len(widgets) - 1
		} else if currentFocus >= len(widgets) {
			currentFocus = 0
		}
		setFocusTo(currentFocus)
	}

	// Setup input capture for each widget
	for i := range widgets {
		idx := i
		fw := &widgets[idx]

		switch fw.field.Type {
		case FieldTypeInput:
			input := fw.widget.(*tview.InputField)
			input.SetDoneFunc(func(key tcell.Key) {
				if key == tcell.KeyEnter {
					closeModal()
					onSubmit(getResults())
				} else if key == tcell.KeyEsc {
					closeModal()
				}
			})
			input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyDown || event.Key() == tcell.KeyTab {
					moveFocus(1)
					return nil
				}
				if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyBacktab {
					moveFocus(-1)
					return nil
				}
				return event
			})

		case FieldTypeCheckbox:
			checkbox := fw.widget.(*tview.TextView)
			checkbox.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyDown || event.Key() == tcell.KeyTab {
					moveFocus(1)
					return nil
				}
				if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyBacktab {
					moveFocus(-1)
					return nil
				}
				if event.Rune() == ' ' {
					*fw.boolValue = !*fw.boolValue
					fw.setFocus(true)
					return nil
				}
				if event.Key() == tcell.KeyEsc {
					closeModal()
					return nil
				}
				if event.Key() == tcell.KeyEnter {
					closeModal()
					onSubmit(getResults())
					return nil
				}
				return event
			})
		}
	}

	pages.AddPage("form", modal, true, true)
	setFocusTo(0)
	app.UpdateShortcuts()
}
