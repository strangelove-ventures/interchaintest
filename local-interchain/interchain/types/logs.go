package types

type MainLogs struct {
	StartTime uint64       `json:"start_time" yaml:"start_time"`
	Chains    []LogOutput  `json:"chains" yaml:"chains"`
	Channels  []IBCChannel `json:"ibc_channels" yaml:"ibc_channels"`
}

type LogOutput struct {
	ChainID     string   `json:"chain_id" yaml:"chain_id"`
	ChainName   string   `json:"chain_name" yaml:"chain_name"`
	RPCAddress  string   `json:"rpc_address" yaml:"rpc_address"`
	RESTAddress string   `json:"rest_address" yaml:"rest_address"`
	GRPCAddress string   `json:"grpc_address" yaml:"grpc_address"`
	P2PAddress  string   `json:"p2p_address" yaml:"p2p_address"`
	IBCPath     []string `json:"ibc_paths" yaml:"ibc_paths"`
}
