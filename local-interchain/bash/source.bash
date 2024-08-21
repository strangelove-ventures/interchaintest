# IMPORT ME WITH: source <(curl -s https://raw.githubusercontent.com/strangelove-ventures/interchaintest/main/local-interchain/bash/source.bash)

# exitIfEmpty "$someKey" someKey
function ICT_exitIfEmpty() {
    if [ -z "$1" ]; then
        echo "Exiting because ${2} is empty"
        exit 1
    fi
}

# === BASE ===

# ICT_MAKE_REQUEST http://127.0.0.1:8080 localjuno-1 "q" "bank total"
ICT_MAKE_REQUEST() {
    local API=$1 CHAIN_ID=$2 ACTION=$3
    shift 3 # get the 4th argument and up as the command
    local COMMAND="$*"

    DATA=`printf '{"chain_id":"%s","action":"%s","cmd":"MYCOMMAND"}' $CHAIN_ID $ACTION`
    DATA=`echo $DATA | sed "s/MYCOMMAND/$COMMAND/g"`

    curl "$API" -ss --no-progress-meter --header "Content-Type: application/json" -X POST -d "$DATA"
}

# ICT_QUERY "http://localhost:8080" "localjuno-1" "bank balances juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0"
ICT_QUERY() {
    local API=$1 CHAIN_ID=$2 CMD=$3 # can be multiple words
    ICT_MAKE_REQUEST "$API" $CHAIN_ID "q" "$CMD"
}

# ICT_BIN "http://localhost:8080" "localjuno-1" "decode"
ICT_BIN() {
    local API=$1 CHAIN_ID=$2 CMD=$3 # can be multiple words
    ICT_MAKE_REQUEST "$API" $CHAIN_ID "bin" "$CMD"
}

# ICT_SH_EXEC "http://localhost:8080" "localjuno-1" "ls -l"
# NOTE: if using a /, make sure to escape it with \
ICT_SH_EXEC() {
    local API=$1 CHAIN_ID=$2 CMD=$3 # can be multiple words
    ICT_MAKE_REQUEST "$API" $CHAIN_ID "exec" "$CMD"
}

# === RELAYER ===

# ICT_RELAYER_STOP http://127.0.0.1 "localjuno-1"
ICT_RELAYER_STOP() {
    local API=$1 CHAIN_ID=$2

    # TODO: how does this function?
    ICT_MAKE_REQUEST $API $CHAIN_ID "stop-relayer" ""
}

# ICT_RELAYER_START http://127.0.0.1 "localjuno-1" "demo-path2 --max-tx-size 10"
ICT_RELAYER_START() {
    local API=$1 CHAIN_ID=$2 CMD=$3

    ICT_MAKE_REQUEST $API $CHAIN_ID "start-relayer" "$CMD"
}

# RELAYER_EXEC http://127.0.0.1:8080 "localjuno-1" "rly paths list"
ICT_RELAYER_EXEC() {
    local API=$1 CHAIN_ID=$2
    shift 2 # get the 3rd argument and up as the command
    local CMD="$*"

    ICT_MAKE_REQUEST $API $CHAIN_ID "relayer-exec" "$CMD"
}

# RELAYER_CHANNELS http://127.0.0.1:8080 "localjuno-1"
ICT_RELAYER_CHANNELS() {
    local API=$1 CHAIN_ID=$2

    ICT_MAKE_REQUEST $API $CHAIN_ID "get_channels" ""
}

# === COSMWASM ===

# ICT_WASM_DUMP_CONTRACT_STATE "http://localhost:8080" "localjuno-1" "cosmos1contractaddress" "100"
ICT_WASM_DUMP_CONTRACT_STATE() {
    local API=$1 CHAIN_ID=$2 CONTRACT=$3 HEIGHT=$4

    ICT_MAKE_REQUEST $API $CHAIN_ID "recover-key" "contract=$CONTRACT;height=$HEIGHT"
}

# ICT_WASM_STORE_FILE "http://localhost:8080" "localjuno-1" "/host/absolute/path.wasm" "keyName"
# returns the code_id of the uploaded contract
ICT_WASM_STORE_FILE() {
    local API=$1 CHAIN_ID=$2 FILE=$3 KEYNAME=$4

    DATA=`printf '{"chain_id":"%s","file_path":"%s","key_name":"%s"}' $CHAIN_ID $FILE $KEYNAME`
    curl "$API/upload" --header "Content-Type: application/json" --header "Upload-Type: cosmwasm" -X POST -d "$DATA"
}

# === OTHER ===

# ICT_POLL_FOR_START "http://localhost:8080" 50
ICT_POLL_FOR_START() {
    local API=$1 ATTEMPTS_MAX=$2

    curl --head -X GET --retry $ATTEMPTS_MAX --retry-connrefused --retry-delay 3 $API
}

# ICT_KILL_ALL "http://localhost:8080" "localjuno-1"
# (Kills all running, keeps local-ic process. `killall local-ic` to kill that as well)
ICT_KILL_ALL() {
    local API=$1 CHAIN_ID=$2
    ICT_MAKE_REQUEST $API $CHAIN_ID "kill-all" ""
}

# ICT_GET_PEER "http://localhost:8080" "localjuno-1"
ICT_GET_PEER() {
    local API=$1 CHAIN_ID=$2

    if [[ $API != */info ]]; then
        API="$API/info"
    fi

    curl -G -d "chain_id=$CHAIN_ID" -d "request=peer" $API
}

# ICT_FAUCET_REQUEST "http://localhost:8080" "localjuno-1" "1000000000ujuno" "juno1qk7zqy3k2v3jx2zq2z2zq2zq2zq2zq2zq2zq"
ICT_FAUCET_REQUEST() {
    local API=$1 CHAIN_ID=$2 AMOUNT=$3 ADDRESS=$4
    ICT_MAKE_REQUEST $API $CHAIN_ID "faucet" "amount=$AMOUNT;address=$ADDRESS"
}

# ICT_ADD_FULL_NODE http://127.0.0.1:8080 "localjuno-1" "1"
ICT_ADD_FULL_NODE() {
    local API=$1 CHAIN_ID=$2 AMOUNT=$3

    ICT_MAKE_REQUEST $API $CHAIN_ID "add-full-nodes" "amount=$AMOUNT"
}

# ICT_RECOVER_KEY "http://localhost:8080" "localjuno-1" "mykey" "my mnemonic string here"
ICT_RECOVER_KEY() {
    local API=$1 CHAIN_ID=$2 KEYNAME=$3
    shift 3 # get the 4th argument and up as the command
    local MNEMONIC="$*"

    ICT_MAKE_REQUEST $API $CHAIN_ID "recover-key" "keyname=$KEYNAME;mnemonic=$MNEMONIC"
}

# ICT_STORE_FILE "http://localhost:8080" "localjuno-1" "/host/absolute/path"
# Uploads any arbitrary host file to the chain node.
ICT_STORE_FILE() {
    local API=$1 CHAIN_ID=$2 FILE=$3

    DATA=`printf '{"chain_id":"%s","file_path":"%s"}' $CHAIN_ID $FILE`
    curl "$API/upload" --header "Content-Type: application/json" -X POST -d "$DATA"
}

