package relayer

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/avast/retry-go"
	"github.com/cosmos/cosmos-sdk/types"
	clientypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	cosmosRelayerHelpers "github.com/cosmos/relayer/helpers"
	"github.com/cosmos/relayer/relayer"
	cosmosRelayer "github.com/cosmos/relayer/relayer"
	"github.com/stretchr/testify/require"
)

var (
	chainTimeout = 3 * time.Second
)

type CosmosRelayer struct {
	path    *cosmosRelayer.Path
	src     *cosmosRelayer.Chain
	dst     *cosmosRelayer.Chain
	rlyDone func()
	t       *testing.T
}

func NewCosmosRelayerFromChains(src, dst *cosmosRelayer.Chain, t *testing.T) *CosmosRelayer {
	path := cosmosRelayer.GenPath(src.ChainID, dst.ChainID, "transfer", "transfer", "UNORDERED", "ics20-1")
	src.PathEnd = path.Src
	dst.PathEnd = path.Dst
	return &CosmosRelayer{
		t:    t,
		path: path,
		src:  src,
		dst:  dst,
	}
}

func NewCosmosRelayerFromPath(path *cosmosRelayer.Path) *CosmosRelayer {
	return &CosmosRelayer{
		path: path,
		src:  cosmosRelayer.UnmarshalChain(*path.Src),
		dst:  cosmosRelayer.UnmarshalChain(*path.Dst),
	}
}

// Implements Relayer interface
func (relayer *CosmosRelayer) SetSourceRPC(rpcAddress string) error {
	relayer.src.RPCAddr = rpcAddress
	return nil
}

// Implements Relayer interface
func (relayer *CosmosRelayer) SetDestinationRPC(rpcAddress string) error {
	relayer.dst.RPCAddr = rpcAddress
	return nil
}

// Implements Relayer interface
func (relayer *CosmosRelayer) StartRelayer() error {
	err := relayer.src.Init("", chainTimeout, nil, true)
	if err != nil {
		return err
	}

	err = relayer.dst.Init("", chainTimeout, nil, true)
	if err != nil {
		return err
	}

	_, err = relayer.src.CreateClients(relayer.dst, true, true, false)
	if err != nil {
		return err
	}
	testClientPair(relayer.t, relayer.src, relayer.dst)
	timeout := relayer.src.GetTimeout()

	fmt.Printf("Client pair tested")

	_, err = relayer.src.CreateOpenConnections(relayer.dst, 3, timeout)
	if err != nil {
		return err
	}
	testConnectionPair(relayer.t, relayer.src, relayer.dst)

	fmt.Printf("Connection pair tested")

	_, err = relayer.src.CreateOpenChannels(relayer.dst, 3, timeout)
	if err != nil {
		return err
	}
	testChannelPair(relayer.t, relayer.src, relayer.dst)

	fmt.Printf("Channel pair tested")

	relayer.rlyDone, err = cosmosRelayer.RunStrategy(relayer.src, relayer.dst, relayer.path.MustGetStrategy())
	if err != nil {
		return err
	}

	return nil
}

// Implements Relayer interface
func (relayer *CosmosRelayer) InitializeSourceWallet() (WalletAmount, error) {
	_ = relayer.src.Keybase.Delete(relayer.src.Key)
	key, err := cosmosRelayerHelpers.KeyAddOrRestore(relayer.src, relayer.src.Key, 118)
	if err != nil {
		return WalletAmount{}, err
	}
	return WalletAmount{Mnemonic: key.Mnemonic, Address: key.Address}, nil
}

// Implements Relayer interface
func (relayer *CosmosRelayer) InitializeDestinationWallet() (WalletAmount, error) {
	_ = relayer.dst.Keybase.Delete(relayer.src.Key)
	key, err := cosmosRelayerHelpers.KeyAddOrRestore(relayer.dst, relayer.dst.Key, 118)
	if err != nil {
		return WalletAmount{}, err
	}
	return WalletAmount{Mnemonic: key.Mnemonic, Address: key.Address}, nil
}

func getWalletAmountFromCoins(balances types.Coins, denom string) (WalletAmount, error) {
	for _, balance := range balances {
		if balance.Denom == denom {
			return WalletAmount{
				Denom:  denom,
				Amount: balance.Amount.Int64(),
			}, nil
		}
	}
	return WalletAmount{}, errors.New("no balance found for that denom")
}

// Implements Relayer interface
func (relayer *CosmosRelayer) GetSourceBalance(denom string) (WalletAmount, error) {
	balances, err := relayer.src.QueryBalance(relayer.src.Key)
	if err != nil {
		return WalletAmount{}, err
	}
	return getWalletAmountFromCoins(balances, denom)
}

// Implements Relayer interface
func (relayer *CosmosRelayer) GetDestinationBalance(denom string) (WalletAmount, error) {
	balances, err := relayer.dst.QueryBalance(relayer.dst.Key)
	if err != nil {
		return WalletAmount{}, err
	}
	return getWalletAmountFromCoins(balances, denom)
}

