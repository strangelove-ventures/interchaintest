use std::path::{Path, PathBuf};

use serde_json::Value;

use crate::{
    errors::LocalError,
    relayer::Relayer,
    transactions::ChainRequestBuilder,
    types::{Contract, TransactionResponse},
};

#[derive(Clone)]
pub struct CosmWasm<'a> {
    rb: &'a ChainRequestBuilder,

    pub file_path: Option<PathBuf>,
    pub code_id: Option<u64>,
    pub contract_addr: Option<String>,
}

impl CosmWasm<'_> {
    #[must_use]
    pub fn new(rb: &ChainRequestBuilder) -> CosmWasm {
        CosmWasm {
            rb,
            file_path: None,
            code_id: None,
            contract_addr: None,
        }
    }

    #[must_use]
    pub fn new_from_existing(
        rb: &ChainRequestBuilder,
        file_path: Option<PathBuf>,
        code_id: Option<u64>,
        contract_addr: Option<String>,
    ) -> CosmWasm {
        CosmWasm {
            rb,
            file_path,
            code_id,
            contract_addr,
        }
    }

    /// # Errors
    ///
    /// Returns `Err` if the `code_id` is not found when uploading the contract.
    pub fn store(&mut self, key_name: &str, abs_path: &Path) -> Result<u64, LocalError> {
        // TODO: add cache
        println!(
            "uploading contract to {}: {}",
            self.rb.chain_id.as_str(),
            abs_path.to_str().unwrap_or_default()
        );
        match self.rb.upload_contract(key_name, abs_path) {
            Ok(code_id) => {
                self.code_id = Some(code_id);
                self.file_path = Some(abs_path.to_path_buf());
                Ok(code_id)
            }
            Err(e) => Err(e),
        }
    }

    /// # Errors
    ///
    /// Returns `Err` if the contract address is not found in the transaction logs.
    pub fn instantiate(
        &mut self,
        account_key: &str,
        msg: &str,
        label: &str,
        admin: Option<&str>,
        flags: &str,
    ) -> Result<Contract, LocalError> {
        let code_id: u64 = match &self.code_id {
            Some(code) => code.to_owned(),
            None => {
                return Err(LocalError::CWValueIsNone {
                    value_type: "code_id".to_string(),
                })
            }
        };

        match contract_instantiate(self.rb, account_key, code_id, msg, label, admin, flags) {
            Ok(contract) => {
                self.contract_addr = Some(contract.address.clone());
                Ok(contract)
            }
            Err(e) => Err(e),
        }
    }

    /// # Errors
    ///
    /// Returns `Err` if the sdk status code can not be found in the JSON blob.
    pub fn execute(
        &self,
        account_key: &str,
        msg: &str,
        flags: &str,
    ) -> Result<TransactionResponse, LocalError> {
        let contract_addr: &str = match &self.contract_addr {
            Some(addr) => addr.as_ref(),
            None => {
                return Err(LocalError::CWValueIsNone {
                    value_type: "contract_addr".to_string(),
                })
            }
        };
        contract_execute(self.rb, contract_addr, account_key, msg, flags)
    }

    /// # Panics
    ///
    /// Panics if the contract address already set in the `CosmWasm` object.
    #[must_use]
    pub fn query(&self, msg: &str) -> Value {
        let contract_addr: &str = match &self.contract_addr {
            Some(addr) => addr.as_ref(),
            None => panic!("contract_addr is none"),
        };
        contract_query(self.rb, contract_addr, msg)
    }

    // make a query request which uses a serde_json::Value as the msg
    #[must_use]
    pub fn query_value(&self, msg: &Value) -> Value {
        self.query(msg.to_string().as_str())
    }

    /// # Errors
    ///
    /// Returns `Err` if the contract address is not found in the transaction logs.
    pub fn create_wasm_connection(
        &self,
        r: &Relayer,
        path: &str,
        dst: &CosmWasm,
        order: &str,
        version: &str,
    ) -> Result<Value, LocalError> {
        let contract_addr: String = match &self.contract_addr {
            Some(addr) => addr.to_string(),
            None => {
                return Err(LocalError::CWValueIsNone {
                    value_type: "contract_addr".to_string(),
                })
            }
        };

        let dst_contract_addr: String = match &dst.contract_addr {
            Some(addr) => addr.to_string(),
            None => {
                return Err(LocalError::CWValueIsNone {
                    value_type: "dst.contract_addr".to_string(),
                })
            }
        };

        let src: String = format!("wasm.{}", contract_addr.as_str());
        let dst = format!("wasm.{}", dst_contract_addr.as_str());

        r.create_connection(path, src.as_str(), dst.as_str(), order, version)
    }
}

