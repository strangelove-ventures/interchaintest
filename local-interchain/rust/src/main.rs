#![allow(dead_code)]

use cosmwasm_std::Coin;
use cosmwasm_std::Uint128;
use localic_std::cosmwasm::CosmWasm;
use reqwest::blocking::Client;

// TODO: Temp wildcards
use localic_std::balances::*;
use localic_std::bank::*;
use localic_std::files::*;
use localic_std::polling::*;
use localic_std::transactions::*;

pub mod base;
use base::{
    get_contract_cache_path, get_contract_path, get_current_dir, get_local_interchain_dir, API_URL,
};

fn main() {
    let client = Client::new();
    poll_for_start(client.clone(), &API_URL, 150);

    let rb: ChainRequestBuilder =
        ChainRequestBuilder::new(API_URL.to_string(), "localjuno-1".to_string(), true);

    test_paths(&rb);
    test_queries(&rb);
    test_binary(&rb);
    test_bank_send(&rb);
    test_cosmwasm(&rb);
}

// == test functions ==
fn test_paths(rb: &ChainRequestBuilder) {
    println!("current_dir: {:?}", get_current_dir());
    println!("local_interchain_dir: {:?}", get_local_interchain_dir());
    println!("contract_path: {:?}", get_contract_path());
    println!("contract_json_path: {:?}", get_contract_cache_path());

    // upload Makefile to the chain's home dir
    let arb_file = get_current_dir().join("Makefile");
    match rb.upload_file(arb_file, true) {
        Ok(req) => {
            let res = req.send().unwrap();
            let body = res.text().unwrap();
            println!("body: {}", body);
            assert_eq!(body, "{\"success\":\"file uploaded to localjuno-1\",\"location\":\"/var/cosmos-chain/localjuno-1/Makefile\"}");
        }
        Err(err) => {
            panic!("upload_file failed {:?}", err);
        }
    };

    let files = get_files(rb, "/var/cosmos-chain/localjuno-1");        
    assert!(files.contains(&"Makefile".to_string()));
    assert!(files.contains(&"config".to_string()));
    assert!(files.contains(&"data".to_string()));
    assert!(files.contains(&"keyring-test".to_string()));
    println!("files: {:?}", files);
}

fn test_cosmwasm(rb: &ChainRequestBuilder) {
    let cw = CosmWasm::new(&rb);

    let file_path = get_contract_path().join("cw_ibc_example.wasm");
    let code_id = cw.clone().store_contract("acc0", file_path);
    println!("code_id: {:?}", code_id);

    let code_id = code_id.unwrap_or_default();
    if code_id == 0 {
        panic!("code_id is 0");
    }

    let msg = r#"{}"#;
    let res = cw.instantiate_contract(
        "acc0",
        code_id,
        msg,
        "my-label",
        Some("juno1hj5fveer5cjtn4wd6wstzugjfdxzl0xps73ftl"),
        "",
    );
    println!("res: {:?}", res);

    let contract = match res {
        Ok(contract) => contract,
        Err(err) => {
            println!("err: {}", err);
            return;
        }
    };

    let data = cw.query_contract(&contract.address, "{\"get_count\":{\"channel\":\"0\"}}");
    println!("data: {}", data);
}

fn test_bank_send(rb: &ChainRequestBuilder) {
    let before_bal = get_balance(&rb, "juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0");

    let res = bank_send(
        &rb,
        "acc0",
        "juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0",
        vec![Coin {
            denom: "ujuno".to_string(),
            amount: Uint128::new(5),
        }],
        Coin {
            denom: "ujuno".to_string(),
            amount: Uint128::new(5000),
        },
    );
    match res {
        Ok(res) => {
            println!("res: {}", res);
        }
        Err(err) => {
            println!("err: {}", err);
        }
    }

    let after_amount = get_balance(&rb, "juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0");

    println!("before: {:?}", before_bal);
    println!("after: {:?}", after_amount);
}

fn test_queries(rb: &ChainRequestBuilder) {
    test_all_accounts(&rb);
    get_bank_total_supply(&rb);
}
fn test_binary(rb: &ChainRequestBuilder) {
    rb.binary("config", false);
    get_keyring_accounts(&rb);

    let decoded = rb.decode_transaction("ClMKUQobL2Nvc21vcy5nb3YudjFiZXRhMS5Nc2dWb3RlEjIIpwISK2p1bm8xZGM3a2MyZzVrZ2wycmdmZHllZGZ6MDl1YTlwZWo1eDNsODc3ZzcYARJmClAKRgofL2Nvc21vcy5jcnlwdG8uc2VjcDI1NmsxLlB1YktleRIjCiECxjGMmYp4MlxxfFWi9x4u+jOleJVde3Cru+HnxAVUJmgSBAoCCH8YNBISCgwKBXVqdW5vEgMyMDQQofwEGkDPE4dCQ4zUh6LIB9wqNXDBx+nMKtg0tEGiIYEH8xlw4H8dDQQStgAe6xFO7I/oYVSWwa2d9qUjs9qyB8r+V0Gy", false);
    println!("decoded: {}", decoded);
}

fn test_all_accounts(rb: &ChainRequestBuilder) {
    let res = rb.query("q auth accounts --output=json", false);
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

fn get_keyring_accounts(rb: &ChainRequestBuilder) {
    let accounts = rb.binary("keys list --keyring-backend=test", false);

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
