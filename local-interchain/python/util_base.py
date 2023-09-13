import argparse
import json
import os

current_dir = os.path.dirname(os.path.realpath(__file__))
parent_dir = os.path.dirname(current_dir)

contracts_path = os.path.join(parent_dir, "contracts")

contracts_json_path = os.path.join(parent_dir, "configs", "contracts.json")


# create contracts folder if not already
if not os.path.exists(contracts_path):
    os.mkdir(contracts_path)

server_config = {}
with open(os.path.join(parent_dir, "configs", "server.json")) as f:
    server_config = json.load(f)["server"]

HOST = server_config["host"]
PORT = server_config["port"]

# == Setup global parsers ==
parser = argparse.ArgumentParser(
    prog="api_test.py",
    description="Test the rest server to ensure it functions properly.",
)

parser.add_argument(
    "--api-address",
    type=str,
    default=HOST,
    help="The host/address to use for the rest server.",
)
parser.add_argument(
    "--api-port",
    type=int,
    default=PORT,
    help="The port to use for the rest server.",
)
args = parser.parse_args()

API_URL = f"http://{args.api_address}:{args.api_port}"
