//go:build unit
// +build unit

package addresscodec

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeBase58(t *testing.T) {
	tt := []struct {
		description    string
		input          []byte
		expectedOutput string
	}{
		{
			description:    "successful encode with XRP alphabet - 1",
			input:          []byte("rDTXLQ7ZKZVKz33zJbHjgVShjsBnqMBhmN"),
			expectedOutput: "s2Fku4vaPpFiqqXdAD3V5rYrSx5a9h9qvUJW3423akZSCeD",
		},
		{
			description:    "successful encode with XRP alphabet - 2",
			input:          []byte("rJrpjzcxwQxokkqPxm62o5rtNfe2XimrTr"),
			expectedOutput: "s2i2Jk6bF44eDSXnnMjxeVhnYZ3qmbteqesuhS6Tz7CSd9j",
		},
		{
			description:    "successful encode with XRP alphabet - 3",
			input:          []byte("rUxb5vn9fGYRV3KZcnu3JLM4q5DTnNSavf"),
			expectedOutput: "s2uiNSCBQnQfsVtnX49adC9QqtWNP8upC16t7GFLrmbR7tm",
		},
	}

	for _, tc := range tt {
		t.Run(tc.description, func(t *testing.T) {
			require.Equal(t, tc.expectedOutput, EncodeBase58(tc.input))
		})
	}
}

func TestDecodeBase58(t *testing.T) {
	tt := []struct {
		description    string
		input          string
		expectedOutput []byte
	}{
		{
			description:    "successful decode with XRP alphabet - 1",
			input:          "s2Fku4vaPpFiqqXdAD3V5rYrSx5a9h9qvUJW3423akZSCeD",
			expectedOutput: []byte("rDTXLQ7ZKZVKz33zJbHjgVShjsBnqMBhmN"),
		},
		{
			description:    "successful decode with XRP alphabet - 2",
			input:          "s2i2Jk6bF44eDSXnnMjxeVhnYZ3qmbteqesuhS6Tz7CSd9j",
			expectedOutput: []byte("rJrpjzcxwQxokkqPxm62o5rtNfe2XimrTr"),
		},
		{
			description:    "successful decode with XRP alphabet - 3",
			input:          "s2uiNSCBQnQfsVtnX49adC9QqtWNP8upC16t7GFLrmbR7tm",
			expectedOutput: []byte("rUxb5vn9fGYRV3KZcnu3JLM4q5DTnNSavf"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.description, func(t *testing.T) {
			require.Equal(t, tc.expectedOutput, DecodeBase58(tc.input))
		})
	}
}
