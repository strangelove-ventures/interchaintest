// Command ibctest allows running the relayer tests with command-line configuration.
package ibctest

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/strangelove-ventures/ibc-test-framework/ibctest"
	"github.com/strangelove-ventures/ibc-test-framework/log"
	"github.com/strangelove-ventures/ibc-test-framework/relayertest"
)

// The value of the test matrix.
var testMatrix struct {
	Relayers []string

	ChainSets [][]ibctest.BuiltinChainFactoryEntry

	CustomChainSets [][]ibctest.CustomChainFactoryEntry
}

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())
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

var extraFlags mainFlags

// setUpTestMatrix populates the testMatrix singleton with
// the parsed contents of the file referenced by the matrix flag,
// or with a small reasonable default of rly against one gaia-osmosis set.
func setUpTestMatrix() error {
	if extraFlags.MatrixFile == "" {
		fmt.Fprintln(os.Stderr, "No matrix file provided, falling back to rly with gaia and osmosis")

		testMatrix.Relayers = []string{"rly"}
		testMatrix.ChainSets = [][]ibctest.BuiltinChainFactoryEntry{
			{
				{Name: "gaia", Version: "latest", ChainID: "cosmoshub-1004", NumValidators: 1, NumFullNodes: 1},
				{Name: "osmosis", Version: "latest", ChainID: "osmosis-1001", NumValidators: 1, NumFullNodes: 1},
			},
		}

		return nil
	}

	// Otherwise parse the given file.
	fmt.Fprintf(os.Stderr, "Loading matrix file from %s\n", extraFlags.MatrixFile)
	j, err := os.ReadFile(extraFlags.MatrixFile)
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
		if _, err := getRelayerFactory(r, log.Nop()); err != nil {
			return err
		}
	}

	for _, cs := range testMatrix.ChainSets {
		if _, err := getChainFactory(cs, log.Nop()); err != nil {
			return err
		}
	}

	for _, ccs := range testMatrix.CustomChainSets {
		if _, err := getCustomChainFactory(ccs, log.Nop()); err != nil {
			return err
		}
	}

	return nil
}

func getRelayerFactory(name string, logger log.Logger) (ibctest.RelayerFactory, error) {
	switch name {
	case "rly", "cosmos/relayer":
		return ibctest.NewBuiltinRelayerFactory(ibc.CosmosRly, logger), nil
	case "hermes":
		return ibctest.NewBuiltinRelayerFactory(ibc.Hermes, logger), nil
	default:
		return nil, fmt.Errorf("unknown relayer type %q (valid types: rly, hermes)", name)
	}
}

func getChainFactory(chainSet []ibctest.BuiltinChainFactoryEntry, logger log.Logger) (ibctest.ChainFactory, error) {
	if len(chainSet) != 2 {
		return nil, fmt.Errorf("chain sets must have length 2 (found a chain set of length %d)", len(chainSet))
	}
	return ibctest.NewBuiltinChainFactory(chainSet, logger), nil
}

func getCustomChainFactory(customChainSet []ibctest.CustomChainFactoryEntry, logger log.Logger) (ibctest.ChainFactory, error) {
	if len(customChainSet) != 2 {
		return nil, fmt.Errorf("chain sets must have length 2 (found a chain set of length %d)", len(customChainSet))
	}
	return ibctest.NewCustomChainFactory(customChainSet, logger), nil
}

// TestRelayer is the root test for the relayer.
// It runs each subtest in parallel;
// if this is too taxing on a system, the -test.parallel flag
// can be used to reduce how many tests actively run at once.
func TestRelayer(t *testing.T) {
	t.Parallel()

	logger, err := extraFlags.Logger()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	t.Logf("View chain and relayer logs at %s", logger.FilePath)

	// One layer of subtests for each relayer to be tested.
	for _, r := range testMatrix.Relayers {
		rf, err := getRelayerFactory(r, logger.WithField("process", "relayer"))
		if err != nil {
			// This error should have been validated before running tests.
			panic(err)
		}

		t.Run(r, func(t *testing.T) {
			t.Parallel()

			// Collect all the chain factories from both the builtins and the customs.
			chainFactories := make([]ibctest.ChainFactory, 0, len(testMatrix.ChainSets)+len(testMatrix.CustomChainSets))
			for _, cs := range testMatrix.ChainSets {
				cf, err := getChainFactory(cs, logger)
				if err != nil {
					panic(err)
				}
				chainFactories = append(chainFactories, cf)
			}
			for _, ccs := range testMatrix.CustomChainSets {
				ccf, err := getCustomChainFactory(ccs, logger)
				if err != nil {
					panic(err)
				}
				chainFactories = append(chainFactories, ccf)
			}

			for _, cf := range chainFactories {
				// As of writing, it's fine to build a chain pair just to inspect names and versions.
				srcChain, dstChain, err := cf.Pair("")
				if err != nil {
					panic(err)
				}

				chainTestName := fmt.Sprintf(
					"%s@%s+%s@%s",
					srcChain.Config().Name, srcChain.Config().Images[0].Version,
					dstChain.Config().Name, dstChain.Config().Images[0].Version,
				)

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
	flag.StringVar(&extraFlags.MatrixFile, "matrix", "", "Path to matrix file defining what configurations to test")
	flag.StringVar(&extraFlags.LogFile, "log-file", "ibctest.log", "File to write chain and relayer logs. If a file name, logs written to $HOME/.ibctest/logs directory. Use 'stderr' or 'stdout' to print logs in line tests.")
	flag.StringVar(&extraFlags.LogFormat, "log-format", "console", "Chain and relayer log format: console|json")
	flag.StringVar(&extraFlags.LogLevel, "log-level", "info", "Chain and relayer log level: debug|info|error")
}
