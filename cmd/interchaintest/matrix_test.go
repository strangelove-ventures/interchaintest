package interchaintest_test

import (
	_ "embed"
	"encoding/json"
	"testing"

	interchaintest "github.com/strangelove-ventures/interchaintest/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// Embed the matrix files as strings since they aren't intended to be changed.
var (
	//go:embed example_matrix.json
	exampleMatrix string

	//go:embed example_matrix_custom.json
	exampleMatrixCustom string
)

func TestMatrixValid(t *testing.T) {
	type matrix struct {
		ChainSets [][]*interchaintest.ChainSpec
	}

	for _, tc := range []struct {
		name string
		j    string
	}{
		{name: "example_matrix.json", j: exampleMatrix},
		{name: "example_matrix_custom.json", j: exampleMatrixCustom},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var m matrix
			require.NoError(t, json.Unmarshal([]byte(tc.j), &m))

			for i, cs := range m.ChainSets {
				for j, c := range cs {
					_, err := c.Config(zaptest.NewLogger(t))
					require.NoErrorf(t, err, "failed to generate config from chainset at index %d-%d", i, j)
				}
			}
		})
	}
}
