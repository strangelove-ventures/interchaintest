package testreporter

import (
	"encoding/json"
	"fmt"
	"time"
)

type Message interface {
	typ() string
}

type BeginSuiteMessage struct {
	StartedAt time.Time

	// TODO: it would be nice to embed the ibc-test-framework commit in this message,
	// but while https://github.com/golang/go/issues/33976 is outstanding,
	// we'll have to fall back to ldflags to embed it.
}

func (m BeginSuiteMessage) typ() string {
	return "BeginSuite"
}

type FinishSuiteMessage struct {
	FinishedAt time.Time
}

func (m FinishSuiteMessage) typ() string {
	return "FinishSuite"
}

type BeginTestMessage struct {
	Name      string
	StartedAt time.Time
}

func (m BeginTestMessage) typ() string {
	return "BeginTest"
}

type FinishTestMessage struct {
	Name       string
	FinishedAt time.Time

	Failed, Skipped bool
}

func (m FinishTestMessage) typ() string {
	return "FinishTest"
}

type PauseTestMessage struct {
	Name string
	When time.Time
}

func (m PauseTestMessage) typ() string {
	return "PauseTest"
}

type ContinueTestMessage struct {
	Name string
	When time.Time
}

func (m ContinueTestMessage) typ() string {
	return "ContinueTest"
}

type TestErrorMessage struct {
	Name    string
	When    time.Time
	Message string
}

func (m TestErrorMessage) typ() string {
	return "TestError"
}

// WrappedMessage wraps a Message with an outer Type field
// so that decoders can determine the underlying message's type.
type WrappedMessage struct {
	Type string
	Message
}

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
