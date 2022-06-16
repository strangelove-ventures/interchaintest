package presenter

import (
	"fmt"
	"regexp"
	"strconv"
)

type Highlight struct {
	regionText string
	regions    []string
}

// NewHighlight returns a presenter that adds regions around text that matches searchTerm.
func NewHighlight(fullText string, searchTerm string) *Highlight {
	r, err := regexp.Compile(fmt.Sprintf(`(?is)(%s)`, regexp.QuoteMeta(searchTerm)))
	if err != nil {
		// Should always be valid given regexp.QuoteMeta above.
		panic(err)
	}
	h := &Highlight{}
	var i int
	text := r.ReplaceAllStringFunc(fullText, func(s string) string {
		region := strconv.Itoa(i)
		h.regions = append(h.regions, region)
		s = fmt.Sprintf(`["%s"]%s[""]`, region, s)
		i++
		return s
	})
	h.regionText = text

	return h
}

// Text returns the text decorated with tview.TextView regions given the "searchTerm" from NewHighlight.
func (h *Highlight) Text() string { return h.regionText }

// Regions returns all region ids.
// Meant to pair with (*tview.TextView).Highlight and ScrollToHighlight
// See https://github.com/rivo/tview/wiki/TextView for an example.
func (h *Highlight) Regions() []string { return h.regions }
