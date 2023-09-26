use cosmwasm_std::Coin;
use serde_json::Value;

use crate::{errors::LocalError, transactions::ChainRequestBuilder, types::get_coin_from_json};

/// # Errors
///
/// Returns `Err` if the transaction fails (ex: not enough balance, fees, or gas).
pub fn send(
    rb: &ChainRequestBuilder,
    from_key: &str,
    to_address: &str,
    tokens: &[Coin],
    fee: &Coin,
) -> Result<Value, LocalError> {
    let str_coins = tokens
        .iter()
        .map(|coin| format!("{}{}", coin.amount, coin.denom))
        .collect::<Vec<String>>()
        .join(",");

    let cmd = format!("tx bank send {from_key} {to_address} {str_coins} --fees={fee} --node=%RPC% --chain-id=%CHAIN_ID% --yes --output=json --keyring-backend=test");
    rb.tx(&cmd, true)
}

pub fn get_balance(req_builder: &ChainRequestBuilder, address: &str) -> Vec<Coin> {
    let res = req_builder.query(&format!("q bank balances {address}"), false);
    let Some(balances) = res["balances"].as_array() else { return vec![] };

    let coins: Vec<Coin> = balances.iter().map(get_coin_from_json).collect();
    coins
}

pub fn get_total_supply(req_builder: &ChainRequestBuilder) -> Vec<Coin> {
    let res = req_builder.query("q bank total", false);
    let Some(supplies) = res["supply"].as_array() else { return vec![] };

    let coins: Vec<Coin> = supplies.iter().map(get_coin_from_json).collect();
    coins
}
