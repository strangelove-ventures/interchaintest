package presenter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHighlighter_Text(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		h := NewHighlight(highlighterFixture, "Day")

		const want = `Tomorrow, and tomorrow, and tomorrow,
Creeps in this petty pace from ["0"]day[""] to ["1"]day[""],
To the last syllable of recorded time;
And all our yester["2"]day[""]s have lighted fools
The way to dusty death. Out, out, brief candle!`

		require.Equal(t, want, h.Text())
	})

	t.Run("ignores regex meta characters", func(t *testing.T) {
		h := NewHighlight("(one paren", "(one paren")

		require.Equal(t, `["0"](one paren[""]`, h.Text())
	})
}

func TestHighlighter_Regions(t *testing.T) {
	h := NewHighlight(highlighterFixture, "Day")

	require.Equal(t, []string{"0", "1", "2"}, h.Regions())
}

const highlighterFixture = `Tomorrow, and tomorrow, and tomorrow,
Creeps in this petty pace from day to day,
To the last syllable of recorded time;
And all our yesterdays have lighted fools
The way to dusty death. Out, out, brief candle!`
