
MAKE_REQUEST() {
    curl http://localhost:8080/ --include --header "Content-Type: application/json" -X $1 --data "$2"
}
MAKE_REQUEST POST '{"chain_id":"localjuno-1","action":"q","cmd":"bank total"}'

RELAYER_REQUEST() {
    curl http://localhost:8080/ --include --header "Content-Type: application/json" -X $1 --data "$2"
}

# MAKE_REQUEST POST '{"chain_id":"localjuno-1","action":"bin","cmd":"keys list --keyring-backend=test"}'

RELAYER_REQUEST POST '{"chain_id":"localjuno-1","action":"get_channels"}'

RELAYER_REQUEST POST '{"chain_id":"localjuno-1","action":"relayer-exec","cmd":"rly q channels localjuno-1"}'

RELAYER_REQUEST POST '{"chain_id":"localjuno-1","action":"relayer-exec","cmd":"rly keys list localjuno-1"}'

RELAYER_REQUEST POST '{"chain_id":"localjuno-1","action":"relayer-exec","cmd":"rly keys add localjuno-1 testkey"}'


RELAYER_REQUEST POST '{"chain_id":"localjuno-1","action":"relayer-exec","cmd":"rly paths list"}'
RELAYER_REQUEST POST '{"chain_id":"localjuno-1","action":"relayer-exec","cmd":"rly transact flush juno-ibc-1 channel-1"}'

# wasm contract relaying.
RELAYER_REQUEST POST '{"chain_id":"localjuno-1","action":"relayer-exec","cmd":"rly transact channel juno-ibc-1 --src-port transfer --dst-port transfer --order unordered --version ics20-1"}'

RELAYER_REQUEST POST '{"chain_id":"localjuno-1","action":"stop"}'
RELAYER_REQUEST POST '{"chain_id":"localjuno-1","action":"start","cmd":"juno-ibc-1"}'
