package log

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogger(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		out := new(bytes.Buffer)
		lg := New(out, JSON, DebugLevel)
		lg.Debug("test", "debug")
		lg.Infof("test %s", "info")
		lg.Errorf("test %s", "error")

		type logLine struct {
			Level, Message string
		}

		var (
			dec   = json.NewDecoder(out)
			lines []logLine
		)

		for {
			var line logLine
			err := dec.Decode(&line)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					require.Fail(t, err.Error())
				}
				break
			}
			lines = append(lines, line)
		}

		require.Equal(t, logLine{"debug", "test debug"}, lines[0])
		require.Equal(t, logLine{"info", "test info"}, lines[1])
		require.Equal(t, logLine{"error", "test error"}, lines[2])
	})

	t.Run("console", func(t *testing.T) {
		out := new(bytes.Buffer)
		lg := New(out, Console, DebugLevel)
		lg.Debugf("test %s", "debug")
		lg.Info("test info")
		lg.Error("test error")

		require.Contains(t, out.String(), "test debug")
		require.Contains(t, out.String(), "test info")
		require.Contains(t, out.String(), "error")

	})

	t.Run("log level", func(t *testing.T) {
		out := new(bytes.Buffer)
		lg := New(out, Console, InfoLevel)
		lg.Debug("should not see me")

		require.Empty(t, out.String())
	})
}

func TestLogger_WithField(t *testing.T) {
	out := new(bytes.Buffer)
	lg := New(out, JSON, InfoLevel)
	lg.WithField("myField", "test").Info()

	var logLine struct {
		MyField string
	}
	err := json.Unmarshal(out.Bytes(), &logLine)

	require.NoError(t, err)
	require.Equal(t, "test", logLine.MyField)
}
