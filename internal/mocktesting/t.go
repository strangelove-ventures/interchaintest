package mocktesting

import (
	"fmt"
	"runtime"
	"time"
)

// T satisfies a subset of testing.TB useful for tests around how ibctest interacts with instances of testing.T.
//
// The methods that are unique to T are RunCleanups and Simulate
type T struct {
	name string

	HelperCalled bool

	failCalled bool

	cleanups    []func()
	ranCleanups bool

	Logs   []string
	Errors []string
	Skips  []string

	// ParallelDelay sets how long to sleep on a call to t.Parallel,
	// for tests that need to simulate t.Parallel blocking.
	ParallelDelay time.Duration

	// runner is set when using the RunTest function.
	// Methods on T that require control flow will panic if runner is nil.
	// runner *runner
	simulating bool
}

// NewT returns a new T with the given name.
func NewT(name string) *T {
	if name == "" {
		panic(fmt.Errorf("NewT: name must not be empty"))
	}
	return &T{name: name}
}

// Helper sets the t.HelperCalled field to true.
func (t *T) Helper() {
	t.HelperCalled = true
}

// Name returns the name provided to NewT.
func (t *T) Name() string {
	return t.name
}

// Failed reports whether t.Errorf was ever called.
func (t *T) Failed() bool {
	return t.failCalled || len(t.Errors) > 0
}

// Skipped reports whether t.Skip was ever called.
func (t *T) Skipped() bool {
	return len(t.Skips) > 0
}

// Cleanup adds f to the list of cleanup functions to invoke.
// To actually invoke the functions, use t.RunCleanups.
func (t *T) Cleanup(f func()) {
	t.cleanups = append(t.cleanups, f)
}

// RunCleanups runs all the functions passed to t.Cleanup,
// in reverse order just like the real testing.T.
func (t *T) RunCleanups() {
	if t.ranCleanups {
		panic(fmt.Errorf("(*mocktesting.T).RunCleanups may only be called once per instance"))
	}

	// Cleanups are run in reverse order of insertion.
	for i := len(t.cleanups) - 1; i >= 0; i-- {
		t.cleanups[i]()
	}
	t.cleanups = nil // Can't use the slice again, so make it eligible for GC anyway.
	t.ranCleanups = true
}

// Logf appends the formatted message to t.Logs.
func (t *T) Logf(format string, args ...any) {
	t.Logs = append(t.Logs, fmt.Sprintf(format, args...))
}

// Errorf appends the formatted error message to t.Errors.
func (t *T) Errorf(format string, args ...any) {
	t.Errors = append(t.Errors, fmt.Sprintf(format, args...))
}

// Skip appends fmt.Sprint(args...) to t.Skips.
// Skip panics if called outside the context of RunTest.
func (t *T) Skip(args ...any) {
	if !t.simulating {
		panic(fmt.Errorf("(*mocktesting.T).Skip may only be called from inside (*mocktesting.T).Simulate"))
	}

	t.Skips = append(t.Skips, fmt.Sprint(args...))
	runtime.Goexit()
}

// Fail marks t as failed, without any control flow interaction.
func (t *T) Fail() {
	t.failCalled = true
}

// FailNow marks T as failed and stops execution.
// FailNow panics if called outside the context of RunTest.
func (t *T) FailNow() {
	if !t.simulating {
		panic(fmt.Errorf("(*mocktesting.T).FailNow may only be called from inside (*mocktesting.T).Simulate"))
	}

	t.failCalled = true
	runtime.Goexit()
}

// Parallel blocks for the configured t.ParallelDelay and then returns.
func (t *T) Parallel() {
	time.Sleep(t.ParallelDelay)
}

// Simulate executes the given function in its own goroutine,
// which is necessary for exercising methods that affect control flow,
// such as t.Skip or t.FailNow.
//
// Simulate also runs t.RunCleanups after fn's execution finishes.
func (t *T) Simulate(fn func()) {
	t.simulating = true
	defer func() {
		t.simulating = false
	}()

	ch := make(chan struct{})

	go func() {
		defer close(ch)
		defer t.RunCleanups()
		fn()
	}()

	<-ch
}
