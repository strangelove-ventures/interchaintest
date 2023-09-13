import json
import os

from helpers.file_cache import Cache

current_dir = os.path.dirname(os.path.realpath(__file__))
parent_dir = os.path.dirname(current_dir)

contracts_path = os.path.join(parent_dir, "contracts")

contracts_json_path = os.path.join(parent_dir, "configs", "contracts.json")


Cache.default_contracts_json()


# create contracts folder if not already
if not os.path.exists(contracts_path):
    os.mkdir(contracts_path)

server_config = {}
with open(os.path.join(parent_dir, "configs", "server.json")) as f:
    server_config = json.load(f)["server"]

PORT = server_config["port"]
HOST = server_config["host"]

API_URL = f"http://{HOST}:{PORT}"
