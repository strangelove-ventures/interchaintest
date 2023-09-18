use reqwest::blocking::Client;

pub mod polling;
use polling::poll_for_start;

pub mod base;
use base::API_URL;

pub mod transactions;
use transactions::RequestBuilder;

use cosmwasm_std::{Coin, Uint128};

fn main() {
    let client = Client::new();
    poll_for_start(client.clone(), &API_URL, 150);

    let req_builder = RequestBuilder::new(API_URL.to_string(), "localjuno-1".to_string(), true);

    // == queries ==
    get_all_accounts(&req_builder);
    get_bank_total_supply(&req_builder);

    // == binary ==
    req_builder.binary("config");
    get_keyring_accounts(&req_builder);
    transaction_decode(&req_builder);

    // transactions
    let cmd = "tx bank send acc0 juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0 5ujuno --fees 5000ujuno --node %RPC% --chain-id=%CHAIN_ID% --yes --output json --keyring-backend=test";
    let tx_data = req_builder.transaction(&cmd);
    println!("tx_data: {}", tx_data);

    let bal = get_balance(&req_builder, "juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0");
    println!("bal: {}", bal[0]);
}

// TODO: put these into its own module helpers. (ensure to return the proper data.)
fn get_all_accounts(req_builder: &RequestBuilder) {
    let res = req_builder.query("q auth accounts --output=json");
    let accounts = res["accounts"].as_array().unwrap();

    accounts.iter().for_each(|account| {
        let acc_type = account["@type"].as_str().unwrap_or_default();

        let addr: &str = match acc_type {
            "/cosmos.auth.v1beta1.ModuleAccount" => account["base_account"]["address"]
                .as_str()
                .unwrap_or_default(),
            _ => account["address"].as_str().unwrap_or_default(),
        };

        println!("{}: {}", acc_type, addr);
    });
}

fn get_bank_total_supply(req_builder: &RequestBuilder) {
    // Total supply: {"pagination":{"next_key":null,"total":"0"},"supply":[{"amount":"110048643629768","denom":"ujuno"}]}
    let res = req_builder.query("q bank total");

    let supplies = res["supply"].as_array().unwrap();
    supplies.iter().for_each(|supply| {
        let amount = supply["amount"].as_str().unwrap_or_default();
        let denom = supply["denom"].as_str().unwrap_or_default();
        let amount = amount.parse::<u128>().unwrap_or_default();

        let human_denom = denom[1..].to_string().to_uppercase();
        let human_amount = amount / 1000000;
        println!(
            "{}: {} = ({}: {})",
            denom, amount, human_denom, human_amount
        );
    });
}

fn get_balance(req_builder: &RequestBuilder, address: &str) -> Vec<Coin> {
    let res = req_builder.query(&format!("q bank balances {}", address));
    let balances = res["balances"].as_array().unwrap();

    let mut coins: Vec<Coin> = Vec::new();
    balances.iter().for_each(|balance| {
        let amount = balance["amount"].as_str().unwrap_or_default();
        let denom = balance["denom"].as_str().unwrap_or_default();
        let amount = amount.parse::<Uint128>().unwrap_or_default();

        let coin = Coin {
            denom: denom.to_string(),
            amount: amount,
        };
        coins.push(coin);
    });

    coins
}

fn get_keyring_accounts(req_builder: &RequestBuilder) {
    let accounts = req_builder.binary("keys list --keyring-backend=test");

    let addrs = accounts["addresses"].as_array();
    match addrs {
        Some(addrs) => {
            addrs.iter().for_each(|acc| {
                let name = acc["name"].as_str().unwrap_or_default();
                let address = acc["address"].as_str().unwrap_or_default();
                println!("Key '{}': {}", name, address);
            });
        }
        None => {
            println!("No accounts found.");
        }
    }
}

fn transaction_decode(req_builder: &RequestBuilder) {
    let cmd = "tx decode ClMKUQobL2Nvc21vcy5nb3YudjFiZXRhMS5Nc2dWb3RlEjIIpwISK2p1bm8xZGM3a2MyZzVrZ2wycmdmZHllZGZ6MDl1YTlwZWo1eDNsODc3ZzcYARJmClAKRgofL2Nvc21vcy5jcnlwdG8uc2VjcDI1NmsxLlB1YktleRIjCiECxjGMmYp4MlxxfFWi9x4u+jOleJVde3Cru+HnxAVUJmgSBAoCCH8YNBISCgwKBXVqdW5vEgMyMDQQofwEGkDPE4dCQ4zUh6LIB9wqNXDBx+nMKtg0tEGiIYEH8xlw4H8dDQQStgAe6xFO7I/oYVSWwa2d9qUjs9qyB8r+V0Gy";
    let res = req_builder.binary(cmd);
    println!("{}", res);
}
