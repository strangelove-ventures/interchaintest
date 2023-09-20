use std::fmt;

use cosmwasm_std::{Coin, Uint128};
use reqwest::blocking::Client;
use serde_json::Value;

#[derive(Debug, PartialEq, Eq, Clone)]
pub enum RequestType {
    Bin,
    Query,
    Exec,
}
impl RequestType {
    pub fn as_str(&self) -> &'static str {
        match self {
            RequestType::Bin => "bin",
            RequestType::Query => "query",
            RequestType::Exec => "exec",
        }
    }
}

#[derive(Debug, Clone)]
pub struct RequestBase {
    pub client: Client,
    pub url: String,
    pub chain_id: String,
    pub request_type: RequestType,
}

impl RequestBase {
    pub fn new(
        client: Client,
        url: String,
        chain_id: String,
        request_type: RequestType,
    ) -> RequestBase {
        RequestBase {
            client,
            url,
            chain_id,
            request_type,
        }
    }
}

pub struct ActionHandler {
    pub chain_id: String,
    pub action: String,
    pub cmd: String,
}

impl ActionHandler {
    pub fn new(chain_id: String, action: String, cmd: String) -> ActionHandler {
        ActionHandler {
            chain_id,
            action,
            cmd,
        }
    }

    pub fn to_json_str(&self) -> String {
        let escaped_cmd = self.cmd.replace("\"", "\\\"");

        let json = format!(
            r#"{{"chain_id":"{}","action":"{}","cmd":"{}"}}"#,
            self.chain_id, self.action, escaped_cmd
        );
        json
    }

    pub fn to_json(&self) -> serde_json::Value {
        let json = self.to_json_str();
        let json: serde_json::Value = serde_json::from_str(&json).unwrap();
        json
    }
}

#[derive(Debug, PartialEq, Eq)]
pub struct TransactionResponse {
    pub tx_hash: Option<String>,
    pub rawlog: Option<String>,
}

impl fmt::Display for TransactionResponse {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "tx_hash: {:?}, rawlog: {:?}", self.tx_hash, self.rawlog)
    }
}

#[derive(Debug, PartialEq, Eq)]
pub struct Contract {
    pub address: String,
    pub tx_hash: String,
    pub admin: Option<String>,
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
