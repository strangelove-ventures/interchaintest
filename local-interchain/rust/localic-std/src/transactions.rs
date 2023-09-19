use std::path::PathBuf;

use reqwest::blocking::Client;
use serde_json::Value;

use crate::{
    errors::LocalError,
    types::{ActionHandler, RequestType},
};

pub struct ChainRequestBuilder {
    client: Client,
    api: String,
    chain_id: String,
    log_output: bool,
    return_text: bool,
}

impl ChainRequestBuilder {
    pub fn new(
        api: String,
        chain_id: String,
        log_output: bool,
        return_text: bool,
    ) -> ChainRequestBuilder {
        if api.is_empty() {
            panic!("api cannot be empty");
        }
        if chain_id.is_empty() {
            panic!("chain_id cannot be empty");
        }

        ChainRequestBuilder {
            client: Client::new(),
            api,
            chain_id,
            log_output,
            return_text: return_text,
        }
    }

    // app binary commands
    pub fn binary(&self, cmd: &str) -> Value {
        self.send_request(RequestType::Bin, cmd)
    }
    pub fn bin(&self, cmd: &str) -> Value {
        self.binary(cmd)
    }

    // app query commands
    pub fn query(&self, cmd: &str) -> Value {
        self.send_request(RequestType::Query, cmd)
    }
    pub fn q(&self, cmd: &str) -> Value {
        self.query(cmd)
    }

    // container execution commands
    pub fn execute(&self, cmd: &str) -> Value {
        self.send_request(RequestType::Exec, cmd)
    }
    pub fn exec(&self, cmd: &str) -> Value {
        self.execute(cmd)
    }

    // app transaction commands
    pub fn transaction(&self, cmd: &str, get_data: bool) -> Result<Value, LocalError> {
        let res = self.binary(&cmd);
        if !get_data {
            return Ok(res);
        }

        let tx_hash = self.get_tx_hash(&res);

        match tx_hash {
            Some(tx_hash) => {
                let data = self.query_tx_hash(tx_hash);
                Ok(data)
            }
            None => Err(LocalError::TxHashNotFound {}),
        }
    }
    pub fn tx(&self, cmd: &str, get_data: bool) -> Result<Value, LocalError> {
        self.transaction(cmd, get_data)
    }

    // transaction helpers
    pub fn decode_transaction(&self, protobuf_bytes: &str, use_hex: bool) -> Value {
        let mut cmd = format!("tx decode {}", protobuf_bytes);
        if use_hex {
            cmd = format!("{} --hex", cmd);
        }

        self.binary(cmd.as_str())
    }

    pub fn get_tx_hash(&self, tx_res: &Value) -> Option<String> {
        let tx_hash = tx_res["txhash"].as_str();
        match tx_hash {
            Some(tx_hash) => Some(tx_hash.to_string()),
            None => None,
        }
    }

    pub fn get_raw_log(&self, tx_res: &Value) -> Option<String> {
        let raw_log = tx_res["raw_log"].as_str();
        match raw_log {
            Some(raw_log) => Some(raw_log.to_string()),
            None => None,
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

    pub fn upload_file(&self, key_name: &str, abs_path: PathBuf) -> Result<u64, LocalError> {
        let file = abs_path.to_str().unwrap();
        if !abs_path.exists() {
            return Err(LocalError::CWUploadFailed {
                path: file.to_string(),
                reason: "file does not exist".to_string(),
            });
        }

        let payload = serde_json::json!({
            "chain_id": &self.chain_id,
            "key_name": key_name,
            "file_path": file.to_string(),
        });

        let url = (&self.api).to_string();
        let url = if url.ends_with("/") {
            url + "upload"
        } else {
            url + "/upload"
        };

        let req_base = self.client.post(&url).json(&payload);

        let req: reqwest::blocking::RequestBuilder;
        if self.return_text {
            req = req_base
                .header("Content-Type", "text/plain")
                .header("Accept", "text/plain");
        } else {
            req = req_base
                .header("Content-Type", "application/json")
                .header("Accept", "application/json");
        }

        let resp = req.send().unwrap();

        match resp.text() {
            Ok(text) => {
                // convert text to json
                let json = match serde_json::from_str::<Value>(&text) {
                    Ok(json) => json,
                    Err(err) => {
                        return Err(LocalError::CWUploadFailed {
                            path: file.to_string(),
                            reason: err.to_string(),
                        });
                    }
                };

                // get code_id from json
                let code_id = match json["code_id"].as_u64() {
                    Some(code_id) => code_id,
                    None => {
                        return Err(LocalError::CWUploadFailed {
                            path: file.to_string(),
                            reason: "code_id not found".to_string(),
                        });
                    }
                };

                // return code_id
                Ok(code_id)
            }
            Err(err) => {
                return Err(LocalError::CWUploadFailed {
                    path: file.to_string(),
                    reason: err.to_string(),
                });
            }
        }
    }

    pub fn send_request(&self, request_type: RequestType, command: &str) -> Value {
        if command.is_empty() {
            panic!("cmd cannot be empty");
        }

        let mut cmd: String = command.to_string();
        match request_type {
            RequestType::Query => {
                if cmd.to_lowercase().starts_with("query ") {
                    cmd = cmd[6..].to_string();
                } else if cmd.to_lowercase().starts_with("q ") {
                    cmd = cmd[2..].to_string();
                }
            }
            _ => {}
        }

        if !self.return_text {
            if !cmd.contains("--output=json") && !cmd.contains("--output json") {
                cmd = format!("{} --output=json", cmd);
            }
        }

        let payload = ActionHandler::new(
            (&self.chain_id).to_owned(),
            request_type.as_str().to_string(),
            cmd,
        )
        .to_json();

        if self.log_output {
            println!("[send_request payload]: {}", payload);
            // println!("[send_request url]: {}", &self.api);
        }

        let req_base = self.client.post(&self.api).json(&payload);

        let req: reqwest::blocking::RequestBuilder;
        if self.return_text {
            req = req_base
                .header("Content-Type", "text/plain")
                .header("Accept", "text/plain");
        } else {
            req = req_base
                .header("Content-Type", "application/json")
                .header("Accept", "application/json");
        }

        let res = req.send().unwrap();
        if self.return_text {
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
}

fn return_text_json(text: String, err: Option<String>) -> Value {
    serde_json::json!({
        "text": &text,
        "error": err,
    })
}
