package types

import (
	"github.com/cosmos/ibc-go/v10/modules/core/exported"
)

var _ exported.ClientMessage = &ClientMessage{}

// ClientType is a Wasm light client.
func (c ClientMessage) ClientType() string {
	return "08-wasm"
}

// ValidateBasic defines a basic validation for the wasm client message.
func (c ClientMessage) ValidateBasic() error {
	return nil
}
