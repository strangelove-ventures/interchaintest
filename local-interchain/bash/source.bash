# IMPORT ME WITH: source <(curl -s https://github.com/strangelove-ventures/interchaintest/tree/main/local-interchain/bash/source.bash)

# TODOL: prefix with ICT_ for all functions

# MAKE_REQUEST http://127.0.0.1:8080 localjuno-1 "q" "bank total"
MAKE_REQUEST() {
    API=$1

    CHAIN_ID=$2
    ACTION=$3
    shift 3 # get the 4th argument and up as the command
    COMMAND="$*"

    DATA=`printf '{"chain_id":"%s","action":"%s","cmd":"MYCOMMAND"}' $CHAIN_ID $ACTION`
    DATA=`echo $DATA | sed "s/MYCOMMAND/$COMMAND/g"`

    curl "$API" -ss --no-progress-meter --header "Content-Type: application/json" -X POST -d "$DATA"
}


# QUERY "http://localhost:8080" "localjuno-1" "bank balances juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0"
QUERY() {
    API=$1
    CHAIN_ID=$2
    CMD=$3 # can be multiple words
    MAKE_REQUEST "$API" $CHAIN_ID "q" "$CMD"
}

# MAKE_BIN "http://localhost:8080" "localjuno-1" "decode"
MAKE_BIN() {
    API=$1
    CHAIN_ID=$2
    CMD=$3 # can be multiple words
    MAKE_REQUEST "$API" $CHAIN_ID "bin" "$CMD"
}

# MAKE_SH_EXEC "http://localhost:8080" "localjuno-1" "ls -l"
MAKE_SH_EXEC() {
    API=$1
    CHAIN_ID=$2
    CMD=$3 # can be multiple words
    MAKE_REQUEST "$API" $CHAIN_ID "exec" "$CMD"
}

# FAUCET_REQUEST "http://localhost:8080" "localjuno-1" "1000000000ujuno" "juno1qk7zqy3k2v3jx2zq2z2zq2zq2zq2zq2zq2zq"
FAUCET_REQUEST() {
    API=$1
    CHAIN_ID=$2
    AMOUNT=$3
    ADDRESS=$4
    MAKE_REQUEST $API $CHAIN_ID "faucet" "amount=$AMOUNT;address=$ADDRESS"
}



# === COSMWASM ===
DUMP_CONTRACT_STATE() {
    API=$1
    CHAIN_ID=$2
    CONTRACT=$3
    HEIGHT=$4

    MAKE_REQUEST $API $CHAIN_ID "recover-key" "contract=$CONTRACT;height=$HEIGHT"
}

# === OTHER ===
KILL_ALL() {
    API=$1
    CHAIN_ID=$2
    MAKE_REQUEST $API $CHAIN_ID "kill-all" ""
}

GET_PEER() {
    API=$1
    CHAIN_ID=$2

    if [[ $API != */info ]]; then
        API="$API/info"
    fi

    curl -G -d "chain_id=$CHAIN_ID" -d "request=peer" $API
}

# ADD_FULL_NODE http://127.0.0.1:8080 "localjuno-1" "1"
ADD_FULL_NODE() {
    API=$1
    CHAIN_ID=$2
    AMOUNT=$3

    MAKE_REQUEST $API $CHAIN_ID "add-full-nodes" "amount=$AMOUNT"
}

# RECOVER_KEY "http://localhost:8080" "localjuno-1" "mykey" "my mnemonic string here"
RECOVER_KEY() {
    API=$1
    CHAIN_ID=$2
    KEYNAME=$3
    shift 3 # get the 4th argument and up as the command
    MNEMONIC="$*"

    MAKE_REQUEST $API $CHAIN_ID "recover-key" "keyname=$KEYNAME;mnemonic=$MNEMONIC"
}

# === RELAYER ===
RELAYER_STOP() {
    API=$1
    CHAIN_ID=$2

    # TODO: how does this function?
    MAKE_REQUEST $API $CHAIN_ID "stop-relayer" ""
}

RELAYER_START() {
    API=$1
    CHAIN_ID=$2
    CMD=$3

    MAKE_REQUEST $API $CHAIN_ID "start-relayer" "$CMD"
}

# RELAYER_EXEC http://127.0.0.1:8080 "localjuno-1" "rly paths list"
RELAYER_EXEC() {
    API=$1
    CHAIN_ID=$2
    shift 2 # get the 3rd argument and up as the command
    CMD="$*"

    MAKE_REQUEST $API $CHAIN_ID "relayer-exec" "$CMD"
}

# RELAYER_CHANNELS http://127.0.0.1:8080 "localjuno-1"
RELAYER_CHANNELS() {
    API=$1
    CHAIN_ID=$2

    MAKE_REQUEST $API $CHAIN_ID "get_channels" ""
}


