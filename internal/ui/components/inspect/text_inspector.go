package inspect

import (
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

// TextInspector implements Inspector for static text (JSON, YAML, Env, etc)
type TextInspector struct {
	App    common.AppController
	Viewer *TextViewer
	
	Action  string
	Subject string
	Content string
	Lang    string
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
	return i.Viewer.GetPrimitive()
}

func (i *TextInspector) GetTitle() string {
	filter, idx, count := i.Viewer.GetSearchInfo()
	return FormatInspectorTitle(i.Action, i.Subject, i.Lang, filter, idx, count)
}

func (i *TextInspector) GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("esc", "Close"),
		common.FormatSCHeader("c", "Copy"),
		common.FormatSCHeader("/", "Search"),
		common.FormatSCHeader("n/p", "Next/Prev"),
	}
}

func (i *TextInspector) OnMount(app common.AppController) {
	i.App = app
	i.Viewer = NewTextViewer(app)
	i.Viewer.Update(i.Content, i.Lang)
	
	tv := i.Viewer.View
	tv.SetBorder(true).
		SetTitle(i.GetTitle()).
		SetTitleColor(styles.ColorTitle)
	
	// Hook up title updates
	i.Viewer.TitleUpdateFunc = func() {
		tv.SetTitle(i.GetTitle())
	}
}

func (i *TextInspector) OnUnmount() {
	// Cleanup if needed
}

func (i *TextInspector) ApplyFilter(filter string) {
	i.Viewer.ApplyFilter(filter)
}

func (i *TextInspector) InputHandler(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyEsc {
		i.App.CloseInspector()
		return nil
	}
	
	if i.Viewer.InputHandler(event) {
		return nil
	}
	
	return event
}
