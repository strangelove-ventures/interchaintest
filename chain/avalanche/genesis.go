package avalanche

import "time"

type (
	GenesisLockedAmount struct {
		Amount   uint32 `json:"amount"`
		Locktime uint32 `json:"locktime"`
	}
	GenesisAllocation struct {
		ETHAddr        string                `json:"ethAddr"`
		AVAXAddr       string                `json:"avaxAddr"`
		InitialAmount  uint32                `json:"initialAmount"`
		UnlockSchedule []GenesisLockedAmount `json:"unlockSchedule"`
	}
	StakerSigner struct {
		PublicKey         string `json:"publicKey"`
		ProofOfPossession string `json:"proofOfPossession"`
	}
	GenesisStaker struct {
		NodeID        string       `json:"nodeID"`
		RewardAddress string       `json:"rewardAddress"`
		DelegationFee uint32       `json:"delegationFee"`
		Signer        StakerSigner `json:"signer"`
	}
	Genesis struct {
		NetworkID uint32 `json:"networkID"`

		Allocations []GenesisAllocation `json:"allocations"`

		StartTime                  uint32          `json:"startTime"`
		InitialStakeDuration       uint32          `json:"initialStakeDuration"`
		InitialStakeDurationOffset uint32          `json:"initialStakeDurationOffset"`
		InitialStakedFunds         []string        `json:"initialStakedFunds"`
		InitialStakers             []GenesisStaker `json:"initialStakers"`

		CChainGenesis string `json:"cChainGenesis"`

		Message string `json:"message"`
	}
)

func NewGenesis(networkID uint32, allocations []GenesisAllocation, initialStakedFunds []string, stakers []GenesisStaker) Genesis {
	return Genesis{
		NetworkID:                  networkID,
		Allocations:                allocations,
		StartTime:                  uint32(time.Now().Unix()),
		InitialStakeDuration:       31536000,
		InitialStakeDurationOffset: 5400,
		InitialStakedFunds:         initialStakedFunds,
		InitialStakers:             stakers,
		CChainGenesis:              "{\"config\":{\"chainId\":43112,\"homesteadBlock\":0,\"daoForkBlock\":0,\"daoForkSupport\":true,\"eip150Block\":0,\"eip150Hash\":\"0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0\",\"eip155Block\":0,\"eip158Block\":0,\"byzantiumBlock\":0,\"constantinopleBlock\":0,\"petersburgBlock\":0,\"istanbulBlock\":0,\"muirGlacierBlock\":0,\"apricotPhase1BlockTimestamp\":0,\"apricotPhase2BlockTimestamp\":0},\"nonce\":\"0x0\",\"timestamp\":\"0x0\",\"extraData\":\"0x00\",\"gasLimit\":\"0x5f5e100\",\"difficulty\":\"0x0\",\"mixHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"coinbase\":\"0x0000000000000000000000000000000000000000\",\"alloc\":{\"" + allocations[0].ETHAddr[2:] + "\":{\"balance\":\"0x295BE96E64066972000000\"}},\"number\":\"0x0\",\"gasUsed\":\"0x0\",\"parentHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\"}",
		Message:                    "{{ fun_quote }}",
	}
}
