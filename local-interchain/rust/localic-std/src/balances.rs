use cosmwasm_std::{Coin, Uint128};

use crate::transactions::ChainRequestBuilder;

pub fn get_balance(req_builder: &ChainRequestBuilder, address: &str) -> Vec<Coin> {
    let res = req_builder.query(&format!("q bank balances {}", address));
    let balances = match res["balances"].as_array() {
        Some(s) => s,
        None => return vec![],
    };

    let coins: Vec<Coin> = balances
        .iter()
        .map(|balance_coin| get_coin_from_json(balance_coin))
        .collect();
    coins
}

pub fn get_bank_total_supply(req_builder: &ChainRequestBuilder) -> Vec<Coin> {
    let res = req_builder.query("q bank total");
    let supplies = match res["supply"].as_array() {
        Some(s) => s,
        None => return vec![],
    };

    let coins: Vec<Coin> = supplies
        .iter()
        .map(|supply_coin| get_coin_from_json(supply_coin))
        .collect();
    coins
}

pub fn get_coin_from_json(value: &serde_json::Value) -> Coin {
    let amount = value["amount"].as_str().unwrap_or_default();
    let denom = value["denom"].as_str().unwrap_or_default();
    let amount = amount.parse::<Uint128>().unwrap_or_default();

    Coin {
        denom: denom.to_string(),
        amount,
    }
}
