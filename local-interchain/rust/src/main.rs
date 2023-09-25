#![allow(dead_code, unused_must_use)]

use cosmwasm_std::Coin;
use cosmwasm_std::Uint128;
use localic_std::cosmwasm::CosmWasm;
use reqwest::blocking::Client;

// TODO: Temp wildcards
use localic_std::balances::*;
use localic_std::bank::*;
use localic_std::files::*;
use localic_std::node::*;
use localic_std::polling::*;
use localic_std::relayer::Relayer;
use localic_std::transactions::*;

pub mod base;
use base::{
    get_contract_cache_path, get_contract_path, get_current_dir, get_local_interchain_dir, API_URL,
};
use serde_json::json;


// cargo run --package localic-bin --bin localic-bin
fn main() {
    poll_for_start(Client::new(), &API_URL, 150);

    let rb: ChainRequestBuilder =
        ChainRequestBuilder::new(API_URL.to_string(), "localjuno-1".to_string(), true);

    let rb2: ChainRequestBuilder =
        ChainRequestBuilder::new(API_URL.to_string(), "localjuno-2".to_string(), true);

    test_paths(&rb);
    test_queries(&rb);
    test_binary(&rb);
    test_bank_send(&rb);
    test_cosmwasm(&rb);
    test_ibc_contract_relaying(&rb, &rb2);

    let node: ChainNode = ChainNode::new(&rb);
    test_node_information(&node);
    test_node_actions(&node);
}

fn test_ibc_contract_relaying(rb1: &ChainRequestBuilder, rb2: &ChainRequestBuilder) {
    // local-ic start juno_ibc
    let file_path = get_contract_path().join("cw_ibc_example.wasm");
    let key1 = "acc0";
    let key2 = "second0";

    let relayer = Relayer::new(&rb2);

    let mut contract_a = CosmWasm::new(&rb1);
    let mut contract_b = CosmWasm::new(&rb2);

    let c1_store = contract_a.store(key1, &file_path);
    let c2_store = contract_b.store(key2, &file_path);
    assert_eq!(c1_store.unwrap_or_default(), contract_a.code_id.unwrap());
    assert_eq!(c2_store.unwrap_or_default(), contract_b.code_id.unwrap());

    let ca = contract_a.instantiate(key1, "{}", "contractA", None, "");
    let cb = contract_b.instantiate(key2, "{}", "contractB", None, "");
    println!("contract_a: {:?}", ca);
    println!("contract_b: {:?}", cb);

    // example: manual relayer connection
    // let wc = relayer.create_wasm_connection(
    //     "juno-ibc-1",
    //     &contract_a.contract_addr.as_ref().unwrap(),
    //     &contract_b.contract_addr.as_ref().unwrap(),
    //     "unordered",
    //     "counter-1",
    // );

    contract_a.create_wasm_connection(
        &relayer,
        "juno-ibc-1",
        &contract_b,
        "unordered",
        "counter-1",
    );

    let channels = relayer.get_channels(rb1.chain_id.to_string());
    println!("channels: {:?}", channels);

    let channel_id = "channel-1";

    // then execute on the contract
    let res = contract_b.execute(
        &key2,
        json!({"increment":{"channel":channel_id}})
            .to_string()
            .as_str(),
        "--gas-adjustment=3.0",
    );
    println!("\ncw2.execute_contract: {res:?}");

    // flush packets
    println!(
        "relayer.flush: {:?}",
        relayer.flush("juno-ibc-1", channel_id)
    );

    // query contract
    let query_res = contract_a.query(
        json!({"get_count":{"channel":channel_id}})
            .to_string()
            .as_str(),
    );
    println!("\nquery_res: {}", query_res);
    assert_eq!(query_res, serde_json::json!({"data":{"count":1}}));
}

fn test_node_actions(node: &ChainNode) {
    let keyname = "abc";
    let words = "offer excite scare peanut rally speak suggest unit reflect whale cloth speak joy unusual wink session effort hidden angry envelope click race allow buffalo";
    let expected_addr = "juno1cp8wps50zemt3x5tn3sgqh3x93rlt8cw6tkgx4";

    let res = node.recover_key(keyname, words);
    println!("res: {:?}", res);

    let acc = node.account_key_bech_32("abc");
    println!("acc: {:?}", acc);
    assert_eq!(acc.unwrap(), expected_addr);

    let res = node.overwrite_genesis_file(r#"{"test":{}}"#);
    println!("res: {:?}", res);
    node.get_genesis_file_content(); // verify this is updated

    // TODO: keep this disabled for now. The chain must already have a full node running to not err.
    // let res = node.add_full_node(1);
    // println!("res: {:?}", res);
}

fn test_node_information(node: &ChainNode) {
    let v = node.account_key_bech_32("acc0");
    assert_eq!(v.unwrap(), "juno1hj5fveer5cjtn4wd6wstzugjfdxzl0xps73ftl");

    let v = node.account_key_bech_32("fake-key987");
    assert!(v.is_err());

    node.get_chain_config();

    assert!(node.get_name().starts_with("localjuno-1-val-0"));
    node.get_container_id();
    node.get_host_name();
    node.get_genesis_file_content();
    node.get_home_dir();
    node.get_height();
    node.read_file("./config/app.toml");
    node.is_above_sdk_v47();
    node.has_command("genesis"); // false with sdk 45
    node.has_command("tx"); // every bin has this
    let res = node.get_build_information(); // every bin has this
    println!(
        "res: {}",
        res["cosmos_sdk_version"].as_str().unwrap_or_default()
    );

    // TODO: test these:
    // node.query_proposal("1");
    // node.dump_contract_state("contract", 5);
}

fn test_paths(rb: &ChainRequestBuilder) {
    println!("current_dir: {:?}", get_current_dir());
    println!("local_interchain_dir: {:?}", get_local_interchain_dir());
    println!("contract_path: {:?}", get_contract_path());
    println!("contract_json_path: {:?}", get_contract_cache_path());

    // upload Makefile to the chain's home dir
    let arb_file = get_current_dir().join("Makefile");
    match rb.upload_file(&arb_file, true) {
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
    let code_id = cw.clone().store("acc0", &file_path);
    println!("code_id: {:?}", code_id);

    let code_id = code_id.unwrap_or_default();
    if code_id == 0 {
        panic!("code_id is 0");
    }

    let msg = r#"{}"#;
    let res = cw.contract_instantiate(
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
    // print res
    println!("res: {}", res);
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
