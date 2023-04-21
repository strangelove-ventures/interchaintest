package polkadot

import (
	"encoding/hex"
	"math/big"
	"strconv"

	gsrpc "github.com/misko9/go-substrate-rpc-client/v4"
	"github.com/misko9/go-substrate-rpc-client/v4/signature"
	gstypes "github.com/misko9/go-substrate-rpc-client/v4/types"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
)

// SendFundsTx sends funds to a wallet using the SubstrateAPI
func SendFundsTx(api *gsrpc.SubstrateAPI, senderKeypair signature.KeyringPair, amount ibc.WalletAmount) (gstypes.Hash, error) {
	hash := gstypes.Hash{}
	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		return hash, err
	}

	receiverPubKey, err := DecodeAddressSS58(amount.Address)
	if err != nil {
		return hash, err
	}

	receiver, err := gstypes.NewMultiAddressFromHexAccountID(hex.EncodeToString(receiverPubKey))
	if err != nil {
		return hash, err
	}

	call, err := gstypes.NewCall(meta, "Balances.transfer", receiver, gstypes.NewUCompactFromUInt(uint64(amount.Amount)))
	if err != nil {
		return hash, err
	}

	return CreateSignSubmitExt(api, meta, senderKeypair, call)
}

// Turns on sending and receiving ibc transfers
func EnableIbc(api *gsrpc.SubstrateAPI, senderKeypair signature.KeyringPair) (gstypes.Hash, error) {
	hash := gstypes.Hash{}
	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		return hash, err
	}

	c, err := gstypes.NewCall(meta, "Ibc.set_params", gstypes.NewBool(true), gstypes.NewBool(true))
	if err != nil {
		return hash, err
	}

	sc, err := gstypes.NewCall(meta, "Sudo.sudo", c)
	if err != nil {
		return hash, err
	}

	return CreateSignSubmitExt(api, meta, senderKeypair, sc)
}

// SendIbcFundsTx sends funds to a wallet using the SubstrateAPI
func SendIbcFundsTx(
	api *gsrpc.SubstrateAPI,
	senderKeypair signature.KeyringPair,
	channelID string,
	amount ibc.WalletAmount,
	options ibc.TransferOptions,
) (gstypes.Hash, error) {
	hash := gstypes.Hash{}
	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		return hash, err
	}

	assetNum, err := strconv.ParseInt(amount.Denom, 10, 64)
	if err != nil {
		return hash, err
	}

	raw := gstypes.NewU8(1)
	size := gstypes.NewU8(uint8(len(amount.Address) * 4))
	to := gstypes.NewStorageDataRaw([]byte(amount.Address))
	channel := gstypes.NewU64(0) // Parse channel number from string
	timeout := gstypes.NewU8(1)
	timestamp := gstypes.NewOptionU64(gstypes.NewU64(0))
	height := gstypes.NewOptionU64(gstypes.NewU64(3000)) // Must set timestamp or height
	assetId := gstypes.NewU128(*big.NewInt(assetNum))
	amount2 := gstypes.NewU128(*big.NewInt(amount.Amount))
	memo := gstypes.NewU8(0)

	call, err := gstypes.NewCall(meta, "Ibc.transfer", raw, size, to, channel, timeout, timestamp, height, assetId, amount2, memo)
	if err != nil {
		return hash, err
	}

	return CreateSignSubmitExt(api, meta, senderKeypair, call)
}

// MintFunds mints an asset for a user on parachain, keyName must be the owner of the asset
func MintFundsTx(
	api *gsrpc.SubstrateAPI,
	senderKeypair signature.KeyringPair,
	amount ibc.WalletAmount,
) (gstypes.Hash, error) {
	hash := gstypes.Hash{}
	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		return hash, err
	}

	assetNum, err := strconv.ParseInt(amount.Denom, 10, 64)
	if err != nil {
		return hash, err
	}

	receiverPubKey, err := DecodeAddressSS58(amount.Address)
	if err != nil {
		return hash, err
	}

	receiver, err := gstypes.NewMultiAddressFromHexAccountID(hex.EncodeToString(receiverPubKey))
	if err != nil {
		return hash, err
	}

	assetId := gstypes.NewU128(*big.NewInt(assetNum))
	amount2 := gstypes.NewUCompactFromUInt(uint64(amount.Amount))

	call, err := gstypes.NewCall(meta, "Assets.mint", assetId, receiver, amount2)
	if err != nil {
		return hash, err
	}

	return CreateSignSubmitExt(api, meta, senderKeypair, call)
}

// Common tx function to create an extrinsic and sign/submit it
func CreateSignSubmitExt(
	api *gsrpc.SubstrateAPI,
	meta *gstypes.Metadata,
	senderKeypair signature.KeyringPair,
	call gstypes.Call,
) (gstypes.Hash, error) {
	hash := gstypes.Hash{}
	ext := gstypes.NewExtrinsic(call)
	genesisHash, err := api.RPC.Chain.GetBlockHash(0)
	if err != nil {
		return hash, err
	}

	rv, err := api.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		return hash, err
	}

	pubKey, err := DecodeAddressSS58(senderKeypair.Address)
	if err != nil {
		return hash, err
	}

	key, err := gstypes.CreateStorageKey(meta, "System", "Account", pubKey)
	if err != nil {
		return hash, err
	}

	var accountInfo AccountInfo
	ok, err := api.RPC.State.GetStorageLatest(key, &accountInfo)
	if err != nil || !ok {
		return hash, err
	}

	nonce := uint32(accountInfo.Nonce)
	o := gstypes.SignatureOptions{
		BlockHash:          genesisHash,
		Era:                gstypes.ExtrinsicEra{IsMortalEra: false},
		GenesisHash:        genesisHash,
		Nonce:              gstypes.NewUCompactFromUInt(uint64(nonce)),
		SpecVersion:        rv.SpecVersion,
		Tip:                gstypes.NewUCompactFromUInt(0),
		TransactionVersion: rv.TransactionVersion,
	}

	// Sign the transaction using Alice's default account
	err = ext.Sign(senderKeypair, o)
	if err != nil {
		return hash, err
	}

	// Send the extrinsic
	hash, err = api.RPC.Author.SubmitExtrinsic(ext)

	return hash, err
}
