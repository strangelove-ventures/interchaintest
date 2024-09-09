module github.com/strangelove-ventures/interchaintest/v9

go 1.22.2

replace (
	github.com/ChainSafe/go-schnorrkel => github.com/ChainSafe/go-schnorrkel v0.0.0-20200405005733-88cbf1b4c40d
	github.com/ChainSafe/go-schnorrkel/1 => github.com/ChainSafe/go-schnorrkel v1.0.0
	github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
	github.com/vedhavyas/go-subkey => github.com/strangelove-ventures/go-subkey v1.0.7
	github.com/misko9/go-substrate-rpc-client/v4 => github.com/DimitrisJim/go-substrate-rpc-client/v4 v4.0.0-20240717100841-406da076c1d5
)

//TODO: remove everything below after tags are created
replace (
	cosmossdk.io/api => cosmossdk.io/api v0.7.3-0.20240815194237-858ec2fcb897 // main
	cosmossdk.io/client/v2 => cosmossdk.io/client/v2 v2.0.0-20240905174638-8ce77cbb2450
	cosmossdk.io/core => cosmossdk.io/core v0.12.1-0.20240906083041-6033330182c7 // main
	cosmossdk.io/store => cosmossdk.io/store v1.0.0-rc.0.0.20240906090851-36d9b25e8981 // main
	cosmossdk.io/x/accounts => cosmossdk.io/x/accounts v0.0.0-20240905174638-8ce77cbb2450
	cosmossdk.io/x/accounts/defaults/lockup => cosmossdk.io/x/accounts/defaults/lockup v0.0.0-20240905174638-8ce77cbb2450
	cosmossdk.io/x/accounts/defaults/multisig => cosmossdk.io/x/accounts/defaults/multisig v0.0.0-20240905174638-8ce77cbb2450
	cosmossdk.io/x/authz => cosmossdk.io/x/authz v0.0.0-20240905174638-8ce77cbb2450
	cosmossdk.io/x/bank => cosmossdk.io/x/bank v0.0.0-20240905174638-8ce77cbb2450
	cosmossdk.io/x/circuit => cosmossdk.io/x/circuit v0.0.0-20240905174638-8ce77cbb2450
	cosmossdk.io/x/consensus => cosmossdk.io/x/consensus v0.0.0-20240905174638-8ce77cbb2450
	cosmossdk.io/x/evidence => cosmossdk.io/x/evidence v0.0.0-20240905174638-8ce77cbb2450
	cosmossdk.io/x/feegrant => cosmossdk.io/x/feegrant v0.0.0-20240905174638-8ce77cbb2450
	cosmossdk.io/x/gov => cosmossdk.io/x/gov v0.0.0-20240905174638-8ce77cbb2450
	cosmossdk.io/x/group => cosmossdk.io/x/group v0.0.0-20240905174638-8ce77cbb2450
	cosmossdk.io/x/mint => cosmossdk.io/x/mint v0.0.0-20240909082436-01c0e9ba3581
	cosmossdk.io/x/params => cosmossdk.io/x/params v0.0.0-20240905174638-8ce77cbb2450
	cosmossdk.io/x/protocolpool => cosmossdk.io/x/protocolpool v0.0.0-20240905174638-8ce77cbb2450
	cosmossdk.io/x/slashing => cosmossdk.io/x/slashing v0.0.0-20240905174638-8ce77cbb2450
	cosmossdk.io/x/staking => cosmossdk.io/x/staking v0.0.0-20240905174638-8ce77cbb2450
	cosmossdk.io/x/tx => cosmossdk.io/x/tx v0.13.4-0.20240815194237-858ec2fcb897 // main
	cosmossdk.io/x/upgrade => cosmossdk.io/x/upgrade v0.0.0-20240905174638-8ce77cbb2450
	github.com/cometbft/cometbft => github.com/cometbft/cometbft v1.0.0-rc1.0.20240908111210-ab0be101882f
	github.com/cosmos/cosmos-sdk => github.com/cosmos/cosmos-sdk v0.52.0-alpha.1.0.20240909082436-01c0e9ba3581

	// marko/gomod_change - 26d765eed2f7221f5fadc1866e971dab097e595e - SDK v52 work
	github.com/cosmos/ibc-go/v9 => github.com/cosmos/ibc-go/v9 v9.0.0-20240909163231-26d765eed2f7

	// TODO: wasmd sdk v52
)

require (
	cosmossdk.io/math v1.3.0
	cosmossdk.io/store v1.1.0
	cosmossdk.io/x/feegrant v0.1.0
	cosmossdk.io/x/upgrade v0.1.3
	github.com/99designs/keyring v1.2.2
	github.com/BurntSushi/toml v1.4.0
	github.com/ChainSafe/go-schnorrkel/1 v0.0.0-00010101000000-000000000000
	// github.com/CosmWasm/wasmd v0.42.1-0.20230928145107-894076a25cb2 // TODO: update
	github.com/StirlingMarketingGroup/go-namecase v1.0.0
	github.com/atotto/clipboard v0.1.4
	github.com/avast/retry-go/v4 v4.5.1
	github.com/cosmos/go-bip39 v1.0.0
	github.com/cosmos/gogoproto v1.5.0
	// github.com/cosmos/ibc-go/modules/capability v1.0.0 // TODO: looks like it is removed
	github.com/cosmos/ibc-go/v9 v9.0.0-00000000000000-000000000000
	// github.com/cosmos/interchain-security/v5 v5.0.0-alpha1.0.20240424193412-7cd900ad2a74 // TODO:
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc
	github.com/decred/dcrd/dcrec/secp256k1/v2 v2.0.1
	github.com/docker/docker v24.0.9+incompatible
	github.com/docker/go-connections v0.5.0
	github.com/ethereum/go-ethereum v1.14.5
	github.com/gdamore/tcell/v2 v2.7.4
	github.com/gogo/protobuf v1.3.3
	github.com/google/go-cmp v0.6.0
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/hashicorp/go-version v1.7.0
	github.com/icza/dyno v0.0.0-20220812133438-f0b6f8a18845
	github.com/libp2p/go-libp2p v0.31.0
	github.com/misko9/go-substrate-rpc-client/v4 v4.0.0-20240603204351-26b456ae3afe
	github.com/mr-tron/base58 v1.2.0
	github.com/pelletier/go-toml v1.9.5
	github.com/pelletier/go-toml/v2 v2.2.2
	github.com/rivo/tview v0.0.0-20220307222120-9994674d60a8
	github.com/spf13/cobra v1.8.0
	github.com/stretchr/testify v1.9.0
	github.com/tidwall/gjson v1.17.1
	github.com/tyler-smith/go-bip32 v1.0.0
	github.com/tyler-smith/go-bip39 v1.1.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	golang.org/x/crypto v0.24.0
	golang.org/x/mod v0.18.0
	golang.org/x/sync v0.7.0
	golang.org/x/tools v0.22.0
	google.golang.org/grpc v1.65.0
	gopkg.in/yaml.v3 v3.0.1
	modernc.org/sqlite v1.30.1
)
