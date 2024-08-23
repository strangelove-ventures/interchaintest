#!/bin/bash
# local-ic start juno_ibc
#
# bash local-interchain/bash/test.bash

# exits if any command is non 0 status
set -e

thisDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
# EXTERNAL: source <(curl -s https://raw.githubusercontent.com/strangelove-ventures/interchaintest/main/local-interchain/bash/source.bash)
source "$thisDir/source.bash"
API_ADDR="http://localhost:8080"

# === BEGIN TESTS ===

ICT_POLL_FOR_START $API_ADDR 50

# Set standard interaction defaults
ICT_BIN "$API_ADDR" "localjuno-1" "config keyring-backend test"
ICT_BIN "$API_ADDR" "localjuno-1" "config output json"

# Get total bank supply
BANK_TOTAL=`ICT_QUERY $API_ADDR "localjuno-1" "bank total"` && echo "BANK_TOTAL: $BANK_TOTAL"
ICT_exitIfEmpty "$BANK_TOTAL" "BANK_TOTAL"
echo $BANK_TOTAL | jq -r '.supply'

# Get total bank supply another way (directly)
BANK_TOTAL=`ICT_MAKE_REQUEST $API_ADDR "localjuno-1" "q" "bank total"` && echo "BANK_TOTAL: $BANK_TOTAL"
ICT_exitIfEmpty "$BANK_TOTAL" "BANK_TOTAL"
echo $BANK_TOTAL | jq -r '.supply'

# faucet to user
FAUCET_RES=`ICT_FAUCET_REQUEST "$API_ADDR" "localjuno-1" "7" "juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0"` && echo "FAUCET_RES: $FAUCET_RES"
FAUCET_CONFIRM=`ICT_QUERY $API_ADDR "localjuno-1" "bank balances juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0"` && echo "FAUCET_CONFIRM: $FAUCET_CONFIRM"
ICT_exitIfEmpty "$FAUCET_CONFIRM" "FAUCET_CONFIRM"

if [ $(echo $FAUCET_CONFIRM | jq -r '.balances[0].amount') -lt 7 ]; then
    echo "FAUCET_CONFIRM is less than 7"
    exit 1
fi

# CosmWasm - Upload source file to chain & store
parent_dir=$(dirname $thisDir) # local-interchain folder
contract_source="$parent_dir/contracts/cw_ibc_example.wasm"
CODE_ID_JSON=`ICT_WASM_STORE_FILE $API_ADDR "localjuno-1" "$contract_source" "acc0"` && echo "CODE_ID_JSON: $CODE_ID_JSON"
CODE_ID=`echo $CODE_ID_JSON | jq -r '.code_id'` && echo "CODE_ID: $CODE_ID"
ICT_exitIfEmpty "$CODE_ID" "CODE_ID"

# Upload random file
FILE_RESP=`ICT_STORE_FILE $API_ADDR "localjuno-1" "$thisDir/test.bash"` && echo "FILE_RESP: $FILE_RESP"
FILE_LOCATION=`echo $FILE_RESP | jq -r '.location'` && echo "FILE_LOCATION: $FILE_LOCATION"
ICT_exitIfEmpty "$FILE_LOCATION" "FILE_LOCATION"

# Verify file contents are there
FILE_LOCATION_ESC=$(echo $FILE_LOCATION | sed 's/\//\\\//g')
MISC_BASH_CMD=`ICT_SH_EXEC "$API_ADDR" "localjuno-1" "cat $FILE_LOCATION_ESC"` && echo "MISC_BASH_CMD: $MISC_BASH_CMD"
ICT_exitIfEmpty "$MISC_BASH_CMD" "MISC_BASH_CMD"

PEER=`ICT_GET_PEER $API_ADDR "localjuno-1"` && echo "PEER: $PEER"
ICT_exitIfEmpty "$PEER" "PEER"

#  RELAYER
CHANNELS=`ICT_RELAYER_CHANNELS $API_ADDR "localjuno-1"` && echo "CHANNELS: $CHANNELS"
ICT_exitIfEmpty "$CHANNELS" "CHANNELS"

ICT_RELAYER_EXEC $API_ADDR "localjuno-1" "rly paths list"
ICT_RELAYER_EXEC $API_ADDR "localjuno-1" "rly chains list"
RLY_BALANCE=`ICT_RELAYER_EXEC $API_ADDR "localjuno-1" "rly q balance localjuno-1 --output=json"` && echo "RLY_BALANCE: $RLY_BALANCE"
ICT_exitIfEmpty "$RLY_BALANCE" "RLY_BALANCE"
echo $RLY_BALANCE | jq -r '.balance'


# Recover a key and validate
COSMOS_KEY_STATUS=`ICT_RECOVER_KEY $API_ADDR "localjuno-1" "mynewkey" "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"` && echo "COSMOS_KEY_STATUS: $COSMOS_KEY_STATUS"

COSMOS_KEY_ADDRESS=`ICT_BIN "$API_ADDR" "localjuno-1" "keys show mynewkey -a"` && echo "COSMOS_KEY_ADDRESS: $COSMOS_KEY_ADDRESS"
ICT_exitIfEmpty "$COSMOS_KEY_ADDRESS" "COSMOS_KEY_ADDRESS"

FULL_NODE_ADDED=`ICT_ADD_FULL_NODE $API_ADDR "localjuno-1" "1"`
ICT_exitIfEmpty "$FULL_NODE_ADDED" "FULL_NODE_ADDED"

# Stop the relayer
ICT_RELAYER_STOP $API_ADDR "localjuno-1"

# Kills all containers, not the local-ic process. Use `killall local-ic` to kill that as well
ICT_KILL_ALL $API_ADDR "localjuno-1"

exit 0