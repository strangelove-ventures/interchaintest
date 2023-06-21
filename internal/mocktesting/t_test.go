package mocktesting_test

import (
	"testing"
	"time"

	"github.com/strangelove-ventures/interchaintest/v5/internal/mocktesting"
	"github.com/stretchr/testify/require"
)

func TestT_Name(t *testing.T) {
	mt := mocktesting.NewT("foo")
	require.Equal(t, mt.Name(), "foo")

	require.Panics(t, func() {
		_ = mocktesting.NewT("")
	}, "empty name should be rejected")
}

func TestT_RunCleanups(t *testing.T) {
	t.Run("panics if called multiple times", func(t *testing.T) {
		mt := mocktesting.NewT("x")
		require.NotPanics(t, func() {
			mt.RunCleanups()
		}, "first call to RunCleanups must succeed")
		require.Panics(t, func() {
			mt.RunCleanups()
		}, "subsequent call to RunCleanups must panic")
	})

	t.Run("executes cleanups in reverse order", func(t *testing.T) {
		var nums []int

		mt := mocktesting.NewT("x")
		mt.Cleanup(func() {
			nums = append(nums, 1)
		})
		mt.Cleanup(func() {
			nums = append(nums, 2)
		})

		mt.RunCleanups()

		require.Equal(t, []int{2, 1}, nums)
	})
}

func TestT_Logf(t *testing.T) {
	mt := mocktesting.NewT("x")

	require.Empty(t, mt.Logs)

	mt.Logf("1 + 2 = %d", 3)
	mt.Logf("4 + 5 = %d", 9)
	require.Equal(t, []string{"1 + 2 = 3", "4 + 5 = 9"}, mt.Logs)

	// Logging should not cause a failure, of course.
	require.False(t, mt.Failed())
}

func TestT_Errorf(t *testing.T) {
	mt := mocktesting.NewT("x")

	require.Empty(t, mt.Errors)
	require.False(t, mt.Failed())

	mt.Errorf("non-zero exit: %d", 1)
	require.Equal(t, []string{"non-zero exit: 1"}, mt.Errors)
	require.True(t, mt.Failed())

	// Valid to have more than one Errorf call.
	mt.Errorf("non-zero exit: %d", 3)
	require.Equal(t, []string{"non-zero exit: 1", "non-zero exit: 3"}, mt.Errors)
	require.True(t, mt.Failed())
}

func TestT_Skip(t *testing.T) {
	t.Run("panics outside of Simulate", func(t *testing.T) {
		mt := mocktesting.NewT("x")
		require.Panics(t, func() {
			mt.Skip()
		})
	})

	t.Run("stops control flow inside Simulate", func(t *testing.T) {
		continuedAfterSkip := false
		mt := mocktesting.NewT("x")

		mt.Simulate(func() {
			mt.Skip()
			continuedAfterSkip = true
		})

		require.False(t, continuedAfterSkip, "control flow continued after t.Skip")
	})

	t.Run("appends to skip messages", func(t *testing.T) {
		mt := mocktesting.NewT("x")

		mt.Simulate(func() {
			mt.Skip("foo")
		})

		require.Equal(t, []string{"foo"}, mt.Skips)
		require.True(t, mt.Skipped())
	})
}

func TestT_Fail(t *testing.T) {
	mt := mocktesting.NewT("x")

	require.False(t, mt.Failed())
	mt.Fail()
	require.True(t, mt.Failed())
}

func TestT_FailNow(t *testing.T) {
	t.Run("panics outside of Simulate", func(t *testing.T) {
		mt := mocktesting.NewT("x")
		require.Panics(t, func() {
			mt.FailNow()
		})
	})

	t.Run("stops control flow inside Simulate", func(t *testing.T) {
		continuedAfterFailNow := false
		mt := mocktesting.NewT("x")

		mt.Simulate(func() {
			mt.FailNow()
			continuedAfterFailNow = true
		})

		require.False(t, continuedAfterFailNow, "control flow continued after t.FailNow")
		require.True(t, mt.Failed())
	})

	t.Run("does not append error messages", func(t *testing.T) {
		mt := mocktesting.NewT("x")

		mt.Simulate(func() {
			mt.FailNow()
		})

		require.Empty(t, mt.Errors)
	})
}

func TestT_Parallel(t *testing.T) {
	const delay = 10 * time.Millisecond

	mt := mocktesting.NewT("x")
	before := time.Now()
	mt.Parallel()
	after := time.Now()

	require.GreaterOrEqual(t, before.Add(delay).UnixNano(), after.UnixNano(), "mt.Parallel did not delay the minimum of 10ms")
}

func TestT_Simulate(t *testing.T) {
	t.Run("runs cleanups", func(t *testing.T) {
		cleanedUp := false

		mt := mocktesting.NewT("x")
		mt.Cleanup(func() {
			cleanedUp = true
		})

		mt.Simulate(func() {})

		require.True(t, cleanedUp)
	})
}
