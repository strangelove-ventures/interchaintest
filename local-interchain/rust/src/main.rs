#![allow(dead_code)]

use cosmwasm_std::Uint128;
use reqwest::blocking::Client;

pub mod base;
use base::API_URL;

// TODO: Temp wildcards
use localic_std::balances::*;
use localic_std::polling::*;
use localic_std::transactions::*;

fn main() {
    let client = Client::new();
    poll_for_start(client.clone(), &API_URL, 150);

    let req_builder =
        RequestBuilder::new(API_URL.to_string(), "localjuno-1".to_string(), true, false);

    test_queries(&req_builder);
    test_binary(&req_builder);

    test_bank_send(&req_builder);
}

// == test functions ==
fn test_bank_send(req_builder: &RequestBuilder) {
    // TODO: Have a transaction builder for common Txs (like CosmWasm type.)
    let before_bal =
        get_balance(&req_builder, "juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0")[0].amount;

    let cmd = "tx bank send acc0 juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0 5ujuno --fees 5000ujuno --node %RPC% --chain-id=%CHAIN_ID% --yes --output json --keyring-backend=test";
    let tx_data = req_builder.tx(&cmd, true);
    println!("tx_data: {}", tx_data.unwrap_or_default());

    let after_amount =
        get_balance(&req_builder, "juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0")[0].amount;
    assert_eq!(before_bal + Uint128::new(5), after_amount);
}

fn test_queries(req_builder: &RequestBuilder) {
    test_all_accounts(&req_builder);
    get_bank_total_supply(&req_builder);
}
fn test_binary(req_builder: &RequestBuilder) {
    req_builder.binary("config");
    get_keyring_accounts(&req_builder);

    let decoded = req_builder.decode_transaction("ClMKUQobL2Nvc21vcy5nb3YudjFiZXRhMS5Nc2dWb3RlEjIIpwISK2p1bm8xZGM3a2MyZzVrZ2wycmdmZHllZGZ6MDl1YTlwZWo1eDNsODc3ZzcYARJmClAKRgofL2Nvc21vcy5jcnlwdG8uc2VjcDI1NmsxLlB1YktleRIjCiECxjGMmYp4MlxxfFWi9x4u+jOleJVde3Cru+HnxAVUJmgSBAoCCH8YNBISCgwKBXVqdW5vEgMyMDQQofwEGkDPE4dCQ4zUh6LIB9wqNXDBx+nMKtg0tEGiIYEH8xlw4H8dDQQStgAe6xFO7I/oYVSWwa2d9qUjs9qyB8r+V0Gy", false);
    println!("decoded: {}", decoded);
}

fn test_all_accounts(req_builder: &RequestBuilder) {
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
