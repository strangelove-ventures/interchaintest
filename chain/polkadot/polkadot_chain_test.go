package polkadot_test

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"testing"
	"time"

	p2pCrypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/chain/polkadot"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestPolkadotComposableChainStart(t *testing.T) {
	t.Parallel()

	home := ibctest.TempDir(t)
	client, network := ibctest.DockerSetup(t)

	nv := 5
	nf := 3

	chains, err := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
			Name:    "polkadot",
			Version: "polkadot:v0.9.19,composable:v2.1.9",
			ChainConfig: ibc.ChainConfig{
				ChainID: "rococo-local",
			},
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
	},
	).Chains(t.Name())

	require.NoError(t, err, "failed to get polkadot chain")
	require.Len(t, chains, 1)
	chain := chains[0]

	ctx := context.Background()
	t.Cleanup(func() {
		if err := chain.Cleanup(ctx); err != nil {
			t.Logf("Chain cleanup for %s failed: %v", chain.Config().ChainID, err)
		}
	})

	err = chain.Initialize(t.Name(), home, client, network)
	require.NoError(t, err, "failed to initialize polkadot chain")

	err = chain.Start(t.Name(), ctx)
	require.NoError(t, err, "failed to start polkadot chain")

	// TODO
	// _, err = chain.WaitForBlocks(50)
	// require.NoError(t, err, "polkadot chain failed to make blocks")
	time.Sleep(2 * time.Minute)
}

func TestNodeKeyPeerID(t *testing.T) {
	nodeKey, err := hex.DecodeString("1b57e31ddf03e39c58207dfcb5445958924b818c08c303a91838e68cfac551b2")
	require.NoError(t, err, "error decoding node key from hex string")

	privKeyEd25519 := ed25519.NewKeyFromSeed(nodeKey)
	privKey, pubKey, err := p2pCrypto.KeyPairFromStdKey(&privKeyEd25519)
	require.NoError(t, err, "error getting private key")

	id, err := peer.IDFromPrivateKey(privKey)
	require.NoError(t, err, "error getting peer id from private key")
	peerId := peer.Encode(id)
	require.Equal(t, "12D3KooWCqDbuUHRNWPAuHpVnzZGCkkMwgEx7Xd6xgszqtVpH56c", peerId)

	// TODO: determine what expected address should be when not using one of the built-in keys like alice,bob,etc
	pubKeyBytes, err := pubKey.Raw()
	require.NoError(t, err, "error getting public key bytes")
	encodedAddress, err := polkadot.EncodeAddressSS58(pubKeyBytes)
	require.NoError(t, err, "error encoding public key with ss58")
	require.Equal(t, "5D5SFiYAM1HJyCu7FqnibRuSv7gawgyrPDVtKJ1tPQctxbim", encodedAddress)
}
