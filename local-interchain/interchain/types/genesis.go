package types

import "github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"

type GenesisAccount struct {
	Name     string `json:"name"`
	Amount   string `json:"amount"`
	Address  string `json:"address"`
	Mnemonic string `json:"mnemonic"`
}

type Genesis struct {
	// Only apart of my fork for now.
	Modify []cosmos.GenesisKV `json:"modify"` // 'key' & 'val' in the config.

	Accounts []GenesisAccount `json:"accounts"`

	// A list of commands which run after chains are good to go.
	// May need to move out of genesis into its own section? Seems silly though.
	StartupCommands []string `json:"startup_commands"`
}
