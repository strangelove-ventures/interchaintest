set -ex

for i in $(echo $CHAIN_IDS | tr ";" "\n")
do
  echo "chain id $i"
  # import orchestrator Cosmos key
  gorc --config=/root/gorc/$i/config.toml keys cosmos recover orch-key "$ORCH_MNEMONIC"

  # import orchestrator Ethereum key
  gorc --config=/root/gorc/$i/config.toml keys eth import orch-eth-key $ETH_PRIV_KEY

  # start gorc orchestrator
  gorc --config=/root/gorc/$i/config.toml orchestrator start --cosmos-key=orch-key --ethereum-key=orch-eth-key &

done

# Wait for the last run process to exit
wait $!

# Exit with status of the last process
exit $?