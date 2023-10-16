#![allow(dead_code, unused_must_use)]

// Import base libraries
use cosmwasm_std::Coin;
use cosmwasm_std::Uint128;
use reqwest::blocking::Client;
use serde_json::json;

// Import Local-Interchain std library methods
use localic_std::filesystem::get_files;
use localic_std::node::Chain;
use localic_std::polling::poll_for_start;
use localic_std::relayer::Relayer;
use localic_std::transactions::ChainRequestBuilder;

// Import Local-Interchain SDK modules
use localic_std::modules::bank::{get_balance, get_total_supply, send};
use localic_std::modules::cosmwasm::CosmWasm;

// base helpers for this binary
pub mod base;
use base::{
    get_contract_cache_path, get_contract_path, get_current_dir, get_local_interchain_dir, API_URL,
};

// local-ic start juno_ibc
// cargo run --package localic-bin --bin localic-bin
fn main() {
    poll_for_start(&Client::new(), API_URL, 150);

    let rb: ChainRequestBuilder =
        match ChainRequestBuilder::new(API_URL.to_string(), "localjuno-1".to_string(), true) {
            Ok(rb) => rb,
            Err(err) => {
                panic!("ChainRequestBuilder failed: {err:?}");
            }
        };
    let node_a: Chain = Chain::new(&rb);

    let rb2: ChainRequestBuilder =
        match ChainRequestBuilder::new(API_URL.to_string(), "localjuno-2".to_string(), true) {
            Ok(rb) => rb,
            Err(err) => {
                panic!("ChainRequestBuilder failed: {err:?}");
            }
        };

    test_paths(&rb);
    test_queries(&rb);
    test_binary(&rb);
    test_bank_send(&rb);

    test_ibc_contract_relaying(&node_a, &rb, &rb2);
    test_node_information(&node_a);
    test_node_actions(&node_a);
}

