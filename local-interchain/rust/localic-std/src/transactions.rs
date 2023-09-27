use std::path::Path;

use reqwest::blocking::{Client, RequestBuilder};
use serde_json::Value;

use crate::{
    errors::LocalError,
    types::{ActionHandler, RequestType},
};

#[derive(Debug)]
pub struct ChainRequestBuilder {
    pub client: Client,
    pub api: String,
    pub chain_id: String,
    log_output: bool,
}

impl ChainRequestBuilder {
    /// # Errors
    ///
    /// Returns `Err` if the `api` or `chain_id` is empty.    
    pub fn new(
        api: String,
        chain_id: String,
        log_output: bool,
    ) -> Result<ChainRequestBuilder, LocalError> {
        if api.is_empty() {
            println!("api cannot be empty");
            return Err(LocalError::ApiNotFound {});
        }
        if chain_id.is_empty() {
            println!("chain_id cannot be empty");
            return Err(LocalError::ChainIdNotFound {});
        }

        Ok(ChainRequestBuilder {
            client: Client::new(),
            api,
            chain_id,
            log_output,
        })
    }

    // app binary commands
    #[must_use]
    pub fn binary(&self, cmd: &str, return_text: bool) -> Value {
        self.send_request(RequestType::Bin, cmd, return_text)
    }
    #[must_use]
    pub fn bin(&self, cmd: &str, return_text: bool) -> Value {
        self.binary(cmd, return_text)
    }

    // app query commands
    #[must_use]
    pub fn query(&self, cmd: &str, return_text: bool) -> Value {
        self.send_request(RequestType::Query, cmd, return_text)
    }
    #[must_use]
    pub fn q(&self, cmd: &str, return_text: bool) -> Value {
        self.query(cmd, return_text)
    }

    // container execution commands
    #[must_use]
    pub fn execute(&self, cmd: &str, return_text: bool) -> Value {
        self.send_request(RequestType::Exec, cmd, return_text)
    }
    #[must_use]
    pub fn exec(&self, cmd: &str, return_text: bool) -> Value {
        self.execute(cmd, return_text)
    }

    /// # Errors
    ///
    /// Returns `Err` if the transaction fails.
    pub fn transaction(&self, cmd: &str, get_data: bool) -> Result<Value, LocalError> {
        let res = self.binary(cmd, false);
        if !get_data {
            return Ok(res);
        }

        let Some(tx_hash) = self.get_tx_hash(&res) else { return Err(LocalError::TxHashNotFound {}) };

        for _ in 0..5 {
            let data = self.query_tx_hash(&tx_hash);

            if !data.to_string().starts_with("{\"error\":") {
                return Ok(data);
            }

            std::thread::sleep(std::time::Duration::from_secs(1));
        }

        Err(LocalError::TxHashNotFound {})
    }

    /// # Errors
    ///
    /// Returns `Err` if the transaction fails.
    pub fn tx(&self, cmd: &str, get_data: bool) -> Result<Value, LocalError> {
        self.transaction(cmd, get_data)
    }

    #[must_use]
    pub fn decode_transaction(&self, protobuf_bytes: &str, use_hex: bool) -> Value {
        let mut cmd = format!("tx decode {protobuf_bytes}");
        if use_hex {
            cmd = format!("{cmd} --hex");
        }

        self.binary(cmd.as_str(), false)
    }
    #[must_use]
    pub fn get_tx_hash(&self, tx_res: &Value) -> Option<String> {
        let tx_hash = tx_res["txhash"].as_str();
        tx_hash.map(std::string::ToString::to_string)
    }

    /// # Errors
    ///
    /// Returns `Err` if the transaction status code is not found.
    pub fn get_sdk_status_code(&self, tx_res: &Value) -> Result<u64, LocalError> {
        let status_id = tx_res["code"].as_u64();
        match status_id {
            Some(code) => Ok(code),
            None => Err(LocalError::SdkTransactionStatusCodeNotFound {
                reason: "'code' not found in Tx JSON response.".to_string(),
            }),
        }
    }

    #[must_use]
    pub fn get_raw_log(&self, tx_res: &Value) -> Option<String> {
        let raw_log = tx_res["raw_log"].as_str();
        let res = raw_log.map(std::string::ToString::to_string);
        res.filter(|res| !(res.is_empty() || res == "[]"))
    }

    #[must_use]
    pub fn query_tx_hash(&self, tx_hash: &str) -> Value {
        if tx_hash.is_empty() {
            return serde_json::json!({
                "error": "tx_hash cannot be empty",
            });
        }

        let cmd = format!("tx {tx_hash} --output=json");
        self.query(&cmd, false)
    }

