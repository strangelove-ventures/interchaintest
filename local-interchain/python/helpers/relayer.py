import json

import httpx


class Relayer:
    def __init__(self, api: str, chain_id: str, log_output: bool = False):
        self.api = api
        self.chain_id = chain_id
        self.log_output = log_output

    def execute(self, cmd: str, return_text: bool = False) -> dict:
        if self.api == "":
            raise Exception("send_request URL is empty")

        payload = {
            "chain_id": self.chain_id,
            "action": "relayer-exec",
            "cmd": cmd,
        }

        if self.log_output:
            print("[relayer]", payload["cmd"])

        res = httpx.post(
            self.api,
            json=payload,
            headers={"Content-Type": "application/json"},
            timeout=120,
        )

        if return_text:
            return {"text": res.text}

        try:
            # Is there ever a case this does not work?
            return json.loads(res.text)
        except Exception:
            return {"parse_error": res.text}

    def create_wasm_connection(
        self, path: str, src: str, dst: str, order: str, version: str
    ):
        if not src.startswith("wasm."):
            src = f"wasm.{src}"

        if not dst.startswith("wasm."):
            dst = f"wasm.{dst}"

        self.execute(
            f"rly tx channel {path} --src-port {src} --dst-port {dst} --order {order} --version {version}"
        )

    def flush(self, path: str, channel: str, log_output: bool = False) -> dict:
        res = self.execute(
            f"rly transact flush {path} {channel}",
        )
        if log_output:
            print(res)

        return res

    def get_channels(self) -> dict:
        payload = {
            "chain_id": self.chain_id,
            "action": "get_channels",
        }

        res = httpx.post(
            self.api,
            json=payload,
            headers={"Content-Type": "application/json"},
        )
        return json.loads(res.text)
