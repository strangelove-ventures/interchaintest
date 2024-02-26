# Local Interchain

A simple way to config and run IBC local chain testing environments with any language of choice for interaction.

## Repository CI

- [**Rust**](./rust/README.md)

## Installing Locally

**Install on Mac / Linux**
```bash
git clone https://github.com/strangelove-ventures/interchaintest.git

cd interchaintest/local-interchain

# NOTE: your binary will link back to this location of where you install.
# If you rename the folder or move it, you need to `make install` the binary again.
make install
```
**Install on Windows**

Follow [this guide](./docs/WINDOWS.md) to setup the Windows OS environment for installing Local Interchain.

## Running

- Copy: `cp ./chains/base_ibc.json ./chains/mytest1_ignored.json`
- Run: `local-ic start mytest1_ignored.json [flags]`
- Run in different directory: `ICTEST_HOME=/root/local-interchain local-ic start myother_ignored.json [flags]`

**NOTE** The ICTEST_HOME path must contain the directories `chains` to be valid.

*(Default: `make install` links to the cloned directory. `go install .` will use your home directory ~/local-interchain)*

*(Ending the config file with `_ignored.json` or `_ignore.json` will ignore it from git)*

### Optional Start Flags
    --api-address string             override the default API address (default "127.0.0.1")
    --api-port uint16                override the default API port (default 8080)
    --auth-key string                require an auth key to use the internal API
    --help
    --relayer-image string           override the docker relayer image (default "ghcr.io/cosmos/relayer")
    --relayer-startup-flags string   override the default relayer startup flags (default "--block-history=100")
    --relayer-uidgid string          override the default image UID:GID (default "100:1000")
    --relayer-version string         override the default relayer version (default "latest")

---

## REST API

A rest API can be found at `curl 127.0.0.1:8080/` by default. Other actions can take place here such as file uploads, actions, querying chain config information, and more!

Read more about the API [here](./docs/REST_API.md)

## Helpful Tips

- Auto complete: edit ~/.bashrc or ~/.zshrc and add `source <(local-ic completion bash)` or `source <(local-ic completion zsh)`.
    (fish & windows powershell is also supported)

- After starting the chain(s), you can read the `./configs/logs.json` file to get useful information. This includes the chain's id, name, RPC address, and more.

```json
// ./configs/logs.json
[
  {
    "chain_id": "exampleid-1",
    "chain_name": "example",
    "rpc_address": "http://127.0.0.1:38829",
    "grpc_address": "127.0.0.1:34917",
    "ibc_paths": []
  }
]
```

- 'ibc-path' should only be set if you are using 2+ chains. If you are using 1 chain, you can leave it blank.

- You can use `%DENOM%` anywhere in the chain's config to use the `denom` line. This is useful for gas prices, Genesis accounts, etc.

- Configuration's have set defaults. The minimum you need to run a single chain is the following

```json
{
    "name": "juno",
    "chain_id": "localjuno-2",
    "denom": "ujuno",
    "docker_image": {
        "version": "v14.1.0"
    },
    "gas_prices": "0%DENOM%",
    "gas_adjustment": 2.0
}
```

---

## Base Chain Template

Here is a base chain template with every feature the configuration accepts. Accounts have extra data to make it simpler for scripting and read from the file directly.

```json
{
    "name": "juno",
    "chain_id": "localjuno-1",
    "denom": "ujuno",
    "binary": "junod",
    "bech32_prefix": "juno",
    "docker_image": {
        "repository": "ghcr.io/cosmoscontracts/juno-e2e",
        "version": "v14.1.0",
        "uid_gid": "1000:1000"
    },
    "chain_type": "cosmos",
    "coin_type": 118,
    "trusting_period": "112h",
    "gas_prices": "0%DENOM%",
    "gas_adjustment": 2.0,
    "number_vals": 1,
    "number_node": 0,
    "ibc_paths": ["juno-ibc-1"],
    "debugging": true,
    "block_time": "500ms",
    "encoding-options": ["juno"],
    "host_port_override": {
        "26656": "26656",
        "26657": "26657",
        "1317": "1317",
        "9090": "9090"
    },
    "config_file_overrides": [
        {
            "file": "config/config.toml",
            "paths": {
                "rpc.cors_allowed_origins": ["*"],
                "rpc.mempool": ["nop"],
                "moniker": "myNodeMoniker",
            }
        }
    ],
    "genesis": {
        "modify": [
            {
                "key": "app_state.gov.voting_params.voting_period",
                "value": "15s"
            },
            {
                "key": "app_state.gov.deposit_params.max_deposit_period",
                "value": "15s"
            },
            {
                "key": "app_state.gov.deposit_params.min_deposit.0.denom",
                "value": "ujuno"
            }
        ],
        "accounts": [
            {
                "name": "acc0",
                "address": "juno1efd63aw40lxf3n4mhf7dzhjkr453axurv2zdzk",
                "amount": "10000000%DENOM%",
                "mnemonic": "decorate bright ozone fork gallery riot bus exhaust worth way bone indoor calm squirrel merry zero scheme cotton until shop any excess stage laundry"
            }
        ]
    }
},
```

**NOTE** The `host_port_override` section maps internal ports to the external host. If no ports are overridden, random host ports are assigned as usual.