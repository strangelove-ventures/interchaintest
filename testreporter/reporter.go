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
// Allowing all writes to happen in a single goroutine avoids any lock contention
// that could happen with a mutex guarding concurrent writes to the io.Writer.
func (r *Reporter) write() {
	enc := json.NewEncoder(r.w)
	enc.SetEscapeHTML(false)

	for m := range r.in {
		if err := enc.Encode(JSONMessage(m)); err != nil {
			panic(fmt.Errorf("reporter failed to encode message; tests cannot continue: %w", err))
		}
	}

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

// TestifyT returns a TestifyReporter which will track logged errors in test.
// Typically you will use this with the New method on the require or assert package:
//     req := require.New(reporter.TestifyT(t))
//     // ...
//     req.NoError(err, "failed to foo the bar")
func (r *Reporter) TestifyT(t TestifyT) *TestifyReporter {
	return &TestifyReporter{r: r, t: t}
}

// TestifyT is a superset of the testify/require.TestingT interface.
type TestifyT interface {
	Name() string

	Errorf(format string, args ...interface{})
	FailNow()
}

// TestifyReporter wraps a Reporter to satisfy the testify/require.TestingT interface.
// This allows the reporter to track logged errors.
type TestifyReporter struct {
	r *Reporter
	t TestifyT
}

// Errorf records the error message in r's Reporter
// and then passes through to r's underlying TestifyT.
func (r *TestifyReporter) Errorf(format string, args ...interface{}) {
	now := time.Now()

	r.r.in <- TestErrorMessage{
		Name:    r.t.Name(),
		Message: fmt.Sprintf(format, args...),
		When:    now,
	}

	r.t.Errorf(format, args...)
}

// FailNow passes through to r's TestifyT.
// It does not need to log another message
// because r's Reporter should be tracking the test already.
func (r *TestifyReporter) FailNow() {
	r.t.FailNow()
}
