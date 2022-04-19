package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"golang.org/x/sync/errgroup"
)

// customCmd represents the custom command
var customCmd = &cobra.Command{
	Use:   "custom",
	Short: "Run with custom chain configurations",
	Long: `This command allows you to provide all of the possible configuration parameters
for spinning up the source and destination chains
`,
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		relayerImplementationString, _ := flags.GetString("relayer")
		testCasesString := args[0]

		relayerImplementation := parseRelayerImplementation(relayerImplementationString)

		srcType, _ := flags.GetString("src-type")
		dstType, _ := flags.GetString("dst-type")

		// only cosmos chains supported for now
		switch srcType {
		case "cosmos":
			break
		default:
			panic(fmt.Sprintf("chain type not supported: %s", srcType))
		}

		switch dstType {
		case "cosmos":
			break
		default:
			panic(fmt.Sprintf("chain type not supported: %s", dstType))
		}

		srcVals, _ := flags.GetInt("src-vals")
		dstVals, _ := flags.GetInt("dst-vals")

		srcChainID, _ := flags.GetString("src-chain-id")
		dstChainID, _ := flags.GetString("dst-chain-id")

		srcName, _ := flags.GetString("src-name")
		dstName, _ := flags.GetString("dst-name")

		srcImage, _ := flags.GetString("src-image")
		dstImage, _ := flags.GetString("dst-image")

		srcVersion, _ := flags.GetString("src-version")
		dstVersion, _ := flags.GetString("dst-version")

		srcBinary, _ := flags.GetString("src-binary")
		dstBinary, _ := flags.GetString("dst-binary")

		srcBech32Prefix, _ := flags.GetString("src-bech32")
		dstBech32Prefix, _ := flags.GetString("dst-bech32")

		srcDenom, _ := flags.GetString("src-denom")
		dstDenom, _ := flags.GetString("dst-denom")

		srcGasPrices, _ := flags.GetString("src-gas-prices")
		dstGasPrices, _ := flags.GetString("dst-gas-prices")

		srcGasAdjustment, _ := flags.GetFloat64("src-gas-adjustment")
		dstGasAdjustment, _ := flags.GetFloat64("dst-gas-adjustment")

		srcTrustingPeriod, _ := flags.GetString("src-trusting-period")
		dstTrustingPeriod, _ := flags.GetString("dst-trusting-period")

		parallel, _ := flags.GetBool("parallel")

		srcChainCfg := ibc.NewCosmosChainConfig(srcName, srcImage, srcBinary, srcBech32Prefix, srcDenom, srcGasPrices, srcGasAdjustment, srcTrustingPeriod)
		dstChainCfg := ibc.NewCosmosChainConfig(dstName, dstImage, dstBinary, dstBech32Prefix, dstDenom, dstGasPrices, dstGasAdjustment, dstTrustingPeriod)

		srcChainCfg.ChainID = srcChainID
		dstChainCfg.ChainID = dstChainID

		srcChainCfg.Version = srcVersion
		dstChainCfg.Version = dstVersion

		var testCases []func(testName string, cf ibc.ChainFactory, relayerImplementation ibc.RelayerImplementation) error

		for _, testCaseString := range strings.Split(testCasesString, ",") {
			testCase, err := ibc.GetTestCase(testCaseString)
			if err != nil {
				panic(err)
			}
			testCases = append(testCases, testCase)
		}

		cf := ibc.NewCustomChainFactory([]ibc.CustomChainFactoryEntry{
			{Type: srcType, Config: srcChainCfg, NumValidators: srcVals, NumFullNodes: 1},
			{Type: dstType, Config: dstChainCfg, NumValidators: dstVals, NumFullNodes: 1},
		})

		if parallel {
			var eg errgroup.Group
			for i, testCase := range testCases {
				testCase := testCase
				testName := fmt.Sprintf("Test%d", i)
				eg.Go(func() error {
					return testCase(testName, cf, relayerImplementation)
				})
			}
			if err := eg.Wait(); err != nil {
				panic(err)
			}
		} else {
			for i, testCase := range testCases {
				testName := fmt.Sprintf("Test%d", i)
				if err := testCase(testName, cf, relayerImplementation); err != nil {
					panic(err)
				}
			}
		}
		fmt.Println("PASS")
	},
}

func init() {
	testCmd.AddCommand(customCmd)

	customCmd.Flags().StringP("src-name", "s", "gaia", "Source chain name")
	customCmd.Flags().String("src-type", "cosmos", "Type of source chain")
	customCmd.Flags().String("src-bech32", "cosmos", "Bech32 prefix for source chain")
	customCmd.Flags().String("src-denom", "uatom", "Native denomination for source chain")
	customCmd.Flags().String("src-gas-prices", "0.01uatom", "Gas prices for source chain")
	customCmd.Flags().Float64("src-gas-adjustment", 1.3, "Gas adjustment for source chain")
	customCmd.Flags().String("src-trust", "504h", "Trusting period for source chain ")
	customCmd.Flags().String("src-image", "ghcr.io/strangelove-ventures/heighliner/gaia", "Docker image for source chain")
	customCmd.Flags().String("src-version", "v7.0.1", "Docker image version for source chain")
	customCmd.Flags().String("src-binary", "gaiad", "Binary for source chain")
	customCmd.Flags().String("src-chain-id", "srcchain-1", "Chain ID to use for the source chain")
	customCmd.Flags().Int("src-vals", 4, "Number of Validator nodes on source chain")

	customCmd.Flags().StringP("dst-name", "d", "gaia", "Destination chain name")
	customCmd.Flags().String("dst-type", "cosmos", "Type of destination chain")
	customCmd.Flags().String("dst-bech32", "cosmos", "Bech32 prefix for destination chain")
	customCmd.Flags().String("dst-denom", "uatom", "Native denomination for destination chain")
	customCmd.Flags().String("dst-gas-prices", "0.01uatom", "Gas prices for destination chain")
	customCmd.Flags().Float64("dst-gas-adjustment", 1.3, "Gas adjustment for destination chain")
	customCmd.Flags().String("dst-trust", "504h", "Trusting period for destination chain")
	customCmd.Flags().String("dst-image", "ghcr.io/strangelove-ventures/heighliner/gaia", "Docker image for destination chain")
	customCmd.Flags().String("dst-version", "v7.0.1", "Docker image version for destination chain")
	customCmd.Flags().String("dst-binary", "gaiad", "Binary for destination chain")
	customCmd.Flags().String("dst-chain-id", "dstchain-1", "Chain ID to use for the source chain")
	customCmd.Flags().Int("dst-vals", 4, "Number of Validator nodes on destination chain")

	customCmd.Flags().StringP("relayer", "r", "rly", "Relayer implementation to use (rly or hermes)")
	customCmd.Flags().BoolP("parallel", "p", false, "Run tests in parallel")

}
