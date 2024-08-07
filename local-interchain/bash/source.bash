# IMPORT ME WITH: source <(curl -s https://github.com/strangelove-ventures/interchaintest/tree/main/local-interchain/bash/source.bash)

# CONTENT_HEADERS="Content-Type: application/json"

# MAKE_GENERAL_REQUEST http://127.0.0.1:8080 '{"chain_id":"localjuno-1","action":"q","cmd":"bank total"}'
# Call this standalone, not from another function
# MAKE_GENERAL_REQUEST() {
#     API=$1
#     DATA=$2

#     curl "$API" --include --header "\"Content-Type: application/json\"" -X POST --data $DATA
# }

# MAKE_REQUEST http://127.0.0.1:8080 localjuno-1 q "bank total"
MAKE_REQUEST() {
    # echo $4
    API=$1

    CHAIN_ID=$2
    ACTION=$3
    shift 3
    # COMMAND=`echo $* | sed 's/"/\\"/g'` # allows multiple word commands
    COMMAND="$*"
    echo $COMMAND

    DATA=`printf '{"chain_id":"%s","action":"%s","cmd":"MYCOMMAND"}' $CHAIN_ID $ACTION`
    DATA=`echo $DATA | sed "s/MYCOMMAND/$COMMAND/g"`
    echo $DATA

    curl "$API" --include --header "Content-Type: application/json" -X POST -d "$DATA"
}


# FAUCET_REQUEST "http://localhost:8080" "localjuno-1" "1000000000ujuno" "juno1qk7zqy3k2v3jx2zq2z2zq2zq2zq2zq2zq2zq"
FAUCET_REQUEST() {
    API=$1
    CHAIN_ID=$2
    AMOUNT=$3
    ADDRESS=$4
    # MAKE_REQUEST $API '{"chain_id":"'$CHAIN_ID'","action":"faucet","cmd":"amount='$AMOUNT';address='$ADDRESS'"}'
    MAKE_REQUEST $API $CHAIN_ID "faucet" "amount=$AMOUNT;address=$ADDRESS"
}

KILL_ALL() {
    API=$1
    MAKE_REQUEST $API '{"action":"kill-all"}'
}

# QUERY "http://localhost:8080" "localjuno-1" "bank balances juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0"
QUERY() {
    API=$1
    CHAIN_ID=$2
    CMD=$3 # can be multiple words

    # MAKE_REQUEST "$API" \''{"chain_id":"'$CHAIN_ID'","action":"q","cmd":"'$CMD'"}'\'
    MAKE_REQUEST "$API" $CHAIN_ID "q" "$CMD"
}

# relayer

# todo: info request curl -G -d "chain_id=localjuno-1" -d "request=peer" http://127.0.0.1:8080/info

