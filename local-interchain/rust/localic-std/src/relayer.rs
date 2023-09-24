use serde_json::Value;

use crate::{errors::LocalError, transactions::ChainRequestBuilder};

#[derive(Clone)]
pub struct Relayer<'a> {
    rb: &'a ChainRequestBuilder,
}

impl Relayer<'_> {
    // TODO: add hermes support
    pub fn new(rb: &ChainRequestBuilder) -> Relayer {
        Relayer { rb }
    }

    pub fn execute(&self, cmd: &str, return_text: bool) -> Result<Value, LocalError> {
        let payload = serde_json::json!({
            "chain_id": &self.rb.chain_id,
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

    pub fn get_channels(&self) -> Result<Value, LocalError> {
        let payload = serde_json::json!({
            "chain_id": &self.rb.chain_id,
            "action": "get_channels",
        });

        let res = self.rb.client.post(&self.rb.api).json(&payload).send();
        return match res {
            Ok(res) => Ok(res.json().unwrap_or_default()),
            Err(e) => Err(LocalError::Custom { msg: e.to_string() }),
        };
    }
}
