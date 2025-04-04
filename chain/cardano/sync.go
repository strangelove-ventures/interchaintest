package cardano

import (
	"sync"

	"github.com/blinklabs-io/gouroboros/ledger"
	"github.com/blinklabs-io/gouroboros/protocol/chainsync"
	ouroboroscommon "github.com/blinklabs-io/gouroboros/protocol/common"
)

type block struct {
	SlotNumber  uint64
	Hash        string
	BlockNumber uint64
}

type blockDB struct {
	blocks     []block
	blocksLock sync.Mutex
}

func (db *blockDB) append(blockNumber, slotNumber uint64, hash string) {
	db.blocksLock.Lock()
	defer db.blocksLock.Unlock()

	// zero based index, index = blockNumber-1
	for i := len(db.blocks); i < int(blockNumber-1); i++ {
		// fill empty blocks if necessary
		db.blocks = append(db.blocks, block{})
	}

	// append the new block
	db.blocks = append(db.blocks, block{
		SlotNumber:  slotNumber,
		Hash:        hash,
		BlockNumber: blockNumber,
	})
}

func (db *blockDB) get(blockNumber uint64) (block, bool) {
	idx := int(blockNumber - 1)
	db.blocksLock.Lock()
	defer db.blocksLock.Unlock()
	if idx < 0 || idx >= len(db.blocks) {
		return block{}, false
	}
	return db.blocks[idx], true
}

func (db *blockDB) last() (block, bool) {
	db.blocksLock.Lock()
	defer db.blocksLock.Unlock()
	if len(db.blocks) == 0 {
		return block{}, false
	}
	return db.blocks[len(db.blocks)-1], true
}

func (a *AdaChain) chainSyncRollForwardHandler(
	ctx chainsync.CallbackContext,
	blockType uint,
	blockData any,
	tip chainsync.Tip,
) error {
	switch h := blockData.(type) {
	case ledger.Block:
		a.blocks.append(h.BlockNumber(), h.SlotNumber(), h.Hash())

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
