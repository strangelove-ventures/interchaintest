"""
Steps:
- Download contracts from the repo based off release version
- Ensure ictest is started for a chain
- Upload to JUNO (store) and init
- Connect together or what ever'
- Profit
"""

import os

from helpers import CosmWasm
from helpers.transactions import RequestBuilder
from util_base import API_URL

KEY_NAME = "acc0"
chain_id = "localjuno-1"


def main():
    rb = RequestBuilder(API_URL, chain_id)
    rb.binary("config keyring-backend test")

    absolute_path = os.path.abspath(__file__)
    parent_dir = os.path.dirname(os.path.dirname(absolute_path))
    contracts_dir = os.path.join(parent_dir, "contracts")

    CosmWasm.download_mainnet_daodao_contracts()

    # == Create contract object & upload ==
    dao_proposal_single = CosmWasm(API_URL, chain_id).store_contract(
        KEY_NAME, os.path.join(contracts_dir, "dao_proposal_single.wasm")
    )

    dao_voting_native_staked = CosmWasm(API_URL, chain_id).store_contract(
        KEY_NAME, os.path.join(contracts_dir, "dao_voting_native_staked.wasm")
    )

    dao_core = CosmWasm(API_URL, chain_id).store_contract(
        KEY_NAME,
        os.path.join(contracts_dir, "dao_core.wasm"),
    )

    # https://github.com/DA0-DA0/dao-contracts/blob/main/scripts/create-v2-dao-native-voting.sh
    module_msg = {
        "allow_revoting": False,
        "max_voting_period": {"time": 604800},
        "close_proposal_on_execution_failure": True,
        "pre_propose_info": {"anyone_may_propose": {}},
        "only_members_execute": True,
        "threshold": {
            "threshold_quorum": {
                "quorum": {"percent": "0.20"},
                "threshold": {"majority": {}},
            }
        },
    }
    encoded_prop_msg = CosmWasm.base64_encode_msg(module_msg)

    voting_msg = '{"owner":{"core_module":{}},"denom":"ujuno"}'
    encoded_voting_msg = CosmWasm.base64_encode_msg(voting_msg)

    cw_core_init_msg = CosmWasm.remove_msg_spaces(
        {
            "admin": "juno1efd63aw40lxf3n4mhf7dzhjkr453axurv2zdzk",
            "automatically_add_cw20s": True,
            "automatically_add_cw721s": True,
            "description": "V2_DAO",
            "name": "V2_DAO",
            "proposal_modules_instantiate_info": [
                {
                    "admin": {"core_module": {}},
                    "code_id": dao_proposal_single.code_id,
                    "label": "v2_dao",
                    "msg": f"{encoded_prop_msg}",
                }
            ],
            "voting_module_instantiate_info": {
                "admin": {"core_module": {}},
                "code_id": dao_voting_native_staked.code_id,
                "label": "test_v2_dao-cw-native-voting",
                "msg": f"{encoded_voting_msg}",
            },
        }
    ).decode("utf-8")

    dao_core.instantiate_contract(
        account_key=KEY_NAME,
        code_id=dao_core.code_id,
        msg=cw_core_init_msg,
        label="dao_core",
    )
    print(f"{dao_core.address=}")


if __name__ == "__main__":
    main()
