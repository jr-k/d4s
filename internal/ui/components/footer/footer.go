package footer

import (
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

type FlashComponent struct {
	View *tview.TextView
}

func NewFlashComponent() *FlashComponent {
	f := tview.NewTextView()
	f.SetDynamicColors(true).SetTextColor(styles.ColorFlashFg).SetBackgroundColor(styles.ColorFlashBg)
	return &FlashComponent{View: f}
}

func (f *FlashComponent) SetText(text string) {
	f.View.SetText(text)
}

func (f *FlashComponent) Clear() {
	f.View.SetText("")
}
