package polkadot

import (
	"cosmossdk.io/math"
	gsrpc "github.com/misko9/go-substrate-rpc-client/v4"
	gstypes "github.com/misko9/go-substrate-rpc-client/v4/types"
)

// GetBalance fetches the current balance for a specific account address using the SubstrateAPI
func GetBalance(api *gsrpc.SubstrateAPI, address string) (math.Int, error) {
	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		return math.Int{}, err
	}
	pubKey, err := DecodeAddressSS58(address)
	if err != nil {
		return math.Int{}, err
	}
	key, err := gstypes.CreateStorageKey(meta, "System", "Account", pubKey, nil)
	if err != nil {
		return math.Int{}, err
	}

	var accountInfo AccountInfo
	ok, err := api.RPC.State.GetStorageLatest(key, &accountInfo)
	if err != nil {
		return math.Int{}, err
	}
	if !ok {
		return math.Int{}, nil
	}

	return math.NewIntFromBigInt(accountInfo.Data.Free.Int), nil
}
