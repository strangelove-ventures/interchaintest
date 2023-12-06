package cosmos

import (
	"context"
)

// CrisisInvariantBroken executes the crisis invariant broken command.
func (tn *ChainNode) CrisisInvariantBroken(ctx context.Context, keyName, moduleName, route string) error {
	_, err := tn.ExecTx(ctx,
		keyName, "crisis", "invariant-broken", moduleName, route,
	)
	return err
}
