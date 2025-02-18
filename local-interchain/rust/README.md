# Rust Github CI

Leveraging Local-Interchain, rust contract developers can now test their contracts in a rust based e2e environment with a simple CI pipeline.

## Example

[Example Contract Repo](https://github.com/Reecepbcups/interchaintest-rust-example)

## Setup

**requirements** - setup a standard git repository (like the example) for a cosmwasm contract. A good base is [cw-template](https://github.com/CosmWasm/cw-template).

```bash
# create base directory
mkdir interchaintest
cd interchaintest

# generate the testing binary
cargo init --name e2e_testing

# add required dependencies to Cargo.toml
cargo add cosmwasm-std
cargo add localic-std --git https://github.com/strangelove-ventures/interchaintest

# create the 2 sub directories
mkdir chains configs

# add a customizable SDK v47 chain config to the chains directory
echo '{
    "chains": [
        {
            "name": "juno",
            "chain_id": "localjuno-1",
            "denom": "ujuno",
            "binary": "junod",
            "bech32_prefix": "juno",
            "docker_image": {
                "repository": "ghcr.io/cosmoscontracts/juno-e2e",
                "version": "v17.0.0"
            },
            "gas_prices": "0ujuno",
            "chain_type": "cosmos",
            "coin_type": 118,
            "trusting_period": "112h",
            "gas_adjustment": 2.0,
            "number_vals": 1,
            "number_node": 0,
            "debugging": true,
            "block_time": "500ms",
            "genesis": {
                "modify": [
                    {
                        "key": "app_state.gov.params.voting_period",
                        "value": "15s"
                    },
                    {
                        "key": "app_state.gov.params.max_deposit_period",
                        "value": "15s"
                    },
                    {
                        "key": "app_state.gov.params.min_deposit.0.denom",
                        "value": "ujuno"
                    },
                    {
                        "key": "app_state.gov.params.min_deposit.0.amount",
                        "value": "1"
                    }
                ],
                "accounts": [
                    {
                        "name": "acc0",
                        "address": "juno1hj5fveer5cjtn4wd6wstzugjfdxzl0xps73ftl",
                        "amount": "10000000000%DENOM%",
                        "mnemonic": "decorate bright ozone fork gallery riot bus exhaust worth way bone indoor calm squirrel merry zero scheme cotton until shop any excess stage laundry"
                    },
                    {
                        "name": "acc1",
                        "address": "juno1efd63aw40lxf3n4mhf7dzhjkr453axurv2zdzk",
                        "amount": "10000000000%DENOM%",
                        "mnemonic": "wealth flavor believe regret funny network recall kiss grape useless pepper cram hint member few certain unveil rather brick bargain curious require crowd raise"
                    }
                ],
                "startup_commands": [
                    "%BIN% keys add example-key-after --keyring-backend test --home %HOME%"
                ]
            }
        }
    ]
}' > chains/juno.json
```

## CI Configuration

Add the following configuration to your workflows directory.

```bash
mkdir .github/workflows
```

```yaml
# contract-e2e.yml
name: contract-e2e

on:
  # run on every PR and merge to main.
  pull_request:
  push:
    branches: [ master, main ]

# Ensures that only a single workflow per PR will run at a time.
# Cancels in-progress jobs if new commit is pushed.
concurrency:
    group: ${{ github.workflow }}-${{ github.event.pull_request.number || github.ref }}
    cancel-in-progress: true

env:
    GO_VERSION: 1.21

jobs:
  # Builds local-interchain binary
  build:
    runs-on: ubuntu-latest
    name: build
    steps:
      - name: Checkout interchaintest
        uses: actions/checkout@v4
        with:
            repository: strangelove-ventures/interchaintest
            path: interchaintest
            # ref: 'reece/rust' # branch, commit, tag

      - name: Setup go ${{ env.GO_VERSION }}
        uses: actions/setup-go@v5
        with:
            go-version: ${{ env.GO_VERSION }}

      - name: build local-interchain
        run: cd interchaintest/local-interchain && go mod tidy && make install

      - name: Upload localic artifact
        uses: actions/upload-artifact@v4
        with:
          name: local-ic
          path: ~/go/bin/local-ic

  contract-e2e:
    needs: build
    name: contract e2e
    runs-on: ubuntu-latest
    # defaults:
    #   run:
    #     working-directory: ./nested-path-here
    strategy:
      fail-fast: false

    steps:
      - name: checkout this repo (contracts)
        uses: actions/checkout@v3

      - name: Install latest toolchain
        uses: actions-rs/toolchain@v1
        with:
          profile: minimal
          toolchain: stable
          target: wasm32-unknown-unknown
          override: true

      - name: Download Tarball Artifact
        uses: actions/download-artifact@v3
        with:
          name: local-ic
          path: /tmp

      - name: Make local-ic executable
        run: chmod +x /tmp/local-ic

      # TODO: modify me to match your setup
      - name: Compile contract
        run: make compile

      # TODO: You can change `juno` here to any config in the chains/ directory
      # The `&` at the background allows it to run in the background of the CI pipeline.
      - name: Start background ibc local-interchain
        run: ICTEST_HOME=./interchaintest /tmp/local-ic start juno --api-port 8080 &

      # TODO: run the rust binary e2e test. (scripts, makefile, or just work here.)
      - name: Run Rust E2E Script
        run: make run-test

      - name: Cleanup
        run: killall local-ic && exit 0
```

## Makefile

The following is an example makefile for the above CI. You can also use bash scripts or just files in the root directory.

```makefile
#!/usr/bin/make -f
VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT := $(shell git log -1 --format='%H')

CURRENT_DIR := $(shell pwd)
BASE_DIR := $(shell basename $(CURRENT_DIR))

compile:
    @echo "Compiling Contract $(COMMIT)..."
    @docker run --rm -v "$(CURRENT_DIR)":/code \
    --mount type=volume,source="$(BASE_DIR)_cache",target=/code/target \
    --mount type=volume,source=registry_cache,target=/usr/local/cargo/registry \
    cosmwasm/rust-optimizer:0.12.13

run-test:
    cd interchaintest && cargo run --package e2e_testing --bin e2e_testing
```
