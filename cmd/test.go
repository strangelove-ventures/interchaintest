/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/strangelove-ventures/ibc-test-framework/relayertest"
	"golang.org/x/sync/errgroup"
)

func parseChainVersion(chainVersion string) (string, string) {
	split := strings.Split(chainVersion, ":")
	switch len(split) {
	case 1:
		return split[0], "latest"
	case 2:
		return split[0], split[1]
	default:
		panic(fmt.Errorf("unable to parse chain and version from: %s", chainVersion))
	}
}

func parseRelayerImplementation(relayerImplementationString string) ibc.RelayerImplementation {
	switch relayerImplementationString {
	case "rly":
		fallthrough
	case "cosmos/relayer":
		return ibc.CosmosRly
	case "hermes":
		return ibc.Hermes
	default:
		panic(fmt.Errorf("unknown relayer implementation provided: %s", relayerImplementationString))
	}
}

func runTestCase(testName string, testCase func(testName string, cf ibc.ChainFactory, relayerImplementation ibc.RelayerImplementation) error, relayerImplementation ibc.RelayerImplementation, cf ibc.ChainFactory) error {
	return testCase(testName, cf, relayerImplementation)
}

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test IBC interactions between chains",
	Long: `Run all or specific relayer tests for source and destination chains.

e.g.
# Test cosmos relayer for two chains of gaia latest
ibc-test-framework test

# Specify specific chains/versions, relayer implementation, and test cases
ibc-test-framework test --src osmosis:v7.0.4 --dst juno:v2.3.0 --relayer rly RelayPacketTest,RelayPacketTestHeightTimeout

# Shorthand flags
ibc-test-framework test -s osmosis:v7.0.4 -d juno:v2.3.0 -r rly RelayPacketTest
`,
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		srcChainNameVersion, _ := flags.GetString("src")
		dstChainNameVersion, _ := flags.GetString("dst")
		relayerImplementationString, _ := flags.GetString("relayer")
		testCasesString := args[0]

		srcChainName, srcChainVersion := parseChainVersion(srcChainNameVersion)
		dstChainName, dstChainVersion := parseChainVersion(dstChainNameVersion)

		relayerImplementation := parseRelayerImplementation(relayerImplementationString)

		srcVals, _ := flags.GetInt("src-vals")
		dstVals, _ := flags.GetInt("dst-vals")

		srcChainID, _ := flags.GetString("src-chain-id")
		dstChainID, _ := flags.GetString("dst-chain-id")

		parallel, _ := flags.GetBool("parallel")

		var testCases []func(testName string, cf ibc.ChainFactory, relayerImplementation ibc.RelayerImplementation) error

		for _, testCaseString := range strings.Split(testCasesString, ",") {
			// Prefer the new way of getting legacy test cases,
			// but fall back to the old way for now.
			testCase, err := relayertest.GetLegacyTestCase(testCaseString)
			if err != nil {
				testCase, err = ibc.GetTestCase(testCaseString)
				if err != nil {
					panic(err)
				}
			}
			testCases = append(testCases, testCase)
		}

		cf := ibc.NewBuiltinChainFactory([]ibc.BuiltinChainFactoryEntry{
			{Name: srcChainName, Version: srcChainVersion, ChainID: srcChainID, NumValidators: srcVals, NumFullNodes: 1},
			{Name: dstChainName, Version: dstChainVersion, ChainID: dstChainID, NumValidators: dstVals, NumFullNodes: 1},
		})

		if parallel {
			var eg errgroup.Group
			for i, testCase := range testCases {
				testCase := testCase
				testName := fmt.Sprintf("Test%d", i)
				eg.Go(func() error {
					return runTestCase(testName, testCase, relayerImplementation, cf)
				})
			}
			if err := eg.Wait(); err != nil {
				panic(err)
			}
		} else {
			for i, testCase := range testCases {
				testName := fmt.Sprintf("Test%d", i)
				if err := runTestCase(testName, testCase, relayerImplementation, cf); err != nil {
					panic(err)
				}
			}
		}
		fmt.Println("PASS")
	},
}

func init() {
	rootCmd.AddCommand(testCmd)

	testCmd.Flags().StringP("src", "s", "gaia", "Source chain name (e.g. \"gaia\", \"gaia:v6.0.4\")")
	testCmd.Flags().StringP("dst", "d", "gaia", "Destination chain name (e.g. \"gaia\", \"gaia:v6.0.4\")")
	testCmd.Flags().StringP("relayer", "r", "rly", "Relayer implementation to use (rly or hermes)")
	testCmd.Flags().Int("src-vals", 4, "Number of Validator nodes on source chain")
	testCmd.Flags().Int("dst-vals", 4, "Number of Validator nodes on destination chain")
	testCmd.Flags().String("src-chain-id", "srcchain-1", "Chain ID to use for the source chain")
	testCmd.Flags().String("dst-chain-id", "dstchain-1", "Chain ID to use for the source chain")
	testCmd.Flags().BoolP("parallel", "p", false, "Run tests in parallel")

}
