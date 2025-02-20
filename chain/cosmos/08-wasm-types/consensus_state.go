package types

import (
	"github.com/cosmos/ibc-go/v10/modules/core/exported"
)

var _ exported.ConsensusState = (*ConsensusState)(nil)

func (m ConsensusState) ClientType() string {
	return ""
}

func (m ConsensusState) GetTimestamp() uint64 {
	return 0
}

func (m ConsensusState) ValidateBasic() error {
	return nil
}
