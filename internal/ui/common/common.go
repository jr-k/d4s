package common

import (
	"fmt"

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
	GetDocker() *dao.DockerClient

	// Actions
	PerformAction(action func(id string) error, actionName string)
	InspectCurrentSelection()
	
	// State
	GetActiveScope() *Scope
	SetActiveScope(scope *Scope)
	SetFilter(filter string)
	SetFlashText(text string)
	RestoreFocus()
	
	// Direct access for command component (needed for handlers)
	GetActiveFilter() string
	SetActiveFilter(filter string)
	
	// Layout management
	SetCmdLineVisible(visible bool)
	UpdateShortcuts()

	// Inspector Management
	OpenInspector(inspector Inspector)
	CloseInspector()
}

func FormatSCHeader(key, action string) string {
	// Format: <Key> [spaces] Label
	// Using spaces instead of tab for predictable spacing
	return fmt.Sprintf("[#5f87ff]<%s>[-]   %s", key, action)
}

// Helper for footer shortcuts (legacy/logs)
func FormatSC(key, action string) string {
	return fmt.Sprintf("[#5f87ff::b]<%s>[#f8f8f2:-] %s ", key, action)
}

