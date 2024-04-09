package presenter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHighlighter_Text(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		h := NewHighlight("\tDay ")

		got, regions := h.Text(highlighterFixture)

		const want = `Tomorrow, and tomorrow, and tomorrow,
Creeps in this petty pace from ["0"]day[""] to ["1"]day[""],
To the last syllable of recorded time;
And all our yester["2"]day[""]s have lighted fools
The way to dusty death. Out, out, brief candle!`
		require.Equal(t, want, got)
		require.Equal(t, []string{"0", "1", "2"}, regions)
	})

	t.Run("ignores regex meta characters", func(t *testing.T) {
		h := NewHighlight("(one paren")
		got, regions := h.Text("(one paren")

		require.Equal(t, `["0"](one paren[""]`, got)
		require.Equal(t, []string{"0"}, regions)
	})

	t.Run("missing search term", func(t *testing.T) {
		h := NewHighlight("")
		got, regions := h.Text(highlighterFixture)

		require.Empty(t, regions)
		require.Equal(t, highlighterFixture, got)

		h = NewHighlight("          \t")
		got, regions = h.Text(highlighterFixture)

		require.Empty(t, regions)
		require.Equal(t, highlighterFixture, got)
	})
}

const highlighterFixture = `Tomorrow, and tomorrow, and tomorrow,
Creeps in this petty pace from day to day,
To the last syllable of recorded time;
And all our yesterdays have lighted fools
The way to dusty death. Out, out, brief candle!`
