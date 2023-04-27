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

	Skip(...any)

	Parallel()

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

// trackTest tracks the test start and finish time.
// It also records which labels are present on the test.
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

// TrackParallel tracks when the pause begins for a parallel test
// and when it continues to resume.
func (r *Reporter) TrackParallel(t T) {
	name := t.Name()
	r.in <- PauseTestMessage{
		Name: name,
		When: time.Now(),
	}
	t.Parallel()
	r.in <- ContinueTestMessage{
		Name: name,
		When: time.Now(),
	}
}

// TrackSkip records a the reason for a test being skipped,
// and calls t.Skip.
func (r *Reporter) TrackSkip(t T, format string, args ...any) {
	now := time.Now()
	msg := fmt.Sprintf(format, args...)

	r.in <- TestSkipMessage{
		Name:    t.Name(),
		When:    now,
		Message: msg,
	}

	t.Skip(msg)
}

// RelayerExecReporter returns a RelayerExecReporter associated with t.
func (r *Reporter) RelayerExecReporter(t T) *RelayerExecReporter {
	return &RelayerExecReporter{r: r, testName: t.Name()}
}

// RelayerExecReporter provides one method that satisfies the ibc.RelayerExecReporter interface.
// Instances of RelayerExecReporter must be retrieved through (*Reporter).RelayerExecReporter.
type RelayerExecReporter struct {
	r        *Reporter
	testName string
}

// TrackRelayerExec tracks the execution of an individual relayer command.
func (r *RelayerExecReporter) TrackRelayerExec(
	containerName string,
	command []string,
	stdout, stderr string,
	exitCode int,
	startedAt, finishedAt time.Time,
	err error,
) {
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}
	r.r.in <- RelayerExecMessage{
		Name:          r.testName,
		StartedAt:     startedAt,
		FinishedAt:    finishedAt,
		ContainerName: containerName,
		Command:       command,
		Stdout:        stdout,
		Stderr:        stderr,
		ExitCode:      exitCode,
		Error:         errMsg,
	}
}

// TestifyT returns a TestifyReporter which will track logged errors in test.
// Typically you will use this with the New method on the require or assert package:
//
//	req := require.New(reporter.TestifyT(t))
//	// ...
//	req.NoError(err, "failed to foo the bar")
func (r *Reporter) TestifyT(t TestifyT) *TestifyReporter {
	return &TestifyReporter{r: r, t: t}
}

// TestifyT is a superset of the testify/require.TestingT interface.
type TestifyT interface {
	Name() string

	Errorf(format string, args ...any)
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
func (r *TestifyReporter) Errorf(format string, args ...any) {
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

// NewNopReporter returns a reporter that does not write anywhere.
func NewNopReporter() *Reporter {
	return NewReporter(newNopWriteCloser())
}

// nopWriteCloser is a no-op io.WriteCloser used to satisfy the interchaintest TestReporter type.
// Because the relayer is used in-process, all logs are simply streamed to the test log.
type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error {
	return nil
}

func newNopWriteCloser() io.WriteCloser {
	return nopWriteCloser{Writer: io.Discard}
}