    /// # Errors
    ///
    /// Returns `Err` if the file does not exist or if the file path is not valid.
    pub fn upload_file(
        &self,
        abs_path: &Path,
        return_text: bool,
    ) -> Result<RequestBuilder, LocalError> {
        let file: String = match abs_path.to_str() {
            Some(file) => file.to_string(),
            None => {
                return Err(LocalError::UploadFailed {
                    path: match abs_path.to_str() {
                        Some(path) => path.to_string(),
                        None => String::new(),
                    },
                    reason: "file path is not valid".to_string(),
                });
            }
        };
        if !abs_path.exists() {
            return Err(LocalError::UploadFailed {
                path: file,
                reason: "file does not exist".to_string(),
            });
        }

        let payload = serde_json::json!({
            "chain_id": &self.chain_id,
            "file_path": file,
        });

        let url = self.api.to_string();
        let url = if url.ends_with('/') {
            url + "upload"
        } else {
            url + "/upload"
        };

        let header: &str = if return_text {
            "Content-Type: text/plain"
        } else {
            "Content-Type: application/json"
        };

        Ok(self
            .client
            .post(url)
            .json(&payload)
            .header("Accept", header)
            .header("Content-Type", header))
    }

    /// # Errors
    ///
    /// Returns `Err` if the contract is not uploaded.
    pub fn upload_contract(&self, key_name: &str, abs_path: &Path) -> Result<u64, LocalError> {
        let Some(file) = abs_path.to_str() else {
            return Err(LocalError::UploadFailed {
                path: match abs_path.to_str() {
                    Some(path) => path.to_string(),
                    None => String::new(),
                },
                reason: "file path is not valid".to_string(),
            });
        };

        let payload = serde_json::json!({
            "chain_id": &self.chain_id,
            "file_path": file,
            "key_name": key_name,
        });

        let req = self.upload_file(abs_path, false)?;

        let req = req.json(&payload).header("Upload-Type", "cosmwasm");

        // print req
        println!("[upload_contract req]: {req:?}");

        let resp = match req.send() {
            Ok(resp) => resp,
            Err(err) => {
                return Err(LocalError::UploadFailed {
                    path: file.to_string(),
                    reason: err.to_string(),
                });
            }
        };
        match resp.text() {
            Ok(text) => {
                if text.contains("error") {
                    return Err(LocalError::UploadFailed {
                        path: file.to_string(),
                        reason: text,
                    });
                }

                // convert text to json
                let json = match serde_json::from_str::<Value>(&text) {
                    Ok(json) => json,
                    Err(err) => {
                        return Err(LocalError::UploadFailed {
                            path: file.to_string(),
                            reason: err.to_string(),
                        });
                    }
                };

                // get code_id from json
                let Some(code_id) = json["code_id"].as_u64() else {
                                            return Err(LocalError::UploadFailed {
                                                path: file.to_string(),
                                                reason: "code_id not found".to_string(),
                                            });
                                        };

                // return code_id
                Ok(code_id)
            }
            Err(err) => Err(LocalError::UploadFailed {
                path: file.to_string(),
                reason: err.to_string(),
            }),
        }
    }

    #[must_use]
    pub fn send_request(
        &self,
        request_type: RequestType,
        command: &str,
        return_text: bool,
    ) -> Value {
        let mut cmd: String = command.to_string();
        if request_type == RequestType::Query {
            if cmd.to_lowercase().starts_with("query ") {
                cmd = cmd[6..].to_string();
            } else if cmd.to_lowercase().starts_with("q ") {
                cmd = cmd[2..].to_string();
            }
        };

        // return JSON if we we did not override that.
        if !return_text
            && (request_type == RequestType::Query || request_type == RequestType::Bin)
            && !cmd.contains("--output=json")
            && !cmd.contains("--output json")
        {
            cmd = format!("{cmd} --output=json");
        }

        // Build the request payload
        let payload = ActionHandler::new(self.chain_id.clone(), request_type, cmd).to_json();

        if self.log_output {
            println!("[send_request payload]: {payload}");
        }

        let mut rb = self.client.post(&self.api).json(&payload);

        let content_type = if return_text {
            "text/plain"
        } else {
            "application/json"
        };

        rb = rb
            .header("Content-Type", content_type)
            .header("Accept", content_type);

        let res = match rb.send() {
            Ok(res) => res,
            Err(err) => {
                return return_text_json("", Some(err.to_string()));
            }
        };
        let text = res.text();

        if return_text {
            return return_text_json(text.unwrap_or_default().as_str(), None);
        }

        match text {
            Ok(text) => match serde_json::from_str::<Value>(&text) {
                Ok(json) => json,
                Err(err) => return_text_json(text.as_str(), Some(err.to_string())),
            },
            Err(err) => return_text_json("", Some(err.to_string())),
        }
    }
}

fn return_text_json(text: &str, err: Option<String>) -> Value {
    serde_json::json!({
        "text": &text,
        "error": match err.unwrap_or_default() {
            err if err.is_empty() => None,
            err => Some(err),
        },
    })
}
