package main

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/strangelove-ventures/interchaintest/local-interchain/interchain/types"
	ictypes "github.com/strangelove-ventures/interchaintest/local-interchain/interchain/types"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

var reader = bufio.NewReader(os.Stdin)

var newChainCmd = &cobra.Command{
	Use:     "new-chain <name>",
	Aliases: []string{"new", "new-config"},
	Short:   "Create a new chain config",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := strings.TrimSuffix(args[0], ".json")
		filePath := path.Join(GetDirectory(), "chains", fmt.Sprintf("%s.json", name))

		text, _ := os.ReadFile(filePath)
		if len(text) > 0 {
			value := getOrDefault(fmt.Sprintf("File %s already exist at this location, override?", name), "false")
			if res, _ := strconv.ParseBool(value); !res {
				panic(fmt.Sprintf("File %s already exist", filePath))
			}
		}

		var config types.ChainsConfig
		var chains []ictypes.Chain

		for i := 1; i < 20; i++ {
			fmt.Printf("\n===== Creating new chain #%d =====\n", i)

			name := getOrDefault("Name", "cosmos")
			chainID := getOrDefault("Chain ID", "localchain-1")
			binary := getOrDefault("App Binary", "gaiad")
			bech32 := getOrDefault("Bech32 Prefix", "cosmos")

			c := ictypes.NewChainBuilder(name, chainID, binary, "token", bech32)

			denom := getOrDefault("Denom", "utoken")
			c.SetDenom(denom)
			c.SetGasPrices(getOrDefault("Gas Prices (comma separated)", "0.0"+denom))

			c.SetIBCPaths(parseIBCPaths(getOrDefault("IBC Paths (comma separated)", "")))

			c.SetDockerImage(ibc.DockerImage{
				Repository: getOrDefault("Docker Repo", "ghcr.io/strangelove-ventures/heighliner/gaia"),
				Version:    getOrDefault("Docker Tag / Branch Version", "v16.0.0"),
				UIDGID:     "1025:1025",
			})
			if i == 0 {
				c.SetHostPortOverride(types.BaseHostPortOverride())
			}

			if err := c.Validate(); err != nil {
				panic(err)
			}

			c.SetChainDefaults()

			chains = append(chains, *c)

			res, err := strconv.ParseBool(getOrDefault[string]("\n\n\n === Add more chains? ===", "false"))
			if err != nil || !res {
				break
			}
		}
		config.Chains = chains

		if err := config.SaveJSON(filePath); err != nil {
			panic(err)
		}
	},
}

func parseIBCPaths(input string) []string {
	if len(input) == 0 {
		return []string{}
	}

	return strings.Split(input, ",")
}

func getOrDefault[T any](output string, defaultVal T) T {
	defaultOutput := ""

	defaultType := reflect.TypeOf(defaultVal).Kind()

	switch defaultType {
	case reflect.String:
		defaultOutput = any(defaultVal).(string)
	case reflect.Int:
		defaultOutput = strconv.Itoa(any(defaultVal).(int))
	case reflect.Float32, reflect.Float64:
		defaultOutput = fmt.Sprintf("%f", any(defaultVal).(float64))
	case reflect.Slice:
		if reflect.TypeOf(defaultVal).Elem().Kind() == reflect.String {
			defaultOutput = "[]"
		}
	}

	if defaultOutput == "" && defaultType == reflect.String {
		defaultOutput = "''"
	}

	fmt.Printf("- %s. (Default %v)\n>>> ", output, defaultOutput)
	text, err := reader.ReadString('\n')

	if err != nil || text == "\n" {
		return defaultVal
	}

	if defaultType == reflect.String {
		text = strings.ReplaceAll(text, "\n", "")
	}

	return any(text).(T)
}
