// Package testreporter contains a Reporter for collecting detailed test reports.
package testreporter

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// T is a subset of testing.TB,
// representing only the methods required by the reporter.
type T interface {
	Name() string
	Cleanup(func())

	Failed() bool
	Skipped() bool
}

type Reporter struct {
	w io.WriteCloser

	in chan Message

	writerDone chan error
}

func NewReporter(w io.WriteCloser) *Reporter {
	r := &Reporter{
		w: w,

		in:         make(chan Message, 256), // Arbitrary size that seems unlikely to be filled.
		writerDone: make(chan error, 1),
	}

	go r.write()
	r.in <- BeginSuiteMessage{StartedAt: time.Now()}

	return r
}

// write runs in its own goroutine to continually output reporting messages.
func (r *Reporter) write() {
	enc := json.NewEncoder(r.w)
	enc.SetEscapeHTML(false)

	for m := range r.in {
		// Encode the message with an outer type field,
		// so decoders can inspect the type and decide how to unmarshal.
		j := struct {
			Type string
			Message
		}{
			Type:    m.typ(),
			Message: m,
		}
		if err := enc.Encode(j); err != nil {
			panic(fmt.Errorf("reporter failed to encode message; tests cannot continue: %w", err))
		}
	}

	// Before closing the writer
	r.writerDone <- r.w.Close()
}

// Close closes the reporter and blocks until its results are flushed
// to the underlying writer.
func (r *Reporter) Close() error {
	r.in <- FinishSuiteMessage{
		FinishedAt: time.Now(),
	}
	close(r.in)
	return <-r.writerDone
}

// TrackTest tracks the test start and finish time.
func (r *Reporter) TrackTest(t T) {
	name := t.Name()
	r.in <- BeginTestMessage{
		Name:      name,
		StartedAt: time.Now(),
	}
	t.Cleanup(func() {
		r.in <- FinishTestMessage{
			Name:       name,
			FinishedAt: time.Now(),

			Failed:  t.Failed(),
			Skipped: t.Skipped(),
		}
	})
}
