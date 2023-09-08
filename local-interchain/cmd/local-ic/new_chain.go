package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	ictypes "github.com/strangelove-ventures/localinterchain/interchain/types"

	"github.com/tyler-smith/go-bip39"
)

var reader = bufio.NewReader(os.Stdin)

type Chains struct {
	Chains []ictypes.Chain `json:"chains"`
}

var newChainCmd = &cobra.Command{
	Use:     "new-chain <name>",
	Aliases: []string{"new", "new-config"},
	Short:   "Create a new chain config",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := strings.TrimSuffix(args[0], ".json")
		filePath := path.Join(GetDirectory(), "chains", fmt.Sprintf("%s.json", name))

		// while loop to allow for IBC conncetions to work as expected. Else set IBC as []string{}

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

			c := ictypes.Chain{
				// Required
				Name:          getOrDefault("Name", "juno"),
				ChainID:       getOrDefault("chain_id", "local-1"),
				Binary:        getOrDefault("App Binary", "junod"),
				Bech32Prefix:  getOrDefault("Bech32 Prefix", "juno"),
				GasPrices:     getOrDefault("gas_prices (comma seperated)", "0ujuno,0other"),
				GasAdjustment: getOrDefault("gas_adjustment", 2.5),

				// IBCPaths should be unique chain ids?
				IBCPaths: parseIBCPaths(getOrDefault("IBC Paths", "")),
				DockerImage: ictypes.DockerImage{
					Repository: getOrDefault("Docker Repo", "ghcr.io/cosmoscontracts/juno-e2e"),
					Version:    getOrDefault("Docker Tag/Branch Version", "v15.0.0"),
					UidGid:     "1000:1000",
				},

				// genesis accounts (juno1...:100ujuno,10uatom;)

				// Spam through enter typically
				Denom:          getOrDefault("Denom", "ujuno"),
				TrustingPeriod: getOrDefault("Trusting Period", "112h"),
				ChainType:      getOrDefault("Chain Type", "cosmos"),
				CoinType:       getOrDefault("Coin Type", 118),

				// defaults
				Debugging:  false,
				NumberVals: 1,
				NumberNode: 0,
				Genesis: ictypes.Genesis{
					Accounts:        generateRandomAccounts(),
					Modify:          []cosmos.GenesisKV{},
					StartupCommands: []string{},
				},
			}

			if err := c.Validate(); err != nil {
				panic(err)
			}

			chains = append(chains, c)

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

var nonNumeric = regexp.MustCompile("[^0-9]+")

func generateRandomAccounts() []ictypes.GenesisAccount {
	accounts := []ictypes.GenesisAccount{}

	res := nonNumeric.ReplaceAllString(getOrDefault("Number of accounts to generate", "1"), "")
	num, err := strconv.Atoi(res)
	if err != nil {
		panic(err)
	}

	for i := 0; i < num; i++ {
		entropy, _ := bip39.NewEntropy(256)
		mnemonic, _ := bip39.NewMnemonic(entropy)

		// load mnemonic into cosmossdk and get the address
		accounts = append(accounts, ictypes.GenesisAccount{
			Name:     fmt.Sprintf("account%d", i),
			Amount:   "100000%DENOM%", // allow user to alter along with keyname?
			Address:  "",              // TODO:
			Mnemonic: mnemonic,
		})

	}

	return accounts
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

func init() {
	rootCmd.AddCommand(newChainCmd)
}
