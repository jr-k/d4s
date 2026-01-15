package inspect

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

// TextInspector implements Inspector for static text (JSON, YAML, Env, etc)
type TextInspector struct {
	App      common.AppController
	TextView *tview.TextView
	Action   string
	Subject  string
	Content  string
	Lang     string
	filter   string
	
	searchMatches []string
	currentMatch  int
}

// Ensure TextInspector implements common.Inspector
var _ common.Inspector = (*TextInspector)(nil)

func NewTextInspector(action, subject string, content string, lang string) *TextInspector {
	return &TextInspector{
		Action:  action,
		Subject: subject,
		Content: content,
		Lang:    lang,
	}
}

func (i *TextInspector) GetID() string {
	return "inspect"
}

func (i *TextInspector) GetPrimitive() tview.Primitive {
	return i.TextView
}

func (i *TextInspector) GetTitle() string {
	count := len(i.searchMatches)
	return FormatInspectorTitle(i.Action, i.Subject, i.Lang, i.filter, i.currentMatch, count)
}

func (i *TextInspector) GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("Esc", "Close"),
		common.FormatSCHeader("c", "Copy"),
	}
}

func (i *TextInspector) OnMount(app common.AppController) {
	i.App = app
	
	i.TextView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false)
	
	i.updateContent()
	
	i.TextView.SetBorder(true).
		SetTitle(i.GetTitle()).
		SetTitleColor(styles.ColorTitle)
		
	i.TextView.SetBackgroundColor(styles.ColorBg)
}

func (i *TextInspector) OnUnmount() {
	// Cleanup if needed
}

func (i *TextInspector) ApplyFilter(filter string) {
	i.filter = filter
	i.searchMatches = []string{}
	i.currentMatch = 0
	i.updateContent()
}

func (i *TextInspector) updateContent() {
	// Optimization: Move heavy regex/formatting off the UI thread
	// to prevent UI freezing during typing of filter
	
	// Capture current state needed for calculation
	content := i.Content
	lang := i.Lang
	filter := i.filter

	go func() {
		// 1. Highlight Syntax
		coloredContent := i.highlightContent(content, lang)
		var matches []string

		// 2. Apply Filter Regex if needed
		finalText := coloredContent
		if filter != "" {
			pattern := fmt.Sprintf(`(\[[^\]]*\])|(%s)`, regexp.QuoteMeta(filter))
			re, err := regexp.Compile(pattern)
			
			if err == nil {
				matchCount := 0
				finalText = re.ReplaceAllStringFunc(coloredContent, func(s string) string {
					if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
						return s // It's a tag, return as is
					}
					
					// It's a match
					id := fmt.Sprintf("match_%d", matchCount)
					matches = append(matches, id)
					matchCount++
					
					// Highlight background 
					return fmt.Sprintf(`["%s"][black:yellow]%s[""]`, id, s)
				})
			}
		}

		// 3. Update UI
		i.App.GetTviewApp().QueueUpdateDraw(func() {
			// Check if filter changed while we were working? (Optional optimization, but strictly 
			// checking i.filter == filter here ensures we don't overwrite with stale data if user types fast)
			if i.filter != filter {
				return
			}
			
			i.searchMatches = matches
			i.TextView.SetRegions(true)
			i.TextView.SetText(finalText)
			
			if len(matches) > 0 {
				i.TextView.Highlight(matches[0])
				i.TextView.ScrollToHighlight()
			} else {
				i.TextView.Highlight() // Clear
			}
			i.TextView.SetTitle(i.GetTitle())
		})
	}()
}

func (i *TextInspector) InputHandler(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyEsc {
		i.App.CloseInspector()
		return nil
	}
	
	if event.Rune() == '/' {
		i.App.ActivateCmd("/")
		return nil
	}
	
	// Navigation
	if i.filter != "" && len(i.searchMatches) > 0 {
		if event.Rune() == 'n' {
			i.currentMatch++
			if i.currentMatch >= len(i.searchMatches) {
				i.currentMatch = 0 // Cycle
			}
			i.TextView.Highlight(i.searchMatches[i.currentMatch])
			i.TextView.ScrollToHighlight()
			i.TextView.SetTitle(i.GetTitle())
			return nil
		}
		
		if event.Rune() == 'p' {
			i.currentMatch--
			if i.currentMatch < 0 {
				i.currentMatch = len(i.searchMatches) - 1 // Cycle
			}
			i.TextView.Highlight(i.searchMatches[i.currentMatch])
			i.TextView.ScrollToHighlight()
			i.TextView.SetTitle(i.GetTitle())
			return nil
		}
	}
	
	if event.Rune() == 'c' {
		i.copyToClipboard()
		return nil
	}
	
	return event
}

func (i *TextInspector) copyToClipboard() {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip")
	default: // linux
		cmd = exec.Command("xclip", "-selection", "clipboard")
	}

	if cmd == nil {
		i.App.SetFlashText("[red]Clipboard not supported on this OS")
		return
	}

	cmd.Stdin = strings.NewReader(i.Content)
	
	if err := cmd.Run(); err != nil {
		i.App.SetFlashText(fmt.Sprintf("[red]Copy error: %v", err))
	} else {
		i.App.SetFlashText(fmt.Sprintf("[green]Copied %d bytes to clipboard!", len(i.Content)))
	}
}

func (i *TextInspector) highlightContent(content, lang string) string {
	if lang == "" || lang == "text" {
		return content
	}

	// Map generic languages to Chroma lexers if needed
	lexer := lang
	if lang == "env" {
		lexer = "bash"
	}

	var buf bytes.Buffer
	err := quick.Highlight(&buf, content, lexer, "terminal256", "dracula")
	if err != nil {
		// Fallback to simple color if error (e.g. unknown lexer)
		return content
	}

	// Convert ANSI to tview tags
	return tview.TranslateANSI(buf.String())
}
