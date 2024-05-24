package types

import (
	"fmt"

	"github.com/cosmos/go-bip39"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
)

type GenesisAccount struct {
	Name     string `json:"name"`
	Amount   string `json:"amount"`
	Address  string `json:"address"`
	Mnemonic string `json:"mnemonic"`
}

type Genesis struct {
	// Only apart of my fork for now.
	Modify []cosmos.GenesisKV `json:"modify" yaml:"modify"` // 'key' & 'val' in the config.

	Accounts []GenesisAccount `json:"accounts" yaml:"accounts"`

	// A list of commands which run after chains are good to go.
	// May need to move out of genesis into its own section? Seems silly though.
	StartupCommands []string `json:"startup_commands" yaml:"startup_commands"`
}

func GenerateRandomAccounts(bech32 string, num int) []GenesisAccount {
	accounts := []GenesisAccount{}

	for i := 0; i < num; i++ {
		entropy, _ := bip39.NewEntropy(256)
		mnemonic, _ := bip39.NewMnemonic(entropy)

		// load mnemonic into cosmossdk and get the address
		accounts = append(accounts, GenesisAccount{
			Name:     fmt.Sprintf("account%d", i),
			Amount:   "100000%DENOM%", // allow user to alter along with keyname?
			Address:  "",              // TODO:
			Mnemonic: mnemonic,
		})

	}

	return accounts
}
