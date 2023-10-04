# Interacting with the chains

Since local-interchain exposes a REST API, you can interact with the chains using any language that supports HTTP requests. A default python client example can be found in the [scripts folder here](../scripts/).

# Contents

- [REST API](#rest-api)
  - [Defaults](#defaults)
  - [Environment Variables](#environment-variables)
    - [Actions](#actions)
    - [Node Actions](#node-actions)
        - [Chain Query](#chain-query)
        - [App Binary](#app-binary)
        - [Execute](#execute)
    - [Relaying Actions](#relaying-actions)
        - [Relayer Execution](#relayer-execution)
        - [Stop Relayer](#stop-relayer)
        - [Start Relayer](#start-relayer)
        - [Get Channels](#get-channels)
    - [Using Actions](#using-actions)
        - [Unix Curl Command](#unix-curl-command)
        - [Python](#python)

---

# REST API

## Defaults

By default, the API is served at <http://127.0.0.1:8080/>. You can modify this before starting the chain via [the configs/server.json configuration file](../configs/server.json).

## Environment Variables

`%RPC%`, `%HOME%`, and `%CHAIN_ID%` are supported in the configuration files and in this API anywhere. These are replaced with the chain's RPC address, the chain's home directory, and the chain's ID respectively. Useful for transactions and queries which may require such data.

---

## Actions

Local-Interchain supports the following list of actions on any node or relayer. These are all done through POST requests to the API, even if they are just fetching data.

**NOTE** Action values may change in the future. Refer to [the API handler](../interchain/handlers/actions.go) for the latest options.

## Node Actions

### Chain Query

- action values: "q", "query"
- Executes a query action on a specified chain.

### App Binary

- action values: "b", "bin", "binary"
- Description: Executes a binary action on the specified chain (ex: appd).

### Execute

- action values: "e", "exec", "execute"
- Description: Executes a general Linux action on the specified chain's docker instance (ex: ls -la).

## Relaying Actions

### Relayer Execution

- action values: "relayer", "relayer-exec", "relayer_exec", "relayerExec"
- Description: Executes a relayer-specific action on the specified chain.

### Stop Relayer

- action values: "stop-relayer", "stop_relayer", "stopRelayer"
- Description: Stops the relayer associated with the specified chain.

### Start Relayer

- action values: "start-relayer", "start_relayer", "startRelayer"
- Description: Starts the relayer associated with the specified chain.

### Get Channels

- action values: "get_channels", "get-channels", "getChannels"
- Description: Retrieves the channels for the specified chain using the relayer.

---

## Using Actions

The following examples use the [chains/base.json](../chains/base.json) chain example (`local-ic start base`)

### Unix Curl Command

```bash
# Get the total supply
curl -X POST -H "Content-Type: application/json" -d '{
  "chain_id": "localjuno-1",
  "action": "query",
  "cmd": "bank total"
}' http://127.0.0.1:8080/
# {"supply":[{"denom":"ujuno","amount":"110020857635458"}],"pagination":{"next_key":null,"total":"0"}}


# Set the config for a chain's transactions to always use keyring-backend test
# as the default. (Persist for the entire runtime of a session)
curl -X POST -H "Content-Type: application/json" -d '{
  "chain_id": "localjuno-1",
  "action": "binary",
  "cmd": "config keyring-backend test"
}' http://127.0.0.1:8080/


# Execute a Tx sending funds 
# The key here 'acc0' was set by name in the genesis section of the chain's config.
curl -X POST -H "Content-Type: application/json" -d '{
  "chain_id": "localjuno-1",
  "action": "binary",
  "cmd": "tx bank send acc0 juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0 500ujuno --fees 5000ujuno --node %RPC% --chain-id=%CHAIN_ID% --yes --output json"
}' http://127.0.0.1:8080/


# Querying said Tx hash returned from above.
# NOTE: 
# - the cmd does not require 'query' (or 'q') as it is added automatically.
# - This Tx hash will not be available on your machine.
curl -X POST -H "Content-Type: application/json" -d '{
  "chain_id": "localjuno-1",
  "action": "query",
  "cmd": "tx 7C68B0E0AFF733D93636BAD69D645B11C9C11C5C883394E99658BFCC05BF20DD"
}' http://127.0.0.1:8080/
```

### Python

A full Python client can be found in the [scripts folder](../scripts/). This is just a snippet of code for example purposes.

```python
# local-ic start juno_ibc

import httpx  # pip install httpx

api = "http://127.0.0.1:8080/"

# Pushes through any packets pending on channel-0 (both ways)
print(
    httpx.post(
        api,
        json={
            "chain_id": "localjuno-1",
            "action": "relayer-exec",
            "cmd": "rly transact flush juno-ibc-1 channel-0",
        },
    ).text
)

# Queries all channels on the localjuno-1 chain
# returning a JSON object of them
print(
    httpx.post(
        api,
        json={
            "chain_id": "localjuno-1",
            "action": "relayer-exec",
            "cmd": "rly q channels localjuno-1",
        },
    ).json()
)
# {'chain_id': 'localjuno-1', 'channel_id': 'channel-0', 'client_id': '07-tendermint-0', 'connection_hops': ['connection-0'], 'counterparty': {'chain_id': 'localjuno-2', 'channel_id': 'channel-0', 'client_id': '07-tendermint-0', 'connection_id': 'connection-0', 'port_id': 'transfer'}, 'ordering': 'ORDER_UNORDERED', 'port_id': 'transfer', 'state': 'STATE_OPEN', 'version': 'ics20-1'}

```
