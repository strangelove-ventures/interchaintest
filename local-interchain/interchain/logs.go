package interchain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	types "github.com/strangelove-ventures/interchaintest/local-interchain/interchain/types"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/chain/ethereum/foundry"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func WriteRunningChains(configsDir string, bz []byte) {
	path := filepath.Join(configsDir, "configs")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			panic(err)
		}
	}

	file := filepath.Join(path, "logs.json")
	_ = os.WriteFile(file, bz, 0o644)
}

func DumpChainsInfoToLogs(configDir string, config *types.Config, chains []ibc.Chain, connections []types.IBCChannel) {
	mainLogs := types.MainLogs{
		StartTime: uint64(time.Now().Unix()),
		Chains:    []types.LogOutput{},
		Channels:  connections,
	}

	// Iterate chain config & get the ibc chain's to save data to logs.
	for idx, chain := range config.Chains {
		switch chains[idx].(type) {
		case *cosmos.CosmosChain:
			chainObj := chains[idx].(*cosmos.CosmosChain)

			ibcPaths := chain.IBCPaths
			if ibcPaths == nil {
				ibcPaths = []string{}
			}

			log := types.LogOutput{
				ChainID:     chainObj.Config().ChainID,
				ChainName:   chainObj.Config().Name,
				RPCAddress:  chainObj.GetHostRPCAddress(),
				RESTAddress: chainObj.GetHostAPIAddress(),
				GRPCAddress: chainObj.GetHostGRPCAddress(),
				P2PAddress:  chainObj.GetHostPeerAddress(),
				IBCPath:     ibcPaths,
			}

			mainLogs.Chains = append(mainLogs.Chains, log)
		case *foundry.AnvilChain:
			chainObj := chains[idx].(*foundry.AnvilChain)

			log := types.LogOutput{
				ChainID:    chainObj.Config().ChainID,
				ChainName:  chainObj.Config().Name,
				RPCAddress: chainObj.GetHostRPCAddress(),
			}

			mainLogs.Chains = append(mainLogs.Chains, log)
		}
	}

	bz, _ := json.MarshalIndent(mainLogs, "", "  ")
	WriteRunningChains(configDir, []byte(bz))
}

// == Zap Logger ==
func InitLogger(logFile *os.File) (*zap.Logger, error) {
	// Production logger that saves logs to file and console.
	pe := zap.NewProductionEncoderConfig()

	fileEncoder := zapcore.NewJSONEncoder(pe)
	consoleEncoder := zapcore.NewConsoleEncoder(pe)

	level := zap.InfoLevel

	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, zapcore.AddSync(logFile), level),
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level),
	)

	return zap.New(core), nil
}
