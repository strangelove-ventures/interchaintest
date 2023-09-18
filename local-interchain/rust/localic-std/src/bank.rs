use cosmwasm_std::Coin;
use serde_json::Value;

use crate::{transactions::ChainRequestBuilder, errors::LocalError};


pub fn bank_send(req_builder: &ChainRequestBuilder, from_key: &str, to_address: &str, tokens: Vec<Coin>, fee: Coin) -> Result<Value, LocalError>{    
    let str_coins = tokens.iter().map(|coin| format!("{}{}", coin.amount, coin.denom)).collect::<Vec<String>>().join(",");

    let cmd = format!("tx bank send {} {} {} --fees={} --node=%RPC% --chain-id=%CHAIN_ID% --yes --output=json --keyring-backend=test", from_key, to_address, str_coins, fee);
    let tx_data = req_builder.tx(&cmd, true);
    tx_data
}