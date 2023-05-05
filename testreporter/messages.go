package testreporter

import (
	"encoding/json"
	"fmt"
	"time"
)

// Message is the sentinel interface to all testreporter messages.
type Message interface {
	typ() string
}

// BeginSuiteMessage indicates when the Reporter was initialized,
// which should correlate with the beginning of a TestMain function,
// or an init function in a normal test suite.
type BeginSuiteMessage struct {
	StartedAt time.Time

	// TODO: it would be nice to embed the interchaintest commit in this message,
	// but while https://github.com/golang/go/issues/33976 is outstanding,
	// we'll have to fall back to ldflags to embed it.
}

func (m BeginSuiteMessage) typ() string {
	return "BeginSuite"
}

// FinishSuiteMessage indicates the time the test suite has finished.
type FinishSuiteMessage struct {
	FinishedAt time.Time
}

func (m FinishSuiteMessage) typ() string {
	return "FinishSuite"
}

// BeginTestMessage indicates the beginning of a single test.
// If the test uses t.Parallel (via (*Reporter).TrackParallel),
// the reporter will also track a PauseTestMessage and a ContinueTestMessage.
type BeginTestMessage struct {
	Name      string
	StartedAt time.Time
}

func (m BeginTestMessage) typ() string {
	return "BeginTest"
}

// FinishTestMessage is tracked at the end of a single test.
type FinishTestMessage struct {
	Name       string
	FinishedAt time.Time

	Failed, Skipped bool
}

func (m FinishTestMessage) typ() string {
	return "FinishTest"
}

// PauseTestMessage indicates that a test is entering parallel mode
// and waiting for its turn to continue execution.
type PauseTestMessage struct {
	Name string
	When time.Time
}

func (m PauseTestMessage) typ() string {
	return "PauseTest"
}

// ContinueTestMessage indicates that a test has resumed execution
// after a call to t.Parallel.
type ContinueTestMessage struct {
	Name string
	When time.Time
}

func (m ContinueTestMessage) typ() string {
	return "ContinueTest"
}

// TestErrorMessage is tracked when a Reporter's TestifyT().Errorf method is called.
// This is the intended usage of a Reporter with require:
//
//	req := require.New(rep.TestifyT(t))
//	req.NoError(foo())
//
// If req.NoError fails, then rep will track a TestErrorMessage.
type TestErrorMessage struct {
	Name    string
	When    time.Time
	Message string
}

func (m TestErrorMessage) typ() string {
	return "TestError"
}

// TestSkipMessage is tracked when a Reporter's TrackSkip method is called.
// This allows the report to track the reason a test was skipped.
type TestSkipMessage struct {
	Name    string
	When    time.Time
	Message string
}

func (m TestSkipMessage) typ() string {
	return "TestSkip"
}

// RelayerExecMessage is the result of executing a relayer command.
// This message is populated through the RelayerExecReporter type,
// which is returned by the Reporter's RelayerExecReporter method.
type RelayerExecMessage struct {
	Name string // Test name, but "Name" for consistency.

	StartedAt, FinishedAt time.Time

	ContainerName string `json:",omitempty"`

	Command []string

	Stdout, Stderr string

	ExitCode int

	Error string `json:",omitempty"`
}

func (m RelayerExecMessage) typ() string {
	return "RelayerExec"
}

// WrappedMessage wraps a Message with an outer Type field
// so that decoders can determine the underlying message's type.
type WrappedMessage struct {
	Type string
	Message
}

// JSONMessage produces a WrappedMessage
// so that a stream of Message values can be distinguishedu
// by a top-level "Type" key.
func JSONMessage(m Message) WrappedMessage {
	return WrappedMessage{
		Type:    m.typ(),
		Message: m,
	}
}

func (m *WrappedMessage) UnmarshalJSON(b []byte) error {
	var outer struct {
		Type    string
		Message json.RawMessage
	}
	if err := json.Unmarshal(b, &outer); err != nil {
		return err
	}

	raw := outer.Message
	var err error
	var msg Message
	switch outer.Type {
	case "BeginSuite":
		x := BeginSuiteMessage{}
		err = json.Unmarshal(raw, &x)
		msg = x
	case "FinishSuite":
		x := FinishSuiteMessage{}
		err = json.Unmarshal(raw, &x)
		msg = x
	case "BeginTest":
		x := BeginTestMessage{}
		err = json.Unmarshal(raw, &x)
		msg = x
	case "FinishTest":
		x := FinishTestMessage{}
		err = json.Unmarshal(raw, &x)
		msg = x
	case "PauseTest":
		x := PauseTestMessage{}
		err = json.Unmarshal(raw, &x)
		msg = x
	case "ContinueTest":
		x := ContinueTestMessage{}
		err = json.Unmarshal(raw, &x)
		msg = x
	case "TestError":
		x := TestErrorMessage{}
		err = json.Unmarshal(raw, &x)
		msg = x
	case "TestSkip":
		x := TestSkipMessage{}
		err = json.Unmarshal(raw, &x)
		msg = x
	case "RelayerExec":
		x := RelayerExecMessage{}
		err = json.Unmarshal(raw, &x)
		msg = x
	default:
		return fmt.Errorf("unknown message type %q", outer.Type)
	}
	if err != nil {
		return fmt.Errorf("failed to unmarshal message for type %q: %w", outer.Type, err)
	}

	m.Type = outer.Type
	m.Message = msg
	return nil
}
