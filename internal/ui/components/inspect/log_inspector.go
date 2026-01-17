package inspect

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

// LogInspector implements Inspector for streaming logs
type LogInspector struct {
	App          common.AppController
	Flex         *tview.Flex
	HeaderView   *tview.TextView
	TextView     *tview.TextView
	ResourceID   string
	Subject      string
	ResourceType string
	
	// Settings
	AutoScroll   bool
	Wrap         bool
	Timestamps   bool
	filter       string
	since        string
	tail         string // new field
	sinceLabel   string
	
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
		since:        "",
		tail:         "200",
		sinceLabel:   "Tail",
	}
}

func (i *LogInspector) GetID() string {
	return "inspect" // Same ID slot as text inspector
}

func (i *LogInspector) GetPrimitive() tview.Primitive {
	return i.Flex
}

func (i *LogInspector) GetTitle() string {
	// Standard Title on first line
	title := FormatInspectorTitle("Logs", i.Subject, "", i.filter, 0, 0)
	// Remove empty mode brackets from standard title if needed
	title = strings.ReplaceAll(title, " [[white][#00ffff]]", "")
	return title
}

func (i *LogInspector) GetStatus() string {
	fmtStatus := func(label string, active bool) string {
		c := "[gray]Off[-]"
		if active {
			c = "[green]On[-]"
		}
		return fmt.Sprintf("[#5f87ff]%s:[-]%s", label, c)
	}

	parts := []string{}
	parts = append(parts, fmtStatus("[::b]Autoscroll[::-]", i.AutoScroll))
	parts = append(parts, fmtStatus("[::b]FullScreen[::-]", false))
	parts = append(parts, fmtStatus("[::b]Timestamps[::-]", i.Timestamps))
	parts = append(parts, fmtStatus("[::b]Wrap[::-]", i.Wrap))
	parts = append(parts, fmt.Sprintf("[#5f87ff::b]Since:[-::-][white]%s[-]", i.sinceLabel))
	
	return strings.Join(parts, "     ")
}

func (i *LogInspector) GetShortcuts() []string {
	// Helper for alt shortcuts (time/range control)
	altSC := func(key, action string) string {
		return fmt.Sprintf("[#ff00ff::b]<%s>[-]   [gray]%s[-]", key, action)
	}

	altShortcuts := []string{
		altSC("0", "Tail"),
		altSC("1", "Head"),
		altSC("2", "1m"),
		altSC("3", "5m"),
		altSC("4", "15m"),
		altSC("5", "30m"),
		altSC("6", "1h"),
	}

	// Calculate padding to finish the current column
	// Max items per column is 6 (defined in header.go)
	const maxPerCol = 6
	paddingNeeded := maxPerCol - (len(altShortcuts) % maxPerCol)
	if paddingNeeded == maxPerCol {
		paddingNeeded = 0
	}
	
	for j := 0; j < paddingNeeded; j++ {
		altShortcuts = append(altShortcuts, "")
	}

	return append(altShortcuts,
		common.FormatSCHeader("s", "Scroll"),
		common.FormatSCHeader("w", "Wrap"),
		common.FormatSCHeader("t", "Time"),
		common.FormatSCHeader("shift-c", "Clear"),
	)
}

func (i *LogInspector) OnMount(app common.AppController) {
	i.App = app
	
	i.HeaderView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetWrap(false).
		SetText(i.GetStatus())
	i.HeaderView.SetBackgroundColor(styles.ColorBg)

	i.TextView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(i.Wrap)
	
	i.TextView.SetChangedFunc(func() {
		if i.AutoScroll {
			i.TextView.ScrollToEnd()
		}
	})
	i.TextView.SetBackgroundColor(styles.ColorBg)

	i.Flex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(i.HeaderView, 1, 1, false).
		AddItem(i.TextView, 0, 1, true)

	i.Flex.SetBorder(true).
		SetTitle(i.GetTitle()).
		SetTitleColor(styles.ColorTitle).
		SetBackgroundColor(styles.ColorBg).
		SetBorderPadding(0, 0, 1, 1)
		
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
	case 'C': // Shift+c
		i.TextView.Clear()
	case '0':
		i.setSince("tail")
	case '1':
		i.setSince("head")
	case '2':
		i.setSince("1m")
	case '3':
		i.setSince("5m")
	case '4':
		i.setSince("15m")
	case '5':
		i.setSince("30m")
	case '6':
		i.setSince("1h")
	}
	
	return event
}

