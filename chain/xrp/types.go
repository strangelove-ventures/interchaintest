package xrp

//type ListReceivedByAddress []ReceivedByAddress

// {
// 	"key_type" : "ed25519",
// 	"public_key" : "nHBSoPKgRmerXivimvfvUbQwgfzLGtEZZB5zZ7UmB3rtK7r6Dfph",
// 	"revoked" : false,
// 	"secret_key" : "pniSwgVVTaAFWwsPw18AkhFPWyrYxTnoYY7tXNLZins2iffjFRw",
// 	"token_sequence" : 0
//  }
type ValidatorKeyOutput struct {
	KeyType string  `json:"address"`
	PublicKey string `json:"public_key"`
	Revoked bool `json:"revoked"`
	SecretKey string `json:"secret_key"`
	TokenSequence int `json:"token_sequence"`
}

// // PortConfig represents the configuration for different port types
// type PortConfig struct {
// 	Port     int      `toml:"port"`
// 	IP       string   `toml:"ip"`
// 	Admin    string   `toml:"admin,omitempty"`
// 	Protocol string   `toml:"protocol"`
// 	Protocols []string `toml:"-"` // Used internally when protocol contains multiple values
// }

// type ValidatorConfig struct {
// 	Validators []string `toml:"validators"`
// }

// // RippledConfig represents the complete rippled configuration
// type RippledConfig struct {
// 	Server struct {
// 		PortRPCAdminLocal  bool   `toml:"port_rpc_admin_local"`
// 		PortRPC           bool   `toml:"port_rpc"`
// 		PortWSAdminLocal  bool   `toml:"port_ws_admin_local"`
// 		PortWSPublic      bool   `toml:"port_ws_public"`
// 		PortPeer         bool   `toml:"port_peer"`
// 		SSLKey           string `toml:"ssl_key,omitempty"`
// 		SSLCert          string `toml:"ssl_cert,omitempty"`
// 		Standalone       int    `toml:"standalone,omitempty"`
// 	} `toml:"server"`

// 	PortRPCAdminLocal PortConfig `toml:"port_rpc_admin_local"`
// 	PortWSAdminLocal  PortConfig `toml:"port_ws_admin_local"`
// 	PortWSPublic      PortConfig `toml:"port_ws_public"`
// 	PortPeer          PortConfig `toml:"port_peer"`
// 	PortRPC           PortConfig `toml:"port_rpc"`

// 	NodeSize     string `toml:"node_size"`
	
// 	NodeDB struct {
// 		Type           string `toml:"type"`
// 		Path           string `toml:"path"`
// 		AdvisoryDelete int    `toml:"advisory_delete"`
// 		OnlineDelete   int    `toml:"online_delete"`
// 	} `toml:"node_db"`

// 	LedgerHistory    int    `toml:"ledger_history"`
// 	DatabasePath     string `toml:"database_path"`
// 	DebugLogfile     string `toml:"debug_logfile"`
// 	SNTPServers      []string `toml:"sntp_servers"`
// 	IPSFixed         []string `toml:"ips_fixed"`
// 	ValidatorsFile   string `toml:"validators_file"`
// 	RPCStartup       string `toml:"rpc_startup"`
// 	SSLVerify        int    `toml:"ssl_verify"`
// 	ValidationQuorum int    `toml:"validation_quorum"`
// 	NetworkID        int    `toml:"network_id"`
// 	ValidatorToken   string `toml:"validator_token"`
// }

// type ServerInfoResponse struct {
//     Result struct {
//         Info struct {
//             BuildVersion           string `json:"build_version"`
//             CompleteLedgers       string `json:"complete_ledgers"`
//             HostID                string `json:"hostid"`
//             InitialSyncDurationUs string `json:"initial_sync_duration_us"`
//             IoLatencyMs           int    `json:"io_latency_ms"`
//             JqTransOverflow       string `json:"jq_trans_overflow"`
//             LastClose struct {
//                 ConvergeTimeS float64 `json:"converge_time_s"`
//                 Proposers    int     `json:"proposers"`
//             } `json:"last_close"`
//             Load struct {
//                 JobTypes []struct {
//                     InProgress int    `json:"in_progress"`
//                     JobType    string `json:"job_type"`
//                 } `json:"job_types"`
//                 Threads int `json:"threads"`
//             } `json:"load"`
//             LoadFactor                 int    `json:"load_factor"`
//             NetworkID                 int    `json:"network_id"`
//             NodeSize                  string `json:"node_size"`
//             PeerDisconnects          string `json:"peer_disconnects"`
//             PeerDisconnectsResources string `json:"peer_disconnects_resources"`
//             Peers                    int    `json:"peers"`
//             Ports []struct {
//                 Port     string   `json:"port"`
//                 Protocol []string `json:"protocol"`
//             } `json:"ports"`
//             PubkeyNode        string `json:"pubkey_node"`
//             PubkeyValidator   string `json:"pubkey_validator"`
//             ServerState       string `json:"server_state"`
//             ServerStateDurationUs string `json:"server_state_duration_us"`
//             StateAccounting struct {
//                 Connected struct {
//                     DurationUs  string `json:"duration_us"`
//                     Transitions string `json:"transitions"`
//                 } `json:"connected"`
//                 Disconnected struct {
//                     DurationUs  string `json:"duration_us"`
//                     Transitions string `json:"transitions"`
//                 } `json:"disconnected"`
//                 Full struct {
//                     DurationUs  string `json:"duration_us"`
//                     Transitions string `json:"transitions"`
//                 } `json:"full"`
//                 Syncing struct {
//                     DurationUs  string `json:"duration_us"`
//                     Transitions string `json:"transitions"`
//                 } `json:"syncing"`
//                 Tracking struct {
//                     DurationUs  string `json:"duration_us"`
//                     Transitions string `json:"transitions"`
//                 } `json:"tracking"`
//             } `json:"state_accounting"`
//             Time string `json:"time"`
//             Uptime int    `json:"uptime"`
//             ValidatedLedger struct {
//                 Age           int     `json:"age"`
//                 BaseFeeXrp   float64 `json:"base_fee_xrp"`
//                 Hash         string  `json:"hash"`
//                 ReserveBaseXrp float64 `json:"reserve_base_xrp"`
//                 ReserveIncXrp  float64 `json:"reserve_inc_xrp"`
//                 Seq           int     `json:"seq"`
//             } `json:"validated_ledger"`
//             ValidationQuorum int `json:"validation_quorum"`
//             ValidatorList struct {
//                 Count      int    `json:"count"`
//                 Expiration string `json:"expiration"`
//                 Status     string `json:"status"`
//             } `json:"validator_list"`
//         } `json:"info"`
//         Status string `json:"status"`
//     } `json:"result"`
// }

