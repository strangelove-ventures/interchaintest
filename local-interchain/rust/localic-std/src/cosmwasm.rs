use std::path::PathBuf;

use serde_json::Value;

use crate::{
    errors::LocalError,
    transactions::ChainRequestBuilder,
    types::{Contract, TransactionResponse},
};

#[derive(Clone)]
pub struct CosmWasm<'a> {
    rb: &'a ChainRequestBuilder,
}

impl CosmWasm<'_> {
    pub fn new(rb: &ChainRequestBuilder) -> CosmWasm {
        CosmWasm { rb }
    }

    pub fn store_contract(&self, key_name: &str, abs_path: PathBuf) -> Result<u64, LocalError> {
        // TODO: cache
        match self.rb.upload_contract(key_name, abs_path) {
            Ok(code_id) => Ok(code_id),
            Err(e) => Err(e),
        }
    }

    pub fn instantiate_contract(
        &self,
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

        let mut cmd = format!("tx wasm instantiate {code_id} {msg} --label={label} --from={account_key} --keyring-backend=test --node=%RPC% --chain-id=%CHAIN_ID% --output=json --gas=auto --gas-adjustment=3.0 --yes", code_id=code_id, msg=msg, label=label, account_key=account_key);
        if !updated_flags.is_empty() {
            cmd = format!("{}{}", cmd, updated_flags);
        }

        let res = self.rb.tx(cmd.as_str(), false);
        if res.is_err() {
            return Err(res.err().unwrap());
        }

        let res = res.unwrap();

        println!("wasm instantiate res: {}", &res);

        let tx_hash: Option<String> = (&self.rb).get_tx_hash(&res);
        let raw_log: Option<String> = (&self.rb).get_raw_log(&res);

        if raw_log.is_some() {
            println!("raw_log: {}", raw_log.clone().unwrap());
        }

        let contract_addr =
            get_contract_address(&self.rb, tx_hash.clone().unwrap_or_default().as_str());
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

    pub fn execute_contract(
        &self,
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

        let updated_flags = flags.to_string();
        if !updated_flags.is_empty() {
            cmd = format!("{} {}", cmd, updated_flags);
        }

        let res = self.rb.binary(cmd.as_str(), false);
        println!("execute_contract res: {}", &res);

        let tx_hash = self.rb.get_tx_hash(&res);
        let tx_raw_log = self.rb.get_raw_log(&res);

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

    pub fn query_contract(&self, contract_addr: &str, msg: &str) -> Value {
        let cmd = format!(
            "query wasm contract-state smart {} {} --output=json --node=%RPC%",
            contract_addr, msg
        );
        println!("query_contract cmd: {}", cmd);
        let res = self.rb.query(&cmd, false);
        res
    }
}

pub fn get_contract_address(rb: &ChainRequestBuilder, tx_hash: &str) -> Result<String, LocalError> {
    let res = rb.query_tx_hash(tx_hash.to_string());

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

    Err(LocalError::ContractAddressNotFound {})
}
