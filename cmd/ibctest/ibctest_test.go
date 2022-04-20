// Command ibctest allows running the relayer tests with command-line configuration.
package ibctest

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/strangelove-ventures/ibc-test-framework/relayertest"
)

// The value of the extra flags this test supports.
var mainFlags struct {
	MatrixFile string
}

// The value of the test matrix.
var testMatrix struct {
	Relayers []string

	ChainSets [][]ibc.BuiltinChainFactoryEntry

	// TODO: support a slice of ibc.CustomChainFactoryEntry too.
}

func TestMain(m *testing.M) {
	addFlags()
	flag.Parse()

	if err := setUpTestMatrix(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build test matrix: %v\n", err)
		os.Exit(1)
	}

	if err := validateTestMatrix(); err != nil {
		fmt.Fprintf(os.Stderr, "Test matrix invalid: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// setUpTestMatrix populates the testMatrix singleton with
// the parsed contents of the file referenced by the matrix flag,
// or with a small reasonable default of rly against one gaia-osmosis set.
func setUpTestMatrix() error {
	if mainFlags.MatrixFile == "" {
		fmt.Fprintln(os.Stderr, "No matrix file provided, falling back to rly with gaia and osmosis")

		testMatrix.Relayers = []string{"rly"}
		testMatrix.ChainSets = [][]ibc.BuiltinChainFactoryEntry{
			{
				{Name: "gaia", Version: "v6.0.4", ChainID: "cosmoshub-1004", NumValidators: 1, NumFullNodes: 1},
				{Name: "osmosis", Version: "v7.0.4", ChainID: "osmosis-1001", NumValidators: 1, NumFullNodes: 1},
			},
		}

		return nil
	}

	// Otherwise parse the given file.
	fmt.Fprintf(os.Stderr, "Loading matrix file from %s\n", mainFlags.MatrixFile)
	j, err := os.ReadFile(mainFlags.MatrixFile)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(j, &testMatrix); err != nil {
		return err
	}

	return nil
}

func validateTestMatrix() error {
	for _, r := range testMatrix.Relayers {
		if _, err := getRelayerFactory(r); err != nil {
			return err
		}
	}

	for _, cs := range testMatrix.ChainSets {
		if _, err := getChainFactory(cs); err != nil {
			return err
		}
	}

	return nil
}

func getRelayerFactory(name string) (ibc.RelayerFactory, error) {
	switch name {
	case "rly", "cosmos/relayer":
		return ibc.NewBuiltinRelayerFactory(ibc.CosmosRly), nil
	case "hermes":
		return ibc.NewBuiltinRelayerFactory(ibc.Hermes), nil
	default:
		return nil, fmt.Errorf("unknown relayer type %q (valid types: rly, hermes)", name)
	}
}

func getChainFactory(chainSet []ibc.BuiltinChainFactoryEntry) (ibc.ChainFactory, error) {
	if len(chainSet) != 2 {
		return nil, fmt.Errorf("chain sets must have length 2 (found a chain set of length %d)", len(chainSet))
	}

	return ibc.NewBuiltinChainFactory(chainSet), nil
}

// TestRelayer is the root test for the relayer.
// It runs each subtest in parallel;
// if this is too taxing on a system, the -test.parallel flag
// can be used to reduce how many tests actively run at once.
func TestRelayer(t *testing.T) {
	t.Parallel()

	// One layer of subtests for each relayer to be tested.
	for _, r := range testMatrix.Relayers {
		rf, err := getRelayerFactory(r)
		if err != nil {
			// This error should have been validated before running tests.
			panic(err)
		}

		t.Run(r, func(t *testing.T) {
			t.Parallel()

			// And another layer of subtests for each chainset.
			for _, cs := range testMatrix.ChainSets {
				cf, err := getChainFactory(cs)
				if err != nil {
					panic(err)
				}

				chainNames := make([]string, len(cs))
				for i, c := range cs {
					chainNames[i] = c.Name + "@" + c.Version
				}
				chainTestName := strings.Join(chainNames, "+")

				t.Run(chainTestName, func(t *testing.T) {
					t.Parallel()

					// Finally, the relayertest suite.
					relayertest.TestRelayer(t, cf, rf)
				})
			}
		})
	}
}

// addFlags configures additional flags beyond the default testing flags.
// Although pflag would have been slightly more developer friendly,
// I ran out of time to spend on getting pflag to cooperate with the
// testing flags, so I fell back to plain Go standard library flags.
// We can revisit if necessary.
func addFlags() {
	flag.StringVar(&mainFlags.MatrixFile, "matrix", "", "Path to matrix file defining what configurations to test")
}
