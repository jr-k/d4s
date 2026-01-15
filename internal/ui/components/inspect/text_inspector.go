package inspect

import (
	"bytes"
	"fmt"
	"os/exec"
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
	Title    string
	Content  string
	Lang     string
}

// Ensure TextInspector implements common.Inspector
var _ common.Inspector = (*TextInspector)(nil)

func NewTextInspector(title string, content string, lang string) *TextInspector {
	return &TextInspector{
		Title:   title,
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
	return fmt.Sprintf(" Inspect %s (%s) ", i.Title, i.Lang)
}

func (i *TextInspector) GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("Esc", "Close"),
		common.FormatSCHeader("c", "Copy"),
	}
}

func (i *TextInspector) OnMount(app common.AppController) {
	i.App = app
	
	coloredContent := i.highlightContent(i.Content, i.Lang)
	
	i.TextView = tview.NewTextView().
		SetDynamicColors(true).
		SetText(coloredContent).
		SetScrollable(true)
	
	i.TextView.SetBorder(true).
		SetTitle(i.GetTitle()).
		SetTitleColor(styles.ColorTitle)
		
	i.TextView.SetBackgroundColor(styles.ColorBg)
}

func (i *TextInspector) OnUnmount() {
	// Cleanup if needed
}

func (i *TextInspector) InputHandler(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyEsc {
		// Taking advantage of knowing how App works, or App should handle closing
		// usually App calls CloseInspector, but here we might want to signal it.
		// For now, let the App InputHandler intercept Esc if it checks Inspector first.
		// Actually, standard pattern:
		i.App.CloseInspector()
		return nil
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
