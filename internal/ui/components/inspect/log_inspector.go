package inspect

import (
	"bufio"
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
	Subject      string
	ResourceType string
	
	// Settings
	AutoScroll   bool
	Wrap         bool
	Timestamps   bool
	filter       string
	
	// Control
	cancelFunc   context.CancelFunc
}

// Ensure implementation
var _ common.Inspector = (*LogInspector)(nil)

func NewLogInspector(id, subject, resourceType string) *LogInspector {
	return &LogInspector{
		ResourceID:   id,
		Subject:      subject,
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
	if i.AutoScroll { opts = append(opts, "AutoScroll") }
	if i.Wrap { opts = append(opts, "Wrap") }
	if i.Timestamps { opts = append(opts, "Time") }
	
	mode := "Log"
	if len(opts) > 0 {
		mode = strings.Join(opts, " ")
	}
	
	return FormatInspectorTitle("Logs", i.Subject, mode, i.filter, 0, 0)
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

func (i *LogInspector) ApplyFilter(filter string) {
	i.filter = filter
	i.updateTitle()
	i.startStreaming()
}

func (i *LogInspector) InputHandler(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyEsc {
		i.App.CloseInspector()
		return nil
	}
	
	if event.Rune() == '/' {
		i.App.ActivateCmd("/")
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

		// Stream using Scanner for line-by-line filtering
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				line := scanner.Text()
				if i.filter != "" && !strings.Contains(line, i.filter) {
					continue
				}
				
				// Highlight match if filter exists?
				// For logs, grep (filtering lines) is usually what is wanted.
				// But we can also color the match.
				if i.filter != "" {
					line = strings.ReplaceAll(line, i.filter, fmt.Sprintf("[yellow]%s[-]", i.filter))
				}
				
				i.App.GetTviewApp().QueueUpdateDraw(func() {
					fmt.Fprintln(i.TextView, line)
				})
			}
		}
		
		if err := scanner.Err(); err != nil && err != context.Canceled && err != io.EOF {
			i.App.GetTviewApp().QueueUpdateDraw(func() {
				fmt.Fprintf(i.TextView, "\n[red]Stream Error: %v", err)
			})
		}
	}()
}
