package presenter

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Highlight struct {
	rx *regexp.Regexp
}

// NewHighlight returns a presenter that adds regions around text that matches searchTerm.
func NewHighlight(searchTerm string) *Highlight {
	searchTerm = strings.TrimSpace(searchTerm)
	if searchTerm == "" {
		return &Highlight{}
	}
	// Should always be valid with regexp.QuoteMeta.
	return &Highlight{rx: regexp.MustCompile(fmt.Sprintf(`(?i)(%s)`, regexp.QuoteMeta(searchTerm)))}
}

// Text returns the text decorated with tview.TextView regions given the "searchTerm" from NewHighlight.
// The second return value is the highlighted region ids for use with *(tview.TextView).Highlight.
// See https://github.com/rivo/tview/wiki/TextView for more info about regions.
func (h *Highlight) Text(text string) (string, []string) {
	if h.rx == nil {
		return text, nil
	}
	var (
		region    int
		regionIDs []string
	)
	final := h.rx.ReplaceAllStringFunc(text, func(s string) string {
		id := strconv.Itoa(region)
		regionIDs = append(regionIDs, id)
		s = fmt.Sprintf(`["%s"]%s[""]`, id, s)
		region++
		return s
	})
	return final, regionIDs
}
