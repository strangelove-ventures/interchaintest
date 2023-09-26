use std::fmt;

use cosmwasm_std::{Coin, Uint128};
use reqwest::blocking::Client;

#[derive(Debug, PartialEq, Eq, Clone)]
pub enum RequestType {
    Bin,
    Query,
    Exec,
    // custom
    RecoverKey,
    OverwriteGenesisFile,
    SetNewPeers,
    AddFullNodes,
}
impl RequestType {
    #[must_use]
    pub fn as_str(&self) -> &'static str {
        // match handlers/actions.go
        match self {
            RequestType::Bin => "bin",
            RequestType::Query => "query",
            RequestType::Exec => "exec",
            RequestType::RecoverKey => "recover-key",
            RequestType::OverwriteGenesisFile => "overwrite-genesis-file",
            RequestType::SetNewPeers => "set-peers",
            RequestType::AddFullNodes => "add-full-nodes",
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
    #[must_use]
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
    pub action: RequestType,
    pub cmd: String,
}

impl ActionHandler {
    #[must_use]
    pub fn new(chain_id: String, action: RequestType, cmd: String) -> ActionHandler {
        ActionHandler {
            chain_id,
            action,
            cmd,
        }
    }

    #[must_use]
    pub fn to_json_str(&self) -> String {
        let escaped_cmd = self.cmd.replace('\"', "\\\"");

        let json = format!(
            r#"{{"chain_id":"{}","action":"{}","cmd":"{}"}}"#,
            self.chain_id,
            self.action.as_str(),
            escaped_cmd
        );
        json
    }

    #[must_use]
    pub fn to_json(&self) -> serde_json::Value {
        let json = self.to_json_str();
        let json: serde_json::Value = serde_json::from_str(&json).unwrap_or_default();
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

#[must_use]
pub fn get_coin_from_json(value: &serde_json::Value) -> Coin {
    let amount = value["amount"].as_str().unwrap_or_default();
    let denom = value["denom"].as_str().unwrap_or_default();
    let amount = amount.parse::<Uint128>().unwrap_or_default();

    Coin {
        denom: denom.to_string(),
        amount,
    }
}
