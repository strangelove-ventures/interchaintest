use crate::{
    error::ContractError,
    msg::{ExecuteMsg, InstantiateMsg, MigrateMsg, QueryMsg, OwnerResponse},
    state::OWNER,
};
use cosmwasm_std::{
    entry_point,
    to_binary, Addr, Binary, Deps, DepsMut, Env, MessageInfo, Response, StdResult,
};

#[entry_point]
pub fn instantiate(
    deps: DepsMut,
    _env: Env,
    info: MessageInfo,
    _msg: InstantiateMsg,
) -> Result<Response, ContractError> {
    OWNER.save(deps.storage, &info.sender)?;

    Ok(Response::new()
        .add_attribute("action", "instantiate")
        .add_attribute("owner", info.sender))
}

/// Allow contract to be able to migrate if admin is set.
/// This provides option for migration, if admin is not set, this functionality will be disabled
#[entry_point]
pub fn migrate(_deps: DepsMut, _env: Env, _msg: MigrateMsg) -> Result<Response, ContractError> {
    Ok(Response::new().add_attribute("action", "migrate"))
}

#[entry_point]
pub fn execute(
    deps: DepsMut,
    _env: Env,
    info: MessageInfo,
    msg: ExecuteMsg,
) -> Result<Response, ContractError> {
    match msg {
        // Admin functions
        ExecuteMsg::ChangeContractOwner { new_owner } =>
            change_contract_owner(deps, info, new_owner),
    }
}

fn change_contract_owner(
    deps: DepsMut,
    info: MessageInfo,
    new_owner: String,
) -> Result<Response, ContractError> {
    // Only allow current contract owner to change owner
    check_is_contract_owner(deps.as_ref(), info.sender)?;

    // validate that new owner is a valid address
    let new_owner_addr = deps.api.addr_validate(&new_owner)?;

    // update the contract owner in the contract config
    OWNER.save(deps.storage, &new_owner_addr)?;

    // return OK
    Ok(Response::new()
        .add_attribute("action", "change_contract_owner")
        .add_attribute("new_owner", new_owner))
}

fn check_is_contract_owner(deps: Deps, sender: Addr) -> Result<(), ContractError> {
    let owner = OWNER.load(deps.storage)?;
    if owner != sender {
        Err(ContractError::Unauthorized {})
    } else {
        Ok(())
    }
}

#[entry_point]
pub fn query(deps: Deps, _env: Env, msg: QueryMsg) -> StdResult<Binary> {
    match msg {
        QueryMsg::Owner {} => to_binary(&query_owner(deps)?),
    }
}

fn query_owner(deps: Deps) -> StdResult<OwnerResponse> {
    let owner = OWNER.load(deps.storage)?;
    Ok(OwnerResponse {
        address: owner.to_string(),
    })
}
