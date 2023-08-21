package types

import (
	exported "github.com/cosmos/ibc-go/v7/modules/core/exported"
)

var _ exported.ClientMessage = &Misbehaviour{}

func (Misbehaviour) ClientType() string {
	return ""
}

func (Misbehaviour) ValidateBasic() error {
	return nil
}
