package types

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/codec"
	"github.com/stretchr/testify/require"
)

var (
	encCfg = moduletestutil.MakeTestEncodingConfig(
		auth.AppModuleBasic{},
	)
)

func TestGenesisBech32Conversion(t *testing.T) {
	// entropy, _ := bip39.NewEntropy(256)
	// mnemonic, _ := bip39.NewMnemonic(entropy)
	mnemonic := "scrub rabbit clap staff crowd scissors action gift engine suspect pulse knife swamp ordinary fringe outside arrive soul view palace cancel maze electric render"

	kr := keyring.NewInMemory(encCfg.Codec)

	cfg := sdk.NewConfig()
	cfg.SetCoinType(118)
	cfg.SetPurpose(44)
	cfg.SetBech32PrefixForAccount("juno", "junopub")

	r, err := kr.NewAccount(t.Name(), mnemonic, "", cfg.GetFullBIP44Path(), hd.Secp256k1)
	if err != nil {
		panic(err)
	}

	bz, err := r.GetAddress()
	if err != nil {
		panic(err)
	}
	bech32Addr, err := codec.NewBech32Codec("juno").BytesToString(bz)
	if err != nil {
		panic(err)
	}

	acc := GenesisAccount{
		Name:     "account1",
		Amount:   "100000%DENOM%",
		Address:  bech32Addr,
		Mnemonic: mnemonic,
	}
	require.Equal(t, acc.Address, "juno1v94f4cjdpgz9lv675r2twzlheuzpnxpnc7gmyl", "unexpected address")
}
