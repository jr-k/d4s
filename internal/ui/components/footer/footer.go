package footer

import (
	"github.com/jr-k/d4s/internal/ui/styles"
	"github.com/rivo/tview"
)

type FooterComponent struct {
	View *tview.TextView
}

func NewFooterComponent() *FooterComponent {
	f := tview.NewTextView()
	f.SetDynamicColors(true).SetTextColor(styles.ColorFooterFg).SetBackgroundColor(styles.ColorFooterBg)
	return &FooterComponent{View: f}
}

func (f *FooterComponent) SetText(text string) {
	f.View.SetText(text)
}

type FlashComponent struct {
	View *tview.TextView
}

func NewFlashComponent() *FlashComponent {
	f := tview.NewTextView()
	f.SetDynamicColors(true).SetTextColor(styles.ColorFlashFg).SetBackgroundColor(styles.ColorFlashBg) // Royal Blueish
	return &FlashComponent{View: f}
}

func (f *FlashComponent) SetText(text string) {
	f.View.SetText(text)
}

func (f *FlashComponent) Clear() {
	f.View.SetText("")
}
