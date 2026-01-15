package inspect

import (
	"fmt"
	"strings"
)

// FormatInspectorTitle generates the standard title string for inspectors
// Format: Action(subject) [Mode] <Search>
// Colors:
// - Action, brackets, parenthesis: Blue
// - Subject, Search text: Orange
// - Mode, Counters: White
func FormatInspectorTitle(action, subject, mode, filter string, matchIndex, matchCount int) string {
	// Special handling for @ separator in subject to make it white
	if strings.Contains(subject, "@") {
		subject = strings.ReplaceAll(subject, "@", "[white] @ [orange]")
	}
	
	title := fmt.Sprintf("[blue]%s([orange]%s[blue])", action, subject)
	modeStr := fmt.Sprintf(" [blue][[white]%s[blue]]", mode)
	
	search := ""
	if filter != "" {
		idx := 0
		if matchCount > 0 {
			idx = matchIndex + 1
		}
		
		search = fmt.Sprintf(" [blue]<[orange]%s [%d:%d][blue]>", filter, idx, matchCount)
	}

	return fmt.Sprintf(" %s%s%s ", title, modeStr, search)
}
