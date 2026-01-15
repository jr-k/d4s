package inspect

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

// LogInspector implements Inspector for streaming logs
type LogInspector struct {
	App          common.AppController
	TextView     *tview.TextView
	ResourceID   string
	ResourceType string
	
	// Settings
	AutoScroll   bool
	Wrap         bool
	Timestamps   bool
	
	// Control
	cancelFunc   context.CancelFunc
}

// Ensure implementation
var _ common.Inspector = (*LogInspector)(nil)

func NewLogInspector(id, resourceType string) *LogInspector {
	return &LogInspector{
		ResourceID:   id,
		ResourceType: resourceType,
		AutoScroll:   true,
		Wrap:         false,
		Timestamps:   false,
	}
}

func (i *LogInspector) GetID() string {
	return "inspect" // Same ID slot as text inspector
}

func (i *LogInspector) GetPrimitive() tview.Primitive {
	return i.TextView
}

func (i *LogInspector) GetTitle() string {
	opts := []string{}
	if i.AutoScroll { opts = append(opts, "AutoScroll: ON") }
	if i.Wrap { opts = append(opts, "Wrap: ON") }
	if i.Timestamps { opts = append(opts, "Time: ON") }
	
	status := ""
	if len(opts) > 0 {
		status = fmt.Sprintf(" (%s)", strings.Join(opts, ", "))
	}
	
	return fmt.Sprintf(" Logs: %s%s ", i.ResourceID, status)
}

func (i *LogInspector) GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("Esc", "Close"),
		common.FormatSCHeader("s", "Toggle AutoScroll"),
		common.FormatSCHeader("w", "Toggle Wrap"),
		common.FormatSCHeader("t", "Toggle Times"),
		common.FormatSCHeader("C", "Clear"),
	}
}

func (i *LogInspector) OnMount(app common.AppController) {
	i.App = app
	
	i.TextView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(i.Wrap)
	
	i.TextView.SetChangedFunc(func() {
		if i.AutoScroll {
			i.TextView.ScrollToEnd()
		}
	})

	i.TextView.SetBorder(true).
		SetTitle(i.GetTitle()).
		SetTitleColor(styles.ColorTitle).
		SetBackgroundColor(styles.ColorBg)
		
	i.startStreaming()
}

func (i *LogInspector) OnUnmount() {
	if i.cancelFunc != nil {
		i.cancelFunc()
	}
}

func (i *LogInspector) InputHandler(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyEsc {
		i.App.CloseInspector()
		return nil
	}
	
	switch event.Rune() {
	case 's':
		i.AutoScroll = !i.AutoScroll
		i.updateTitle()
		if i.AutoScroll {
			i.TextView.ScrollToEnd()
		}
	case 'w':
		i.Wrap = !i.Wrap
		i.TextView.SetWordWrap(i.Wrap)
		i.updateTitle()
	case 't':
		i.Timestamps = !i.Timestamps
		i.updateTitle()
		i.startStreaming() // Restart with new setting
	case 'C': // Shift+c usually
		if event.Modifiers()&tcell.ModShift != 0 {
			i.TextView.Clear()
		}
	}
	
	return event
}

func (i *LogInspector) updateTitle() {
	i.TextView.SetTitle(i.GetTitle())
}

func (i *LogInspector) startStreaming() {
	if i.cancelFunc != nil {
		i.cancelFunc()
	}

	ctx, cancel := context.WithCancel(context.Background())
	i.cancelFunc = cancel

	i.TextView.Clear()
	i.TextView.SetText("[yellow]Loading logs...\n")

	go func() {
		var reader io.ReadCloser
		var err error
		
		docker := i.App.GetDocker()
		
		if i.ResourceType == "service" {
			reader, err = docker.GetServiceLogs(i.ResourceID, i.Timestamps)
		} else {
			// Container
			reader, err = docker.GetContainerLogs(i.ResourceID, i.Timestamps)
		}

		if err != nil {
			i.App.GetTviewApp().QueueUpdateDraw(func() {
				i.TextView.SetText(fmt.Sprintf("[red]Error fetching logs: %v", err))
			})
			return
		}
		defer reader.Close()

		// Stream copy
		// We use a small buffer or standard copy
		// But we need to handle context cancellation to stop reading
		
		buf := make([]byte, 4096)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, readErr := reader.Read(buf)
				if n > 0 {
					// Clean output? For now just write
					chunk := string(buf[:n])
					// Handle some basic coloring if needed or just dump
					
					// Thread safety for tview update
					// Optim: Batch updates? For now simple queue
					// Just writing to TextView is thread safe? No.
					// Must use Write() if it implements io.Writer? 
					// tview.TextView is NOT thread safe for Write() unless invoked in QueueUpdate
					
					// Simple implementation:
					finalChunk := chunk // can process ANSI here if needed
					i.App.GetTviewApp().QueueUpdateDraw(func() {
						fmt.Fprint(i.TextView, finalChunk)
					})
				}
				if readErr != nil {
					if readErr != io.EOF {
						i.App.GetTviewApp().QueueUpdateDraw(func() {
							fmt.Fprintf(i.TextView, "\n[red]Stream Error: %v", readErr)
						})
					}
					return
				}
			}
		}
	}()
}
