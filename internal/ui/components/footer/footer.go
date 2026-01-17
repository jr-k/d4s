package footer

import (
	"strings"

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

// Appends text to existing content temporarily, replacing any existing "copied" message
func (f *FlashComponent) Append(text string) {
	current := f.View.GetText(false)
	
	// Remove existing copy notifications to prevent stacking
	// Looking for patterns like " <copied: ...> "
	// A simple but effective way is to split by " <copied:" and take the first part
	// Or use a more robust regex if needed, but string manipulation is faster here
	// Assuming the tag starts with " [black:#50fa7b:b] <copied:" and ends with "[-] "
	
	// Strategy: Split by the start of our known copy tag style
	// Tag format from app_actions: [black:#50fa7b] <copied:
	
	const tagStart = " [black:#50fa7b] <copied:"
	
	if idx := strings.Index(current, tagStart); idx != -1 {
		current = current[:idx]
	}
	
	f.View.SetText(current + " " + text)
}
