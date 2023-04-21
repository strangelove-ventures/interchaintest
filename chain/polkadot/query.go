package polkadot

import (
	gsrpc "github.com/misko9/go-substrate-rpc-client/v4"
	gstypes "github.com/misko9/go-substrate-rpc-client/v4/types"
)

// GetBalance fetches the current balance for a specific account address using the SubstrateAPI
func GetBalance(api *gsrpc.SubstrateAPI, address string) (int64, error) {
	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		return -1, err
	}
	pubKey, err := DecodeAddressSS58(address)
	if err != nil {
		return -2, err
	}
	key, err := gstypes.CreateStorageKey(meta, "System", "Account", pubKey, nil)
	if err != nil {
		return -3, err
	}

	var accountInfo AccountInfo
	ok, err := api.RPC.State.GetStorageLatest(key, &accountInfo)
	if err != nil {
		return -4, err
	}
	if !ok {
		return -5, nil
	}

	return accountInfo.Data.Free.Int64(), nil
}
