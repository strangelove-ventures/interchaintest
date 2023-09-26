// ref: chain_node.go
use crate::{errors::LocalError, transactions::ChainRequestBuilder, types::RequestType};
use serde_json::{json, Value};

// TODO: This should be a POST action rather than a GET. Make a new function here to do so.
// pub fn overwite_genesis_file(content: &str)
// SetPeers
// pub fn VoteOnProposal(rb: &ChainRequestBuilder, proposal_id: String, vote: String) -> String {
// pub fn SubmitProposal(key_name: String, proposal_json_path: Pathbuf)
// pub fn UpgradeProposal(key_name: String, upgradeheight, title, description, deposit) // need to write other code for this to work
// pub fn ExportState(rb: &ChainRequestBuilder, height: u64) -> String {
// pub fn UnsafeResetAll(rb: &ChainRequestBuilder) -> String {

#[derive(Clone)]
pub struct Chain<'a> {
    rb: &'a ChainRequestBuilder,
}

impl Chain<'_> {
    #[must_use]
    pub fn new(rb: &ChainRequestBuilder) -> Chain {
        Chain { rb }
    }

    fn info_builder(&self, request: &str, extra_params: Option<&[(&str, &str)]>) -> String {
        let query_params: Vec<(&str, &str)> = {
            let mut params = vec![
                ("chain_id", self.rb.chain_id.as_str()),
                ("request", request),
            ];

            if let Some(extra_params) = extra_params {
                params.extend_from_slice(extra_params);
            }

            params
        };
        let query_params: &[(&str, &str)] = query_params.as_slice();

        let res = self
            .rb
            .client
            .get(format!("{}/info", self.rb.api).as_str())
            .query(query_params)
            .send()
            .unwrap()
            .text()
            .unwrap();
        res
    }

    // actions
    #[must_use]
    pub fn recover_key(&self, key_name: &str, mnemonic: &str) -> Value {
        let cmd = format!("keyname={key_name};mnemonic={mnemonic}");
        self.rb
            .send_request(RequestType::RecoverKey, cmd.as_str(), false)
    }

    #[must_use]
    pub fn overwrite_genesis_file(&self, content: &str) -> Value {
        let cmd = format!("new_genesis={content}");
        self.rb
            .send_request(RequestType::OverwriteGenesisFile, cmd.as_str(), false)
    }

    #[must_use]
    pub fn set_peers(&self, peers: &str) -> Value {
        let cmd = format!("new_peers={peers}");
        self.rb
            .send_request(RequestType::SetNewPeers, cmd.as_str(), false)
    }

    /// `add_full_node` adds a full node to the network. A full node must already be running on the same network.
    #[must_use]
    pub fn add_full_node(&self, amount: u64) -> Value {
        let cmd = format!("amount={amount}");
        self.rb
            .send_request(RequestType::AddFullNodes, cmd.as_str(), false)
    }

    // info request
    /// # Errors
    ///
    /// Returns `Err` if the key bech32 fails on the node.
    pub fn account_key_bech_32(&self, key_name: &str) -> Result<String, LocalError> {
        self.key_bech32(key_name, "")
    }

    /// # Errors
    ///
    /// Returns `Err` if the key bech32 fails on the node.
    pub fn key_bech32(&self, key_name: &str, bech_prefix: &str) -> Result<String, LocalError> {
        let mut cmd =
            format!("keys show --address {key_name} --home=%HOME% --keyring-backend=test",);

        if !bech_prefix.is_empty() {
            cmd = format!("{cmd} --bech {bech_prefix}");
        }

        let res = &self.rb.bin(cmd.as_str(), true);
        let text = res["text"].to_string();
        if text.contains("Error:") {
            return Err(LocalError::KeyBech32Failed {
                reason: res["text"].to_string(),
            });
        }

        if !text.is_empty() {
            let text = text.replace("\\n", "");
            let text = text.replace('\"', "");
            return Ok(text);
        }

        Err(LocalError::KeyBech32Failed {
            reason: res["error"].to_string(),
        })
    }

    #[must_use]
    pub fn get_chain_config(&self) -> Value {
        let res = self.info_builder("config", None);

        match serde_json::from_str::<Value>(&res) {
            Ok(res) => res,
            Err(_) => {
                json!({})
            }
        }
    }

    #[must_use]
    pub fn get_name(&self) -> String {
        self.info_builder("name", None)
    }

    #[must_use]
    pub fn get_container_id(&self) -> String {
        self.info_builder("container_id", None)
    }

    #[must_use]
    pub fn get_host_name(&self) -> String {
        self.info_builder("hostname", None)
    }

    #[must_use]
    pub fn get_genesis_file_content(&self) -> Option<String> {
        match self.info_builder("genesis_file_content", None).as_str() {
            "" => None,
            res => Some(res.to_string()),
        }
    }

    #[must_use]
    pub fn get_home_dir(&self) -> String {
        self.info_builder("home_dir", None)
    }

    #[must_use]
    pub fn get_height(&self) -> u64 {
        let res = self.info_builder("height", None);
        res.parse::<u64>().unwrap_or(0)
    }

    #[must_use]
    pub fn read_file(&self, relative_path: &str) -> String {
        self.info_builder("read_file", Some(&[("relative_path", relative_path)]))
    }

    #[must_use]
    pub fn is_above_sdk_v47(&self) -> bool {
        let res = self.info_builder("is_above_sdk_v47", None);
        res.parse::<bool>().unwrap_or(false)
    }

    #[must_use]
    pub fn has_command(&self, command: &str) -> String {
        self.info_builder("has_command", Some(&[("command", command)]))
    }

    #[must_use]
    pub fn get_build_information(&self) -> Value {
        let res = self.info_builder("build_information", None);

        match serde_json::from_str::<Value>(&res) {
            Ok(res) => res,
            Err(_) => {
                json!({})
            }
        }
    }

    // TODO: test / & change to Result
    #[must_use]
    pub fn query_proposal(&self, proposal_id: &str) -> Value {
        let res = self.info_builder("query_proposal", Some(&[("proposal_id", proposal_id)]));

        match serde_json::from_str::<Value>(&res) {
            Ok(res) => res,
            Err(_) => {
                json!({})
            }
        }
    }

    // TODO: test. Use result
    #[must_use]
    pub fn dump_contract_state(&self, contract_addr: &str, height: u64) -> Value {
        let res = self.info_builder(
            "dump_contract_state",
            Some(&[("contract", contract_addr), ("height", &height.to_string())]),
        );

        match serde_json::from_str::<Value>(&res) {
            Ok(res) => res,
            Err(_) => {
                json!({})
            }
        }
    }
}
