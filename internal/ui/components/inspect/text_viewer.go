package inspect

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

// TextViewer encapsulates a TextView with syntax highlighting, search, and navigation.
// It is intended to be used by Inspectors that need to display text/json/yaml.
type TextViewer struct {
	View   *tview.TextView
	Search *SearchController
	App    common.AppController
	TitleUpdateFunc func() // Optional callback to update parent title on navigation

	// Content State
	content string
	lang    string

	// Scroll Persistence
	lastRow int
	lastCol int
	mu      sync.Mutex // Protects scroll state during async updates
}

func NewTextViewer(app common.AppController) *TextViewer {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false)
	tv.SetBackgroundColor(styles.ColorBg)

	return &TextViewer{
		View:   tv,
		Search: NewSearchController(),
		App:    app,
	}
}

// Update updates the content of the viewer.
// This triggers an asynchronous highlight and search process.
func (t *TextViewer) Update(content, lang string) {
	t.content = content
	t.lang = lang
	t.refresh()
}

// ApplyFilter updates the search filter and refreshes the view.
func (t *TextViewer) ApplyFilter(filter string) {
	t.Search.ApplyFilter(filter)
	t.refresh()
}

func (t *TextViewer) refresh() {
	// Snapshot state for async closure
	content := t.content
	lang := t.lang
	filter := t.Search.Filter

	go func() {
		// 1. Highlight Syntax
		colored := t.highlightContent(content, lang)

		// 2. Apply Search
		finalText, matches := t.Search.ProcessContent(colored, filter)

		// 3. Update UI
		t.App.GetTviewApp().QueueUpdateDraw(func() {
			// Staleness check
			if t.Search.Filter != filter {
				return
			}

			// Capture current scroll before overwriting text
			if t.View.GetText(false) != "" {
				r, c := t.View.GetScrollOffset()
				if r > 0 || c > 0 {
					t.mu.Lock()
					t.lastRow, t.lastCol = r, c
					t.mu.Unlock()
				}
			}

			t.Search.SearchMatches = matches
			t.View.SetRegions(true)
			t.View.SetText(finalText)

			// Scroll / Highlight logic
			if len(matches) > 0 {
				// Highlight the current match region (handling navigation automatically via SearchController)
				t.Search.highlightCurrent(t.View)
			} else {
				// No matches, clear highlight and restore scroll
				t.View.Highlight()
				t.mu.Lock()
				r, c := t.lastRow, t.lastCol
				t.mu.Unlock()
				t.View.ScrollTo(r, c)
			}
			
			// Notify parent to update title (e.g. counters changed)
			if t.TitleUpdateFunc != nil {
				t.TitleUpdateFunc()
			}
		})
	}()
}

// InputHandler handles common keys: n, p, c, /
// Returns true if handled
func (t *TextViewer) InputHandler(event *tcell.EventKey) bool {
	// Search Navigation
	if t.Search.Filter != "" && len(t.Search.SearchMatches) > 0 {
		if event.Rune() == 'n' {
			t.Search.NextMatch(t.View)
			if t.TitleUpdateFunc != nil { t.TitleUpdateFunc() }
			return true
		}
		if event.Rune() == 'p' {
			t.Search.PrevMatch(t.View)
			if t.TitleUpdateFunc != nil { t.TitleUpdateFunc() }
			return true
		}
	}

	if event.Rune() == 'c' {
		t.copyToClipboard()
		return true
	}

	if event.Rune() == '/' {
		t.App.ActivateCmd("/")
		return true
	}

	return false
}

func (t *TextViewer) highlightContent(content, lang string) string {
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
		return content
	}

	// Convert ANSI to tview tags
	return tview.TranslateANSI(buf.String())
}

func (t *TextViewer) copyToClipboard() {
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
		t.App.SetFlashText("[red]Clipboard not supported on this OS")
		return
	}

	cmd.Stdin = strings.NewReader(t.content)

	if err := cmd.Run(); err != nil {
		t.App.SetFlashText(fmt.Sprintf("[red]Copy error: %v", err))
	} else {
		t.App.SetFlashText(fmt.Sprintf("[green]Copied %d bytes to clipboard!", len(t.content)))
	}
}

// GetSearchInfo returns the current search state for title formatting
func (t *TextViewer) GetSearchInfo() (filter string, index int, count int) {
	return t.Search.Filter, t.Search.CurrentMatch, len(t.Search.SearchMatches)
}

func (t *TextViewer) GetPrimitive() tview.Primitive {
	return t.View
}
