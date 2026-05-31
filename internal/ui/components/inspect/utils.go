package inspect

import (
	"fmt"
	"strings"

	"github.com/jr-k/d4s/internal/ui/styles"
)

// FormatInspectorTitle generates the standard title string for inspectors
// Format: Action(subject) [Mode] <Search>
func FormatInspectorTitle(action, subject, mode, filter string, matchIndex, matchCount int) string {
	// Special handling for @ separator in subject
	if strings.Contains(subject, "@") {
		subject = strings.ReplaceAll(subject, "@", fmt.Sprintf("[%s] @ [%s]", styles.TagFg, styles.TagPink))
	}
	
	title := fmt.Sprintf("[%s::b]%s([%s]%s[%s])", styles.TagCyan, action, styles.TagPink, subject, styles.TagCyan)
	modeStr := fmt.Sprintf(" [%s::b][[%s]%s[%s]]", styles.TagCyan, styles.TagFg, mode, styles.TagCyan)
	
	search := ""
	if filter != "" {
		idx := 0
		if matchCount > 0 {
			idx = matchIndex + 1
		}
		
		search = fmt.Sprintf(" [%s::b]</[%s]%s [-][[%s]%d[-]:[%s]%d[-]][%s]>", styles.TagCyan, styles.TagPink, filter, styles.TagFg, idx, styles.TagFg, matchCount, styles.TagCyan)
	}

	return fmt.Sprintf(" %s%s%s ", title, modeStr, search)
}
