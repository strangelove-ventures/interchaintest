package testreporter_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/strangelove-ventures/ibctest/label"
	"github.com/strangelove-ventures/ibctest/testreporter"
	"github.com/stretchr/testify/require"
)

// nopCloser wraps an io.Writer to provide a Close method that always returns nil.
type nopCloser struct {
	io.Writer
}

func (n nopCloser) Close() error {
	return nil
}

// mockT is a stand-in testreporter.T for asserting against the reporter's behavior.
type mockT struct {
	name            string
	cleanups        []func()
	failed, skipped bool

	errors, skips []string

	parallelDelay time.Duration
}

func (t *mockT) Name() string {
	return t.name
}

func (t *mockT) Cleanup(fn func()) {
	t.cleanups = append(t.cleanups, fn)
}

func (t *mockT) Failed() bool  { return t.failed }
func (t *mockT) Skipped() bool { return t.skipped }

func (t *mockT) RunCleanups() {
	// Cleanup calls are executed most recently first.
	for i := len(t.cleanups) - 1; i >= 0; i-- {
		t.cleanups[i]()
	}
}

func (t *mockT) Errorf(format string, args ...interface{}) {
	t.errors = append(t.errors, fmt.Sprintf(format, args...))
}

func (t *mockT) Skip(args ...interface{}) {
	t.skips = append(t.skips, fmt.Sprint(args...))
	t.skipped = true
}

// FailNow just marks t as failed without any control flow influences.
// FailNow on a real *testing.T would call runtime.Goexit,
// but we don't need that for these tests (yet).
func (t *mockT) FailNow() {
	t.failed = true
}

// Parallel blocks for the configured t.parallelDelay and then returns.
func (t *mockT) Parallel() {
	time.Sleep(t.parallelDelay)
}

// ReporterMessages decodes all the messages from r.
// If anything fails, t.Fatal is called.
func ReporterMessages(t *testing.T, r io.Reader) []testreporter.Message {
	t.Helper()

	var msgs []testreporter.Message

	dec := json.NewDecoder(r)
	for {
		var wm testreporter.WrappedMessage
		if err := dec.Decode(&wm); err != nil {
			if err == io.EOF {
				return msgs
			}

			t.Fatalf("Failed to decode message: %v", err)
		}

		msgs = append(msgs, wm.Message)
	}
}

// Check message content and timestamps for a typical, basic, passing test.
func TestReporter_TrackPassingSingleTest(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	beforeStartSuite := time.Now()
	r := testreporter.NewReporter(nopCloser{Writer: buf})
	afterStartSuite := time.Now()

	mt := &mockT{name: "my_test"}

	beforeStartTest := time.Now()
	r.TrackTest(mt, label.Timeout)
	afterStartTest := time.Now()

	time.Sleep(10 * time.Millisecond)

	beforeFinishTest := time.Now()
	mt.RunCleanups()
	afterFinishTest := time.Now()

	beforeFinishSuite := time.Now()
	require.NoError(t, r.Close())
	afterFinishSuite := time.Now()

	msgs := ReporterMessages(t, buf)
	require.Len(t, msgs, 4)

	beginSuiteMsg := msgs[0].(testreporter.BeginSuiteMessage)
	requireTimeInRange(t, beginSuiteMsg.StartedAt, beforeStartSuite, afterStartSuite)

	beginTestMsg := msgs[1].(testreporter.BeginTestMessage)
	require.Equal(t, beginTestMsg.Name, "my_test")
	require.Equal(t, beginTestMsg.Labels, testreporter.LabelSet{Test: []label.Test{label.Timeout}})
	requireTimeInRange(t, beginTestMsg.StartedAt, beforeStartTest, afterStartTest)

	finishTestMsg := msgs[2].(testreporter.FinishTestMessage)
	require.Equal(t, finishTestMsg.Name, "my_test")
	require.False(t, finishTestMsg.Failed)
	require.False(t, finishTestMsg.Skipped)
	requireTimeInRange(t, finishTestMsg.FinishedAt, beforeFinishTest, afterFinishTest)

	finishSuiteMsg := msgs[3].(testreporter.FinishSuiteMessage)
	requireTimeInRange(t, finishSuiteMsg.FinishedAt, beforeFinishSuite, afterFinishSuite)
}

func TestReporter_TrackFailingSingleTest(t *testing.T) {
	t.Parallel()

	// Most of the timing was validated in TrackPassingSingleTest,
	// so this only adds assertions around the failure that occurs.

	buf := new(bytes.Buffer)
	r := testreporter.NewReporter(nopCloser{Writer: buf})

	mt := &mockT{name: "my_test"}

	r.TrackTest(mt)

	time.Sleep(10 * time.Millisecond)

	beforeFailure := time.Now()
	require.Fail(r.TestifyT(mt), "forced failure")
	afterFailure := time.Now()

	mt.RunCleanups()

	require.NoError(t, r.Close())

	msgs := ReporterMessages(t, buf)
	require.Len(t, msgs, 5)

	testErrorMsg := msgs[2].(testreporter.TestErrorMessage)
	require.Equal(t, testErrorMsg.Name, "my_test")
	// require.Fail adds some detail to the error message that complicates a plain string equality check.
	require.Contains(t, testErrorMsg.Message, "forced failure")
	requireTimeInRange(t, testErrorMsg.When, beforeFailure, afterFailure)

	finishTestMsg := msgs[3].(testreporter.FinishTestMessage)
	require.Equal(t, finishTestMsg.Name, "my_test")
	require.True(t, finishTestMsg.Failed)
	require.False(t, finishTestMsg.Skipped)
}

