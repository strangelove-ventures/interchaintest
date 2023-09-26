use serde_json::Value;

use crate::transactions::ChainRequestBuilder;

#[must_use]
pub fn get_proposal(req_builder: &ChainRequestBuilder, proposal_id: u64) -> Value {
    let cmd = format!("q gov proposal {proposal_id} --output=json");
    req_builder.query(cmd.as_str(), false)
}
