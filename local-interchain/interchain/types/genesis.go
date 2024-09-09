package types

import (
	"fmt"

	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/go-bip39"
	"github.com/strangelove-ventures/interchaintest/v9/chain/cosmos"
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

func GenerateRandomAccounts(num int, prefix string, coinType int) []GenesisAccount {
	accounts := []GenesisAccount{}

	for i := 0; i < num; i++ {
		entropy, _ := bip39.NewEntropy(256)
		mnemonic, _ := bip39.NewMnemonic(entropy)

		addr := MnemonicToAddress(mnemonic, prefix, uint32(coinType))

		accounts = append(accounts, GenesisAccount{
			Name:     fmt.Sprintf("user%d", i),
			Amount:   "100000%DENOM%",
			Address:  addr,
			Mnemonic: mnemonic,
		})

	}

	return accounts
}

func MnemonicToAddress(mnemonic, prefix string, coinType uint32) string {
	e := moduletestutil.MakeTestEncodingConfig()

	kr := keyring.NewInMemory(e.Codec)

	bip44Path := fmt.Sprintf("m/%d'/%d'/0'/0/0", sdk.Purpose, coinType)
	r, err := kr.NewAccount("tempkey", mnemonic, "", bip44Path, hd.Secp256k1)
	if err != nil {
		panic(err)
	}

	bz, err := r.GetAddress()
	if err != nil {
		panic(err)
	}

	bech32Addr, err := addresscodec.NewBech32Codec(prefix).BytesToString(bz)
	if err != nil {
		panic(err)
	}

	return bech32Addr
}
