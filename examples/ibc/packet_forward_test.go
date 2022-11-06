package ibc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	transfertypes "github.com/cosmos/ibc-go/v6/modules/apps/transfer/types"
	ibctest "github.com/strangelove-ventures/ibctest/v6"
	"github.com/strangelove-ventures/ibctest/v6/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"github.com/strangelove-ventures/ibctest/v6/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type PacketMetadata struct {
	Forward *ForwardMetadata `json:"forward"`
}

type ForwardMetadata struct {
	Receiver       string        `json:"receiver"`
	Port           string        `json:"port"`
	Channel        string        `json:"channel"`
	Timeout        time.Duration `json:"timeout"`
	Retries        *uint8        `json:"retries,omitempty"`
	Next           *string       `json:"next,omitempty"`
	RefundSequence *uint64       `json:"refund_sequence,omitempty"`
}

func TestPacketForwardMiddleware(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	client, network := ibctest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	chainID_A, chainID_B, chainID_C, chainID_D := "chain-a", "chain-b", "chain-c", "chain-d"

	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{Name: "gaia", Version: "strangelove-forward_middleware_memo_v3", ChainConfig: ibc.ChainConfig{ChainID: chainID_A, GasPrices: "0.0uatom"}},
		{Name: "gaia", Version: "strangelove-forward_middleware_memo_v3", ChainConfig: ibc.ChainConfig{ChainID: chainID_B, GasPrices: "0.0uatom"}},
		{Name: "gaia", Version: "strangelove-forward_middleware_memo_v3", ChainConfig: ibc.ChainConfig{ChainID: chainID_C, GasPrices: "0.0uatom"}},
		{Name: "gaia", Version: "strangelove-forward_middleware_memo_v3", ChainConfig: ibc.ChainConfig{ChainID: chainID_D, GasPrices: "0.0uatom"}},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chainA, chainB, chainC, chainD := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain), chains[2].(*cosmos.CosmosChain), chains[3].(*cosmos.CosmosChain)

	r := ibctest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
	).Build(
		t, client, network,
	)

	const pathAB = "ab"
	const pathBC = "bc"
	const pathCD = "cd"

	ic := ibctest.NewInterchain().
		AddChain(chainA).
		AddChain(chainB).
		AddChain(chainC).
		AddChain(chainD).
		AddRelayer(r, "relayer").
		AddLink(ibctest.InterchainLink{
			Chain1:  chainA,
			Chain2:  chainB,
			Relayer: r,
			Path:    pathAB,
		}).
		AddLink(ibctest.InterchainLink{
			Chain1:  chainB,
			Chain2:  chainC,
			Relayer: r,
			Path:    pathBC,
		}).
		AddLink(ibctest.InterchainLink{
			Chain1:  chainC,
			Chain2:  chainD,
			Relayer: r,
			Path:    pathCD,
		})

	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: ibctest.DefaultBlockDatabaseFilepath(),

		SkipPathCreation: false,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	const userFunds = int64(10_000_000_000)
	users := ibctest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, chainA, chainB, chainC, chainD)

	abChan, err := ibc.GetTransferChannel(ctx, r, eRep, chainID_A, chainID_B)
	require.NoError(t, err)

	baChan := abChan.Counterparty

	cbChan, err := ibc.GetTransferChannel(ctx, r, eRep, chainID_C, chainID_B)
	require.NoError(t, err)

	bcChan := cbChan.Counterparty

	dcChan, err := ibc.GetTransferChannel(ctx, r, eRep, chainID_D, chainID_C)
	require.NoError(t, err)

	cdChan := dcChan.Counterparty

	// Start the relayer on both paths
	err = r.StartRelayer(ctx, eRep, pathAB, pathBC, pathCD)
	require.NoError(t, err)

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				t.Logf("an error occured while stopping the relayer: %s", err)
			}
		},
	)

	// Get original account balances
	userA, userB, userC, userD := users[0], users[1], users[2], users[3]

	// Send packet from Chain A->Chain B->Chain C->Chain D
	const transferAmount int64 = 100000
	transfer := ibc.WalletAmount{
		Address: userB.Bech32Address(chainB.Config().Bech32Prefix),
		Denom:   chainA.Config().Denom,
		Amount:  transferAmount,
	}

	secondHopMetadata := &PacketMetadata{
		Forward: &ForwardMetadata{
			Receiver: userD.Bech32Address(chainD.Config().Bech32Prefix),
			Channel:  cdChan.ChannelID,
			Port:     cdChan.PortID,
		},
	}

	nextBz, err := json.Marshal(secondHopMetadata)
	require.NoError(t, err)

	next := string(nextBz)

	metadata := &PacketMetadata{
		Forward: &ForwardMetadata{
			Receiver: userC.Bech32Address(chainC.Config().Bech32Prefix),
			Channel:  bcChan.ChannelID,
			Port:     bcChan.PortID,
			Next:     &next,
		},
	}

	memo, err := json.Marshal(metadata)
	require.NoError(t, err)

	_, err = chainA.SendIBCTransfer(ctx, abChan.ChannelID, userA.KeyName, transfer, nil, string(memo))
	require.NoError(t, err)

	// // Compose the prefixed denoms and ibc denom for asserting balances
	firstHopDenom := transfertypes.GetPrefixedDenom(baChan.PortID, baChan.ChannelID, chainA.Config().Denom)
	secondHopDenom := transfertypes.GetPrefixedDenom(cbChan.PortID, cbChan.ChannelID, firstHopDenom)
	thirdHopDenom := transfertypes.GetPrefixedDenom(dcChan.PortID, dcChan.ChannelID, secondHopDenom)

	firstHopDenomTrace := transfertypes.ParseDenomTrace(firstHopDenom)
	secondHopDenomTrace := transfertypes.ParseDenomTrace(secondHopDenom)
	thirdHopDenomTrace := transfertypes.ParseDenomTrace(thirdHopDenom)

	firstHopIBCDenom := firstHopDenomTrace.IBCDenom()
	secondHopIBCDenom := secondHopDenomTrace.IBCDenom()
	thirdHopIBCDenom := thirdHopDenomTrace.IBCDenom()

	fmt.Printf("third hop denom: %s, third hop denom trace: %+v, third hop ibc denom: %s", thirdHopDenom, thirdHopDenomTrace, thirdHopIBCDenom)

	// Check that the funds sent are gone from the acc on Chain A
	err = cosmos.PollForBalance(ctx, chainA, 2, ibc.WalletAmount{
		Address: userA.Bech32Address(chainA.Config().Bech32Prefix),
		Denom:   chainA.Config().Denom,
		Amount:  userFunds - transferAmount,
	})
	require.NoError(t, err)

	// Check that the funds sent are present in the acc on Chain D
	err = cosmos.PollForBalance(ctx, chainD, 20, ibc.WalletAmount{
		Address: userD.Bech32Address(chainD.Config().Bech32Prefix),
		Denom:   thirdHopIBCDenom,
		Amount:  transferAmount,
	})
	require.NoError(t, err)

	// Send packet back from Chain D->Chain C->Chain B->Chain A
	transfer = ibc.WalletAmount{
		Address: userC.Bech32Address(chainC.Config().Bech32Prefix),
		Denom:   thirdHopIBCDenom,
		Amount:  transferAmount,
	}

	secondHopMetadata = &PacketMetadata{
		Forward: &ForwardMetadata{
			Receiver: userA.Bech32Address(chainA.Config().Bech32Prefix),
			Channel:  baChan.ChannelID,
			Port:     baChan.PortID,
		},
	}

	nextBz, err = json.Marshal(secondHopMetadata)
	require.NoError(t, err)

	next = string(nextBz)

	metadata = &PacketMetadata{
		Forward: &ForwardMetadata{
			Receiver: userB.Bech32Address(chainB.Config().Bech32Prefix),
			Channel:  cbChan.ChannelID,
			Port:     cbChan.PortID,
			Next:     &next,
		},
	}

	memo, err = json.Marshal(metadata)
	require.NoError(t, err)

	_, err = chainD.SendIBCTransfer(ctx, dcChan.ChannelID, userD.KeyName, transfer, nil, string(memo))
	require.NoError(t, err)

	// Check that the funds sent are gone from the acc on Chain D
	err = cosmos.PollForBalance(ctx, chainD, 2, ibc.WalletAmount{
		Address: userD.Bech32Address(chainD.Config().Bech32Prefix),
		Denom:   thirdHopIBCDenom,
		Amount:  int64(0),
	})
	require.NoError(t, err)

	// Check that the funds sent are present in the acc on Chain A
	err = cosmos.PollForBalance(ctx, chainA, 20, ibc.WalletAmount{
		Address: userA.Bech32Address(chainA.Config().Bech32Prefix),
		Denom:   chainA.Config().Denom,
		Amount:  userFunds,
	})
	require.NoError(t, err)

	// Send a malformed packet with invalid receiver address from Chain A->Chain B->Chain C
	// This should succeed in the first hop and fail to make the second hop; funds should then be refunded to Chain A.
	transfer = ibc.WalletAmount{
		Address: userB.Bech32Address(chainB.Config().Bech32Prefix),
		Denom:   chainA.Config().Denom,
		Amount:  transferAmount,
	}

	metadata = &PacketMetadata{
		Forward: &ForwardMetadata{
			Receiver: "xyz1t8eh66t2w5k67kwurmn5gqhtq6d2ja0vp7jmmq", // malformed receiver address on Chain C
			Channel:  bcChan.ChannelID,
			Port:     bcChan.PortID,
		},
	}

	memo, err = json.Marshal(metadata)
	require.NoError(t, err)

	_, err = chainA.SendIBCTransfer(ctx, abChan.ChannelID, userA.KeyName, transfer, nil, string(memo))
	require.NoError(t, err)

	// Wait until the funds sent are gone from the acc on osmosis
	err = cosmos.PollForBalance(ctx, chainA, 2, ibc.WalletAmount{
		Address: userA.Bech32Address(chainA.Config().Bech32Prefix),
		Denom:   chainA.Config().Denom,
		Amount:  userFunds - transferAmount,
	})
	require.NoError(t, err)

	// // Wait until the funds sent are back in the acc on osmosis
	err = cosmos.PollForBalance(ctx, chainA, 15, ibc.WalletAmount{
		Address: userA.Bech32Address(chainA.Config().Bech32Prefix),
		Denom:   chainA.Config().Denom,
		Amount:  userFunds,
	})
	require.NoError(t, err)

	// Check that the chain B account is empty
	chainBBal, err := chainB.GetBalance(ctx, userB.Bech32Address(chainB.Config().Bech32Prefix), firstHopIBCDenom)
	require.NoError(t, err)
	require.Equal(t, int64(0), chainBBal)

	// Check that the chain C account is empty
	chainCBal, err := chainC.GetBalance(ctx, userC.Bech32Address(chainC.Config().Bech32Prefix), secondHopIBCDenom)
	require.NoError(t, err)
	require.Equal(t, int64(0), chainCBal)

	// Send packet from Osmosis->Hub->Juno with the timeout so low that it can not make it from Hub to Juno, which should result in a refund from Hub to Osmosis after two retries.
	transfer = ibc.WalletAmount{
		Address: userB.Bech32Address(chainB.Config().Bech32Prefix),
		Denom:   chainA.Config().Denom,
		Amount:  transferAmount,
	}

	retries := uint8(2)
	metadata = &PacketMetadata{
		Forward: &ForwardMetadata{
			Receiver: userC.Bech32Address(chainC.Config().Bech32Prefix),
			Channel:  bcChan.ChannelID,
			Port:     bcChan.PortID,
			Retries:  &retries,
			Timeout:  1 * time.Second,
		},
	}

	memo, err = json.Marshal(metadata)
	require.NoError(t, err)

	_, err = chainA.SendIBCTransfer(ctx, abChan.ChannelID, userA.KeyName, transfer, nil, string(memo))
	require.NoError(t, err)

	// Wait until the funds sent are gone from the acc on chain A
	err = cosmos.PollForBalance(ctx, chainA, 2, ibc.WalletAmount{
		Address: userA.Bech32Address(chainA.Config().Bech32Prefix),
		Denom:   chainA.Config().Denom,
		Amount:  userFunds - transferAmount,
	})
	require.NoError(t, err)

	// Wait until the funds leave the chain B wallet (attempting to send to chain C)
	err = cosmos.PollForBalance(ctx, chainB, 5, ibc.WalletAmount{
		Address: userB.Bech32Address(chainB.Config().Bech32Prefix),
		Denom:   firstHopIBCDenom,
		Amount:  0,
	})
	require.NoError(t, err)

	// Wait until the funds are back in the acc on osmosis
	err = cosmos.PollForBalance(ctx, chainA, 15, ibc.WalletAmount{
		Address: userA.Bech32Address(chainA.Config().Bech32Prefix),
		Denom:   chainA.Config().Denom,
		Amount:  userFunds,
	})
	require.NoError(t, err)

	// Send a malformed packet with invalid receiver address from Osmosis->Hub->Juno->Gaia2
	// This should succeed in the first hop and second hop, then fail to make the third hop; funds should then be refunded to hub and then to osmosis.
	transfer = ibc.WalletAmount{
		Address: userB.Bech32Address(chainB.Config().Bech32Prefix),
		Denom:   chainA.Config().Denom,
		Amount:  transferAmount,
	}

	secondHopMetadata = &PacketMetadata{
		Forward: &ForwardMetadata{
			Receiver: "xyz1t8eh66t2w5k67kwurmn5gqhtq6d2ja0vp7jmmq", // malformed receiver address on chain D
			Channel:  cdChan.ChannelID,
			Port:     cdChan.PortID,
		},
	}

	nextBz, err = json.Marshal(secondHopMetadata)
	require.NoError(t, err)

	next = string(nextBz)

	firstHopMetadata := &PacketMetadata{
		Forward: &ForwardMetadata{
			Receiver: userC.Bech32Address(chainC.Config().Bech32Prefix),
			Channel:  bcChan.ChannelID,
			Port:     bcChan.PortID,
			Next:     &next,
		},
	}

	memo, err = json.Marshal(firstHopMetadata)
	require.NoError(t, err)

	_, err = chainA.SendIBCTransfer(ctx, abChan.ChannelID, userA.KeyName, transfer, nil, string(memo))
	require.NoError(t, err)

	// Wait until the funds sent are gone from the acc on osmosis
	err = cosmos.PollForBalance(ctx, chainA, 2, ibc.WalletAmount{
		Address: userA.Bech32Address(chainA.Config().Bech32Prefix),
		Denom:   chainA.Config().Denom,
		Amount:  userFunds - transferAmount,
	})
	require.NoError(t, err)

	// Wait until the funds sent are back in the acc on osmosis
	err = cosmos.PollForBalance(ctx, chainA, 20, ibc.WalletAmount{
		Address: userA.Bech32Address(chainA.Config().Bech32Prefix),
		Denom:   chainA.Config().Denom,
		Amount:  userFunds,
	})
	require.NoError(t, err)

	// Check that the chain B account is empty
	chainBBal, err = chainB.GetBalance(ctx, userB.Bech32Address(chainB.Config().Bech32Prefix), firstHopIBCDenom)
	require.NoError(t, err)
	require.Equal(t, int64(0), chainBBal)

	// Check that the chain C account is empty
	chainCBal, err = chainC.GetBalance(ctx, userC.Bech32Address(chainC.Config().Bech32Prefix), secondHopIBCDenom)
	require.NoError(t, err)
	require.Equal(t, int64(0), chainCBal)

	// Check that the chain D account is empty
	chainDBal, err := chainD.GetBalance(ctx, userD.Bech32Address(chainD.Config().Bech32Prefix), thirdHopIBCDenom)
	require.NoError(t, err)
	require.Equal(t, int64(0), chainDBal)

}
