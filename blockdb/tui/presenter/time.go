package presenter

import "time"

// FormatTime returns a shortened local time.
func FormatTime(t time.Time) string {
	return t.Format("01-02 03:04PM MST")
}
