package common

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/config"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

type Scope struct {
	Type       string // e.g. "compose"
	Value      string // e.g. "project-name"
	Label      string // e.g. "~/docker-compose.yml"
	OriginView string // View which initiated this scope
	Parent     *Scope // Previous scope in the stack
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
	GetConfig() *config.Config

	// Actions
	PerformAction(action func(id string) error, actionName string, color tcell.Color)
	GetActionState(viewName string, id string) (string, tcell.Color, bool)
	InspectCurrentSelection()

	// State
	IsReadOnly() bool
	GetActiveScope() *Scope
	SetActiveScope(scope *Scope)
	SetFilter(filter string)
	SetFlashText(text string)
	SetFlashMessage(text string, duration time.Duration)
	SetFlashError(text string)
	SetFlashPending(text string)
	SetFlashSuccess(text string)
	AppendFlash(text string)
	AppendFlashError(text string)
	AppendFlashPending(text string)
	AppendFlashSuccess(text string)
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
	return fmt.Sprintf("[%s::b]<%s>[-]   [%s]%s[-]", styles.TagSCKey, key, styles.TagDim, action)
}

func FormatSCHeaderGlobal(key, action string) string {
	return fmt.Sprintf("[%s::b]<%s>[-]   [%s]%s[-]", styles.TagAccent, key, styles.TagDim, action)
}

// Helper for footer shortcuts (legacy/logs)
func FormatSC(key, action string) string {
	return fmt.Sprintf("[%s::b]<%s>[%s:-] [%s]%s[-] ", styles.TagSCKey, key, styles.TagFg, styles.TagDim, action)
}

func DockerCommand(app AppController, args ...string) *exec.Cmd {
	cmdArgs := append([]string{}, args...)
	if docker := app.GetDocker(); docker != nil && docker.ContextName != "" && docker.ContextName != "default" {
		cmdArgs = append([]string{"--context", docker.ContextName}, cmdArgs...)
	}
	return exec.Command("docker", cmdArgs...)
}
