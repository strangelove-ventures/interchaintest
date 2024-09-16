package cosmos

import (
	"context"
	"errors"
	"fmt"

	govv1 "cosmossdk.io/x/gov/types/v1"
	govv1beta1 "cosmossdk.io/x/gov/types/v1beta1"
	"cosmossdk.io/x/tx/decode"
	baseapptestutil "github.com/cosmos/cosmos-sdk/baseapp/testutil"
	codectestutil "github.com/cosmos/cosmos-sdk/codec/testutil"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"go.uber.org/zap"

	"github.com/strangelove-ventures/interchaintest/v9/ibc"
	"github.com/strangelove-ventures/interchaintest/v9/testutil"
)

// PollForProposalStatus attempts to find a proposal with matching ID and status using gov v1.
func PollForProposalStatusV1(ctx context.Context, chain *CosmosChain, startHeight, maxHeight int64, proposalID uint64, status govv1.ProposalStatus) (*govv1.Proposal, error) {
	var pr *govv1.Proposal
	doPoll := func(ctx context.Context, height int64) (*govv1.Proposal, error) {
		p, err := chain.GovQueryProposalV1(ctx, proposalID)
		if err != nil {
			return pr, err
		}

		if p.Status.String() != status.String() {
			return pr, fmt.Errorf("proposal status (%d / %s) does not match expected: (%d / %s)", p.Status, p.Status.String(), status, status.String())
		}

		return p, nil
	}
	bp := testutil.BlockPoller[*govv1.Proposal]{CurrentHeight: chain.Height, PollFunc: doPoll}
	return bp.DoPoll(ctx, startHeight, maxHeight)
}

// PollForProposalStatus attempts to find a proposal with matching ID and status.
func PollForProposalStatus(ctx context.Context, chain *CosmosChain, startHeight, maxHeight int64, proposalID uint64, status govv1beta1.ProposalStatus) (*govv1beta1.Proposal, error) {
	var zero *govv1beta1.Proposal
	doPoll := func(ctx context.Context, height int64) (*govv1beta1.Proposal, error) {
		p, err := chain.GovQueryProposal(ctx, proposalID)
		if err != nil {
			return zero, err
		}
		if p.Status.String() != status.String() {
			return zero, fmt.Errorf("proposal status (%s) does not match expected: (%s)", p.Status, status)
		}
		return p, nil
	}
	bp := testutil.BlockPoller[*govv1beta1.Proposal]{CurrentHeight: chain.Height, PollFunc: doPoll}
	return bp.DoPoll(ctx, startHeight, maxHeight)
}

// PollForMessage searches every transaction for a message. Must pass a coded registry capable of decoding the cosmos transaction.
// fn is optional. Return true from the fn to stop polling and return the found message. If fn is nil, returns the first message to match type T.
func PollForMessage[T any](ctx context.Context, chain *CosmosChain, registry codectypes.InterfaceRegistry, startHeight, maxHeight int64, fn func(found T) bool) (T, error) {
	var zero T
	if fn == nil {
		fn = func(T) bool { return true }
	}
	doPoll := func(ctx context.Context, height int64) (T, error) {
		h := int64(height)
		block, err := chain.getFullNode().Client.Block(ctx, &h)
		if err != nil {
			return zero, err
		}
		for _, tx := range block.Block.Txs {

			// TODO: move this to the root
			cdc := codectestutil.CodecOptions{}.NewCodec()
			baseapptestutil.RegisterInterfaces(cdc.InterfaceRegistry())
			signingCtx := cdc.InterfaceRegistry().SigningContext()
			ac := signingCtx.AddressCodec()
			// txCfg := authTx.NewTxConfig(cdc, signingCtx.AddressCodec(), signingCtx.ValidatorAddressCodec(), authTx.DefaultSignModes)

			dec, err := decode.NewDecoder(decode.Options{
				SigningContext: signingCtx,
				ProtoCodec:     cdc,
			})
			if err != nil {
				zap.L().Error("failed to create decoder", zap.Error(err))
				continue
			}

			sdkTx, err := decodeTX(ac, registry, dec, tx)
			if err != nil {
				return zero, err
			}
			for _, msg := range sdkTx.GetMsgs() {
				if found, ok := msg.(T); ok {
					if fn(found) {
						return found, nil
					}
				}
			}
		}
		return zero, errors.New("not found")
	}

	bp := testutil.BlockPoller[T]{CurrentHeight: chain.Height, PollFunc: doPoll}
	return bp.DoPoll(ctx, startHeight, maxHeight)
}

// PollForBalance polls until the balance matches
func PollForBalance(ctx context.Context, chain *CosmosChain, deltaBlocks int64, balance ibc.WalletAmount) error {
	h, err := chain.Height(ctx)
	if err != nil {
		return fmt.Errorf("failed to get height: %w", err)
	}
	doPoll := func(ctx context.Context, height int64) (any, error) {
		bal, err := chain.GetBalance(ctx, balance.Address, balance.Denom)
		if err != nil {
			return nil, err
		}
		if !balance.Amount.Equal(bal) {
			return nil, fmt.Errorf("balance (%s) does not match expected: (%s)", bal.String(), balance.Amount.String())
		}
		return nil, nil
	}
	bp := testutil.BlockPoller[any]{CurrentHeight: chain.Height, PollFunc: doPoll}
	_, err = bp.DoPoll(ctx, h, h+deltaBlocks)
	return err
}
