package hyperspace_test

import (
	"testing"

	"github.com/strangelove-ventures/ibctest/v6/relayer/hyperspace"
	"github.com/stretchr/testify/require"
)

func TestKeys(t *testing.T) {
	bech32Prefix := "cosmos"
	coinType := "118"
	mnemonic := "taste shoot adapt slow truly grape gift need suggest midnight burger horn whisper hat vast aspect exit scorpion jewel axis great area awful blind"

	expectedKeyEntry := hyperspace.KeyEntry{
		PublicKey: "02c1732ca9cb7c6efaa7c205887565b9787cab5ebdb7bc1dd872a21fc8c9efb56a",
		PrivateKey: "ac26db8374e68403a3cf38cc2b196d688d2f094cec0908978b2460d4442062f7",
		Address: []byte{69, 6, 166, 110, 97, 215, 215, 210, 224, 48, 93, 126, 44, 86, 4, 36, 109, 137, 43, 242},
		Account: "cosmos1g5r2vmnp6lta9cpst4lzc4syy3kcj2lj0nuhmy",
	}
	
	keyEntry := hyperspace.GenKeyEntry(bech32Prefix, coinType, mnemonic)
	require.Equal(t, expectedKeyEntry.PublicKey, keyEntry.PublicKey, "PublicKey is wrong")
	require.Equal(t, expectedKeyEntry.PrivateKey, keyEntry.PrivateKey, "PrivateKey is wrong")
	require.Equal(t, expectedKeyEntry.Account, keyEntry.Account, "Account is wrong")
	require.Equal(t, expectedKeyEntry.Address, keyEntry.Address, "Address is wrong")
}
