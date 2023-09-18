use reqwest::blocking::Client;
use serde_json::Value;

#[derive(Debug, PartialEq, Eq, Clone)]
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

#[derive(Debug, PartialEq, Eq)]
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
    client: Client,

    api: String,
    chain_id: String,
    log_output: bool,
}

impl RequestBuilder {
    pub fn new(api: String, chain_id: String, log_output: bool) -> RequestBuilder {
        if api.is_empty() {
            panic!("api cannot be empty");
        }
        if chain_id.is_empty() {
            panic!("chain_id cannot be empty");
        }

        RequestBuilder {
            client: Client::new(),
            api,
            chain_id,
            log_output,
        }
    }

    pub fn binary(&self, cmd: &str) -> serde_json::Value {
        let request_base = RequestBase::new(
            self.client.clone(),
            self.api.clone(),
            self.chain_id.clone(),
            RequestType::Bin,
        );
        send_request(request_base, cmd.to_string(), false, self.log_output)
    }

    pub fn query(&self, cmd: &str) -> serde_json::Value {
        let request_base = RequestBase::new(
            self.client.clone(),
            self.api.clone(),
            self.chain_id.clone(),
            RequestType::Query,
        );
        send_request(request_base, cmd.to_string(), false, self.log_output)
    }

    pub fn transaction(&self, cmd: &str) -> serde_json::Value {
        let res = self.binary(&cmd);

        let tx_hash = self.get_tx_hash(&res);
        let data = self.query_tx_hash(tx_hash);
        data
    }

    pub fn get_tx_hash(&self, tx_res: &Value) -> String {
        let tx_hash = get_tx_hash(tx_res);
        match tx_hash {
            Some(tx_hash) => tx_hash,
            None => panic!("tx_hash not found"),
        }
    }

    pub fn query_tx_hash(&self, tx_hash: String) -> Value {
        if tx_hash.is_empty() {
            panic!("tx_hash cannot be empty");
        }

        let cmd = format!("tx {} --output=json", tx_hash);
        let res = self.query(&cmd);
        // TODO: the python api returns it as {"tx": res} I am not sure why
        res
    }
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
    match request_base.request_type {
        RequestType::Bin => {
            if !cmd.to_lowercase().starts_with("tx ") {
                cmd = format!("tx {}", cmd);
            }
        }
        RequestType::Query => {
            if cmd.to_lowercase().starts_with("query ") {
                cmd = cmd[6..].to_string();
            } else if cmd.to_lowercase().starts_with("q ") {
                cmd = cmd[2..].to_string();
            }
        }
        _ => {}
    }

    if !return_text {
        if !cmd.contains("--output=json") && !cmd.contains("--output json") {
            cmd = format!("{} --output=json", cmd);
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

    let req_base = request_base.client.post(request_base.url).json(&payload);

    let req: reqwest::blocking::RequestBuilder;
    if return_text {
        req = req_base
            .header("Content-Type", "text/plain")
            .header("Accept", "text/plain");
    } else {
        req = req_base
            .header("Content-Type", "application/json")
            .header("Accept", "application/json");
    }

    let res = req.send().unwrap();

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

pub fn get_transaction_response(send_req_res: Value) -> TransactionResponse {
    let tx_hash = send_req_res["txhash"].as_str();
    let raw_log = send_req_res["raw_log"].as_str();

    let txr = TransactionResponse {
        tx_hash: tx_hash.map(|s| s.to_string()),
        rawlog: raw_log.map(|s| s.to_string()),
    };

    println!("txr: {:?}", txr);
    txr
}

pub fn get_tx_hash(res: &Value) -> Option<String> {
    let tx_hash = res["txhash"].as_str();
    match tx_hash {
        Some(tx_hash) => Some(tx_hash.to_string()),
        None => None,
    }
}

fn return_text_json(text: String, err: Option<String>) -> serde_json::Value {
    serde_json::json!({
        "text": &text,
        "error": err,
    })
}