fn test_ibc_contract_relaying(node: &Chain, rb1: &ChainRequestBuilder, rb2: &ChainRequestBuilder) {
    // local-ic start juno_ibc
    let file_path = get_contract_path().join("cw_ibc_example.wasm");
    let key1 = "acc0";
    let key2 = "second0";

    let relayer = Relayer::new(rb2);

    let mut contract_a = CosmWasm::new(rb1);
    let mut contract_b = CosmWasm::new(rb2);

    let c1_store = contract_a.store(key1, &file_path);
    let c2_store = contract_b.store(key2, &file_path);
    assert_eq!(
        c1_store.unwrap_or_default(),
        contract_a.code_id.unwrap_or_default()
    );
    assert_eq!(
        c2_store.unwrap_or_default(),
        contract_b.code_id.unwrap_or_default()
    );

    let ca = contract_a.instantiate(key1, "{}", "contractA", None, "");
    let cb = contract_b.instantiate(key2, "{}", "contractB", None, "");
    println!("contract_a: {ca:?}");
    println!("contract_b: {cb:?}");

    // example: manual relayer connection
    // let wc = relayer.create_channel(
    //     "juno-ibc-1",
    //     format!("wasm.{}", &contract_a.contract_addr.as_ref().unwrap()).as_str(),
    //     format!("wasm.{}", &contract_b.contract_addr.as_ref().unwrap()).as_str(),
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

    let channels = relayer.get_channels(rb1.chain_id.as_str());
    println!("channels: {channels:?}");

    let channel_id = "channel-1";

    // then execute on the contract
    let res = contract_b.execute(
        key2,
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

    let query_res = contract_a.query_value(&json!({"get_count":{"channel":channel_id}}));
    println!("\nquery_res: {query_res:?}");
    assert_eq!(query_res, serde_json::json!({"data":{"count":1}}));

    // dump the contracts state to JSON
    let height = node.get_height();
    let dump_res = node.dump_contract_state(&contract_a.contract_addr.as_ref().unwrap(), height);
    println!("dump_res: {dump_res:?}");
}

fn test_node_actions(node: &Chain) {
    let keyname = "abc";
    let words = "offer excite scare peanut rally speak suggest unit reflect whale cloth speak joy unusual wink session effort hidden angry envelope click race allow buffalo";
    let expected_addr = "juno1cp8wps50zemt3x5tn3sgqh3x93rlt8cw6tkgx4";

    let res = node.recover_key(keyname, words);
    println!("res: {res:?}");

    let acc = node.account_key_bech_32("abc");
    println!("acc: {acc:?}");
    assert_eq!(acc.unwrap_or_default(), expected_addr);

    let res = node.overwrite_genesis_file(r#"{"test":{}}"#);
    println!("res: {res:?}");
    node.get_genesis_file_content(); // verify this is updated

    // TODO: keep this disabled for now. The chain must already have a full node running to not err.
    // let res = node.add_full_node(1);
    // println!("res: {:?}", res);
}

fn test_node_information(node: &Chain) {
    let v = node.account_key_bech_32("acc0");
    assert_eq!(
        v.unwrap_or_default(),
        "juno1hj5fveer5cjtn4wd6wstzugjfdxzl0xps73ftl"
    );

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

    // TODO: test:
    // get_proposal(rb, "1");
}

fn test_paths(rb: &ChainRequestBuilder) {
    println!("current_dir: {:?}", get_current_dir());
    println!("local_interchain_dir: {:?}", get_local_interchain_dir());
    println!("contract_path: {:?}", get_contract_path());
    println!("contract_json_path: {:?}", get_contract_cache_path());

    // upload Makefile to the chain's home dir
    let arb_file = get_current_dir().join("Makefile");
    match rb.upload_file(&arb_file, true) {
        Ok(rb) => {
            let res = match rb.send() {
                Ok(r) => r,
                Err(err) => {
                    panic!("upload_file failed on request send {err:?}");
                }
            };
            let body = match res.text() {
                Ok(body) => body,
                Err(err) => {
                    panic!("upload_file failed on response body {err:?}");
                }
            };
            println!("body: {body:?}");
            assert_eq!(body, "{\"success\":\"file uploaded to localjuno-1\",\"location\":\"/var/cosmos-chain/localjuno-1/Makefile\"}");
        }
        Err(err) => {
            panic!("upload_file failed {err:?}");
        }
    };

    let files = match get_files(rb, "/var/cosmos-chain/localjuno-1") {
        Ok(files) => files,
        Err(err) => {
            panic!("get_files failed {err:?}");
        }
    };

    assert!(files.contains(&"Makefile".to_string()));
    assert!(files.contains(&"config".to_string()));
    assert!(files.contains(&"data".to_string()));
    assert!(files.contains(&"keyring-test".to_string()));
    println!("files: {files:?}");
}

fn test_bank_send(rb: &ChainRequestBuilder) {
    let before_bal = get_balance(rb, "juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0");

    let res = send(
        rb,
        "acc0",
        "juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0",
        &[Coin {
            denom: "ujuno".to_string(),
            amount: Uint128::new(5),
        }],
        &Coin {
            denom: "ujuno".to_string(),
            amount: Uint128::new(5000),
        },
    );
    match res {
        Ok(res) => {
            println!("res: {res}");
        }
        Err(err) => {
            println!("err: {err}");
        }
    }

    let after_amount = get_balance(rb, "juno10r39fueph9fq7a6lgswu4zdsg8t3gxlq670lt0");

    println!("before: {before_bal:?}");
    println!("after: {after_amount:?}");
}

fn test_queries(rb: &ChainRequestBuilder) {
    test_all_accounts(rb);
    let c = get_total_supply(rb);
    println!("total supply: {c:?}");
}
fn test_binary(rb: &ChainRequestBuilder) {
    rb.binary("config", false);
    get_keyring_accounts(rb);

    let decoded = rb.decode_transaction("ClMKUQobL2Nvc21vcy5nb3YudjFiZXRhMS5Nc2dWb3RlEjIIpwISK2p1bm8xZGM3a2MyZzVrZ2wycmdmZHllZGZ6MDl1YTlwZWo1eDNsODc3ZzcYARJmClAKRgofL2Nvc21vcy5jcnlwdG8uc2VjcDI1NmsxLlB1YktleRIjCiECxjGMmYp4MlxxfFWi9x4u+jOleJVde3Cru+HnxAVUJmgSBAoCCH8YNBISCgwKBXVqdW5vEgMyMDQQofwEGkDPE4dCQ4zUh6LIB9wqNXDBx+nMKtg0tEGiIYEH8xlw4H8dDQQStgAe6xFO7I/oYVSWwa2d9qUjs9qyB8r+V0Gy", false);
    println!("decoded: {decoded:?}");
}

fn test_all_accounts(rb: &ChainRequestBuilder) {
    let res = rb.query("q auth accounts", false);
    println!("res: {res}");

    let Some(accounts) = res["accounts"].as_array() else {
        println!("No accounts found.");
        return;
    };

    for account in accounts.iter() {
        let acc_type = account["@type"].as_str().unwrap_or_default();

        let addr: &str = match acc_type {
            // "/cosmos.auth.v1beta1.ModuleAccount" => account["base_account"]["address"]
            "/cosmos.auth.v1beta1.ModuleAccount" => account.get("base_account").unwrap()["address"]
                .as_str()
                .unwrap_or_default(),
            _ => account["address"].as_str().unwrap_or_default(),
        };

        println!("{acc_type}: {addr}");
    }
}

fn get_keyring_accounts(rb: &ChainRequestBuilder) {
    let accounts = rb.binary("keys list --keyring-backend=test", false);

    let addrs = accounts["addresses"].as_array();
    addrs.map_or_else(
        || {
            println!("No accounts found.");
        },
        |addrs| {
            for acc in addrs.iter() {
                let name = acc["name"].as_str().unwrap_or_default();
                let address = acc["address"].as_str().unwrap_or_default();
                println!("Key '{name}': {address}");
            }
        },
    );
}
