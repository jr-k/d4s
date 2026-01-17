package command

import (
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

var colorTagRegex = regexp.MustCompile(`\[[^\]]*\]`)

// stripColorTags removes tview color tags from a string
func stripColorTags(text string) string {
	return colorTagRegex.ReplaceAllString(text, "")
}

// AutocompleteInputField is a custom InputField that displays autocomplete suggestions
type AutocompleteInputField struct {
	*tview.InputField
	suggestionText string
	app            *tview.Application
}

func NewAutocompleteInputField() *AutocompleteInputField {
	return &AutocompleteInputField{
		InputField: tview.NewInputField(),
	}
}

func (a *AutocompleteInputField) SetSuggestion(text string) {
	if a.suggestionText != text {
		a.suggestionText = text
		// Removing explicit Draw() call as it causes freezes/deadlocks when called from the main thread
		// The tview event loop will handle the redraw automatically since the InputField content has changed
	}
}

func (a *AutocompleteInputField) SetApplication(app *tview.Application) {
	a.app = app
}

func (a *AutocompleteInputField) Draw(screen tcell.Screen) {
	// Draw the base InputField first
	a.InputField.Draw(screen)
	
	// Then draw the suggestion in gray if it exists and we have focus
	if a.suggestionText != "" && a.HasFocus() {
		x, y, width, _ := a.GetInnerRect()
		text := a.GetText()
		label := a.GetLabel()
		
		// Calculate the position where the suggestion should appear
		// We use simple length calculation for stability
		labelStr := stripColorTags(label)
		labelWidth := len(labelStr)
		textWidth := len(text)
		
		// Position after the text
		suggestionX := x + labelWidth + textWidth
		suggestionY := y
		
		// Only draw if we are within bounds and there is space
		if suggestionX < x+width {
			currentX := suggestionX
			style := tcell.StyleDefault.Foreground(styles.ColorDim).Background(styles.ColorBg)
			
			for _, r := range a.suggestionText {
				if currentX >= x+width {
					break
				}
				screen.SetContent(currentX, suggestionY, r, nil, style)
				currentX++
			}
		}
	}
}

type CommandComponent struct {
	View        *AutocompleteInputField
	App         common.AppController
	currentText string
}

func NewCommandComponent(app common.AppController) *CommandComponent {
	c := NewAutocompleteInputField()
	c.InputField.
		SetFieldBackgroundColor(styles.ColorBg).
		SetLabelColor(styles.ColorWhite).
		SetFieldTextColor(styles.ColorFg).
		SetLabel("[#ffb86c::b]VIEW> [-:-:-]")
	
	c.SetBorder(true).
		SetBorderColor(styles.ColorAccent).
		SetBackgroundColor(styles.ColorBg)
	
	c.SetApplication(app.GetTviewApp())
	
	comp := &CommandComponent{View: c, App: app}
	comp.setupHandlers()
	return comp
}

// List of available commands for autocompletion
var availableCommands = []string{
	"quit",
	"containers",
	"images",
	"volumes",
	"networks",
	"services",
	"nodes",
	"compose",
	"help",
	"aliases",
	"q",
	"c",
	"i",
	"v",
	"n",
	"s",
	"no",
	"p",
	"a",
}

// findBestSuggestion finds the best matching command for autocompletion
func findBestSuggestion(input string) string {
	if input == "" {
		return ""
	}
	
	input = strings.ToLower(input)
	bestMatch := ""
	
	for _, cmd := range availableCommands {
		if strings.HasPrefix(cmd, input) && len(cmd) > len(input) {
			if bestMatch == "" || len(cmd) < len(bestMatch) {
				bestMatch = cmd
			}
		}
	}
	
	// Robust check
	if bestMatch != "" && len(input) < len(bestMatch) {
		return bestMatch[len(input):] // Return only the suffix
	}
	
	return ""
}

func (c *CommandComponent) updateSuggestion() {
	text := c.View.GetText()
	c.currentText = text
	
	// Only show suggestion in CMD mode (starts with :)
	if strings.HasPrefix(text, ":") {
		cmd := strings.TrimPrefix(text, ":")
		suggestion := findBestSuggestion(cmd)
		c.View.SetSuggestion(suggestion)
	} else {
		c.View.SetSuggestion("")
	}
}

func (c *CommandComponent) acceptSuggestion() {
	text := c.View.GetText()
	if strings.HasPrefix(text, ":") {
		cmd := strings.TrimPrefix(text, ":")
		suggestion := findBestSuggestion(cmd)
		if suggestion != "" {
			c.View.SetText(":" + cmd + suggestion)
			// c.updateSuggestion() // Redundant: SetText triggers SetChangedFunc
		}
	}
}

func (c *CommandComponent) setupHandlers() {
	c.View.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			c.Reset()
			c.App.SetActiveFilter("")
			
			c.App.RefreshCurrentView()
			c.App.SetFlashText("")
			
			// Restore focus and hide cmdline
			c.App.SetCmdLineVisible(false)
			c.App.RestoreFocus()
			return nil
		}
		
		// Handle Tab to accept suggestion
		if event.Key() == tcell.KeyTab {
			c.acceptSuggestion()
			return nil
		}
		
		return event
	})
	
	c.View.SetChangedFunc(func(text string) {
		c.updateSuggestion()
	})

	c.View.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			cmd := c.View.GetText()
			if strings.HasPrefix(cmd, "/") {
				// Search Mode
				filter := ""
				if len(cmd) > 1 {
					filter = strings.TrimPrefix(cmd, "/")
				}
				
				// Apply Filter (even if empty, to clear it)
				c.App.SetActiveFilter(filter)
				
				// Flash Message Context
				// msg := ""
				// if filter != "" {
				// 	msg = fmt.Sprintf("Filter: %s", filter)
				// 	front, _ := c.App.GetPages().GetFrontPage()
				// 	if front == "inspect" {
				// 		msg = fmt.Sprintf("Search: %s", filter)
				// 	}
				// }
				// c.App.SetFlashText(msg)

			} else {
				// Command Mode
				c.App.ExecuteCmd(cmd)
			}
			
			c.Reset()
			
			// Restore focus and hide cmdline
			c.App.SetCmdLineVisible(false)
			c.App.RestoreFocus()
			c.App.RefreshCurrentView()
		}
	})
}

func (c *CommandComponent) Activate(initial string) {
	label := "[#ffb86c::b]CMD> [-:-:-]" // Defaults to Command
	
	if strings.HasPrefix(initial, "/") {
		label = "[#ffb86c::b]FILTER> [-:-:-]"
		
		// Check if we are in Inspector -> SEARCH context
		front, _ := c.App.GetPages().GetFrontPage()
		if front == "inspect" {
			label = "[#ffb86c::b]SEARCH> [-:-:-]"
		}
	}
	
	c.View.SetLabel(label)
	c.View.SetText(initial)
	c.App.GetTviewApp().SetFocus(c.View)
}

func (c *CommandComponent) Reset() {
	c.View.SetText("")
	c.View.SetLabel("[#ffb86c::b]VIEW> [-:-:-]")
	c.View.SetSuggestion("")
	c.currentText = ""
}

func (c *CommandComponent) HasFocus() bool {
	return c.View.HasFocus()
}

func (c *CommandComponent) SetFilter(filter string) {
	c.View.SetLabel("[#ffb86c::b]FILTER> [-:-:-]")
	c.View.SetText(filter)
	c.View.SetSuggestion("")
}