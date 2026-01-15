package inspect

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/guptarohit/asciigraph"
	daoCommon "github.com/jr-k/d4s/internal/dao/common"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

type StatsInspector struct {
	App           common.AppController
	ContainerID   string
	ContainerName string
	Layout        *tview.Flex

	// Text Mode
	Viewer *TextViewer

	// Graph Mode (Dashboard)
	Grid      *tview.Grid
	GraphCPU  *tview.TextView
	GraphMem  *tview.TextView
	GraphNet  *tview.TextView
	GraphDisk *tview.TextView

	Mode     string // "text" or "graph"
	StopChan chan struct{}

	cpuHistory       []float64
	memHistory       []float64
	netRxHistory     []float64
	netTxHistory     []float64
	diskReadHistory  []float64
	diskWriteHistory []float64

	// Previous values for rate calculation
	prevNetRx     float64
	prevNetTx     float64
	prevDiskRead  float64
	prevDiskWrite float64
	firstSample   bool

	maxPoints int

	// State management
	mu        sync.RWMutex
	lastStats map[string]interface{}

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
	return newStatsInspectorInternal(containerID, containerName, "text")
}

func NewMonitorInspector(containerID, containerName string) *StatsInspector {
	return newStatsInspectorInternal(containerID, containerName, "graph")
}

func newStatsInspectorInternal(containerID, containerName, mode string) *StatsInspector {
	return &StatsInspector{
		ContainerID:   containerID,
		ContainerName: containerName,
		Mode:          mode,
		StopChan:      make(chan struct{}),
		maxPoints:     120,
		firstSample:   true,
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

	filter, idx, count := "", 0, 0
	if i.Viewer != nil {
		filter, idx, count = i.Viewer.GetSearchInfo()
	}
	return FormatInspectorTitle("Stats", subject, mode, filter, idx, count)
}

func (i *StatsInspector) GetShortcuts() []string {
	shortcuts := []string{
		common.FormatSCHeader("Esc", "Close"),
	}
	if i.Mode == "text" {
		shortcuts = append(shortcuts, common.FormatSCHeader("c", "Copy"))
		shortcuts = append(shortcuts, common.FormatSCHeader("n/p", "Next/Prev"))
	}
	return shortcuts
}

func (i *StatsInspector) OnMount(app common.AppController) {
	i.App = app

	// Initialize ViewModel for Text Mode
	i.Viewer = NewTextViewer(app)
	i.Viewer.TitleUpdateFunc = func() {
		// Update parent layout title when search status changes
		i.updateLayout()
	}

	// Initialize Grid (Graph Mode)
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
	i.drawDashboard(0, 0, 0, 0, 0, 0, 0)
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
		i.Layout.AddItem(i.Viewer.GetPrimitive(), 0, 1, true)
	} else {
		i.Layout.AddItem(i.Grid, 0, 1, true)
	}
}

func (i *StatsInspector) OnUnmount() {
	close(i.StopChan)
}

func (i *StatsInspector) ApplyFilter(filter string) {
	if i.Mode == "text" {
		i.Viewer.ApplyFilter(filter)
		// No need to redraw, Viewer handles it
	}
}

func (i *StatsInspector) InputHandler(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyEsc {
		i.App.CloseInspector()
		return nil
	}

	if i.Mode == "text" {
		// Delegate input to Viewer
		if i.Viewer.InputHandler(event) {
			return nil
		}
		// Also allow native scrolling
		if handler := i.Viewer.View.InputHandler(); handler != nil {
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
	rxRate := netRx - i.prevNetRx
	txRate := netTx - i.prevNetTx
	readRate := diskRead - i.prevDiskRead
	writeRate := diskWrite - i.prevDiskWrite

	if rxRate < 0 {
		rxRate = 0
	}
	if txRate < 0 {
		txRate = 0
	}
	if readRate < 0 {
		readRate = 0
	}
	if writeRate < 0 {
		writeRate = 0
	}

	i.mu.Lock()
	i.lastStats = v

	// Store calculated values
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
	v := i.lastStats
	mode := i.Mode
	
	// Dashboard snapshots
	cpu := i.curCPU
	mem := i.curMem
	limit := i.curLimit
	rx := i.curRx
	tx := i.curTx
	dread := i.curRead
	dwrite := i.curWrite
	i.mu.RUnlock()

	if mode == "text" {
		// Update Text View
		// Marshal logic is heavy, do it off UI thread (we are in background ticker usually here)
		pretty, _ := json.MarshalIndent(v, "", "  ")

		// Push to UI thread
		i.App.GetTviewApp().QueueUpdateDraw(func() {
			i.Viewer.Update(string(pretty), "json")
		})
	} else {
		// Update Dashboard
		i.App.GetTviewApp().QueueUpdateDraw(func() {
			if i.Mode != "graph" {
				return
			}
			i.drawDashboard(cpu, mem, limit, rx, tx, dread, dwrite)
		})
	}
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
		if limit > 0 {
			memPct = float64(mem) / float64(limit) * 100.0
		}
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
	if graphHeight < 1 {
		graphHeight = 1
	}

	graphWidth := w - 8 // Reserve space for axis labels (approx)
	if graphWidth < 10 {
		graphWidth = 10
	}

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
