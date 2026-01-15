package common

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Inspector defines a component that can be shown in the inspection modal layer.
type Inspector interface {
	// GetPrimitive returns the tview component to be displayed
	GetPrimitive() tview.Primitive

	// GetID returns a unique ID for this inspector instance (usually "inspect")
	GetID() string

	// InputHandler handles keyboard events. Returns nil if handled.
	InputHandler(event *tcell.EventKey) *tcell.EventKey

	// Helpers for the App to display info
	GetTitle() string
	GetShortcuts() []string

	// Lifecycle
	OnMount(app AppController)
	OnUnmount()
	
	// ApplyFilter applies a search/filter to the inspector view
	ApplyFilter(filter string)
}
