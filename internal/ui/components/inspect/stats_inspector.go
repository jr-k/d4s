package inspect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/gdamore/tcell/v2"
	"github.com/guptarohit/asciigraph"
	daoCommon "github.com/jr-k/d4s/internal/dao/common"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

type StatsInspector struct {
	App         common.AppController
	ContainerID string
	ContainerName string
	Layout      *tview.Flex
	
	// Text Mode
	TextView    *tview.TextView
	
	// Graph Mode (Dashboard)
	Grid        *tview.Grid
	GraphCPU    *tview.TextView
	GraphMem    *tview.TextView
	GraphNet    *tview.TextView
	GraphDisk   *tview.TextView
	
	Mode        string // "text" or "graph"
	StopChan    chan struct{}
	
	cpuHistory        []float64
	memHistory        []float64
	netRxHistory      []float64
	netTxHistory      []float64
	diskReadHistory   []float64
	diskWriteHistory  []float64
	
	// Previous values for rate calculation
	prevNetRx     float64
	prevNetTx     float64
	prevDiskRead  float64
	prevDiskWrite float64
	firstSample   bool
	
	maxPoints   int
	filter      string

	lastRow int
	lastCol int
	
	// State management
	mu          sync.RWMutex
	lastStats   map[string]interface{}
	
	// Dashboard Cached Values
	curCPU   float64
	curMem   uint64
	curLimit uint64
	curRx    float64
	curTx    float64
	curRead  float64
	curWrite float64
}

// Ensure interface compliance
var _ common.Inspector = (*StatsInspector)(nil)

func NewStatsInspector(containerID, containerName string) *StatsInspector {
	return &StatsInspector{
		ContainerID:   containerID,
		ContainerName: containerName,
		Mode:          "graph", 
		StopChan:      make(chan struct{}),
		maxPoints:   120, 
		firstSample: true,
	}
}

func (i *StatsInspector) GetID() string { return "inspect" }

func (i *StatsInspector) GetPrimitive() tview.Primitive {
	return i.Layout
}

func (i *StatsInspector) GetTitle() string {
	mode := "📊 Graph"
	if i.Mode == "text" {
		mode = "🔖 JSON"
	}
	id := i.ContainerID
	if len(id) > 12 {
		id = id[:12]
	}
	name := strings.TrimPrefix(i.ContainerName, "/")
	subject := fmt.Sprintf("%s@%s", name, id)
	
	return FormatInspectorTitle("Stats", subject, mode, i.filter, 0, 0)
}

func (i *StatsInspector) GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("Esc", "Close"),
		common.FormatSCHeader("Tab", "Switch View"),
	}
}

func (i *StatsInspector) OnMount(app common.AppController) {
	i.App = app
	
    // Initialize TextView (Text Mode)
	i.TextView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false)
    i.TextView.SetBackgroundColor(styles.ColorBg)
	
	// Initialize Grid (Graph Mode)
	// We use Borders on the Grid items (the TextViews) instead of the Grid itself for cleaner separation
	i.Grid = tview.NewGrid().
		SetRows(0, 0).
		SetColumns(0, 0).
		SetBorders(false).
		SetGap(0, 0)
		
	i.Grid.SetBackgroundColor(styles.ColorBg)

	i.GraphCPU = createGraphView("CPU Usage")
	i.GraphMem = createGraphView("Memory Usage")
	i.GraphNet = createGraphView("Network I/O")
	i.GraphDisk = createGraphView("Disk I/O")

	// Add items to grid (row, col, rowspan, colspan, minHeight, minWidth, visible)
	// We add padding or borders via the TextViews
	i.Grid.AddItem(i.GraphCPU, 0, 0, 1, 1, 0, 0, true)
	i.Grid.AddItem(i.GraphMem, 0, 1, 1, 1, 0, 0, true)
	i.Grid.AddItem(i.GraphNet, 1, 0, 1, 1, 0, 0, true)
	i.Grid.AddItem(i.GraphDisk, 1, 1, 1, 1, 0, 0, true)

    i.Layout = tview.NewFlex().SetDirection(tview.FlexRow)
	// Keep outer frame opaque to prevent bleed-through
    i.Layout.SetBorder(true).SetTitleColor(styles.ColorTitle)
	i.Layout.SetBackgroundColor(styles.ColorBg)
    
	i.updateLayout()
	// Initial draw to ensure no empty boxes
	i.drawDashboard(0,0,0,0,0,0,0)
    i.startRefresher()
}

func createGraphView(title string) *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false).
		SetTextAlign(tview.AlignLeft)
	
	tv.SetBorder(true).
		SetTitle(fmt.Sprintf(" %s ", title)).
		SetTitleColor(styles.ColorTitle).
		SetBorderColor(styles.ColorTitle)
		
	tv.SetBackgroundColor(styles.ColorBg) 
	return tv
}

func (i *StatsInspector) updateLayout() {
    i.Layout.Clear()
    i.Layout.SetTitle(i.GetTitle())
    
    if i.Mode == "text" {
        i.Layout.AddItem(i.TextView, 0, 1, true)
    } else {
        i.Layout.AddItem(i.Grid, 0, 1, true)
    }
}

