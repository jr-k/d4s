package view

import (
	"fmt"
	"io"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

type LogView struct {
	*tview.TextView
	App          common.AppController
	ResourceID   string
	ResourceType string
	AutoScroll   bool
	Wrap         bool
	Timestamps   bool
	cancelFunc   func()
}

func NewLogView(app common.AppController, id, resourceType string) *LogView {
	lv := &LogView{
		TextView:     tview.NewTextView(),
		App:          app,
		ResourceID:   id,
		ResourceType: resourceType,
		AutoScroll:   true,
		Wrap:         false,
		Timestamps:   false,
	}

	lv.SetDynamicColors(true)
	lv.SetScrollable(true)
	lv.SetChangedFunc(func() {
		if lv.AutoScroll {
			lv.ScrollToEnd()
		}
	})
	
	lv.SetBorder(true)
	lv.SetTitle(fmt.Sprintf(" Logs: %s ", id))
	lv.SetTitleColor(styles.ColorTitle)
	lv.SetBackgroundColor(styles.ColorBg)

	lv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			if lv.cancelFunc != nil {
				lv.cancelFunc()
			}
			// Accessing internal/ui methods is hard if not in interface.
			// AppController needs RemovePage
			app.GetPages().RemovePage("logs")
			// Restore footer shortcuts
			app.GetTviewApp().SetFocus(app.GetPages()) // Focus pages/table?
			app.UpdateShortcuts()
			return nil
		}
		
		switch event.Rune() {
		case 's':
			lv.AutoScroll = !lv.AutoScroll
			if lv.AutoScroll {
				lv.SetTitle(fmt.Sprintf(" Logs: %s (AutoScroll: ON) ", lv.ResourceID))
				lv.ScrollToEnd()
			} else {
				lv.SetTitle(fmt.Sprintf(" Logs: %s (AutoScroll: OFF) ", lv.ResourceID))
			}
		case 'w':
			lv.Wrap = !lv.Wrap
			lv.SetWordWrap(lv.Wrap)
		case 't':
			lv.Timestamps = !lv.Timestamps
			lv.startStreaming() // Restart stream
		case 'c':
			if event.Modifiers()&tcell.ModShift != 0 { // Shift+C = Clear
				lv.Clear()
			} else {
				// Copy (Not implemented fully in this snippet, needs clipboard logic)
			}
		}
		
		return event
	})

	lv.startStreaming()
	return lv
}

func (lv *LogView) startStreaming() {
	if lv.cancelFunc != nil {
		lv.cancelFunc()
	}

	// Create cancellable context? 
	// Or just a simple bool check in loop?
	// Using a channel to stop
	stop := make(chan struct{})
	lv.cancelFunc = func() {
		close(stop)
	}

	lv.Clear()
	lv.SetText("[yellow]Loading logs...\n")

	go func() {
		var reader io.ReadCloser
		var err error
		_ = reader
		_ = err
		return
	}()
}
