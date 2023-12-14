import argparse
import json
import os

current_dir = os.path.dirname(os.path.realpath(__file__))
parent_dir = os.path.dirname(current_dir)

contracts_path = os.path.join(parent_dir, "contracts")

contracts_json_path = os.path.join(parent_dir, "configs", "contracts.json")


# create contracts folder if not already
if not os.path.exists(contracts_path):
    os.makedirs(contracts_path, exist_ok=True)

HOST = "127.0.0.1"
PORT = 8080

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
