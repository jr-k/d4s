package inspect

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
)

// SearchController manages search logic, highlighting, and navigation for a TextView
type SearchController struct {
	Filter        string
	CurrentMatch  int
	SearchMatches []string // List of region IDs
}

// NewSearchController creates a new search controller
func NewSearchController() *SearchController {
	return &SearchController{
		CurrentMatch: -1,
	}
}

// ApplyFilter sets the current filter string and resets state
func (s *SearchController) ApplyFilter(filter string) {
	s.Filter = filter
	s.CurrentMatch = -1
	s.SearchMatches = nil
}

// ProcessContent scans the content for the filter string, wraps matches in region tags,
// and returns the modified content plus a list of region IDs.
// Note: This operates on the string with tview color tags already applied.
// Matches inside tags or spanning tags may be missed or cause artifacts, 
// but this is a tradeoff for performance/simplicity in this context.
func (s *SearchController) ProcessContent(content, filter string) (string, []string) {
	if filter == "" {
		return content, nil
	}

	var sb strings.Builder
	var matches []string
	
	// Case-insensitive search
	lowerContent := strings.ToLower(content)
	lowerFilter := strings.ToLower(filter)
	filterLen := len(filter)

	lastIdx := 0
	matchCount := 0

	for {
		idx := strings.Index(lowerContent[lastIdx:], lowerFilter)
		if idx == -1 {
			sb.WriteString(content[lastIdx:])
			break
		}

		absIdx := lastIdx + idx
		
		// Append text before match
		sb.WriteString(content[lastIdx:absIdx])

		// Create Region ID
		regionID := fmt.Sprintf("match_%d", matchCount)
		matches = append(matches, regionID)

		// Append Match with Region Tags
		// We use the original content string for the match to preserve case
		matchEnd := absIdx + filterLen
		sb.WriteString(fmt.Sprintf(`["%s"]%s[""]`, regionID, content[absIdx:matchEnd]))

		lastIdx = matchEnd
		matchCount++
	}

	return sb.String(), matches
}

// highlightCurrent focuses the view on the current match
func (s *SearchController) highlightCurrent(view *tview.TextView) {
	if len(s.SearchMatches) == 0 {
		return
	}

	// Bounds check
	if s.CurrentMatch < 0 {
		s.CurrentMatch = 0
	}
	if s.CurrentMatch >= len(s.SearchMatches) {
		s.CurrentMatch = len(s.SearchMatches) - 1
	}

	// Highlight ONLY the current match to draw attention
	view.Highlight(s.SearchMatches[s.CurrentMatch])
	
	// Ensure it's visible
	view.ScrollToHighlight()
}

// NextMatch moves selection to the next result
func (s *SearchController) NextMatch(view *tview.TextView) {
	if len(s.SearchMatches) == 0 {
		return
	}
	s.CurrentMatch++
	if s.CurrentMatch >= len(s.SearchMatches) {
		s.CurrentMatch = 0
	}
	s.highlightCurrent(view)
}

// PrevMatch moves selection to the previous result
func (s *SearchController) PrevMatch(view *tview.TextView) {
	if len(s.SearchMatches) == 0 {
		return
	}
	s.CurrentMatch--
	if s.CurrentMatch < 0 {
		s.CurrentMatch = len(s.SearchMatches) - 1
	}
	s.highlightCurrent(view)
}
