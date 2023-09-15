use cosmwasm_schema::{cw_serde, QueryResponses};

#[cw_serde]
pub struct InstantiateMsg {}

#[cw_serde]
pub struct MigrateMsg {}

#[cw_serde]
pub enum ExecuteMsg {
    ChangeContractOwner {
        new_owner: String,
    },
}

#[cw_serde]
#[derive(QueryResponses)]
pub enum QueryMsg {
    /// Owner returns the owner of the contract. Response: OwnerResponse
    #[returns(OwnerResponse)]
    Owner {},
}

#[cw_serde]
pub struct OwnerResponse {
    pub address: String,
}
