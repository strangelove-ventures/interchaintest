package xrp

import (
	"fmt"
	"strings"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

func DefaultXrpChainConfig(
	name string,
) ibc.ChainConfig {
	return ibc.ChainConfig{
		Type:           "xrp",
		Name:           name,
		ChainID:        "1234",
		Bech32Prefix:   "n/a",
		CoinType:       "144",
		Denom:          "drop",
		GasPrices:      "0.00001", // flat fee
		GasAdjustment:  0,         // N/A
		TrustingPeriod: "0",
		NoHostMount:    false,
		Images: []ibc.DockerImage{
			{
				Repository: "xrpllabsofficial/xrpld",
				Version:    "2.3.0",
				UIDGID:     "1025:1025",
			},
		},
		Bin: "rippled,/opt/ripple/bin/validator-keys",
		HostPortOverride: map[int]int{
			80:    8001,
			5005:  5005,
			6006:  6006,
			51235: 51235,
		},
	}
}

func NewValidatorConfig(validator string) []byte {
	return []byte(
		fmt.Sprintf("[validators]\n    %s", validator),
	)
}

func NewRippledConfig(validatorTokenInput string) []byte {
	server := "[server]\nport_rpc_admin_local\nport_rpc\nport_ws_admin_local\nport_ws_public\nport_peer\nstandalone=1\n\n"
	portRPCAdminLocal := "[port_rpc_admin_local]\nport = 5005\nip = 0.0.0.0\nadmin = 0.0.0.0\nprotocol = http\n\n"
	portWsAdminLocal := "[port_ws_admin_local]\nport = 6006\nip = 0.0.0.0\nadmin = 0.0.0.0\nprotocol = ws\n\n"
	portWsPublic := "[port_ws_public]\nport = 80\nip = 0.0.0.0\nprotocol = ws\n\n"
	portPeer := "[port_peer]\nport = 51235\nip = 0.0.0.0\nprotocol = peer\n\n"
	portRPC := "[port_rpc]\nport = 51234\nip = 0.0.0.0\nadmin = 127.0.0.1\nprotocol = https, http\n\n"
	nodeSize := "[node_size]\nsmall\n\n"
	nodeDB := "[node_db]\ntype=NuDB\npath=/var/lib/rippled/db/nudb\nadvisory_delete=0\nonline_delete=256\n\n"
	ledgerHistory := "[ledger_history]\n256\n\n"
	dbPath := "[database_path]\n/var/lib/rippled/db\n\n"
	debugLogfile := "[debug_logfile]\n/var/log/rippled/debug.log\n\n"
	sntpServers := "[sntp_servers]\ntime.windows.com\ntime.apple.com\ntime.nist.gov\npool.ntp.org\n\n"
	validatorsFile := "[validators_file]\n/home/xrp/config/validators.txt\n\n"
	rpcStartup := "[rpc_startup]\n{ \"command\": \"log_level\", \"severity\": \"debug\" }\n\n"
	// rpcStartup := "[rpc_startup]\n{ \"command\": \"log_level\", \"severity\": \"warning\" }\n\n"
	sslVerify := "[ssl_verify]\n0\n\n"
	validationQuorum := "[validation_quorum]\n0\n\n"
	networkID := "[network_id]\n1234\n\n"
	validatorToken := "[validator_token]\n"
	ipsFixed := "[ips_fixed]\nxrp-1234-TestXrp 51235\n\n"
	voting := "[voting]\nreference_fee=10\naccount_reserve=1000000\nowner_reserve=200000\n\n"

	return []byte(
		fmt.Sprintf(
			"%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s\n",
			server,
			portRPCAdminLocal,
			portWsAdminLocal,
			portWsPublic,
			portPeer,
			portRPC,
			nodeSize,
			nodeDB,
			ledgerHistory,
			dbPath,
			debugLogfile,
			sntpServers,
			validatorsFile,
			rpcStartup,
			sslVerify,
			validationQuorum,
			voting,
			networkID,
			ipsFixed,
			validatorToken,
			strings.TrimSpace(validatorTokenInput),
		),
	)
}
