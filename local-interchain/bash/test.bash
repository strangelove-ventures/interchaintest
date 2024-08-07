source ./source.bash
# source <(curl -s https://github.com/strangelove-ventures/interchaintest/tree/main/local-interchain/bash/source.bash)

QUERY http://127.0.0.1:8080/ "localjuno-1" "bank total"
# MAKE_REQUEST http://127.0.0.1:8080 "localjuno-1" "q" "bank total"


FAUCET_REQUEST "http://localhost:8080" "localjuno-1" "7" "juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0"
QUERY http://127.0.0.1:8080/ "localjuno-1" "bank balances juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0"