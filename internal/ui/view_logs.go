package ui

import (
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type LogView struct {
	*tview.TextView
	App            *App
	ContainerID    string
	ResourceType   string
	
	// State
	AutoScroll     bool
	Wrap           bool
	ShowTimestamps bool
	border         bool
	
	// Stream control
	stopChan       chan struct{}
}

func NewLogView(app *App, containerID string, resourceType string) *LogView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetRegions(true).
		SetWordWrap(false) // Default no wrap
	
	tv.SetBackgroundColor(tcell.ColorBlack)
	tv.SetTextColor(tcell.ColorWhite)
	
	lv := &LogView{
		TextView:       tv,
		App:            app,
		ContainerID:    containerID,
		ResourceType:   resourceType,
		AutoScroll:     true,
		Wrap:           false,
		ShowTimestamps: false,
		border:         true,
		stopChan:       make(chan struct{}),
	}

	lv.SetChangedFunc(func() {
		app.TviewApp.Draw()
	})

	lv.updateTitle()
	lv.setupInput()
	lv.startStreaming()

	return lv
}

func (lv *LogView) updateTitle() {
	title := fmt.Sprintf(" Logs: %s ", lv.ContainerID)
	
	// Status indicators
	status := ""
	if lv.AutoScroll { status += "[green]Autoscroll:ON " } else { status += "[dim]Autoscroll:OFF " }
	if lv.Wrap { status += "[green]Wrap:ON " } else { status += "[dim]Wrap:OFF " }
	if lv.ShowTimestamps { status += "[green]Time:ON " } else { status += "[dim]Time:OFF " }

	lv.SetTitle(fmt.Sprintf("%s %s", title, status))
	lv.SetBorder(lv.border).SetTitleColor(ColorTitle).SetBorderColor(ColorTitle).SetBackgroundColor(tcell.ColorBlack)
}

func (lv *LogView) setupInput() {
	lv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			lv.Stop()
			lv.App.Pages.RemovePage("logs")
			// Restore focus and refresh footer
			if view, ok := lv.App.Views[TitleContainers]; ok {
				lv.App.TviewApp.SetFocus(view.Table)
			}
			lv.App.RefreshCurrentView()
			return nil
		}

		switch event.Rune() {
		case 's', 'S':
			lv.AutoScroll = !lv.AutoScroll
			if lv.AutoScroll {
				lv.ScrollToEnd()
			}
			lv.updateTitle()
			return nil
		case 'w', 'W':
			lv.Wrap = !lv.Wrap
			lv.SetWordWrap(lv.Wrap)
			lv.updateTitle()
			return nil
		case 't', 'T':
			lv.ShowTimestamps = !lv.ShowTimestamps
			lv.restartStreaming()
			lv.updateTitle()
			return nil
		case 'c':
			// Copy buffer (simple implementation)
			// TODO: Implement copy logic
			lv.App.Flash.SetText("[yellow]Copying logs...")
			return nil
		case 'C': // Shift+C
			lv.Clear()
			return nil
		}
		
		return event
	})
}

func (lv *LogView) Stop() {
	select {
	case <-lv.stopChan:
	default:
		close(lv.stopChan)
	}
}

func (lv *LogView) restartStreaming() {
	lv.Stop()
	lv.Clear()
	lv.stopChan = make(chan struct{})
	// Small delay to ensure previous goroutine stopped
	time.Sleep(100 * time.Millisecond)
	lv.startStreaming()
}

func (lv *LogView) startStreaming() {
	go func() {
		var reader io.ReadCloser
		var err error
		var hasTTY bool

		if lv.ResourceType == "service" {
			reader, err = lv.App.Docker.GetServiceLogs(lv.ContainerID, lv.ShowTimestamps)
			hasTTY = false // Services usually don't have TTY logs
		} else {
			// Determine if TTY is enabled
			hasTTY, _ = lv.App.Docker.HasTTY(lv.ContainerID)
			reader, err = lv.App.Docker.GetContainerLogs(lv.ContainerID, lv.ShowTimestamps)
		}

		if err != nil {
			lv.App.TviewApp.QueueUpdateDraw(func() {
				lv.SetText(fmt.Sprintf("[red]Error fetching logs: %v", err))
			})
			return
		}
		defer reader.Close()

		// Pipe reader to check for stop signal
		// Since stdcopy/io.Copy blocks, we run it in a sub-goroutine and close reader on stop
		
		done := make(chan struct{})
		
		go func() {
			select {
			case <-lv.stopChan:
				reader.Close()
			case <-done:
			}
		}()

		writer := &LogWriter{Tv: lv.TextView, App: lv.App.TviewApp, AutoScroll: &lv.AutoScroll}
		
		if hasTTY {
			_, err = io.Copy(writer, reader)
		} else {
			_, err = stdcopy.StdCopy(writer, writer, reader)
		}
		
		close(done)
		
		if err != nil && err != io.EOF && err.Error() != "http: read on closed response body" && !isClosedErr(err) {
			lv.App.TviewApp.QueueUpdateDraw(func() {
				lv.Write([]byte(fmt.Sprintf("\n[red]Stream ended: %v\n", err)))
			})
		}
	}()
}

func isClosedErr(err error) bool {
	return err.Error() == "use of closed network connection" || err.Error() == "file already closed"
}

type LogWriter struct {
	Tv         *tview.TextView
	App        *tview.Application
	AutoScroll *bool
}

func (w *LogWriter) Write(p []byte) (n int, err error) {
	text := string(p)
	w.App.QueueUpdateDraw(func() {
		w.Tv.Write([]byte(text))
		if *w.AutoScroll {
			w.Tv.ScrollToEnd()
		}
	})
	return len(p), nil
}

