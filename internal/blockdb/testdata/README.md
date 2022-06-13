# How to dump JSON from sqlite3.

For sample_txs.json

Produces an array of JSON objects:

```shell
sqlite3 ~/.ibctest/databases/block.db 'select tx_id, tx from v_tx_flattened where chain_kid = 1 order by tx_id asc' -json > /some/file/path.json
```