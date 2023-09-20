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
// pub fn StartContainer(rb: &ChainRequestBuilder) -> String {
// pub fn PauseContainer(rb: &ChainRequestBuilder) -> String {
// pub fn UnpauseContainer(rb: &ChainRequestBuilder) -> String {
// pub fn StopContainer(rb: &ChainRequestBuilder) -> String {
// pub fn RemoveContainer(rb: &ChainRequestBuilder) -> String {

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

    // pub fn recover_key(&self, key_name: &str, mnemonic: &str) -> Value {
    //     let config = self.get_chain_config();
    //     let cmd = format!(
    //         "echo {} | {} keys add {} --recover --keyring-backend test --coin-type {} --home=%HOME% --output json",
    //         mnemonic,
    //         config["bin"].as_str().unwrap(),
    //         key_name,
    //         config["coin_type"].as_str().unwrap(),
    //     );
    //     println!("recover_key cmd: {}", cmd);
    //     self.rb.send_request(RequestType::Exec, cmd.as_str(), true)
    // }

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
        println!("get_name res: {}", res);
        match serde_json::from_str::<Value>(&res) {
            Ok(res) => res,
            Err(_) => {
                return json!({});
            }
        }
    }

    pub fn get_name(&self) -> String {
        let res = self.info_builder("name", None);
        println!("get_name res: {}", res);
        res
    }

    pub fn get_container_id(&self) -> String {
        let res = self.info_builder("container_id", None);
        println!("get_container_id res: {}", res);
        res
    }

    pub fn get_host_name(&self) -> String {
        let res = self.info_builder("hostname", None);
        println!("get_host_name res: {}", res);
        res
    }

    pub fn get_genesis_file_content(&self) -> Option<String> {
        match self.info_builder("genesis_file_content", None).as_str() {
            "" => None,
            res => {
                println!("get_genesis_file_content res: {}", res);
                Some(res.to_string())
            }
        }
    }
    pub fn get_home_dir(&self) -> String {
        let res = self.info_builder("home_dir", None);
        println!("get_home_dir res: {}", res);
        res
    }

    pub fn get_height(&self) -> u64 {
        let res = self.info_builder("height", None);
        println!("get_height res: {}", res);

        match res.parse::<u64>() {
            Ok(res) => res,
            Err(_) => {
                return 0;
            }
        }
    }

    pub fn read_file(&self, relative_path: &str) -> String {
        let res = self.info_builder("read_file", Some(&[("relative_path", &relative_path)]));
        println!("read_file res: {}", res);
        res
    }

    pub fn is_above_sdk_v47(&self) -> bool {
        let res = self.info_builder("is_above_sdk_v47", None);
        println!("is_above_sdk_v47 res: {}", res);
        match res.parse::<bool>() {
            Ok(res) => res,
            Err(_) => {
                return false;
            }
        }
    }

    pub fn has_command(&self, command: &str) -> String {
        let res = self.info_builder("has_command", Some(&[("command", &command)]));
        println!("has_command res: {}", res);
        res
    }

    pub fn get_build_information(&self) -> Value {
        let res = self.info_builder("build_information", None);
        println!("get_build_inforamtion res: {}", res);
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
        println!("query_proposal res: {}", res);
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
        println!("dump_contract_state res: {}", res);

        match serde_json::from_str::<Value>(&res) {
            Ok(res) => res,
            Err(_) => {
                return json!({});
            }
        }
    }
}