// type AccountInfoResponse struct {
//     Result struct {
//         // Common fields present in both success and error responses
//         Status             string `json:"status"`
//         Validated          bool   `json:"validated"`
//         LedgerCurrentIndex int    `json:"ledger_current_index"`

//         // Error response fields
//         Account       *string `json:"account,omitempty"`
//         Error        *string `json:"error,omitempty"`
//         ErrorCode    *int    `json:"error_code,omitempty"`
//         ErrorMessage *string `json:"error_message,omitempty"`
//         Request      *struct {
//             Account    string `json:"account"`
//             APIVersion int    `json:"api_version"`
//             Command    string `json:"command"`
//         } `json:"request,omitempty"`

//         // Success response fields
//         AccountData *struct {
//             Account           string `json:"Account"`
//             Balance          string `json:"Balance"`
//             Flags           int    `json:"Flags"`
//             LedgerEntryType string `json:"LedgerEntryType"`
//             OwnerCount      int    `json:"OwnerCount"`
//             PreviousTxnID   string `json:"PreviousTxnID"`
//             PreviousTxnLgrSeq int    `json:"PreviousTxnLgrSeq"`
//             Sequence         int    `json:"Sequence"`
//             Index           string `json:"index"`
//         } `json:"account_data,omitempty"`
//         AccountFlags *struct {
//             DefaultRipple         bool `json:"defaultRipple"`
//             DepositAuth          bool `json:"depositAuth"`
//             DisableMasterKey     bool `json:"disableMasterKey"`
//             DisallowIncomingXRP  bool `json:"disallowIncomingXRP"`
//             GlobalFreeze         bool `json:"globalFreeze"`
//             NoFreeze            bool `json:"noFreeze"`
//             PasswordSpent       bool `json:"passwordSpent"`
//             RequireAuthorization bool `json:"requireAuthorization"`
//             RequireDestinationTag bool `json:"requireDestinationTag"`
//         } `json:"account_flags,omitempty"`
//     } `json:"result"`
// }

type WalletResponse struct {
    Result struct {
        AccountID     string `json:"account_id"`
        KeyType       string `json:"key_type"`
        MasterKey     string `json:"master_key"`
        MasterSeed    string `json:"master_seed"`
        MasterSeedHex string `json:"master_seed_hex"`
        PublicKey     string `json:"public_key"`
        PublicKeyHex  string `json:"public_key_hex"`
        Status        string `json:"status"`
    } `json:"result"`
}

// type SubmitResponse struct {
// 	Result struct {
// 		Deprecated          string `json:"deprecated,omitempty"`
// 		EngineResult        string `json:"engine_result"`
// 		EngineResultCode    int    `json:"engine_result_code"`
// 		EngineResultMessage string `json:"engine_result_message"`
// 		Status              string `json:"status"`
// 		TxBlob              string `json:"tx_blob"`
// 		TxJson struct {
// 			Account         string `json:"Account"`
// 			Amount          string `json:"Amount"`
// 			DeliverMax      string `json:"DeliverMax"`
// 			Destination     string `json:"Destination"` 
// 			Fee             string `json:"Fee"`
// 			Flags           int64  `json:"Flags"`
// 			Sequence        int    `json:"Sequence"`
// 			SigningPubKey   string `json:"SigningPubKey"`
// 			TxnSignature    string `json:"TxnSignature"`
// 			TransactionType string `json:"TransactionType"`
// 			Hash            string `json:"hash"`
// 		} `json:"tx_json"`
// 	} `json:"result"`
//  }

//  type Transaction struct {
// 	TransactionType string `json:"TransactionType"`
// 	Account         string `json:"Account"`
// 	Destination     string `json:"Destination"`
// 	Amount          string `json:"Amount"`
// 	NetworkID       int    `json:"NetworkID"`
//  }