
MAKE_REQUEST() {
    curl http://127.0.0.1:8080/ --include --header "Content-Type: application/json" -X $1 --data "$2"
}

MAKE_GET() {
    curl http://127.0.0.1:8080/info --include --header "Content-Type: application/json" -X GET --data "$1"
}

# Example with Auth
# MAKE_REQUEST POST '{"chain_id":"localjuno-1","action":"q","cmd":"bank balances juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0","auth_key":"mySecretKeyExample"}'

MAKE_REQUEST POST '{"chain_id":"localjuno-1","action":"q","cmd":"bank total"}'

## ---- test interacting with multiple nodes (0 and 1) --
MAKE_REQUEST POST '{"chain_id":"localjuno-1","node_index":0, "action":"bin","cmd":"keys add testKey1 --keyring-backend=test"}'
MAKE_REQUEST POST '{"chain_id":"localjuno-1", "action":"bin","cmd":"keys list --keyring-backend=test"}' # default is 0

MAKE_REQUEST POST '{"chain_id":"localjuno-1","node_index":1, "action":"bin","cmd":"keys add testKey1 --keyring-backend=test"}'
MAKE_REQUEST POST '{"chain_id":"localjuno-1","node_index":1, "action":"bin","cmd":"keys list --keyring-backend=test"}'

MAKE_REQUEST POST '{"chain_id":"localjuno-1","node_index":999, "action":"bin","cmd":"keys list --keyring-backend=test"}' # fails
## ----

MAKE_REQUEST POST '{"chain_id":"localjuno-1","action":"get_channels"}'

MAKE_REQUEST POST '{"chain_id":"localjuno-1","action":"relayer-exec","cmd":"rly q channels localjuno-1"}'

MAKE_REQUEST POST '{"chain_id":"localjuno-1","action":"relayer-exec","cmd":"rly keys list localjuno-1"}'

MAKE_REQUEST POST '{"chain_id":"localjuno-1","action":"relayer-exec","cmd":"rly keys add localjuno-1 testkey"}'


MAKE_REQUEST POST '{"chain_id":"localjuno-1","action":"relayer-exec","cmd":"rly paths list"}'
MAKE_REQUEST POST '{"chain_id":"localjuno-1","action":"relayer-exec","cmd":"rly transact flush juno-ibc-1 channel-1"}'

# wasm contract relaying.
MAKE_REQUEST POST '{"chain_id":"localjuno-1","action":"relayer-exec","cmd":"rly transact channel juno-ibc-1 --src-port transfer --dst-port transfer --order unordered --version ics20-1"}'

MAKE_REQUEST POST '{"chain_id":"localjuno-1","action":"stop"}'
MAKE_REQUEST POST '{"chain_id":"localjuno-1","action":"start","cmd":"juno-ibc-1"}'

MAKE_REQUEST POST '{"action":"kill-all"}'


MAKE_REQUEST POST '{"chain_id":"localjuno-1","action":"q","cmd":"bank balances juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0"}'
MAKE_REQUEST POST '{"chain_id":"localjuno-1","action":"faucet","cmd":"amount=100;address=juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0"}'

# Get requests from info
curl -G -d "chain_id=localjuno-1" -d "request=peer" http://127.0.0.1:8080/info
curl -G -d "chain_id=localjuno-1" -d "request=height" http://127.0.0.1:8080/info
curl -G -d "chain_id=localjuno-1" -d "request=genesis_file_content" http://127.0.0.1:8080/info