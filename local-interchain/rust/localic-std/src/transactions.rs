use std::path::PathBuf;

use reqwest::blocking::{Client, RequestBuilder};
use serde_json::Value;

use crate::{
    errors::LocalError,
    types::{ActionHandler, RequestType},
};

pub struct ChainRequestBuilder {
    pub client: Client,
    pub api: String,
    pub chain_id: String,
    log_output: bool,
}

impl ChainRequestBuilder {
    pub fn new(api: String, chain_id: String, log_output: bool) -> ChainRequestBuilder {
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
        }
    }

    // app binary commands
    pub fn binary(&self, cmd: &str, return_text: bool) -> Value {
        self.send_request(RequestType::Bin, cmd, return_text)
    }
    pub fn bin(&self, cmd: &str, return_text: bool) -> Value {
        self.binary(cmd, return_text)
    }

    // app query commands
    pub fn query(&self, cmd: &str, return_text: bool) -> Value {
        self.send_request(RequestType::Query, cmd, return_text)
    }
    pub fn q(&self, cmd: &str, return_text: bool) -> Value {
        self.query(cmd, return_text)
    }

    // container execution commands
    pub fn execute(&self, cmd: &str, return_text: bool) -> Value {
        self.send_request(RequestType::Exec, cmd, return_text)
    }
    pub fn exec(&self, cmd: &str, return_text: bool) -> Value {
        self.execute(cmd, return_text)
    }

    // app transaction commands
    pub fn transaction(&self, cmd: &str, get_data: bool) -> Result<Value, LocalError> {
        let res = self.binary(&cmd, false);
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

        self.binary(cmd.as_str(), false)
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
        let res = self.query(&cmd, false);
        // TODO: the python api returns it as {"tx": res} I am not sure why
        res
    }

    pub fn upload_file(
        &self,
        abs_path: &PathBuf,
        return_text: bool,
    ) -> Result<RequestBuilder, LocalError> {
        let file: String = abs_path.to_str().unwrap().to_string();
        if !abs_path.exists() {
            return Err(LocalError::UploadFailed {
                path: file,
                reason: "file does not exist".to_string(),
            });
        }

        let payload = serde_json::json!({
            "chain_id": &self.chain_id,
            "file_path": file.to_string(),
        });

        let url = (&self.api).to_string();
        let url = if url.ends_with("/") {
            url + "upload"
        } else {
            url + "/upload"
        };

        let header: &str;
        if return_text {
            header = "Content-Type: text/plain";
        } else {
            header = "Content-Type: application/json";
        }

        Ok(self
            .client
            .post(&url)
            .json(&payload)
            .header("Accept", header)
            .header("Content-Type", header))
    }

    pub fn upload_contract(&self, key_name: &str, abs_path: &PathBuf) -> Result<u64, LocalError> {
        let file = abs_path.to_str().unwrap().to_string();
        let payload = serde_json::json!({
            "chain_id": &self.chain_id,
            "file_path": file,
            "key_name": key_name,
        });

        let req = self.upload_file(abs_path, false);
        if req.is_err() {
            return Err(req.err().unwrap());
        }

        let req = req
            .unwrap()
            .json(&payload)
            .header("Upload-Type", "cosmwasm");

        // print req
        println!("[upload_contract req]: {:?}", req);

        let resp = req.send().unwrap();
        match resp.text() {
            Ok(text) => {
                if text.contains("error") {
                    return Err(LocalError::UploadFailed {
                        path: file,
                        reason: text.to_string(),
                    });
                }

                // convert text to json
                let json = match serde_json::from_str::<Value>(&text) {
                    Ok(json) => json,
                    Err(err) => {
                        return Err(LocalError::UploadFailed {
                            path: file,
                            reason: err.to_string(),
                        });
                    }
                };

                // get code_id from json
                let code_id = match json["code_id"].as_u64() {
                    Some(code_id) => code_id,
                    None => {
                        return Err(LocalError::UploadFailed {
                            path: file.to_string(),
                            reason: "code_id not found".to_string(),
                        });
                    }
                };

                // return code_id
                Ok(code_id)
            }
            Err(err) => {
                return Err(LocalError::UploadFailed {
                    path: file.to_string(),
                    reason: err.to_string(),
                });
            }
        }
    }

    pub fn send_request(
        &self,
        request_type: RequestType,
        command: &str,
        return_text: bool,
    ) -> Value {
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
        };

        // return JSON if we we did not override that.
        if !return_text && (request_type == RequestType::Query || request_type == RequestType::Bin)
        {
            if !cmd.contains("--output=json") && !cmd.contains("--output json") {
                cmd = format!("{} --output=json", cmd);
            }
        }

        // Build the request payload
        let payload = ActionHandler::new((&self.chain_id).to_owned(), request_type, cmd).to_json();

        if self.log_output {
            println!("[send_request payload]: {}", payload);
        }

        let mut rb = self.client.post(&self.api).json(&payload);

        let content_type = match return_text {
            true => "text/plain",
            false => "application/json",
        };

        rb = rb
            .header("Content-Type", content_type)
            .header("Accept", content_type);

        let res = rb.send().unwrap();
        let text = res.text();

        if return_text {
            return return_text_json(text.unwrap_or_default(), None);
        }

        match text {
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
