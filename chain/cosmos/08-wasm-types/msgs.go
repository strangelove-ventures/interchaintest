package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	_ sdk.Msg = &MsgPushNewWasmCode{}
)

func (m MsgPushNewWasmCode) ValidateBasic() error {
	return nil
}


func (m MsgPushNewWasmCode) GetSigners() []sdk.AccAddress {
	signer, err := sdk.AccAddressFromBech32(m.Signer)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{signer}
}

