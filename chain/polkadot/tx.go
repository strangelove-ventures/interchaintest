package polkadot

import (
	"encoding/hex"

	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	gstypes "github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/strangelove-ventures/interchaintest/v6/ibc"
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

	// Create the extrinsic
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
