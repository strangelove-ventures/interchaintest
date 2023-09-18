use reqwest::blocking::Client;

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
        let json = format!(
            r#"{{"chain_id":"{}","action":"{}","cmd":"{}"}}"#,
            self.chain_id, self.action, self.cmd
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
