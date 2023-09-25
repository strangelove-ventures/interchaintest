use std::path::PathBuf;

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
    pub fn new(rb: &ChainRequestBuilder) -> CosmWasm {
        CosmWasm {
            rb: rb,
            file_path: None,
            code_id: None,
            contract_addr: None,
        }
    }

    pub fn new_from_existing(
        rb: &ChainRequestBuilder,
        file_path: Option<PathBuf>,
        code_id: Option<u64>,
        contract_addr: Option<String>,
    ) -> CosmWasm {
        CosmWasm {
            rb: rb,
            file_path: file_path,
            code_id,
            contract_addr,
        }
    }

    pub fn store(&mut self, key_name: &str, abs_path: &PathBuf) -> Result<u64, LocalError> {
        // TODO: add cache
        match self.rb.upload_contract(key_name, abs_path) {
            Ok(code_id) => {
                self.code_id = Some(code_id);
                self.file_path = Some(abs_path.to_owned());
                Ok(code_id)
            }
            Err(e) => Err(e),
        }
    }

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
            None => panic!("contract_addr is none"),
        };

        match contract_instantiate(self.rb, account_key, code_id, msg, label, admin, flags) {
            Ok(contract) => {
                self.contract_addr = Some(contract.address.to_owned());
                Ok(contract)
            }
            Err(e) => Err(e),
        }
    }

    pub fn execute(
        &self,
        account_key: &str,
        msg: &str,
        flags: &str,
    ) -> Result<TransactionResponse, LocalError> {
        let contract_addr: &str = match &self.contract_addr {
            Some(addr) => addr.as_ref(),
            None => panic!("contract_addr is none"),
        };
        contract_execute(self.rb, contract_addr, account_key, msg, flags)
    }

    pub fn query(&self, msg: &str) -> Value {
        let contract_addr: &str = match &self.contract_addr {
            Some(addr) => addr.as_ref(),
            None => panic!("contract_addr is none"),
        };
        contract_query(self.rb, contract_addr, msg)
    }

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
            None => panic!("contract_addr is none"),
        };

        let dst_contract_addr: String = match &dst.contract_addr {
            Some(addr) => addr.to_string(),
            None => panic!("dst.contract_addr is none"),
        };

        r.create_wasm_connection(
            path,
            contract_addr.as_str(),
            dst_contract_addr.as_str(),
            order,
            version,
        )
    }
}

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
    if admin.is_none() && !flags.contains("--no-admin") {
        updated_flags = format!("{} --no-admin", flags);
    } else if admin.is_some() {
        updated_flags = format!("{} --admin={}", flags, admin.unwrap());
    }
    updated_flags = updated_flags.trim().to_string();

    let mut cmd = format!("tx wasm instantiate {code_id} {msg} --label={label} --from={account_key} --keyring-backend=test --node=%RPC% --chain-id=%CHAIN_ID% --output=json --gas=auto --gas-adjustment=3.0 --yes");
    if !updated_flags.is_empty() {
        cmd = format!("{} {}", cmd, updated_flags);
    }

    let res = rb.tx(cmd.as_str(), false);
    if res.is_err() {
        return Err(res.err().unwrap());
    }

    let res = res.unwrap();

    println!("wasm instantiate res: {}", &res);

    let tx_hash: Option<String> = rb.get_tx_hash(&res);
    let raw_log: Option<String> = rb.get_raw_log(&res);

    if raw_log.is_some() {
        println!("raw_log: {}", raw_log.clone().unwrap());
    }

    let contract_addr = get_contract_address(rb, tx_hash.clone().unwrap_or_default().as_str());
    match contract_addr {
        Ok(contract_addr) => {
            return Ok(Contract {
                address: contract_addr,
                tx_hash: tx_hash.unwrap_or_default(),
                admin: admin.map(|s| s.to_string()),
            });
        }
        Err(e) => {
            return Err(e);
        }
    }
}

pub fn contract_execute(
    rb: &ChainRequestBuilder,
    contract_addr: &str,
    account_key: &str,
    msg: &str,
    flags: &str,
) -> Result<TransactionResponse, LocalError> {
    let mut cmd = format!(
        "tx wasm execute {contract_addr} {msg} --from={account_key} --keyring-backend=test --home=%HOME% --node=%RPC% --chain-id=%CHAIN_ID% --yes {flags}",
        contract_addr = contract_addr,
        msg = msg,
        account_key = account_key,
        flags = flags
    );
    cmd = cmd.trim().to_string();

    let updated_flags = flags.to_string();
    if !updated_flags.is_empty() {
        cmd = format!("{} {}", cmd, updated_flags);
    }

    let res = rb.binary(cmd.as_str(), false);
    println!("execute_contract res: {}", &res);

    let tx_hash = rb.get_tx_hash(&res);
    let tx_raw_log = rb.get_raw_log(&res);

    if let Some(raw_log) = &tx_raw_log {
        if raw_log != "[]" {
            println!("execute_contract raw_log: {}", raw_log);
        }
    }

    Ok(TransactionResponse {
        tx_hash,
        rawlog: tx_raw_log,
    })
}

pub fn contract_query(rb: &ChainRequestBuilder, contract_addr: &str, msg: &str) -> Value {
    let cmd = format!(
        "query wasm contract-state smart {} {} --output=json --node=%RPC%",
        contract_addr, msg
    );
    println!("query_contract cmd: {}", cmd);
    let res = rb.query(&cmd, false);
    res
}

pub fn get_contract_address(rb: &ChainRequestBuilder, tx_hash: &str) -> Result<String, LocalError> {
    let mut last_error = LocalError::ContractAddressNotFound {
        events: "".to_string(),
    };
    for _ in 0..5 {
        let res = get_contract(rb, tx_hash);
        if res.is_ok() {
            return res;
        } else {
            last_error = res.err().unwrap();
        }
        std::thread::sleep(std::time::Duration::from_secs(1));
    }

    return Err(last_error);
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
