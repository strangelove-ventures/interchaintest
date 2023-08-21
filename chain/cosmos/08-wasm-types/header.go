package types

import (
	"github.com/cosmos/ibc-go/v7/modules/core/exported"
)

var _ exported.ClientMessage = &Header{}

func (Header) ClientType() string {
	return ""
}

func (Header) ValidateBasic() error {
	return nil
}
