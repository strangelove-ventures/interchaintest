package interchaintest

import (
	"context"
	"fmt"

	"github.com/strangelove-ventures/interchaintest/v9/ibc"
)

// ErrPclientdInitialization is returned if the CreateClientNode call fails while initializing a new instance of
// pclientd for a newly created user account on Penumbra.
var ErrPclientdInitialization = fmt.Errorf("failed to initialize new pclientd instance")

// CreatePenumbraClient should be called after a new test user account has been created on Penumbra.
// This function initializes a new instance of pclientd which allows private user state to be tracked and managed
// via a client daemon i.e. it is used to sign and broadcast txs as well as querying private user state on chain.
//
// Note: this function cannot be called until the chain is started as pclientd attempts to dial the running pd instance,
// so that it can sync with the current chain tip. It also should be noted that this function should ONLY be called
// after a new test user has been generated via one of the GetAndFundTestUser helper functions or a call to the
// chain.CreateKey or chain.RecoverKey methods.
func CreatePenumbraClient(ctx context.Context, c ibc.Chain, keyName string) error {
	//if pen, ok := c.(*penumbra.PenumbraChain); ok {
	//	err := pen.CreateClientNode(ctx, keyName)
	//	if err != nil {
	//		return fmt.Errorf("%w for keyname %s: %w", ErrPclientdInitialization, keyName, err)
	//	}
	//}

	return nil
}
