package types

type MainLogs struct {
	StartTime uint64       `json:"start_time"`
	Chains    []LogOutput  `json:"chains"`
	Channels  []IBCChannel `json:"ibc_channels"`
}

type LogOutput struct {
	ChainID     string   `json:"chain_id"`
	ChainName   string   `json:"chain_name"`
	RPCAddress  string   `json:"rpc_address"`
	RESTAddress string   `json:"rest_address"`
	GRPCAddress string   `json:"grpc_address"`
	IBCPath     []string `json:"ibc_paths"`
}