func (i *LogInspector) setSince(mode string) {
	if mode == "tail" {
		i.since = ""
		i.tail = "200" // Tail default
		i.sinceLabel = "Tail"
		i.AutoScroll = true
	} else if mode == "head" {
		i.since = "" 
		i.tail = "all"
		i.sinceLabel = "Head"
		i.AutoScroll = false
	} else {
		// Time modes
		i.since = mode
		i.tail = "all"
		i.sinceLabel = mode
		i.AutoScroll = true
	}

	i.updateTitle()
	i.startStreaming()
}

func (i *LogInspector) updateTitle() {
	if i.Flex != nil {
		i.Flex.SetTitle(i.GetTitle())
	}
	if i.HeaderView != nil {
		i.HeaderView.SetText(i.GetStatus())
	}
}

func (i *LogInspector) startStreaming() {
	if i.cancelFunc != nil {
		i.cancelFunc()
	}

	ctx, cancel := context.WithCancel(context.Background())
	i.cancelFunc = cancel

	i.TextView.Clear()
	i.TextView.SetText("[yellow]Loading logs...\n")

	// Channels for buffering
	logCh := make(chan string, 1000)
	
	go func() {
		defer close(logCh)
		
		var reader io.ReadCloser
		var err error
		
		docker := i.App.GetDocker()
		
		if i.ResourceType == "service" {
			reader, err = docker.GetServiceLogs(i.ResourceID, i.since, i.tail, i.Timestamps)
			if err == nil {
				// We assume services are multiplexed (TTY=false usually)
				// TODO: Check Service Spec for TTY
				reader = demux(reader)
			}
		} else {
			// Container
			reader, err = docker.GetContainerLogs(i.ResourceID, i.since, i.tail, i.Timestamps)
			if err == nil {
				// Check for TTY
				hasTTY, _ := docker.HasTTY(i.ResourceID)
				if !hasTTY {
					reader = demux(reader)
				}
			}
		}

		if err != nil {
			i.App.GetTviewApp().QueueUpdateDraw(func() {
				i.TextView.SetText(fmt.Sprintf("[red]Error fetching logs: %v", err))
			})
			return
		}
		defer reader.Close()

		// Stream using Scanner
		scanner := bufio.NewScanner(reader)
		// Increase buffer size to handle large lines
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 5*1024*1024) // 5MB limit
		
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				line := scanner.Text()
				logCh <- line
			}
		}
		
		if err := scanner.Err(); err != nil && err != context.Canceled && err != io.EOF {
			logCh <- fmt.Sprintf("[red]Stream Error: %v", err)
		}
	}()

	// Flusher Goroutine
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		var buffer []string
		firstWrite := true

		flush := func() {
			if len(buffer) == 0 {
				return
			}
			text := strings.Join(buffer, "\n") + "\n"
			buffer = buffer[:0] // Clear buffer but keep capacity

			i.App.GetTviewApp().QueueUpdateDraw(func() {
				if firstWrite {
					i.TextView.Clear()
				}
				// We append to the existing text
				// tview.TextView is an io.Writer
				w := i.TextView
				fmt.Fprint(w, text)

				if firstWrite {
					if !i.AutoScroll {
						i.TextView.ScrollToBeginning()
					}
					firstWrite = false
				}
			})
		}

		for {
			select {
			case line, ok := <-logCh:
				if !ok {
					flush()
					return
				}
				
				// Filter logic
				if i.filter != "" {
					if !strings.Contains(line, i.filter) {
						continue
					}
					// Highlight
					line = strings.ReplaceAll(line, i.filter, fmt.Sprintf("[yellow]%s[-]", i.filter))
				}
				
				// Timestamp Coloring
				// Assuming Docker log format: "2023-01-01T00:00:00.0000Z message"
				// Or if demuxed, it's just raw bytes, but we requested Timestamps=true in API
				if i.Timestamps {
					parts := strings.SplitN(line, " ", 2)
					if len(parts) == 2 {
						// Check if first part looks like a timestamp? 
						// Just blind replace for perf
						line = fmt.Sprintf("[gray]%s[-] %s", parts[0], parts[1])
					}
				}
				
				buffer = append(buffer, line)
				
				// Optional: if buffer gets too big, flush immediately to avoid lag
				if len(buffer) >= 1000 {
					flush()
				}
				
			case <-ticker.C:
				flush()

			case <-ctx.Done():
				return
			}
		}
	}()
}

func demux(r io.ReadCloser) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		defer r.Close()
		// Determine which writer to use for stdout/stderr?
		// For logs view, we just merge them.
		_, _ = stdcopy.StdCopy(pw, pw, r)
	}()
	return pr
}
