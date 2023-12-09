package cosmos

// TODO: convert all to tn or c CosmosChain? (i think tn so we can chose the server to run it on)

// poad tx distribution [command]
//   fund-community-pool         Funds the community pool with the specified amount
//   fund-validator-rewards-pool Fund the validator rewards pool with the specified amount
//   set-withdraw-addr           change the default withdraw address for rewards associated with an address
//   withdraw-all-rewards        withdraw all delegations rewards for a delegator
//   withdraw-rewards

// poad tx slashing [command]
//   unjail

// poad tx staking
// cancel-unbond    Cancel unbonding delegation and delegate back to the validator
// create-validator create new validator initialized with a self-delegation to it
// delegate         Delegate liquid tokens to a validator
// edit-validator   edit an existing validator account
// redelegate       Redelegate illiquid tokens from one validator to another
// unbond           Unbond shares from a validator

// poad tx circuit
// Available Commands:
//   authorize   Authorize an account to trip the circuit breaker.
//   disable     disable a message from being executed

// poad tx feegrant [command]
//   grant       Grant Fee allowance to an address
//   revoke

// poad tx upgrade [command]
//   cancel-software-upgrade Cancel the current software upgrade proposal
//   software-upgrade

// ---
// TODO:
// - move anything from chain_node to its respective module

// Auth accounts

// poad tx gov [command]
//   cancel-proposal        Cancel governance proposal before the voting period ends. Must be signed by the proposal creator.
//   deposit                Deposit tokens for an active proposal
//   draft-proposal         Generate a draft proposal json file. The generated proposal json contains only one message (skeleton).
//   submit-legacy-proposal Submit a legacy proposal along with an initial deposit
//   submit-proposal        Submit a proposal along with some messages, metadata and deposit
//   vote                   Vote for an active proposal, options: yes/no/no_with_veto/abstain
//   weighted-vote

// poad tx vesting [command]
//   create-periodic-vesting-account Create a new vesting account funded with an allocation of tokens.
//   create-permanent-locked-account Create a new permanently locked account funded with an allocation of tokens.
//   create-vesting-account
