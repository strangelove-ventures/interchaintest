#!/bin/bash

# local-ic start base_ibc

source ./source.bash
# source <(curl -s https://github.com/strangelove-ventures/interchaintest/tree/main/local-interchain/bash/source.bash)


API_ADDR="http://localhost:8080"

# Set standard interaction defaults
MAKE_BIN "$API_ADDR" "localjuno-1" "config keyring-backend test"
MAKE_BIN "$API_ADDR" "localjuno-1" "config output json"

# exitIfEmpty "$someKey" someKey
function exitIfEmpty() {
    if [ -z "$1" ]; then
        echo "Exiting because ${2} is empty"
        exit 1
    fi
}

# Get total bank supply
BANK_TOTAL=`QUERY $API_ADDR "localjuno-1" "bank total"` && echo "BANK_TOTAL: $BANK_TOTAL"
exitIfEmpty "$BANK_TOTAL" "BANK_TOTAL"
echo $BANK_TOTAL | jq -r '.supply'

# Get total bank supply another way (directly)
BANK_TOTAL=`MAKE_REQUEST $API_ADDR "localjuno-1" "q" "bank total"` && echo "BANK_TOTAL: $BANK_TOTAL"
exitIfEmpty "$BANK_TOTAL" "BANK_TOTAL"
echo $BANK_TOTAL | jq -r '.supply'


# faucet to user
FAUCET_RES=`FAUCET_REQUEST "$API_ADDR" "localjuno-1" "7" "juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0"` && echo "FAUCET_RES: $FAUCET_RES"
FAUCET_CONFIRM=`QUERY $API_ADDR "localjuno-1" "bank balances juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0"` && echo "FAUCET_CONFIRM: $FAUCET_CONFIRM"
exitIfEmpty "$FAUCET_CONFIRM" "FAUCET_CONFIRM"

if [ $(echo $FAUCET_CONFIRM | jq -r '.balances[0].amount') -lt 7 ]; then
    echo "FAUCET_CONFIRM is less than 7"
    exit 1
fi


PEER=`GET_PEER $API_ADDR "localjuno-1"` && echo "PEER: $PEER"
exitIfEmpty "$PEER" "PEER"

#  RELAYER
CHANNELS=`RELAYER_CHANNELS $API_ADDR "localjuno-1"` && echo "CHANNELS: $CHANNELS"
exitIfEmpty "$CHANNELS" "CHANNELS"

RELAYER_EXEC $API_ADDR "localjuno-1" "rly paths list"
RELAYER_EXEC $API_ADDR "localjuno-1" "rly chains list"
RLY_BALANCE=`RELAYER_EXEC $API_ADDR "localjuno-1" "rly q balance localjuno-1 --output=json"` && echo "RLY_BALANCE: $RLY_BALANCE"
exitIfEmpty "$RLY_BALANCE" "RLY_BALANCE"
echo $RLY_BALANCE | jq -r '.balance'


# Recover a key and validate
COSMOS_KEY_STATUS=`RECOVER_KEY $API_ADDR "localjuno-1" "mynewkey" "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"` && echo "COSMOS_KEY_STATUS: $COSMOS_KEY_STATUS"

COSMOS_KEY_ADDRESS=`MAKE_BIN "$API_ADDR" "localjuno-1" "keys show mynewkey -a"` && echo "COSMOS_KEY_ADDRESS: $COSMOS_KEY_ADDRESS"
exitIfEmpty "$COSMOS_KEY_ADDRESS" "COSMOS_KEY_ADDRESS"

# Run a bash / shell command in the docekr instance
MISC_BASH_CMD=`MAKE_SH_EXEC "$API_ADDR" "localjuno-1" "ls -l"` && echo "MISC_BASH_CMD: $MISC_BASH_CMD"
exitIfEmpty "$MISC_BASH_CMD" "MISC_BASH_CMD"

FULL_NODE_ADDED=`ADD_FULL_NODE $API_ADDR "localjuno-1" "1"`
exitIfEmpty "$FULL_NODE_ADDED" "FULL_NODE_ADDED"

# Stop the relayer
RELAYER_STOP $API_ADDR "localjuno-1"

KILL_ALL $API_ADDR "localjuno-1"

killall local-ic
exit 0