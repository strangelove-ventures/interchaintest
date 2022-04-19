package relayertest

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/strangelove-ventures/ibc-test-framework/ibc"
)

// GetLegacyTestCase looks up the given testCase and returns a function suitable for the existing cmd structure.
func GetLegacyTestCase(testCase string) (func(string, ibc.ChainFactory, ibc.RelayerImplementation) error, error) {
	var testFunc func(t TestingT, cf ibc.ChainFactory, rf ibc.RelayerFactory)
	switch testCase {
	case "RelayPacketTest":
		testFunc = TestRelayer_RelayPacket
	default:
		return nil, fmt.Errorf("no test case exists for %s", testCase)
	}

	return func(testName string, cf ibc.ChainFactory, ri ibc.RelayerImplementation) error {
		w := &tWrapper{
			name: testName,
			cf:   cf,
			rf:   ibc.NewBuiltinRelayerFactory(ri),
		}
		return w.run(testFunc)
	}, nil
}

// tWrapper satisfies TestingT for use in a non-test binary.
type tWrapper struct {
	name string
	cf   ibc.ChainFactory
	rf   ibc.RelayerFactory

	mu     sync.Mutex
	buf    bytes.Buffer
	failed bool
}

var _ TestingT = (*tWrapper)(nil)

func (w *tWrapper) Name() string {
	return w.name
}

func (w *tWrapper) Logf(format string, args ...interface{}) {
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	fmt.Fprintf(&w.buf, format, args...)
}

func (w *tWrapper) Errorf(format string, args ...interface{}) {
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.failed = true
	fmt.Fprintf(&w.buf, format, args...)
}

func (w *tWrapper) FailNow() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.failed = true
	runtime.Goexit()
}

// run runs the test function in a goroutine,
// so that t.FailNow can correctly call runtime.Goexit.
func (w *tWrapper) run(
	fn func(t TestingT, cf ibc.ChainFactory, rf ibc.RelayerFactory),
) error {
	ch := make(chan struct{})
	go func() {
		defer close(ch)

		fn(w, w.cf, w.rf)
	}()

	<-ch
	if w.failed {
		return fmt.Errorf("test %s failed; logs:\n%s", w.name, w.buf.String())
	}
	return nil
}