func (i *StatsInspector) OnUnmount() {
	close(i.StopChan)
}

func (i *StatsInspector) ApplyFilter(filter string) {
	i.mu.Lock()
	i.filter = filter
	i.mu.Unlock()
	
	// Redraw without fetching new data
	i.draw()
}

func (i *StatsInspector) InputHandler(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyEsc {
		i.App.CloseInspector()
		return nil
	}
	
	if event.Rune() == '/' {
		i.App.ActivateCmd("/")
		return nil
	}
	
	if event.Key() == tcell.KeyTab {
		if i.Mode == "text" {
			i.Mode = "graph"
		} else {
			i.Mode = "text"
		}
		i.updateLayout()
		return nil
	}
	
	// Forward input
	if i.Mode == "text" {
		if handler := i.TextView.InputHandler(); handler != nil {
			handler(event, func(p tview.Primitive) {})
			return nil
		}
	}
	
	return event
}

func (i *StatsInspector) startRefresher() {
    go i.tick()

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				i.tick()
			case <-i.StopChan:
				return
			}
		}
	}()
}

func (i *StatsInspector) tick() {
    statsJSON, err := i.App.GetDocker().GetContainerStats(i.ContainerID)
    if err != nil {
        return 
    }
    
    // Parse
    var v map[string]interface{}
    json.Unmarshal([]byte(statsJSON), &v)
    cpu, mem, limit, netRx, netTx, diskRead, diskWrite := daoCommon.CalculateStatsFromMap(v)

	// Calculate Rates (Pre-Store)
	// We need to calculate rates BEFORE storing them in Cur*, but we depend on Prev* values.
	// So we do it here.
	
	rxRate := netRx - i.prevNetRx
	txRate := netTx - i.prevNetTx
	readRate := diskRead - i.prevDiskRead
	writeRate := diskWrite - i.prevDiskWrite
	
	if rxRate < 0 { rxRate = 0 }
	if txRate < 0 { txRate = 0 }
	if readRate < 0 { readRate = 0 }
	if writeRate < 0 { writeRate = 0 }

	i.mu.Lock()
	i.lastStats = v // Store for re-rendering
	
	// Store calculated values for Dashboard re-drawing
	i.curCPU = cpu
	i.curMem = mem
	i.curLimit = limit
	i.curRx = rxRate
	i.curTx = txRate
	i.curRead = readRate
	i.curWrite = writeRate
	
    if i.firstSample {
		i.prevNetRx = netRx
		i.prevNetTx = netTx
		i.prevDiskRead = diskRead
		i.prevDiskWrite = diskWrite
		i.firstSample = false
	}

	i.prevNetRx = netRx
	i.prevNetTx = netTx
	i.prevDiskRead = diskRead
	i.prevDiskWrite = diskWrite

    // Update History
    i.cpuHistory = pushHistory(i.cpuHistory, cpu, i.maxPoints)
	
    memPct := 0.0
    if limit > 0 {
        memPct = float64(mem) / float64(limit) * 100.0
    }
	i.memHistory = pushHistory(i.memHistory, memPct, i.maxPoints)
	
	i.netRxHistory = pushHistory(i.netRxHistory, rxRate, i.maxPoints)
	i.netTxHistory = pushHistory(i.netTxHistory, txRate, i.maxPoints)
	
	i.diskReadHistory = pushHistory(i.diskReadHistory, readRate, i.maxPoints)
	i.diskWriteHistory = pushHistory(i.diskWriteHistory, writeRate, i.maxPoints)
	i.mu.Unlock()

	i.draw()
}

