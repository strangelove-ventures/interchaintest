package client

import (
    "encoding/json"
)

type RPCRequest struct {
    Method string      `json:"method"`
    Params []any      `json:"params"`
    ID     int         `json:"id"`
}

type RPCResponse struct {
    Result json.RawMessage `json:"result"`
    Error  *struct {
        Message string `json:"message"`
        Code    int    `json:"code"`
    } `json:"error,omitempty"`
    ID int `json:"id"`
}

type ServerInfoResponse struct {
	Info struct {
		BuildVersion           string `json:"build_version"`
		CompleteLedgers       string `json:"complete_ledgers"`
		HostID                string `json:"hostid"`
		InitialSyncDurationUs string `json:"initial_sync_duration_us"`
		IoLatencyMs           int    `json:"io_latency_ms"`
		JqTransOverflow       string `json:"jq_trans_overflow"`
		LastClose struct {
			ConvergeTimeS float64 `json:"converge_time_s"`
			Proposers    int     `json:"proposers"`
		} `json:"last_close"`
		Load struct {
			JobTypes []struct {
				InProgress int    `json:"in_progress"`
				JobType    string `json:"job_type"`
			} `json:"job_types"`
			Threads int `json:"threads"`
		} `json:"load"`
		LoadFactor                 int    `json:"load_factor"`
		NetworkID                 int    `json:"network_id"`
		NodeSize                  string `json:"node_size"`
		PeerDisconnects          string `json:"peer_disconnects"`
		PeerDisconnectsResources string `json:"peer_disconnects_resources"`
		Peers                    int    `json:"peers"`
		Ports []struct {
			Port     string   `json:"port"`
			Protocol []string `json:"protocol"`
		} `json:"ports"`
		PubkeyNode              string `json:"pubkey_node"`
		PubkeyValidator         string `json:"pubkey_validator"`
		ServerState             string `json:"server_state"`
		ServerStateDurationUs   string `json:"server_state_duration_us"`
		StateAccounting struct {
			Connected struct {
				DurationUs  string `json:"duration_us"`
				Transitions string `json:"transitions"`
			} `json:"connected"`
			Disconnected struct {
				DurationUs  string `json:"duration_us"`
				Transitions string `json:"transitions"`
			} `json:"disconnected"`
			Full struct {
				DurationUs  string `json:"duration_us"`
				Transitions string `json:"transitions"`
			} `json:"full"`
			Syncing struct {
				DurationUs  string `json:"duration_us"`
				Transitions string `json:"transitions"`
			} `json:"syncing"`
			Tracking struct {
				DurationUs  string `json:"duration_us"`
				Transitions string `json:"transitions"`
			} `json:"tracking"`
		} `json:"state_accounting"`
		Time            string `json:"time"`
		Uptime         int    `json:"uptime"`
		ValidatedLedger struct {
			Age           int     `json:"age"`
			BaseFeeXrp   float64 `json:"base_fee_xrp"`
			Hash         string  `json:"hash"`
			ReserveBaseXrp float64 `json:"reserve_base_xrp"`
			ReserveIncXrp  float64 `json:"reserve_inc_xrp"`
			Seq           int     `json:"seq"`
		} `json:"validated_ledger"`
		ValidationQuorum int `json:"validation_quorum"`
		ValidatorList struct {
			Count      int    `json:"count"`
			Expiration string `json:"expiration"`
			Status     string `json:"status"`
		} `json:"validator_list"`
	} `json:"info"`
	Status string `json:"status"`
 }

 type AccountInfoResponse struct {
    // Common fields present in both success and error responses
    Status             string `json:"status"`
    Validated          bool   `json:"validated"`
    LedgerCurrentIndex int    `json:"ledger_current_index"`

    // Error response fields
    Account       *string `json:"account,omitempty"`
    Error        *string `json:"error,omitempty"`
    ErrorCode    *int    `json:"error_code,omitempty"`
    ErrorMessage *string `json:"error_message,omitempty"`
    Request      *struct {
        Account    string `json:"account"`
        APIVersion int    `json:"api_version"`
        Command    string `json:"command"`
    } `json:"request,omitempty"`

    // Success response fields
    AccountData *struct {
        Account           string `json:"Account"`
        Balance          string `json:"Balance"`
        Flags           int    `json:"Flags"`
        LedgerEntryType string `json:"LedgerEntryType"`
        OwnerCount      int    `json:"OwnerCount"`
        PreviousTxnID   string `json:"PreviousTxnID"`
        PreviousTxnLgrSeq int    `json:"PreviousTxnLgrSeq"`
        Sequence         int    `json:"Sequence"`
        Index           string `json:"index"`
    } `json:"account_data,omitempty"`
    AccountFlags *struct {
        DefaultRipple         bool `json:"defaultRipple"`
        DepositAuth          bool `json:"depositAuth"`
        DisableMasterKey     bool `json:"disableMasterKey"`
        DisallowIncomingXRP  bool `json:"disallowIncomingXRP"`
        GlobalFreeze         bool `json:"globalFreeze"`
        NoFreeze            bool `json:"noFreeze"`
        PasswordSpent       bool `json:"passwordSpent"`
        RequireAuthorization bool `json:"requireAuthorization"`
        RequireDestinationTag bool `json:"requireDestinationTag"`
    } `json:"account_flags,omitempty"`
}

// Transaction structures
type Payment struct {
    TransactionType    string `json:"TransactionType"`
    Account           string `json:"Account"`
    Destination      string `json:"Destination"`
    Amount           string `json:"Amount"`
    Sequence         int    `json:"Sequence"`
    Fee             string `json:"Fee"`
    SigningPubKey   string `json:"SigningPubKey"`
    NetworkID       uint32 `json:"NetworkID"`
    TxnSignature    string `json:"TxnSignature,omitempty"`
	Flags uint32
}

type SubmitResponse struct {
	Error string `json:"error,omitempty"`
	ErrorCode int `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	Request any `json:"request,omitempty"`
	Status string `json:"status,omitempty"`
}