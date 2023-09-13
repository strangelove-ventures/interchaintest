import json
from dataclasses import dataclass
from enum import Enum

from httpx import post


def get_tx_hash(res: str | dict) -> str:
    # String is a JSON blob, but was input as a string
    if isinstance(res, str):
        try:
            res = json.loads(res)
        except Exception:
            pass

    tx_hash = ""
    if isinstance(res, dict):
        tx_hash = res["txhash"]
    else:
        tx_hash = res

    return tx_hash


class RequestType(Enum):
    BIN = "bin"
    QUERY = "query"
    EXEC = "exec"


@dataclass(frozen=True)
class RequestBase:
    URL: str
    chain_id: str
    request_type: RequestType


# TODO: type handler this better with a dataclass
@dataclass
class TransactionResponse:
    TxHash: str = ""
    RawLog: str | None = ""


@dataclass
class ActionHandler:
    chain_id: str = ""
    action: str = ""
    cmd: str = ""

    def __init__(self, chain_id: str, action: str, cmd: str):
        self.chain_id = chain_id
        self.action = action
        self.cmd = cmd

    def to_json(self) -> dict:
        return {
            "chain_id": self.chain_id,
            "action": self.action,
            "cmd": self.cmd,
        }


class RequestBuilder:
    def __init__(self, api: str, chain_id: str, log_output: bool = False):
        self.api = api
        self.chain_id = chain_id
        self.log = log_output

        if self.api == "":
            raise Exception("RequestBuilder api is empty")

        if self.chain_id == "":
            raise Exception("RequestBuilder chain_id is empty")

    def binary(self, cmd: str, log_output: bool = False) -> dict:
        rb = RequestBase(self.api, self.chain_id, RequestType.BIN)
        return send_request(
            rb, cmd, log_output=(log_output if log_output else self.log)
        )

    def query(self, cmd: str, log_output: bool = False) -> dict:
        if cmd.lower().startswith("query "):
            cmd = cmd[6:]
        elif cmd.lower().startswith("q "):
            cmd = cmd[2:]

        rb = RequestBase(self.api, self.chain_id, RequestType.QUERY)
        return send_request(
            rb, cmd, log_output=(log_output if log_output else self.log)
        )

    # What / when is response?
    def query_tx(self, response: str | dict, log_output: bool = False) -> dict:
        tx_hash = get_tx_hash(response)
        if len(tx_hash) == 0:
            return {"error": "tx_hash is empty"}

        res = self.query(
            f"tx {tx_hash} --output json",
            log_output=(log_output if log_output else self.log),
        )
        return {"tx": res}


def send_request(
    base: RequestBase,
    cmd: str = "",
    return_text: bool = False,
    log_output: bool = False,
) -> dict:
    if base.request_type == RequestType.QUERY:
        if cmd.lower().startswith("query "):
            cmd = cmd[6:]
        elif cmd.lower().startswith("q "):
            cmd = cmd[2:]

    payload = ActionHandler(base.chain_id, base.request_type.value, cmd).to_json()

    if log_output:
        print("[send_request data]", payload)

    res = post(
        base.URL,
        json=payload,
        headers={"Content-Type": "application/json"},
    )

    if log_output:
        if res.text != "{}":
            print("[send_request resp]", res.text)

    # This is messy, clean up
    if return_text:
        return {"text": res.text}

    try:
        # Is there ever a case this does not work?
        return json.loads(res.text)
    except Exception:
        return {"parse_error": res.text}


def get_transaction_response(send_req_res: str | dict) -> TransactionResponse:
    txr = TransactionResponse()

    if isinstance(send_req_res, str):
        try:
            json.loads(send_req_res)
        except Exception:
            txr.RawLog = send_req_res
            return txr

        txr.TxHash = json.loads(send_req_res)["txhash"]

    if isinstance(send_req_res, dict):
        thash = send_req_res.get("txhash")
        txr.TxHash = thash if thash is not None else ""
        txr.RawLog = send_req_res.get("raw_log")

    if txr.TxHash is None:
        raise Exception("No txHash found", send_req_res)

    return txr