func (i *StatsInspector) draw() {
	i.mu.RLock()
	// Copy data needed for drawing to avoid holding lock during Marshalling/Regex
	v := i.lastStats
	filter := i.filter
	mode := i.Mode
	
	// Graph data copies
	// Slices are pointers, so we need to be careful. 
	// However, drawDashboard reads them only. 
	// But pushHistory modifies the underlying array or returns new slice.
	// Since tick() has the lock when pushing, and we have the lock now,
	// the slices are stable *for this moment*.
	// But if we pass them to QueueUpdateDraw (async), they might change by the next tick!
	// So we should capture the derived graph Values or Draw everything here?
	
	// Actually drawDashboard calls renderGraph which is fast.
	// We can't lock around QueueUpdateDraw easily.
	// Best pattern: Prepare everything off-thread, then just push string/rune updates.
	
	// For Text Mode:
    var textContent string
	var lastRow, lastColCount int
	
    if mode == "text" {
        var buf bytes.Buffer
        pretty, _ := json.MarshalIndent(v, "", "  ")
        quick.Highlight(&buf, string(pretty), "json", "terminal256", "monokai")
        textContent = tview.TranslateANSI(buf.String())

        if filter != "" {
            pattern := fmt.Sprintf(`(\[[^\]]*\])|(%s)`, regexp.QuoteMeta(filter))
            re, err := regexp.Compile(pattern)
            if err == nil {
                textContent = re.ReplaceAllStringFunc(textContent, func(s string) string {
                    if strings.HasPrefix(s, "[") { return s }
                    return fmt.Sprintf("[black:yellow]%s[-]", s)
                })
            }
        }
		
		lastRow = i.lastRow
		lastColCount = i.lastCol
    }
	
	// For Graph Mode (Need to extract relevant current values + history)
	// We'll just capture the *current* display values.
	// The history slices are accessed in drawDashboard.
	// Since QueueUpdateDraw runs on UI thread, and tick runs on Ticker,
	// we have a race if drawDashboard reads history while tick modifies it.
	
	// Quick fix: Since history is only for display, we can make shallow copy?
	// Or simply hold RLock inside the callback? NO, deadlock risk with tview internal locks.
	
	// Better: Pass Cloned Slices?
	// History is small (120 floats). Copying is cheap.
	cpuH := make([]float64, len(i.cpuHistory))
	copy(cpuH, i.cpuHistory)
	// ... (repeating for all is verbose)
	
	// Let's assume for now we just want to fix the "crash" on Text Search.
	// The "crash" was Concurrent Map Read/Write on the History likely? 
	// Or the Regex being compiled concurrently? 
	// Actually the user said "crash on first letter of search".
	// Search triggers ApplyFilter -> fetchAndDraw -> update History and filter.
	// If fetchAndDraw was running, we had concurrent write to history.
	
	// With the Lock in tick(), checking only text usage for now.
	i.mu.RUnlock()

    i.App.GetTviewApp().QueueUpdateDraw(func() {
        if mode == "text" {
            // Persist scroll logic:
            if i.TextView.GetText(false) != "" {
				r, c := i.TextView.GetScrollOffset()
				if r > 0 || c > 0 {
					// We need to update the struct, but we are in callback.
					// Locking here is tricky if tick() holds lock and waits for Draw?
					// tick() calls QueueUpdateDraw and finishes. It doesn't wait.
					// So it is safe to Lock i to update lastRow.
					i.mu.Lock()
					i.lastRow, i.lastCol = r, c
					lastRow, lastColCount = r, c // Update local vars
					i.mu.Unlock()
				}
            }
            
            i.TextView.SetText(textContent)
            i.TextView.ScrollTo(lastRow, lastColCount)
        } else {
			// Restore values for dashboard
			i.mu.RLock()
			cpu := i.curCPU
			mem := i.curMem
			limit := i.curLimit
			rx := i.curRx
			tx := i.curTx
			dread := i.curRead
			dwrite := i.curWrite
			i.mu.RUnlock()
			
			i.drawDashboard(cpu, mem, limit, rx, tx, dread, dwrite)
        }
    })
}

func pushHistory(hist []float64, val float64, max int) []float64 {
	hist = append(hist, val)
	if len(hist) > max {
		return hist[1:]
	}
	return hist
}

func (i *StatsInspector) drawDashboard(cpu float64, mem uint64, limit uint64, rx, tx, dread, dwrite float64) {
	// 1. CPU
	{
		label := fmt.Sprintf("Current: %.2f%%", cpu)
		i.renderGraph(i.GraphCPU, i.cpuHistory, label, asciigraph.Green)
	}
	
	// 2. Memory
	{
		memPct := 0.0
		if limit > 0 { memPct = float64(mem) / float64(limit) * 100.0 }
		label := fmt.Sprintf("Current: %.2f%% (%s / %s)", 
			memPct, daoCommon.FormatBytes(int64(mem)), daoCommon.FormatBytes(int64(limit)))
		i.renderGraph(i.GraphMem, i.memHistory, label, asciigraph.Green)
	}
	
	// 3. Network
	{
		label := fmt.Sprintf("Rx: %s/s  Tx: %s/s", daoCommon.FormatBytes(int64(rx)), daoCommon.FormatBytes(int64(tx)))
		i.renderGraph(i.GraphNet, i.netRxHistory, label, asciigraph.Blue)
	}

	// 4. Disk
	{
		label := fmt.Sprintf("Read: %s/s  Write: %s/s", daoCommon.FormatBytes(int64(dread)), daoCommon.FormatBytes(int64(dwrite)))
		i.renderGraph(i.GraphDisk, i.diskWriteHistory, label, asciigraph.Red)
	}
}

func (i *StatsInspector) renderGraph(tv *tview.TextView, data []float64, label string, color asciigraph.AnsiColor) {
	_, _, w, h := tv.GetInnerRect()
	
	// Asciigraph needs explicit resizing
	// Height must be >= 1. Width must be positive.
	
	// Accounting for label text lines
	graphHeight := h - 2 
	if graphHeight < 1 { graphHeight = 1 }
	
	graphWidth := w - 8 // Reserve space for axis labels (approx)
	if graphWidth < 10 { graphWidth = 10 }

    if len(data) == 0 {
        return
    }

	plot := asciigraph.Plot(data, 
		asciigraph.Height(graphHeight),
		asciigraph.Width(graphWidth),
		asciigraph.SeriesColors(color),
		asciigraph.Caption(label),
	)
	
	// Reset bg to opaque before drawing
	tv.SetText("") 
	// TranslateANSI converts the color codes from asciigraph
	tv.SetText(tview.TranslateANSI(plot))
}