/// # Errors
///
/// Returns `Err` if the transaction fails or the contract address is not found.
pub fn contract_instantiate(
    rb: &ChainRequestBuilder,
    account_key: &str,
    code_id: u64,
    msg: &str,
    label: &str,
    admin: Option<&str>,
    flags: &str,
) -> Result<Contract, LocalError> {
    let mut updated_flags = flags.to_string();
    if let Some(admin) = admin {
        updated_flags = format!("{flags} --admin={admin}");
    } else if !flags.contains("--no-admin") {
        updated_flags = format!("{flags} --no-admin");
    }

    updated_flags = updated_flags.trim().to_string();

    let mut cmd = format!("tx wasm instantiate {code_id} {msg} --label={label} --from={account_key} --output=json --gas=auto --gas-adjustment=3.0");
    if !updated_flags.is_empty() {
        cmd = format!("{cmd} {updated_flags}");
    }

    let res = rb.tx(cmd.as_str(), false)?;

    let tx_hash: Option<String> = rb.get_tx_hash(&res);
    let raw_log: Option<String> = rb.get_raw_log(&res);

    if raw_log.is_some() {
        println!("raw_log: {}", raw_log.unwrap_or_default());
    }

    let contract_addr = get_contract_address(rb, tx_hash.clone().unwrap_or_default().as_str());
    match contract_addr {
        Ok(contract_addr) => Ok(Contract {
            address: contract_addr,
            tx_hash: tx_hash.unwrap_or_default(),
            admin: admin.map(std::string::ToString::to_string),
        }),
        Err(e) => Err(e),
    }
}

/// # Errors
///
/// Returns `Err` if the SDK response code can not be found in the transaction data.
pub fn contract_execute(
    rb: &ChainRequestBuilder,
    contract_addr: &str,
    account_key: &str,
    msg: &str,
    flags: &str,
) -> Result<TransactionResponse, LocalError> {
    let mut cmd = format!("tx wasm execute {contract_addr} {msg} --from={account_key} {flags}");
    cmd = cmd.trim().to_string();

    let updated_flags = flags.to_string();
    if !updated_flags.is_empty() {
        cmd = format!("{cmd} {updated_flags}");
    }

    let res = match rb.tx(cmd.as_str(), false) {
        Ok(res) => res,
        Err(e) => {
            return Err(e);
        }
    };

    let tx_hash = rb.get_tx_hash(&res);
    let raw_log = rb.get_raw_log(&res);

    let status_code = match rb.get_sdk_status_code(&res) {
        Ok(code_id) => code_id,
        Err(e) => {
            return Err(e);
        }
    };

    if let Some(tx_raw_log) = &raw_log {
        println!("raw_log: {tx_raw_log}");
    }

    Ok(TransactionResponse {
        status_code,
        tx_hash,
        raw_log,
    })
}

#[must_use]
pub fn contract_query(rb: &ChainRequestBuilder, contract_addr: &str, msg: &str) -> Value {
    let cmd = format!("query wasm contract-state smart {contract_addr} {msg}",);
    rb.query(&cmd, false)
}

/// # Errors
///
/// Returns `Err` if the contract address is not found in the transaction logs.
pub fn get_contract_address(rb: &ChainRequestBuilder, tx_hash: &str) -> Result<String, LocalError> {
    let mut last_error = LocalError::ContractAddressNotFound {
        events: String::new(),
    };
    for _ in 0..5 {
        let res = get_contract(rb, tx_hash);
        if res.is_ok() {
            return res;
        }

        let Some(err) = res.err() else { continue };
        last_error = err;
        std::thread::sleep(std::time::Duration::from_secs(1));
    }

    Err(last_error)
}

fn get_contract(rb: &ChainRequestBuilder, tx_hash: &str) -> Result<String, LocalError> {
    let res = rb.query_tx_hash(tx_hash);

    let code = res["code"].as_i64().unwrap_or_default();
    if code != 0 {
        let raw = res["raw_log"].as_str().unwrap_or_default();
        return Err(LocalError::TxNotSuccessful {
            code_status: code,
            raw_log: raw.to_string(),
        });
    }

    for event in res["logs"][0]["events"].as_array().iter() {
        for attr in event.iter() {
            for attr_values in attr["attributes"].as_array().iter() {
                for attr in attr_values.iter() {
                    if let Some(key) = attr["key"].as_str() {
                        if key.contains("contract_address") {
                            let contract_address = attr["value"].as_str().unwrap_or_default();
                            return Ok(contract_address.to_string());
                        }
                    }
                }
            }
        }
    }

    Err(LocalError::ContractAddressNotFound {
        events: res.to_string(),
    })
}
