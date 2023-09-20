// ref: chain_node.go

// make this a self so we do not have to do rb allt he time

use std::path::PathBuf;

use crate::{errors::LocalError, transactions::ChainRequestBuilder};

#[derive(Clone)]
pub struct ChainNode<'a> {
    rb: &'a ChainRequestBuilder,
}

impl ChainNode<'_> {
    pub fn new(rb: &ChainRequestBuilder) -> ChainNode {
        ChainNode { rb }
    }

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

    fn info_builder(&self, request: &str, extra_params: Option<&[(&str, &str)]>) -> String {
        // let query_params: &[(&str, &str)] = &[
        //     ("chain_id", self.rb.chain_id.as_str()),
        //     ("request", request),
        // ];
        
        // let query_params = match extra_params {
        //     Some(extra_params) => {
        //         let mut query_params = query_params.to_vec();
        //         query_params.extend_from_slice(extra_params);
        //         query_params.as_slice().clone()
        //     }
        //     None => query_params,
        // };
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
}

// TODO: This should be an action
// pub fn overwite_genesis_file(content: &str)
// SetPeers

// pub fn ReadFile(rb: &ChainRequestBuilder, file_path: String) -> String {

// }

// pub fn RecoverKey(rb: &ChainRequestBuilder, key_name: String, mnemonic: String) -> String {

// }

// pub fn IsAboveSDK47(rb: &ChainRequestBuilder) -> bool {

// }

// // this is done on the bin
// pub fn HasCommand(rb: &ChainRequestBuilder, command: String) -> bool {

// }

// // return a custom struct here
// pub fn GetBuildInformation(rb: &ChainRequestBuilder) -> String {

// }

// pub fn VoteOnProposal(rb: &ChainRequestBuilder, proposal_id: String, vote: String) -> String {

// }

// pub fn QueryProposal(rb: &ChainRequestBuilder, proposal_id: String) -> String {

// }

// // pub fn SubmitProposal(key_name: String, proposal_json_path: Pathbuf)
// // pub fn UpgradeProposal(key_name: String, upgradeheight, title, description, deposit) // need to write other code for this to work

// // pub fn DumpContractState(rb: &ChainRequestBuilder, contract_addr: String, height: u64) -> String {
// //     stdout, _, err := tn.ExecQuery(ctx,
// // 		"wasm", "contract-state", "all", contractAddress,
// // 		"--height", fmt.Sprint(height),
// // 	)
// // }

// pub fn ExportState(rb: &ChainRequestBuilder, height: u64) -> String {

// }

// // pub fn UnsafeResetAll(rb: &ChainRequestBuilder) -> String {

// // }

// // pub fn StartContainer(rb: &ChainRequestBuilder) -> String {

// // }
// // pub fn PauseContainer(rb: &ChainRequestBuilder) -> String {

// // }
// // pub fn UnpauseContainer(rb: &ChainRequestBuilder) -> String {

// // }
// // pub fn StopContainer(rb: &ChainRequestBuilder) -> String {

// // }
// // pub fn RemoveContainer(rb: &ChainRequestBuilder) -> String {

// // }

// // pub fn KeyBech32
// // command := []string{tn.Chain.Config().Bin, "keys", "show", "--address", name,
// // "--home", tn.HomeDir(),
// // "--keyring-backend", keyring.BackendTest,
// // }

// // if bech != "" {
// // command = append(command, "--bech", bech)
// // }

// // stdout, stderr, err := tn.Exec(ctx, command, nil)
// // if err != nil {
// // return "", fmt.Errorf("failed to show key %q (stderr=%q): %w", name, stderr, err)
// // }
// // pub fn AccountKeyBech32(ctx context.Context, name string)
