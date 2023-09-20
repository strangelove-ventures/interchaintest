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
pub struct ChainNode<'a> {
    rb: &'a ChainRequestBuilder,
}

impl ChainNode<'_> {
    pub fn new(rb: &ChainRequestBuilder) -> ChainNode {
        ChainNode { rb }
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
    pub fn recover_key(&self, key_name: &str, mnemonic: &str) -> Value {
        let cmd = format!("keyname={};mnemonic={}", key_name, mnemonic);
        self.rb
            .send_request(RequestType::RecoverKey, cmd.as_str(), false)
    }

    pub fn overwrite_genesis_file(&self, content: &str) -> Value {
        let cmd = format!("new_genesis={}", content);
        self.rb
            .send_request(RequestType::OverwriteGenesisFile, cmd.as_str(), false)
    }

    pub fn set_peers(&self, peers: &str) -> Value {
        let cmd = format!("new_peers={}", peers);
        self.rb
            .send_request(RequestType::SetNewPeers, cmd.as_str(), false)
    }

    /// add_full_node adds a full node to the network. A full node must already be running on the same network.
    pub fn add_full_node(&self, amount: u64) -> Value {
        let cmd = format!("amount={}", amount);
        self.rb
            .send_request(RequestType::AddFullNodes, cmd.as_str(), false)
    }

    // info request
    pub fn account_key_bech_32(&self, key_name: &str) -> Result<String, LocalError> {
        self.key_bech32(key_name, "")
    }

    pub fn key_bech32(&self, key_name: &str, bech_prefix: &str) -> Result<String, LocalError> {
        let mut cmd = format!(
            "keys show --address {} --home=%HOME% --keyring-backend=test",
            key_name
        );

        if bech_prefix != "" {
            cmd = format!("{} --bech {}", cmd, bech_prefix);
        }

        let res = &self.rb.bin(cmd.as_str(), true);
        let text = res["text"].to_string();
        if text.contains("Error:") {
            return Err(LocalError::KeyBech32Failed {
                reason: res["text"].to_string(),
            });
        }

        if &text != "" {
            let text = text.replace("\\n", "");
            let text = text.replace("\"", "");
            return Ok(text);
        }

        Err(LocalError::KeyBech32Failed {
            reason: res["error"].to_string(),
        })
    }

    pub fn get_chain_config(&self) -> Value {
        let res = self.info_builder("config", None);

        match serde_json::from_str::<Value>(&res) {
            Ok(res) => res,
            Err(_) => {
                return json!({});
            }
        }
    }

    pub fn get_name(&self) -> String {
        let res = self.info_builder("name", None);
        res
    }

    pub fn get_container_id(&self) -> String {
        let res = self.info_builder("container_id", None);
        res
    }

    pub fn get_host_name(&self) -> String {
        let res = self.info_builder("hostname", None);

        res
    }

    pub fn get_genesis_file_content(&self) -> Option<String> {
        match self.info_builder("genesis_file_content", None).as_str() {
            "" => None,
            res => Some(res.to_string()),
        }
    }
    pub fn get_home_dir(&self) -> String {
        let res = self.info_builder("home_dir", None);

        res
    }

    pub fn get_height(&self) -> u64 {
        let res = self.info_builder("height", None);

        match res.parse::<u64>() {
            Ok(res) => res,
            Err(_) => {
                return 0;
            }
        }
    }

    pub fn read_file(&self, relative_path: &str) -> String {
        let res = self.info_builder("read_file", Some(&[("relative_path", &relative_path)]));

        res
    }

    pub fn is_above_sdk_v47(&self) -> bool {
        let res = self.info_builder("is_above_sdk_v47", None);

        match res.parse::<bool>() {
            Ok(res) => res,
            Err(_) => {
                return false;
            }
        }
    }

    pub fn has_command(&self, command: &str) -> String {
        let res = self.info_builder("has_command", Some(&[("command", &command)]));

        res
    }

    pub fn get_build_information(&self) -> Value {
        let res = self.info_builder("build_information", None);

        match serde_json::from_str::<Value>(&res) {
            Ok(res) => res,
            Err(_) => {
                return json!({});
            }
        }
    }

    // TODO: test / & change to Result
    // {"error": "exit code 1:  Error: rpc error: code = NotFound desc = rpc error: code = NotFound desc = proposal 1 doesn't exist: key not found
    pub fn query_proposal(&self, proposal_id: &str) -> Value {
        let res = self.info_builder("query_proposal", Some(&[("proposal_id", &proposal_id)]));

        match serde_json::from_str::<Value>(&res) {
            Ok(res) => res,
            Err(_) => {
                return json!({});
            }
        }
    }

    // TODO: test. Use result
    pub fn dump_contract_state(&self, contract_addr: &str, height: u64) -> Value {
        let res = self.info_builder(
            "dump_contract_state",
            Some(&[
                ("contract", &contract_addr),
                ("height", &height.to_string()),
            ]),
        );

        match serde_json::from_str::<Value>(&res) {
            Ok(res) => res,
            Err(_) => {
                return json!({});
            }
        }
    }
}
