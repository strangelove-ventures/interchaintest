use serde_json::Value;

use crate::{errors::LocalError, transactions::ChainRequestBuilder};

#[derive(Clone)]
pub struct Relayer<'a> {
    rb: &'a ChainRequestBuilder,
}

#[derive(Clone, Debug)]
pub struct Channel {
    pub channel_id: String,
    pub connection_hops: Vec<String>,
    pub counterparty: Counterparty,
    pub ordering: String,
    pub port_id: String,
    pub state: String,
    pub version: String,
}

#[derive(Clone, Debug)]
pub struct Counterparty {
    pub channel_id: String,
    pub port_id: String,
}

impl Relayer<'_> {
    // TODO: add hermes support
    pub fn new(rb: &ChainRequestBuilder) -> Relayer {
        Relayer { rb }
    }

    pub fn execute(&self, cmd: &str, return_text: bool) -> Result<Value, LocalError> {
        let payload = serde_json::json!({
            "chain_id": self.rb.chain_id,
            "action": "relayer-exec",
            "cmd": cmd,
        });

        let res = self
            .rb
            .client
            .post(&self.rb.api)
            .json(&payload)
            .header("Accept", "Content-Type: application/json")
            .header("Content-Type", "Content-Type: application/json")
            .send();

        println!("relayer execute res: {:?}", res);

        if return_text {
            return Ok(serde_json::json!({
                "text": res.unwrap().text().unwrap(),
            }));
        }

        return match res {
            Ok(res) => Ok(res.json().unwrap_or_default()),
            Err(e) => Err(LocalError::Custom { msg: e.to_string() }),
        };
    }

    pub fn flush(&self, path: &str, channel: &str) -> Result<Value, LocalError> {
        let cmd = format!("rly transact flush {path} {channel}");
        let res = self.execute(cmd.as_str(), false);
        res
    }

    pub fn create_wasm_connection(
        &self,
        path: &str,
        src: &str,
        dst: &str,
        order: &str,
        version: &str,
    ) -> Result<Value, LocalError> {
        let mut source: String = src.to_string();
        let mut destination: String = dst.to_string();

        if !src.starts_with("wasm.") {
            source = format!("wasm.{}", source);
        }
        if !destination.starts_with("wasm.") {
            destination = format!("wasm.{}", destination);
        }

        let cmd = format!(
            "rly tx channel {path} --src-port {source} --dst-port {destination} --order {order} --version {version}",
        );

        println!("create_wasm_connection cmd: {}", cmd);

        let res = self.execute(cmd.as_str(), false);
        println!("create_wasm_connection res: {res:?}");
        res
    }

    pub fn get_channels(&self, chain_id: String) -> Result<Vec<Channel>, LocalError> {
        let payload = serde_json::json!({
            "chain_id": chain_id,
            "action": "get_channels",
        });

        let res = self.rb.client.post(&self.rb.api).json(&payload).send();
        if res.is_err() {
            return Err(LocalError::Custom {
                msg: res.unwrap_err().to_string(),
            });
        }

        let channel_json: Value = res.unwrap().json().unwrap_or_default();

        let mut channels: Vec<Channel> = Vec::new();
        for channel in channel_json.as_array().unwrap() {
            let channel_id = channel["channel_id"].as_str().unwrap().to_string();
            let connection_hops = channel["connection_hops"].as_array().unwrap();
            let mut hops: Vec<String> = Vec::new();
            for hop in connection_hops {
                hops.push(hop.as_str().unwrap().to_string());
            }
            let counterparty = channel["counterparty"].as_object().unwrap();
            let counterparty_channel_id = counterparty["channel_id"].as_str().unwrap().to_string();
            let counterparty_port_id = counterparty["port_id"].as_str().unwrap().to_string();
            let counterparty = Counterparty {
                channel_id: counterparty_channel_id,
                port_id: counterparty_port_id,
            };
            let ordering = channel["ordering"].as_str().unwrap().to_string();
            let port_id = channel["port_id"].as_str().unwrap().to_string();
            let state = channel["state"].as_str().unwrap().to_string();
            let version = channel["version"].as_str().unwrap().to_string();
            let channel = Channel {
                channel_id,
                connection_hops: hops,
                counterparty,
                ordering,
                port_id,
                state,
                version,
            };
            channels.push(channel);
        }

        Ok(channels)
    }
}