package inspect

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/config"
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
	AutoScroll bool
	Timestamps bool
	Wrap       bool // Restored
	filter     string
	since      string
	tail       string
	sinceLabel string

	// Control
	cancelFunc context.CancelFunc
}

// Ensure implementation
var _ common.Inspector = (*LogInspector)(nil)

func NewLogInspector(id, subject, resourceType string) *LogInspector {
	return &LogInspector{
		ResourceID:   id,
		Subject:      subject,
		ResourceType: resourceType,
		AutoScroll:   true,
		Timestamps:   false,
		Wrap:         false,
		since:        "",
		tail:         "200",
		sinceLabel:   "Tail",
	}
}

// NewLogInspectorWithConfig creates a LogInspector with settings from the config.
func NewLogInspectorWithConfig(id, subject, resourceType string, logCfg config.LoggerConfig) *LogInspector {
	return &LogInspector{
		ResourceID:   id,
		Subject:      subject,
		ResourceType: resourceType,
		AutoScroll:   !logCfg.DisableAutoscroll,
		Timestamps:   logCfg.ShowTime,
		Wrap:         logCfg.TextWrap,
		since:        logCfg.GetLogSince(),
		tail:         logCfg.GetLogTail(),
		sinceLabel:   logCfg.GetLogSinceLabel(),
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
	title = strings.ReplaceAll(title, fmt.Sprintf(" [[%s][%s]]", styles.TagFg, styles.TagCyan), "")
	return title
}

func (i *LogInspector) GetStatus() string {
	fmtStatus := func(label string, active bool) string {
		c := fmt.Sprintf("[%s]Off[-]", styles.TagDim)
		if active {
			c = fmt.Sprintf("[%s]On[-]", styles.TagInfo)
		}
		return fmt.Sprintf("[%s]%s:[-]%s", styles.TagSCKey, label, c)
	}

	parts := []string{}
	parts = append(parts, fmtStatus("[::b]Autoscroll[::-]", i.AutoScroll))
	parts = append(parts, fmtStatus("[::b]Timestamps[::-]", i.Timestamps))
	parts = append(parts, fmtStatus("[::b]Wrap[::-]", i.Wrap))
	parts = append(parts, fmt.Sprintf("[%s::b]Since:[-::-][%s]%s[-]", styles.TagSCKey, styles.TagFg, i.sinceLabel))

	return strings.Join(parts, "     ")
}

func (i *LogInspector) GetShortcuts() []string {
	// Helper for alt shortcuts (time/range control)
	altSC := func(key, action string) string {
		return fmt.Sprintf("[%s::b]<%s>[-]   [%s]%s[-]", styles.TagPink, key, styles.TagDim, action)
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
		common.FormatSCHeader("c", "Copy"),
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
	i.HeaderView.SetBackgroundColor(styles.ColorBlack)

	i.TextView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(i.Wrap). // Only affects word boundary, not line wrapping per se if SetWrap matches
		SetWrap(i.Wrap).     // CRITICAL: SetWrap(false) allows horizontal scrolling
		SetTextColor(styles.ColorIdle)

	i.TextView.SetChangedFunc(func() {
		if i.AutoScroll {
			i.TextView.ScrollToEnd()
		}
	})
	i.TextView.SetBackgroundColor(styles.ColorBlack)

	i.Flex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(i.HeaderView, 1, 1, false).
		AddItem(i.TextView, 0, 1, true)

	i.Flex.SetBorder(true).
		SetTitle(i.GetTitle()).
		SetTitleColor(styles.ColorTitle).
		SetBorderColor(styles.ColorIdle).
		SetBackgroundColor(styles.ColorBlack).
		SetBorderPadding(0, 0, 0, 0)

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

	if event.Rune() == ':' {
		i.App.ActivateCmd(":")
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
		if i.AutoScroll && i.TextView != nil {
			i.TextView.ScrollToEnd()
		}
	case 'w':
		i.Wrap = !i.Wrap
		i.updateTitle()
		if i.TextView != nil {
			i.TextView.SetWordWrap(i.Wrap)
			i.TextView.SetWrap(i.Wrap)
		}
	case 't':
		i.Timestamps = !i.Timestamps
		i.updateTitle()
		i.startStreaming() // Restart with new setting
	case 'c':
		i.copyToClipboard()
	case 'C': // Shift+c
		if i.TextView != nil {
			i.TextView.Clear()
		}
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

func (i *LogInspector) copyToClipboard() {
	if i.TextView == nil {
		return
	}
	content := i.TextView.GetText(true)
	if err := clipboard.WriteAll(content); err != nil {
		i.App.AppendFlashError(fmt.Sprintf("%v", err))
	} else {
		i.App.AppendFlashSuccess(fmt.Sprintf("copied %d bytes", len(content)))
	}
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

	if i.TextView != nil {
		i.TextView.Clear()
		i.TextView.SetText(fmt.Sprintf(" [%s]Loading logs...\n", styles.TagAccent))
	}

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
		} else if i.ResourceType == "compose" {
			reader, err = docker.GetComposeLogs(i.ResourceID, i.since, i.tail, i.Timestamps)
			// Compose logs via CLI are already plain text, no demux needed
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
				if i.TextView != nil {
					i.TextView.SetText(fmt.Sprintf("[%s]Error fetching logs: %v", styles.TagError, err))
				}
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
			logCh <- fmt.Sprintf("[%s]Stream Error: %v", styles.TagError, err)
		}
	}()

	// Flusher Goroutine
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		// Color management for Compose
		colorMap := make(map[string]string)
		colorIdx := 0
		nextColor := func() string {
			palette := []string{
				"#00ff00", // Bright Green
				"#00d7ff", // Cyan
				"#d700d7", // Purple-Magenta
				"#ffff00", // Yellow
				"#ff5f00", // Orange
				"#ff005f", // Red/Pink
				"#00ffaf", // Spring Green
				"#d7ff00", // Chartreuse
				"#af00ff", // Violet
				"#00afff", // Blue
			}
			c := palette[colorIdx%len(palette)]
			colorIdx++
			return c
		}

		var buffer []string
		firstWrite := true

		flush := func() {
			if len(buffer) == 0 {
				return
			}
			// text := strings.Join(buffer, "\n") + "\n" // Unused in Table approach
			// buffer handled directly

			i.App.GetTviewApp().QueueUpdateDraw(func() {
				if i.TextView == nil {
					return
				}

				if firstWrite {
					i.TextView.Clear()
				}

				// Calculate starting row
				text := strings.Join(buffer, "\n") + "\n"

				// Apply color - ColorIdle is Blueish
				fmt.Fprint(i.TextView, text)

				if firstWrite {
					if !i.AutoScroll {
						i.TextView.ScrollToBeginning()
					}
					firstWrite = false
				} else if i.AutoScroll {
					i.TextView.ScrollToEnd()
				}

				buffer = buffer[:0] // Clear buffer but keep capacity
			})
		}

		for {
			select {
			case line, ok := <-logCh:
				if !ok {
					flush()
					return
				}

				line = tview.TranslateANSI(tview.Escape(line))

				// Filter logic (supports negation with ^)
				if i.filter != "" {
					filterTerm := i.filter
					negate := false
					if strings.HasPrefix(filterTerm, "^") {
						negate = true
						filterTerm = strings.TrimPrefix(filterTerm, "^")
					}

					// If negate and term is empty (input is "^"), treat as match all (show all)
					if negate && filterTerm == "" {
						// no-op, show line
					} else {
						contains := strings.Contains(line, filterTerm)
						if negate {
							if contains {
								continue
							}
						} else {
							if !contains {
								continue
							}
							// Highlight for positive match only
							line = strings.ReplaceAll(line, filterTerm, fmt.Sprintf("[yellow]%s[-]", filterTerm))
						}
					}
				}

				if i.ResourceType == "compose" {
					// Compose Logs: "ContainerPrefix | LogPayload"
					parts := strings.SplitN(line, "|", 2)
					if len(parts) == 2 {
						prefix := parts[0]
						body := parts[1]

						// Determine unique color for this container prefix
						key := strings.TrimSpace(prefix)

						col, exists := colorMap[key]
						if !exists {
							col = nextColor()
							colorMap[key] = col
						}

						// Handle timestamp which appears inside the body for compose logs
						if i.Timestamps {
							// Body usually starts with a space -> " 2023... msg"
							trimmed := strings.TrimLeft(body, " ")
							indent := body[:len(body)-len(trimmed)]

							tParts := strings.SplitN(trimmed, " ", 2)
							if len(tParts) == 2 {
								body = fmt.Sprintf("%s[%s]%s[-] %s", indent, styles.TagDim, tParts[0], tParts[1])
							}
						}

						// Color the prefix, reset, keep pipe, print body
						line = fmt.Sprintf("[%s]%s[-]|%s", col, prefix, body)
					} else {
						// Fallback for lines without pipe (e.g. "Attaching to...")
						line = " [" + styles.TagIdle + "]" + line
					}
				} else {
					// Standard Container/Service Logs
					// Timestamp Coloring
					// Assuming Docker log format: "2023-01-01T00:00:00.0000Z message"
					if i.Timestamps {
						parts := strings.SplitN(line, " ", 2)
						if len(parts) == 2 {
							// Check if first part looks like a timestamp?
							// Just blind replace for perf
							line = fmt.Sprintf("[%s]%s[-] %s", styles.TagDim, parts[0], parts[1])
						}
					}

					line = " [" + styles.TagIdle + "]" + line + " "
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
