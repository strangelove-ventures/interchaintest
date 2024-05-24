package types

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/codec"
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
	Modify []cosmos.GenesisKV `json:"modify,omitempty" yaml:"modify,omitempty"` // 'key' & 'val' in the config.

	Accounts []GenesisAccount `json:"accounts,omitempty" yaml:"accounts,omitempty"`

	// A list of commands which run after chains are good to go.
	// May need to move out of genesis into its own section? Seems silly though.
	StartupCommands []string `json:"startup_commands,omitempty" yaml:"startup_commands,omitempty"`
}

func NewGenesisAccount(name, bech32, coins string, coinType int, mnemonic string) GenesisAccount {
	if coins == "" {
		coins = "100000%DENOM%"
	}

	if mnemonic == "" {
		entropy, _ := bip39.NewEntropy(256)
		mnemonic, _ = bip39.NewMnemonic(entropy)
	}

	return GenesisAccount{
		Name:     name,
		Amount:   coins,
		Address:  MnemonicToAddress(mnemonic, bech32, uint32(coinType)),
		Mnemonic: mnemonic,
	}
}

func GenerateRandomAccounts(num int, bech32 string, coinType int) []GenesisAccount {
	accounts := []GenesisAccount{}

	for i := 0; i < num; i++ {
		entropy, _ := bip39.NewEntropy(256)
		mnemonic, _ := bip39.NewMnemonic(entropy)

		accounts = append(accounts, GenesisAccount{
			Name:     fmt.Sprintf("account%d", i),
			Amount:   "100000%DENOM%",
			Address:  MnemonicToAddress(mnemonic, bech32, uint32(coinType)),
			Mnemonic: mnemonic,
		})

	}

	return accounts
}

func MnemonicToAddress(mnemonic, bech32 string, coinType uint32) string {
	e := moduletestutil.MakeTestEncodingConfig(
		auth.AppModuleBasic{},
	)

	kr := keyring.NewInMemory(e.Codec)

	cfg := sdk.NewConfig()
	cfg.SetCoinType(coinType)
	cfg.SetBech32PrefixForAccount(bech32, bech32+sdk.PrefixPublic)

	r, err := kr.NewAccount("tempkey", mnemonic, "", cfg.GetFullBIP44Path(), hd.Secp256k1)
	if err != nil {
		panic(err)
	}

	bz, err := r.GetAddress()
	if err != nil {
		panic(err)
	}

	bech32Addr, err := codec.NewBech32Codec(bech32).BytesToString(bz)
	if err != nil {
		panic(err)
	}

	return bech32Addr
}
