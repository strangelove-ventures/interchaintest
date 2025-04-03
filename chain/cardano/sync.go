package cardano

import (
	"encoding/hex"

	"github.com/blinklabs-io/gouroboros/ledger"
	"github.com/blinklabs-io/gouroboros/protocol/chainsync"
	ouroboroscommon "github.com/blinklabs-io/gouroboros/protocol/common"
	"go.uber.org/zap"
)

func (a *AdaChain) chainSyncRollForwardHandler(
	ctx chainsync.CallbackContext,
	blockType uint,
	blockData any,
	tip chainsync.Tip,
) error {
	switch h := blockData.(type) {
	case ledger.Block:
		hashBz, err := hex.DecodeString(h.Hash())
		if err != nil {
			a.log.Error("fail to decode block hash", zap.Error(err))
		}
		a.blocksLock.Lock()
		point := ouroboroscommon.NewPoint(h.SlotNumber(), hashBz)
		a.blocks[h.SlotNumber()] = point
		a.blocksLock.Unlock()

		a.txWaitersLock.Lock()
		for _, tx := range h.Transactions() {
			if waiter, ok := a.txWaiters[tx.Hash()]; ok {
				waiter <- struct{}{}
				delete(a.txWaiters, tx.Hash())
			}
		}
		a.txWaitersLock.Unlock()

	case ledger.BlockHeader:
		a.log.Warn("block type is ledger.BlockHeader, chain-sync n2c protocol expects ledger.Block")
	}
	return nil
}

func (a *AdaChain) chainSyncRollBackwardHandler(
	ctx chainsync.CallbackContext,
	point ouroboroscommon.Point,
	tip chainsync.Tip,
) error {
	return nil
}
