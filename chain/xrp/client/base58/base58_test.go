// MIT License

// Copyright (c) 2022 xyield

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package base58

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
			require.Equal(t, tc.expectedOutput, Encode(tc.input))
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
			require.Equal(t, tc.expectedOutput, Decode(tc.input))
		})
	}
}
