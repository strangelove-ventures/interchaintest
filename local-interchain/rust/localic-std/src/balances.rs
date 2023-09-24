use cosmwasm_std::Coin;

use crate::{transactions::ChainRequestBuilder, types::get_coin_from_json};

pub fn get_balance(req_builder: &ChainRequestBuilder, address: &str) -> Vec<Coin> {
    let res = req_builder.query(&format!("q bank balances {}", address), false);
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
    let res = req_builder.query("q bank total", false);
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
