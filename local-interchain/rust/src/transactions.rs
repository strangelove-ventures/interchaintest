use reqwest::blocking::Client;
use serde_json::{Result, Value};

#[derive(Debug, PartialEq, Eq)]
pub enum RequestType {
    Bin,
    Query,
    Exec,
}
impl RequestType {
    fn as_str(&self) -> &'static str {
        match self {
            RequestType::Bin => "bin",
            RequestType::Query => "query",
            RequestType::Exec => "exec",
        }
    }
}

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

pub struct TransactionResponse {
    pub tx_hash: Option<String>,
    pub rawlog: Option<String>,
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

pub struct RequestBuilder {
    api: String,
    chain_id: String,
    log_output: Option<bool>, // false by default
}

impl RequestBuilder {
    pub fn new(&self, api: String, chain_id: String, log_output: Option<bool>) -> RequestBuilder {
        if api.is_empty() {
            panic!("api cannot be empty");
        }
        if chain_id.is_empty() {
            panic!("chain_id cannot be empty");
        }

        RequestBuilder {
            api,
            chain_id,
            log_output,
        }
    }

    // add a binary function which takes in a cmd, log_output Optional<bool>. It will use the request base and call it with RequestType.Bin
    // pub fn binary(&self, cmd: String, log_output: Option<bool>) -> RequestBase {
    //     let request_base = RequestBase::new(self.api.clone(), self.chain_id.clone(), RequestType::Bin);
    //     request_base
    //     // TODO:
    //     // return send_request(
    //     //     rb, cmd, log_output=(log_output if log_output else self.log)
    //     // )
    // }
}

pub fn send_request(
    request_base: RequestBase,
    cmd: String,
    return_text: bool,
    log_output: bool,
) -> serde_json::Value {
    if cmd.is_empty() {
        panic!("cmd cannot be empty");
    }
    let mut cmd = cmd;
    if request_base.request_type == RequestType::Query {
        if cmd.to_lowercase().starts_with("query ") {
            cmd = cmd[6..].to_string();
        } else if cmd.to_lowercase().starts_with("q ") {
            cmd = cmd[2..].to_string();
        }
    }

    let payload = ActionHandler::new(
        request_base.chain_id,
        request_base.request_type.as_str().to_string(),
        cmd,
    )
    .to_json();

    if log_output {
        println!("[send_request payload]: {}", payload);
        println!("[send_request url]: {}", request_base.url);
    }

    let res = request_base
        .client
        .post(request_base.url)
        .json(&payload)
        .header("Content-Type", "application/json")
        .header("Accept", "application/json")
        .send()
        .unwrap();

    if log_output {
        println!("[send_request resp]: {:?}", &res)
    }

    if return_text {
        return return_text_json(res.text().unwrap(), None);
    }

    match res.text() {
        Ok(text) => match serde_json::from_str::<Value>(&text) {
            Ok(json) => json,
            Err(err) => {
                return return_text_json(text, Some(err.to_string()));
            }
        },
        Err(err) => {
            return return_text_json("".to_string(), Some(err.to_string()));
        }
    }
}

fn return_text_json(text: String, err: Option<String>) -> serde_json::Value {
    serde_json::json!({
        "text": &text,
        "error": err,
    })
}
