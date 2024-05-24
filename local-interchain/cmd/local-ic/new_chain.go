package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	ictypes "github.com/strangelove-ventures/localinterchain/interchain/types"
)

var reader = bufio.NewReader(os.Stdin)

type Chains struct {
	Chains []ictypes.Chain `json:"chains" yaml:"chains"`
}

var newChainCmd = &cobra.Command{
	Use:     "new-chain <name>",
	Aliases: []string{"new", "new-config"},
	Short:   "Create a new chain config",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := strings.TrimSuffix(args[0], ".json")
		filePath := path.Join(GetDirectory(), "chains", fmt.Sprintf("%s.json", name))

		// while loop to allow for IBC connections to work as expected. Else set IBC as []string{}

		text, _ := os.ReadFile(filePath)
		if len(text) > 0 {
			value := getOrDefault(fmt.Sprintf("File %s already exist at this location, override?", name), "false")
			if res, _ := strconv.ParseBool(value); !res {
				panic(fmt.Sprintf("File %s already exist", filePath))
			}
		}

		var config Chains
		var chains []ictypes.Chain

		for i := 1; i < 1000; i++ {
			fmt.Printf("\n===== Creating new chain #%d =====\n", i)

			name := getOrDefault("Name", "juno")
			chainID := getOrDefault("Chain ID", "local-1")
			binary := getOrDefault("App Binary", "junod")

			c := ictypes.NewChainBuilder(name, chainID, binary, "token")

			c.WithDenom(getOrDefault("Denom", "ujuno"))
			c.WithBech32Prefix(getOrDefault("Bech32 Prefix", "juno"))
			c.WithGasPrices(getOrDefault("Gas Prices (comma separated)", "0.025ujuno"))

			c.WithIBCPaths(parseIBCPaths(getOrDefault("IBC Paths (comma separated)", "")))

			c.WithDockerImage(ictypes.DockerImage{
				Repository: getOrDefault("Docker Repo", "ghcr.io/cosmoscontracts/juno"),
				Version:    getOrDefault("Docker Tag / Branch Version", "v20.0.0"),
				UidGid:     "1000:1000",
			})

			if err := c.Validate(); err != nil {
				panic(err)
			}

			chains = append(chains, *c)

			res, err := strconv.ParseBool(getOrDefault[string]("\n\n\n === Add more chains? ===", "false"))
			if err != nil || !res {
				break
			}
		}
		config.Chains = chains

		bz, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			panic(err)
		}

		if err = os.WriteFile(filePath, bz, 0777); err != nil {
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
