package testreporter_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/strangelove-ventures/ibc-test-framework/testreporter"
	"github.com/stretchr/testify/require"
)

func TestWrappedMessage_RoundTrip(t *testing.T) {
	tcs := []struct {
		Message testreporter.Message
	}{
		{Message: testreporter.BeginSuiteMessage{StartedAt: time.Now()}},
		{Message: testreporter.FinishSuiteMessage{FinishedAt: time.Now()}},
		{Message: testreporter.BeginTestMessage{Name: "foo", StartedAt: time.Now()}},
		{Message: testreporter.PauseTestMessage{Name: "foo", When: time.Now()}},
		{Message: testreporter.ContinueTestMessage{Name: "foo", When: time.Now()}},
		{Message: testreporter.FinishTestMessage{Name: "foo", FinishedAt: time.Now(), Skipped: true, Failed: true}},
		{Message: testreporter.TestErrorMessage{Name: "foo", When: time.Now(), Message: "something failed"}},
	}

	for _, tc := range tcs {
		wrapped := testreporter.JSONMessage(tc.Message)

		out, err := json.Marshal(wrapped)
		require.NoError(t, err)

		var unwrapped testreporter.WrappedMessage
		require.NoError(t, json.Unmarshal(out, &unwrapped))

		diff := cmp.Diff(wrapped, unwrapped)
		require.Empty(t, diff)
	}
}