// Check that TrackParallel logs the pause and continue messages.
func TestReporter_TrackParallel(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	r := testreporter.NewReporter(nopCloser{Writer: buf})

	// The underlying call to mt.Parallel will block for this duration.
	parallelDelay := 50 * time.Millisecond
	mt := &mockT{name: "my_test", parallelDelay: parallelDelay}
	r.TrackTest(mt)

	beforeParallel := time.Now()
	r.TrackParallel(mt)
	afterParallel := time.Now()

	mt.RunCleanups()
	require.NoError(t, r.Close())

	msgs := ReporterMessages(t, buf)
	require.Len(t, msgs, 6)

	beginTestMsg := msgs[1].(testreporter.BeginTestMessage)
	require.Equal(t, beginTestMsg.Name, "my_test")

	pauseTestMsg := msgs[2].(testreporter.PauseTestMessage)
	require.Equal(t, pauseTestMsg.Name, "my_test")
	requireTimeInRange(t, pauseTestMsg.When, beforeParallel, beforeParallel.Add(parallelDelay))

	continueTestMsg := msgs[3].(testreporter.ContinueTestMessage)
	require.Equal(t, continueTestMsg.Name, "my_test")
	requireTimeInRange(t, continueTestMsg.When, afterParallel.Add(-parallelDelay), afterParallel)

	finishTestMsg := msgs[4].(testreporter.FinishTestMessage)
	require.Equal(t, finishTestMsg.Name, "my_test")
}

// Check that TrackSkip skips the underlying test.
func TestReporter_TrackSkip(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	r := testreporter.NewReporter(nopCloser{Writer: buf})

	mt := &mockT{name: "my_test"}

	r.TrackTest(mt)

	beforeSkip := time.Now()
	time.Sleep(5 * time.Millisecond)
	r.TrackSkip(mt, "skipping %s", "for reasons")
	time.Sleep(5 * time.Millisecond)
	afterSkip := time.Now()

	mt.RunCleanups()
	require.NoError(t, r.Close())

	msgs := ReporterMessages(t, buf)
	require.Len(t, msgs, 5)

	testSkipMsg := msgs[2].(testreporter.TestSkipMessage)
	require.Equal(t, testSkipMsg.Name, "my_test")
	require.Equal(t, testSkipMsg.Message, "skipping for reasons")
	requireTimeInRange(t, testSkipMsg.When, beforeSkip, afterSkip)

	finishTestMsg := msgs[3].(testreporter.FinishTestMessage)
	require.Equal(t, finishTestMsg.Name, "my_test")
	require.False(t, finishTestMsg.Failed)
	require.True(t, finishTestMsg.Skipped)

	require.Equal(t, mt.skips, []string{"skipping for reasons"})
	require.True(t, mt.skipped)
}

// Check that calling (*Reporter).TestifyT(t).Errorf
// actually calls Errorf on t.
func TestReporter_Errorf(t *testing.T) {
	buf := new(bytes.Buffer)
	r := testreporter.NewReporter(nopCloser{Writer: buf})

	mt := &mockT{name: "my_test"}
	r.TrackTest(mt)
	r.TestifyT(mt).Errorf("failed? %t", true)
	mt.RunCleanups()
	require.NoError(t, r.Close())

	require.Equal(t, mt.errors, []string{"failed? true"})
}

func TestReporter_RelayerExec(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	r := testreporter.NewReporter(nopCloser{Writer: buf})

	mt := &mockT{name: "my_test"}

	r.TrackTest(mt)

	execStartedAt := time.Now()
	execFinishedAt := execStartedAt.Add(time.Second)
	r.RelayerExecReporter(mt).TrackRelayerExec(
		"my_container",
		[]string{"rly", "fake_command"},
		"stdout", "stderr",
		1,
		execStartedAt, execFinishedAt,
		nil,
	)

	mt.RunCleanups()

	require.NoError(t, r.Close())

	msgs := ReporterMessages(t, buf)
	require.Len(t, msgs, 5)

	diff := cmp.Diff(testreporter.RelayerExecMessage{
		Name:          "my_test",
		StartedAt:     execStartedAt,
		FinishedAt:    execFinishedAt,
		ContainerName: "my_container",
		Command:       []string{"rly", "fake_command"},
		Stdout:        "stdout",
		Stderr:        "stderr",
		ExitCode:      1,
		Error:         "",
	}, msgs[2].(testreporter.RelayerExecMessage))
	require.Empty(t, diff)
}

// requireTimeInRange is a helper to assert that a time occurs between a given start and end.
func requireTimeInRange(t *testing.T, actual, notBefore, notAfter time.Time) {
	t.Helper()

	require.Falsef(t, actual.Before(notBefore), "time %v should have occurred on or after %v", actual, notBefore)
	require.Falsef(t, actual.After(notAfter), "time %v should have occurred on or before %v", actual, notAfter)
}
