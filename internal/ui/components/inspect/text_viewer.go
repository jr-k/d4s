package inspect

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

// reTviewColorTag matches tview dynamic color tags like [#ffa503] or [-]
var reTviewColorTag = regexp.MustCompile(`\[#[0-9a-fA-F]+\]|\[-\]|\[-:-\]|\[-:-:-\]`)

// TextViewer encapsulates a TextView with syntax highlighting, search, and navigation.
// Used by Inspectors for displaying text/json/yaml.
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

	// Skip Chroma if content already contains tview color tags (e.g. loading messages).
	// Chroma would treat them as literal text and mangle the tags.
	if reTviewColorTag.MatchString(content) {
		return content
	}

	// Map generic languages to Chroma lexers if needed
	lexer := lang
	if lang == "env" {
		lexer = "bash"
	}

	l := lexers.Get(lexer)
	if l == nil {
		l = lexers.Fallback
	}
	it, err := chroma.Coalesce(l).Tokenise(nil, content)
	if err != nil {
		return content
	}

	// Write tview color tags straight from the tokens. Each run of same-colored
	// text is escaped as a whole, so brackets in the content never form a tag.
	var out, run strings.Builder
	runTag := styles.TagFg
	flush := func() {
		if run.Len() == 0 {
			return
		}
		fmt.Fprintf(&out, "[%s]%s", runTag, tview.Escape(run.String()))
		run.Reset()
	}
	for token := it(); token != chroma.EOF; token = it() {
		// Whitespace has no visible color: keep it in the current run.
		if tag := syntaxTag(token.Type); tag != runTag && strings.TrimSpace(token.Value) != "" {
			flush()
			runTag = tag
		}
		run.WriteString(token.Value)
	}
	flush()
	out.WriteString("[-]")

	return out.String()
}

// syntaxTag maps a Chroma token type to the tview color tag of the active skin.
func syntaxTag(tt chroma.TokenType) string {
	switch {
	case tt.InCategory(chroma.Comment):
		return styles.TagSyntaxComment
	case tt == chroma.NameTag, tt == chroma.NameAttribute, tt == chroma.NameVariable:
		return styles.TagSyntaxKey
	case tt.InSubCategory(chroma.LiteralNumber):
		return styles.TagSyntaxNumber
	case tt.InCategory(chroma.Literal):
		return styles.TagSyntaxString
	case tt.InCategory(chroma.Keyword):
		return styles.TagSyntaxKeyword
	case tt.InCategory(chroma.Punctuation), tt.InCategory(chroma.Operator):
		return styles.TagSyntaxPunctuation
	}
	return styles.TagFg
}

func (t *TextViewer) copyToClipboard() {
	if err := clipboard.WriteAll(t.content); err != nil {
		t.App.AppendFlashError(fmt.Sprintf("%v", err))
	} else {
		t.App.AppendFlashSuccess(fmt.Sprintf("copied %d bytes", len(t.content)))
	}
}

// GetSearchInfo returns the current search state for title formatting
func (t *TextViewer) GetSearchInfo() (filter string, index int, count int) {
	return t.Search.Filter, t.Search.CurrentMatch, len(t.Search.SearchMatches)
}

func (t *TextViewer) GetPrimitive() tview.Primitive {
	return t.View
}
