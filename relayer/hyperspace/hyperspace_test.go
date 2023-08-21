package hyperspace_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/strangelove-ventures/interchaintest/v7/relayer/hyperspace"
)

func TestKeys(t *testing.T) {
	bech32Prefix := "cosmos"
	coinType := "118"
	mnemonic := "taste shoot adapt slow truly grape gift need suggest midnight burger horn whisper hat vast aspect exit scorpion jewel axis great area awful blind"

	expectedKeyEntry := hyperspace.KeyEntry{
		PublicKey:  "xpub6G1GwQBqWwXuCRhri9q1JzxZ9eMWFazo2ssoZNkAsqusDTT6MPUXiPaXMJS9v4RVaSmYPhA1HK5RCD7WPutmUn3eeqXduM142X7YRVBx8bn",
		PrivateKey: "xprvA31vXtewgZybywdPc8Hzws1pbcX1r8GwfexCkzLZKWNtLf7worAHAbG3W3F1SagK47ng5877ihXkDvmNfZnVHSGw7Ad1JkzyPTKEtSpmSxa",
		Address:    []byte{69, 6, 166, 110, 97, 215, 215, 210, 224, 48, 93, 126, 44, 86, 4, 36, 109, 137, 43, 242},
		Account:    "cosmos1g5r2vmnp6lta9cpst4lzc4syy3kcj2lj0nuhmy",
	}

	keyEntry := hyperspace.GenKeyEntry(bech32Prefix, coinType, mnemonic)
	require.Equal(t, expectedKeyEntry.PublicKey, keyEntry.PublicKey, "PublicKey is wrong")
	require.Equal(t, expectedKeyEntry.PrivateKey, keyEntry.PrivateKey, "PrivateKey is wrong")
	require.Equal(t, expectedKeyEntry.Account, keyEntry.Account, "Account is wrong")
	require.Equal(t, expectedKeyEntry.Address, keyEntry.Address, "Address is wrong")
}
