import json
import os
from base64 import b64decode, b64encode

from helpers.file_cache import Cache
from helpers.transactions import RequestBuilder, get_transaction_response
from httpx import get, post

fp = os.path.realpath(__file__)
root_dir = os.path.dirname(os.path.dirname(os.path.dirname(fp)))
contracts_storage_dir = os.path.join(root_dir, "contracts")


def upload_file(rb: RequestBuilder, key_name: str, abs_path: str) -> dict:
    print(f"[upload_file] ({rb.chain_id}) {abs_path}")

    payload = {
        "chain_id": rb.chain_id,
        "key_name": key_name,
        "file_name": abs_path,
    }

    url = rb.api
    if url.endswith("/"):
        url += "upload"
    else:
        url += "/upload"

    res = post(
        url,
        json=payload,
        headers={"Content-Type": "application/json"},
        timeout=120,
    )

    if res.status_code != 200:
        return {"error": res.text}

    return json.loads(res.text.replace("\n", ""))


class CosmWasm:
    def __init__(self, api: str, chain_id: str, addr_override: str = ""):
        self.api = api  # http://127.0.0.1:8080
        self.chain_id = chain_id

        self.code_id: int = -1
        self.address = addr_override

        self.rb = RequestBuilder(self.api, self.chain_id)

        self.default_flag_set = "--home=%HOME% --node=%RPC% --chain-id=%CHAIN_ID% --yes --output=json --keyring-backend=test --gas=auto --gas-adjustment=2.0"

        # the last obtained Tx hash
        self.tx_hash = ""

    def get_latest_tx_hash(self) -> str:
        return self.tx_hash

    def store_contract(self, key_name: str, abs_path: str) -> "CosmWasm":
        ictest_chain_start = Cache.get_chain_start_time_from_logs()
        if ictest_chain_start == -1:
            return self

        Cache.default_contracts_json()

        contracts = Cache.get_cache_or_default({}, ictest_chain_start)

        sha1 = Cache.get_file_hash(abs_path, self.chain_id)
        if sha1 in contracts["file_cache"]:
            self.code_id = contracts["file_cache"][sha1]

            sub_file_path = abs_path.split("/")[-1]
            print(f"[Cache] CodeID={self.code_id} for {sub_file_path}")
            return self

        res = upload_file(self.rb, key_name, abs_path)
        if "error" in res:
            raise Exception(res["error"])

        self.code_id = Cache.update_cache(contracts, res["code_id"], sha1)
        return self

    def instantiate_contract(
        self,
        account_key: str,
        code_id: int | str,
        msg: str | dict,
        label: str,
        admin: str | None = None,
        flags: str = "",
    ) -> "CosmWasm":
        # not sure if we want this logic or not...
        if len(self.address) > 0:
            raise Exception("Contract address already set")

        if admin is None and "--no-admin" not in flags:
            flags += "--no-admin"

        if isinstance(msg, dict):
            msg = json.dumps(msg, separators=(",", ":"))

        cmd = f"""tx wasm instantiate {code_id} {msg} --label={label} --from={account_key} {self.default_flag_set} {flags}"""
        res = self.rb.binary(cmd)

        tx_res = get_transaction_response(res)

        # issue, such as signature verification or lack of fees etc
        if tx_res.RawLog and len(tx_res.RawLog) > 5:
            print(tx_res.RawLog)

        if len(tx_res.TxHash) == 0:
            print("Tx execute error", res)

        contract_addr = CosmWasm.get_contract_address(self.rb, tx_res.TxHash)
        print(f"[instantiate_contract] {label} {contract_addr}")

        self.tx_hash = tx_res.TxHash
        self.address = contract_addr
        return self

    def execute_contract(
        self,
        account_key: str,
        msg: str | dict,
        flags: str = "",
    ) -> "CosmWasm":
        if isinstance(msg, dict):
            msg = json.dumps(msg, separators=(",", ":"))

        # TODO: self.default_flag_set fails here for some reason...
        cmd = f"tx wasm execute {self.address} {msg} --from={account_key} --keyring-backend=test --home=%HOME% --node=%RPC% --chain-id=%CHAIN_ID% --yes --gas=auto --gas-adjustment=2.0"
        print("[execute_contract]", cmd)
        res = self.rb.binary(cmd)
        print(res)

        tx_res = get_transaction_response(res)

        if tx_res.RawLog and len(tx_res.RawLog) > 5:
            print(tx_res.RawLog)

        self.tx_hash = tx_res.TxHash

        return self

    def query_contract(self, msg: str | dict) -> dict:
        if isinstance(msg, dict):
            msg = json.dumps(msg, separators=(",", ":"))

        cmd = f"query wasm contract-state smart {self.address} {msg}"
        res = self.rb.query(cmd)
        return res

    @staticmethod
    def base64_encode_msg(msg: str | dict):
        if isinstance(msg, str):
            msg = dict(json.loads(msg))

        return b64encode(CosmWasm.remove_msg_spaces(msg)).decode("utf-8")

    @staticmethod
    def remove_msg_spaces(msg: dict):
        return json.dumps(msg, separators=(",", ":")).encode("utf-8")

    @staticmethod
    def get_contract_address(rb: RequestBuilder, tx_hash: str) -> str:
        res_json = rb.query(f"tx {tx_hash} --output=json")

        code = int(res_json["code"])
        if code != 0:
            raw = res_json["raw_log"]
            return raw

        contract_addr = ""
        for event in res_json["logs"][0]["events"]:
            for attr in event["attributes"]:
                if attr["key"] == "_contract_address":
                    contract_addr = attr["value"]
                    break

        return contract_addr

    @staticmethod
    def download_base_contracts():
        files = [
            "https://github.com/CosmWasm/cw-plus/releases/latest/download/cw20_base.wasm",
            "https://github.com/CosmWasm/cw-plus/releases/latest/download/cw4_group.wasm",
            "https://github.com/CosmWasm/cw-nfts/releases/latest/download/cw721_base.wasm",
        ]

        for url in files:
            name = url.split("/")[-1]
            file_path = os.path.join(contracts_storage_dir, name)

            if os.path.exists(file_path):
                continue

            print(f"Downloading {name} to {file_path}")
            res = get(url, follow_redirects=True)
            with open(file_path, "wb") as f:
                f.write(res.content)

    @staticmethod
    def download_mainnet_daodao_contracts():
        # From https://github.com/DA0-DA0/dao-contracts/releases
        # v2.1.0 # noqa
        files = """cw20_base.wasm 2443
        cw20_stake.wasm 2444
        cw20_stake_external_rewards.wasm 2445
        cw20_stake_reward_distributor.wasm 2446
        cw4_group.wasm 2447
        cw721_base.wasm 2448
        cw_admin_factory.wasm 2449
        cw_fund_distributor.wasm 2450
        cw_payroll_factory.wasm 2451
        cw_token_swap.wasm 2452
        cw_vesting.wasm 2453
        dao_core.wasm 2454
        dao_migrator.wasm 2455
        dao_pre_propose_approval_single.wasm 2456
        dao_pre_propose_approver.wasm 2457
        dao_pre_propose_multiple.wasm 2458
        dao_pre_propose_single.wasm 2459
        dao_proposal_condorcet.wasm 2460
        dao_proposal_multiple.wasm 2461
        dao_proposal_single.wasm 2462
        dao_voting_cw20_staked.wasm 2463
        dao_voting_cw4.wasm 2464
        dao_voting_cw721_staked.wasm 2465
        dao_voting_native_staked.wasm 2466"""

        for contract_file in files.split("\n"):
            name, code_id = contract_file.strip().split(" ")

            file_path = os.path.join(contracts_storage_dir, name)
            if os.path.exists(file_path):
                continue

            print(f"Downloading {name}")
            response = get(
                f"https://api.juno.strange.love/cosmwasm/wasm/v1/code/{code_id}",
                headers={
                    "accept": "application/json",
                },
                timeout=60,
            )
            resp = response.json()

            binary = b64decode(resp["data"])
            with open(file_path, "wb") as f:
                f.write(binary)


if __name__ == "__main__":
    CosmWasm.download_base_contracts()

    cw = CosmWasm(api="http://127.0.0.1:8080", chain_id="localjuno-1")

    cw.store_contract("acc0", os.path.join(contracts_storage_dir, "cw721_base.wasm"))

    cw.instantiate_contract(
        "acc0",
        cw.code_id,
        {
            "name": "name",
            "symbol": "NFT",
            # account in base.json genesis (acc0) # noqa
            "minter": "juno1hj5fveer5cjtn4wd6wstzugjfdxzl0xps73ftl",
        },
        label="contract",
        flags="",
    )
    print(cw.tx_hash)
    print(cw.address)

    cw.execute_contract(
        "acc0",
        {
            "mint": {
                "token_id": "1",
                "owner": "juno1hj5fveer5cjtn4wd6wstzugjfdxzl0xps73ftl",
                "token_uri": "https://reece.sh",
            }
        },
        flags="--output=json",
    )

    print(cw.query_contract({"contract_info": {}}))
    print(cw.query_contract({"all_nft_info": {"token_id": "1"}}))