// Implements Relayer interface
func (relayer *CosmosRelayer) RelayPacketFromSource(amount WalletAmount) error {
	return relayer.src.SendTransferMsg(relayer.dst, types.Coin{Denom: amount.Denom, Amount: types.NewInt(amount.Amount)}, relayer.dst.MustGetAddress(), 0, 0)
}

// Implements Relayer interface
func (relayer *CosmosRelayer) RelayPacketFromDestination(amount WalletAmount) error {
	return relayer.dst.SendTransferMsg(relayer.src, types.Coin{Denom: amount.Denom, Amount: types.NewInt(amount.Amount)}, relayer.src.MustGetAddress(), 0, 0)
}

// Implements Relayer interface
func (relayer *CosmosRelayer) StopRelayer() error {
	relayer.rlyDone()
	return nil
}

// testClientPair tests that the client for src on dst and dst on src are the only clients on those chains
func testClientPair(t *testing.T, src, dst *cosmosRelayer.Chain) {
	testClient(t, src, dst)
	testClient(t, dst, src)
}

// testClient queries client for existence of dst on src
func testClient(t *testing.T, src, dst *cosmosRelayer.Chain) {
	srch, err := src.QueryLatestHeight()
	require.NoError(t, err)
	var (
		client *clientypes.QueryClientStateResponse
	)
	if err = retry.Do(func() error {
		client, err = src.QueryClientStateResponse(srch)
		if err != nil {
			srch, _ = src.QueryLatestHeight()
		}
		return err
	}); err != nil {
		return
	}
	require.NoError(t, err)
	require.NotNil(t, client)
	cs, err := clientypes.UnpackClientState(client.ClientState)
	require.NoError(t, err)
	require.Equal(t, cs.ClientType(), "07-tendermint")
}

// testConnectionPair tests that the only connection on src and dst is between the two chains
func testConnectionPair(t *testing.T, src, dst *cosmosRelayer.Chain) {
	testConnection(t, src, dst)
	testConnection(t, dst, src)
}

// testConnection tests that the only connection on src has a counterparty that is the connection on dst
func testConnection(t *testing.T, src, dst *cosmosRelayer.Chain) {
	conns, err := src.QueryConnections(relayer.DefaultPageRequest())
	require.NoError(t, err)
	require.Equal(t, len(conns.Connections), 1)
	require.Equal(t, conns.Connections[0].ClientId, src.PathEnd.ClientID)
	require.Equal(t, conns.Connections[0].Counterparty.GetClientID(), dst.PathEnd.ClientID)
	require.Equal(t, conns.Connections[0].Counterparty.GetConnectionID(), dst.PathEnd.ConnectionID)
	require.Equal(t, conns.Connections[0].State.String(), "STATE_OPEN")

	h, err := src.Client.Status(context.Background())
	require.NoError(t, err)

	time.Sleep(time.Second * 5)
	conn, err := src.QueryConnection(h.SyncInfo.LatestBlockHeight)
	require.NoError(t, err)
	require.Equal(t, conn.Connection.ClientId, src.PathEnd.ClientID)
	require.Equal(t, conn.Connection.GetCounterparty().GetClientID(), dst.PathEnd.ClientID)
	require.Equal(t, conn.Connection.GetCounterparty().GetConnectionID(), dst.PathEnd.ConnectionID)
	require.Equal(t, conn.Connection.State.String(), "STATE_OPEN")
}

// testChannelPair tests that the only channel on src and dst is between the two chains
func testChannelPair(t *testing.T, src, dst *cosmosRelayer.Chain) {
	testChannel(t, src, dst)
	testChannel(t, dst, src)
}

// testChannel tests that the only channel on src is a counterparty of dst
func testChannel(t *testing.T, src, dst *cosmosRelayer.Chain) {
	chans, err := src.QueryChannels(cosmosRelayer.DefaultPageRequest())
	require.NoError(t, err)
	require.Equal(t, 1, len(chans.Channels))
	require.Equal(t, chans.Channels[0].Ordering.String(), "ORDER_UNORDERED")
	require.Equal(t, chans.Channels[0].State.String(), "STATE_OPEN")
	require.Equal(t, chans.Channels[0].Counterparty.ChannelId, dst.PathEnd.ChannelID)
	require.Equal(t, chans.Channels[0].Counterparty.GetPortID(), dst.PathEnd.PortID)

	h, err := src.Client.Status(context.Background())
	require.NoError(t, err)

	time.Sleep(time.Second * 5)
	ch, err := src.QueryChannel(h.SyncInfo.LatestBlockHeight)
	require.NoError(t, err)
	require.Equal(t, ch.Channel.Ordering.String(), "ORDER_UNORDERED")
	require.Equal(t, ch.Channel.State.String(), "STATE_OPEN")
	require.Equal(t, ch.Channel.Counterparty.ChannelId, dst.PathEnd.ChannelID)
	require.Equal(t, ch.Channel.Counterparty.GetPortID(), dst.PathEnd.PortID)
}
