package testreporter

import "time"

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
