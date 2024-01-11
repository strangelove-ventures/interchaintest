# flake8: noqa
"""
This test the rest server to ensures it functions properly.

local-ic start base
"""

import time

from helpers.testing import poll_for_start
from helpers.transactions import RequestBuilder
from util_base import API_URL

chain_id = "localjuno-1"
sleep_seconds = 3


rb = RequestBuilder(API_URL, chain_id, sleep_seconds, log_output=True)


def main():
    poll_for_start(API_URL, waitSeconds=120)

    bin_test()
    tx_test()


#
#


# Test to ensure the base layer works and returns data properly
def bin_test():
    res = rb.binary("keys list --keyring-backend=test --output=json")
    assert len(res) > 0

    res = rb.binary(
        "tx decode ClMKUQobL2Nvc21vcy5nb3YudjFiZXRhMS5Nc2dWb3RlEjIIpwISK2p1bm8xZGM3a2MyZzVrZ2wycmdmZHllZGZ6MDl1YTlwZWo1eDNsODc3ZzcYARJmClAKRgofL2Nvc21vcy5jcnlwdG8uc2VjcDI1NmsxLlB1YktleRIjCiECxjGMmYp4MlxxfFWi9x4u+jOleJVde3Cru+HnxAVUJmgSBAoCCH8YNBISCgwKBXVqdW5vEgMyMDQQofwEGkDPE4dCQ4zUh6LIB9wqNXDBx+nMKtg0tEGiIYEH8xlw4H8dDQQStgAe6xFO7I/oYVSWwa2d9qUjs9qyB8r+V0Gy",
        ignore_pause=True,
    )
    print(res)
    assert res == {
        "body": {
            "messages": [
                {
                    "@type": "/cosmos.gov.v1beta1.MsgVote",
                    "proposal_id": "295",
                    "voter": "juno1dc7kc2g5kgl2rgfdyedfz09ua9pej5x3l877g7",
                    "option": "VOTE_OPTION_YES",
                }
            ],
            "memo": "",
            "timeout_height": "0",
            "extension_options": [],
            "non_critical_extension_options": [],
        },
        "auth_info": {
            "signer_infos": [
                {
                    "public_key": {
                        "@type": "/cosmos.crypto.secp256k1.PubKey",
                        "key": "AsYxjJmKeDJccXxVovceLvozpXiVXXtwq7vh58QFVCZo",
                    },
                    "mode_info": {"single": {"mode": "SIGN_MODE_LEGACY_AMINO_JSON"}},
                    "sequence": "52",
                }
            ],
            "fee": {
                "amount": [{"denom": "ujuno", "amount": "204"}],
                "gas_limit": "81441",
                "payer": "",
                "granter": "",
            },
            "tip": None,
        },
        "signatures": [
            "zxOHQkOM1IeiyAfcKjVwwcfpzCrYNLRBoiGBB/MZcOB/HQ0EErYAHusRTuyP6GFUlsGtnfalI7PasgfK/ldBsg=="
        ],
    }

    rb.binary("config keyring-backend test")
    assert rb.binary("config") == {
        "chain-id": "",
        "keyring-backend": "test",
        "output": "text",
        "node": "tcp://localhost:26657",
        "broadcast-mode": "sync",
        "gas": "",
        "gas-prices": "",
        "gas-adjustment": "",
        "fees": "",
        "fee-granter": "",
        "fee-payer": "",
        "note": "",
    }

    assert len(rb.binary("keys list --output=json")) > 0

    # this query returns a dict with supply as the key, validate it is there
    assert "supply" in rb.query("bank total")

    d = rb.query(
        "bank balances juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0 --output=json"
    )
    assert len(d["balances"]) == 0


# Test to ensure Transactions and getting that data returns properly
def tx_test():
    res = rb.binary(
        "tx bank send acc0 juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0 500ujuno --fees 5000ujuno --node %RPC% --chain-id=%CHAIN_ID% --yes --output json --keyring-backend=test"
    )
    assert res["code"] == 0

    tx_data = rb.query_tx(res)
    print(tx_data)

    bal = rb.query(
        "bank balances juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0 --output=json"
    )
    print(bal)
    assert len(bal["balances"]) > 0


if __name__ == "__main__":
    main()
