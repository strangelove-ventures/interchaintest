use cosmwasm_std::Coin;
use serde_json::Value;

use crate::{errors::LocalError, transactions::ChainRequestBuilder, types::get_coin_from_json};

pub fn bank_send(
    rb: &ChainRequestBuilder,
    from_key: &str,
    to_address: &str,
    tokens: Vec<Coin>,
    fee: Coin,
) -> Result<Value, LocalError> {
    let str_coins = tokens
        .iter()
        .map(|coin| format!("{}{}", coin.amount, coin.denom))
        .collect::<Vec<String>>()
        .join(",");

    let cmd = format!("tx bank send {} {} {} --fees={} --node=%RPC% --chain-id=%CHAIN_ID% --yes --output=json --keyring-backend=test", from_key, to_address, str_coins, fee);        
    let tx_data = rb.tx(&cmd, true);
    tx_data
}



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
