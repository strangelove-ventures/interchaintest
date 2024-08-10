# IMPORT ME WITH: source <(curl -s https://github.com/strangelove-ventures/interchaintest/tree/main/local-interchain/bash/source.bash)

# TODOL: prefix with ICT_ for all functions

# ICT_MAKE_REQUEST http://127.0.0.1:8080 localjuno-1 "q" "bank total"
ICT_MAKE_REQUEST() {
    API=$1

    CHAIN_ID=$2
    ACTION=$3
    shift 3 # get the 4th argument and up as the command
    COMMAND="$*"

    DATA=`printf '{"chain_id":"%s","action":"%s","cmd":"MYCOMMAND"}' $CHAIN_ID $ACTION`
    DATA=`echo $DATA | sed "s/MYCOMMAND/$COMMAND/g"`

    curl "$API" -ss --no-progress-meter --header "Content-Type: application/json" -X POST -d "$DATA"
}


# "http://localhost:8080" "localjuno-1" "bank balances juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0"
ICT_QUERY() {
    API=$1
    CHAIN_ID=$2
    CMD=$3 # can be multiple words
    ICT_MAKE_REQUEST "$API" $CHAIN_ID "q" "$CMD"
}

# ICT_BIN "http://localhost:8080" "localjuno-1" "decode"
ICT_BIN() {
    API=$1
    CHAIN_ID=$2
    CMD=$3 # can be multiple words
    ICT_MAKE_REQUEST "$API" $CHAIN_ID "bin" "$CMD"
}

# ICT_SH_EXEC "http://localhost:8080" "localjuno-1" "ls -l"
ICT_SH_EXEC() {
    API=$1
    CHAIN_ID=$2
    CMD=$3 # can be multiple words
    ICT_MAKE_REQUEST "$API" $CHAIN_ID "exec" "$CMD"
}

# ICT_FAUCET_REQUEST "http://localhost:8080" "localjuno-1" "1000000000ujuno" "juno1qk7zqy3k2v3jx2zq2z2zq2zq2zq2zq2zq2zq"
ICT_FAUCET_REQUEST() {
    API=$1
    CHAIN_ID=$2
    AMOUNT=$3
    ADDRESS=$4
    ICT_MAKE_REQUEST $API $CHAIN_ID "faucet" "amount=$AMOUNT;address=$ADDRESS"
}

# === COSMWASM ===

# ICT_DUMP_CONTRACT_STATE "http://localhost:8080" "localjuno-1" "cosmos1contractaddress" "100"
ICT_DUMP_CONTRACT_STATE() {
    API=$1
    CHAIN_ID=$2
    CONTRACT=$3
    HEIGHT=$4

    ICT_MAKE_REQUEST $API $CHAIN_ID "recover-key" "contract=$CONTRACT;height=$HEIGHT"
}

# === OTHER ===

# ICT_KILL_ALL "http://localhost:8080" "localjuno-1"
# (Kills all running, keeps local-ic process. `killall local-ic` to kill that as well)
ICT_KILL_ALL() {
    API=$1
    CHAIN_ID=$2
    ICT_MAKE_REQUEST $API $CHAIN_ID "kill-all" ""
}

# ICT_GET_PEER "http://localhost:8080" "localjuno-1"
ICT_GET_PEER() {
    API=$1
    CHAIN_ID=$2

    if [[ $API != */info ]]; then
        API="$API/info"
    fi

    curl -G -d "chain_id=$CHAIN_ID" -d "request=peer" $API
}

# ICT_ADD_FULL_NODE http://127.0.0.1:8080 "localjuno-1" "1"
ICT_ADD_FULL_NODE() {
    API=$1
    CHAIN_ID=$2
    AMOUNT=$3

    ICT_MAKE_REQUEST $API $CHAIN_ID "add-full-nodes" "amount=$AMOUNT"
}

# ICT_RECOVER_KEY "http://localhost:8080" "localjuno-1" "mykey" "my mnemonic string here"
ICT_RECOVER_KEY() {
    API=$1
    CHAIN_ID=$2
    KEYNAME=$3
    shift 3 # get the 4th argument and up as the command
    MNEMONIC="$*"

    ICT_MAKE_REQUEST $API $CHAIN_ID "recover-key" "keyname=$KEYNAME;mnemonic=$MNEMONIC"
}

# === RELAYER ===

# ICT_RELAYER_STOP http://127.0.0.1 "localjuno-1"
ICT_RELAYER_STOP() {
    API=$1
    CHAIN_ID=$2

    # TODO: how does this function?
    ICT_MAKE_REQUEST $API $CHAIN_ID "stop-relayer" ""
}

# ICT_RELAYER_START http://127.0.0.1 "localjuno-1" "demo-path2 --max-tx-size 10"
ICT_RELAYER_START() {
    API=$1
    CHAIN_ID=$2
    CMD=$3

    ICT_MAKE_REQUEST $API $CHAIN_ID "start-relayer" "$CMD"
}

# RELAYER_EXEC http://127.0.0.1:8080 "localjuno-1" "rly paths list"
ICT_RELAYER_EXEC() {
    API=$1
    CHAIN_ID=$2
    shift 2 # get the 3rd argument and up as the command
    CMD="$*"

    ICT_MAKE_REQUEST $API $CHAIN_ID "relayer-exec" "$CMD"
}

# RELAYER_CHANNELS http://127.0.0.1:8080 "localjuno-1"
ICT_RELAYER_CHANNELS() {
    API=$1
    CHAIN_ID=$2

    ICT_MAKE_REQUEST $API $CHAIN_ID "get_channels" ""
}


