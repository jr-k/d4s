package common

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/rivo/tview"
)

type Scope struct {
	Type       string // e.g. "compose"
	Value      string // e.g. "project-name"
	Label      string // e.g. "~/docker-compose.yml"
	OriginView string // View to return to on Esc
}

// AppController defines the methods that sub-components need from the main App
type AppController interface {
	RefreshCurrentView()
	ActivateCmd(initial string)
	SwitchTo(viewName string)
	ExecuteCmd(cmd string)

	// Accessors
	GetPages() *tview.Pages
	GetTviewApp() *tview.Application
	GetScreen() tcell.Screen
	GetDocker() *dao.DockerClient

	// Actions
	PerformAction(action func(id string) error, actionName string, color tcell.Color)
	InspectCurrentSelection()

	// State
	GetActiveScope() *Scope
	SetActiveScope(scope *Scope)
	SetFilter(filter string)
	SetFlashText(text string)
	AppendFlash(text string)
	RestoreFocus()

	// Direct access for command component (needed for handlers)
	GetActiveFilter() string
	SetActiveFilter(filter string)

	// Layout management
	SetCmdLineVisible(visible bool)
	UpdateShortcuts()

	ScheduleViewHighlight(viewName string, match func(dao.Resource) bool, bg, fg tcell.Color, duration time.Duration)

	// Inspector Management
	OpenInspector(inspector Inspector)
	CloseInspector()

	// Async Task Management
	RunInBackground(task func())
	SetPaused(paused bool)

	// Refactoring: Auto Refresh Control
	StartAutoRefresh()
	StopAutoRefresh()
}

func FormatSCHeader(key, action string) string {
	// Format: <Key> [spaces] Label
	// Using spaces instead of tab for predictable spacing
	return fmt.Sprintf("[#2090ff::b]<%s>[-]   [gray]%s[-]", key, action)
}

func FormatSCHeaderGlobal(key, action string) string {
	// Global shortcuts use orange/pinkish color for alias
	return fmt.Sprintf("[orange::b]<%s>[-]   [gray]%s[-]", key, action)
}

// Helper for footer shortcuts (legacy/logs)
func FormatSC(key, action string) string {
	return fmt.Sprintf("[#2090ff::b]<%s>[#f8f8f2:-] [gray]%s[-] ", key, action)
}
