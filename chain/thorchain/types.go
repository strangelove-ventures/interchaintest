package thorchain

type VersionOutput struct {
	Version string `json:"version"`
}

type NodeAccountPubKeySet struct {
	Secp256k1 string `json:"secp256k1"`
	Ed25519   string `json:"ed25519"`
}

type NodeAccount struct {
	NodeAddress         string               `json:"node_address"`
	Version             string               `json:"version"`
	IpAddress           string               `json:"ip_address"`
	Status              string               `json:"status"`
	Bond                string               `json:"bond"`
	BondUInt            uint64               `json:"-"`
	ActiveBlockHeight   string               `json:"active_block_height"`
	BondAddress         string               `json:"bond_address"`
	SignerMembership    []string             `json:"signer_membership"`
	ValidatorConsPubKey string               `json:"validator_cons_pub_key"`
	PubKeySet           NodeAccountPubKeySet `json:"pub_key_set"`
}

// ProtoMessage is implemented by generated protocol buffer messages.
// Pulled from github.com/cosmos/gogoproto/proto.
type ProtoMessage interface {
	Reset()
	String() string
	ProtoMessage()
}

type ParamChange struct {
	Subspace string `json:"subspace"`
	Key      string `json:"key"`
	Value    any    `json:"value"`
}

type BuildDependency struct {
	Parent  string `json:"parent"`
	Version string `json:"version"`

	IsReplacement      bool   `json:"is_replacement"`
	Replacement        string `json:"replacement"`
	ReplacementVersion string `json:"replacement_version"`
}

type BinaryBuildInformation struct {
	Name             string            `json:"name"`
	ServerName       string            `json:"server_name"`
	Version          string            `json:"version"`
	Commit           string            `json:"commit"`
	BuildTags        string            `json:"build_tags"`
	Go               string            `json:"go"`
	BuildDeps        []BuildDependency `json:"build_deps"`
	CosmosSdkVersion string            `json:"cosmos_sdk_version"`
}

type BankMetaData struct {
	Metadata struct {
		Description string `json:"description"`
		DenomUnits  []struct {
			Denom    string   `json:"denom"`
			Exponent int      `json:"exponent"`
			Aliases  []string `json:"aliases"`
		} `json:"denom_units"`
		Base    string `json:"base"`
		Display string `json:"display"`
		Name    string `json:"name"`
		Symbol  string `json:"symbol"`
		URI     string `json:"uri"`
		URIHash string `json:"uri_hash"`
	} `json:"metadata"`
}

type QueryDenomAuthorityMetadataResponse struct {
	AuthorityMetadata DenomAuthorityMetadata `protobuf:"bytes,1,opt,name=authority_metadata,json=authorityMetadata,proto3" json:"authority_metadata" yaml:"authority_metadata"`
}

type DenomAuthorityMetadata struct {
	// Can be empty for no admin, or a valid address
	Admin string `protobuf:"bytes,1,opt,name=admin,proto3" json:"admin,omitempty" yaml:"admin"`
}

// thorchain openapi types

// InboundAddress struct for InboundAddress.
type InboundAddress struct {
	Chain   *string `json:"chain,omitempty"`
	PubKey  *string `json:"pub_key,omitempty"`
	Address *string `json:"address,omitempty"`
	Router  *string `json:"router,omitempty"`
	// Returns true if trading is unavailable for this chain, either because trading is halted globally or specifically for this chain
	Halted bool `json:"halted"`
	// Returns true if trading is paused globally
	GlobalTradingPaused *bool `json:"global_trading_paused,omitempty"`
	// Returns true if trading is paused for this chain
	ChainTradingPaused *bool `json:"chain_trading_paused,omitempty"`
	// Returns true if LP actions are paused for this chain
	ChainLpActionsPaused *bool `json:"chain_lp_actions_paused,omitempty"`
	// The minimum fee rate used by vaults to send outbound TXs. The actual fee rate may be higher. For EVM chains this is returned in gwei (1e9).
	GasRate *string `json:"gas_rate,omitempty"`
	// Units of the gas_rate.
	GasRateUnits *string `json:"gas_rate_units,omitempty"`
	// Avg size of outbound TXs on each chain. For UTXO chains it may be larger than average, as it takes into account vault consolidation txs, which can have many vouts
	OutboundTxSize *string `json:"outbound_tx_size,omitempty"`
	// The total outbound fee charged to the user for outbound txs in the gas asset of the chain.
	OutboundFee *string `json:"outbound_fee,omitempty"`
	// Defines the minimum transaction size for the chain in base units (sats, wei, uatom). Transactions with asset amounts lower than the dust_threshold are ignored.
	DustThreshold *string `json:"dust_threshold,omitempty"`
}

// LiquidityProvider struct for LiquidityProvider.
type LiquidityProvider struct {
	Asset              string  `json:"asset"`
	RuneAddress        *string `json:"rune_address,omitempty"`
	AssetAddress       *string `json:"asset_address,omitempty"`
	LastAddHeight      *int64  `json:"last_add_height,omitempty"`
	LastWithdrawHeight *int64  `json:"last_withdraw_height,omitempty"`
	Units              string  `json:"units"`
	PendingRune        string  `json:"pending_rune"`
	PendingAsset       string  `json:"pending_asset"`
	PendingTxId        *string `json:"pending_tx_id,omitempty"`
	RuneDepositValue   string  `json:"rune_deposit_value"`
	AssetDepositValue  string  `json:"asset_deposit_value"`
	RuneRedeemValue    *string `json:"rune_redeem_value,omitempty"`
	AssetRedeemValue   *string `json:"asset_redeem_value,omitempty"`
	LuviDepositValue   *string `json:"luvi_deposit_value,omitempty"`
	LuviRedeemValue    *string `json:"luvi_redeem_value,omitempty"`
	LuviGrowthPct      *string `json:"luvi_growth_pct,omitempty"`
}

// Saver struct for Saver.
type Saver struct {
	Asset              string `json:"asset"`
	AssetAddress       string `json:"asset_address"`
	LastAddHeight      *int64 `json:"last_add_height,omitempty"`
	LastWithdrawHeight *int64 `json:"last_withdraw_height,omitempty"`
	Units              string `json:"units"`
	AssetDepositValue  string `json:"asset_deposit_value"`
	AssetRedeemValue   string `json:"asset_redeem_value"`
	GrowthPct          string `json:"growth_pct"`
}

// Pool struct for Pool.
type Pool struct {
	Asset               string  `json:"asset"`
	ShortCode           *string `json:"short_code,omitempty"`
	Status              string  `json:"status"`
	Decimals            *int64  `json:"decimals,omitempty"`
	PendingInboundAsset string  `json:"pending_inbound_asset"`
	PendingInboundRune  string  `json:"pending_inbound_rune"`
	BalanceAsset        string  `json:"balance_asset"`
	BalanceRune         string  `json:"balance_rune"`
	// the USD (TOR) price of the asset in 1e8
	AssetTorPrice string `json:"asset_tor_price"`
	// the total pool units, this is the sum of LP and synth units
	PoolUnits string `json:"pool_units"`
	// the total pool liquidity provider units
	LPUnits string `json:"LP_units"`
	// the total synth units in the pool
	SynthUnits string `json:"synth_units"`
	// the total supply of synths for the asset
	SynthSupply string `json:"synth_supply"`
	// the balance of L1 asset deposited into the Savers Vault
	SaversDepth string `json:"savers_depth"`
	// the number of units owned by Savers
	SaversUnits string `json:"savers_units"`
	// the filled savers capacity in basis points, 4500/10000 = 45%
	SaversFillBps string `json:"savers_fill_bps"`
	// amount of remaining capacity in asset
	SaversCapacityRemaining string `json:"savers_capacity_remaining"`
	// whether additional synths cannot be minted
	SynthMintPaused bool `json:"synth_mint_paused"`
	// the amount of synth supply remaining before the current max supply is reached
	SynthSupplyRemaining string `json:"synth_supply_remaining"`
	// the amount of collateral collects for loans
	LoanCollateral string `json:"loan_collateral"`
	// the amount of remaining collateral collects for loans
	LoanCollateralRemaining string `json:"loan_collateral_remaining"`
	// the current loan collateralization ratio
	LoanCr string `json:"loan_cr"`
	// the depth of the derived virtual pool relative to L1 pool (in basis points)
	DerivedDepthBps string `json:"derived_depth_bps"`
}

// QuoteFees struct for QuoteFees.
type QuoteFees struct {
	// the target asset used for all fees
	Asset string `json:"asset"`
	// affiliate fee in the target asset
	Affiliate *string `json:"affiliate,omitempty"`
	// outbound fee in the target asset
	Outbound *string `json:"outbound,omitempty"`
	// liquidity fees paid to pools in the target asset
	Liquidity string `json:"liquidity"`
	// total fees in the target asset
	Total string `json:"total"`
	// the swap slippage in basis points
	SlippageBps int64 `json:"slippage_bps"`
	// total basis points in fees relative to amount out
	TotalBps int64 `json:"total_bps"`
}

// QuoteSwapResponse struct for QuoteSwapResponse.
type QuoteSwapResponse struct {
	// the inbound address for the transaction on the source chain
	InboundAddress *string `json:"inbound_address,omitempty"`
	// the approximate number of source chain blocks required before processing
	InboundConfirmationBlocks *int64 `json:"inbound_confirmation_blocks,omitempty"`
	// the approximate seconds for block confirmations required before processing
	InboundConfirmationSeconds *int64 `json:"inbound_confirmation_seconds,omitempty"`
	// the number of thorchain blocks the outbound will be delayed
	OutboundDelayBlocks int64 `json:"outbound_delay_blocks"`
	// the approximate seconds for the outbound delay before it will be sent
	OutboundDelaySeconds int64     `json:"outbound_delay_seconds"`
	Fees                 QuoteFees `json:"fees"`
	// the EVM chain router contract address
	Router *string `json:"router,omitempty"`
	// expiration timestamp in unix seconds
	Expiry int64 `json:"expiry"`
	// static warning message
	Warning string `json:"warning"`
	// chain specific quote notes
	Notes string `json:"notes"`
	// Defines the minimum transaction size for the chain in base units (sats, wei, uatom). Transactions with asset amounts lower than the dust_threshold are ignored.
	DustThreshold *string `json:"dust_threshold,omitempty"`
	// The recommended minimum inbound amount for this transaction type & inbound asset. Sending less than this amount could result in failed refunds.
	RecommendedMinAmountIn *string `json:"recommended_min_amount_in,omitempty"`
	// the recommended gas rate to use for the inbound to ensure timely confirmation
	RecommendedGasRate *string `json:"recommended_gas_rate,omitempty"`
	// the units of the recommended gas rate
	GasRateUnits *string `json:"gas_rate_units,omitempty"`
	// generated memo for the swap
	Memo *string `json:"memo,omitempty"`
	// the amount of the target asset the user can expect to receive after fees
	ExpectedAmountOut string `json:"expected_amount_out"`
	// the maximum amount of trades a streaming swap can do for a trade
	MaxStreamingQuantity *int64 `json:"max_streaming_quantity,omitempty"`
	// the number of blocks the streaming swap will execute over
	StreamingSwapBlocks *int64 `json:"streaming_swap_blocks,omitempty"`
	// approx the number of seconds the streaming swap will execute over
	StreamingSwapSeconds *int64 `json:"streaming_swap_seconds,omitempty"`
	// total number of seconds a swap is expected to take (inbound conf + streaming swap + outbound delay)
	TotalSwapSeconds *int64 `json:"total_swap_seconds,omitempty"`
}

// QuoteSaverDepositResponse struct for QuoteSaverDepositResponse.
type QuoteSaverDepositResponse struct {
	// the inbound address for the transaction on the source chain
	InboundAddress string `json:"inbound_address"`
	// the approximate number of source chain blocks required before processing
	InboundConfirmationBlocks *int64 `json:"inbound_confirmation_blocks,omitempty"`
	// the approximate seconds for block confirmations required before processing
	InboundConfirmationSeconds *int64 `json:"inbound_confirmation_seconds,omitempty"`
	// the number of thorchain blocks the outbound will be delayed
	OutboundDelayBlocks *int64 `json:"outbound_delay_blocks,omitempty"`
	// the approximate seconds for the outbound delay before it will be sent
	OutboundDelaySeconds *int64    `json:"outbound_delay_seconds,omitempty"`
	Fees                 QuoteFees `json:"fees"`
	// the EVM chain router contract address
	Router *string `json:"router,omitempty"`
	// expiration timestamp in unix seconds
	Expiry int64 `json:"expiry"`
	// static warning message
	Warning string `json:"warning"`
	// chain specific quote notes
	Notes string `json:"notes"`
	// Defines the minimum transaction size for the chain in base units (sats, wei, uatom). Transactions with asset amounts lower than the dust_threshold are ignored.
	DustThreshold *string `json:"dust_threshold,omitempty"`
	// The recommended minimum inbound amount for this transaction type & inbound asset. Sending less than this amount could result in failed refunds.
	RecommendedMinAmountIn *string `json:"recommended_min_amount_in,omitempty"`
	// the recommended gas rate to use for the inbound to ensure timely confirmation
	RecommendedGasRate string `json:"recommended_gas_rate"`
	// the units of the recommended gas rate
	GasRateUnits string `json:"gas_rate_units"`
	// generated memo for the deposit
	Memo string `json:"memo"`
	// same as expected_amount_deposit, to be deprecated in favour of expected_amount_deposit
	ExpectedAmountOut *string `json:"expected_amount_out,omitempty"`
	// the amount of the target asset the user can expect to deposit after fees
	ExpectedAmountDeposit string `json:"expected_amount_deposit"`
}

// InboundObservedStage struct for InboundObservedStage.
type InboundObservedStage struct {
	// returns true if any nodes have observed the transaction (to be deprecated in favour of counts)
	Started *bool `json:"started,omitempty"`
	// number of signers for pre-confirmation-counting observations
	PreConfirmationCount *int64 `json:"pre_confirmation_count,omitempty"`
	// number of signers for final observations, after any confirmation counting complete
	FinalCount int64 `json:"final_count"`
	// returns true if no transaction observation remains to be done
	Completed bool `json:"completed"`
}

// InboundConfirmationCountedStage struct for InboundConfirmationCountedStage.
type InboundConfirmationCountedStage struct {
	// the THORChain block height when confirmation counting began
	CountingStartHeight *int64 `json:"counting_start_height,omitempty"`
	// the external source chain for which confirmation counting takes place
	Chain *string `json:"chain,omitempty"`
	// the block height on the external source chain when the transaction was observed
	ExternalObservedHeight *int64 `json:"external_observed_height,omitempty"`
	// the block height on the external source chain when confirmation counting will be complete
	ExternalConfirmationDelayHeight *int64 `json:"external_confirmation_delay_height,omitempty"`
	// the estimated remaining seconds before confirmation counting completes
	RemainingConfirmationSeconds *int64 `json:"remaining_confirmation_seconds,omitempty"`
	// returns true if no transaction confirmation counting remains to be done
	Completed bool `json:"completed"`
}

// InboundFinalisedStage struct for InboundFinalisedStage.
type InboundFinalisedStage struct {
	// returns true if the inbound transaction has been finalised (THORChain agreeing it exists)
	Completed bool `json:"completed"`
}

// StreamingStatus struct for StreamingStatus.
type StreamingStatus struct {
	// how often each swap is made, in blocks
	Interval int64 `json:"interval"`
	// the total number of swaps in a streaming swaps
	Quantity int64 `json:"quantity"`
	// the amount of swap attempts so far
	Count int64 `json:"count"`
}

// SwapStatus struct for SwapStatus.
type SwapStatus struct {
	// true when awaiting a swap
	Pending   bool             `json:"pending"`
	Streaming *StreamingStatus `json:"streaming,omitempty"`
}

// SwapFinalisedStage struct for SwapFinalisedStage.
type SwapFinalisedStage struct {
	// (to be deprecated in favor of swap_status) returns true if an inbound transaction's swap (successful or refunded) is no longer pending
	Completed bool `json:"completed"`
}

// OutboundDelayStage struct for OutboundDelayStage.
type OutboundDelayStage struct {
	// the number of remaining THORChain blocks the outbound will be delayed
	RemainingDelayBlocks *int64 `json:"remaining_delay_blocks,omitempty"`
	// the estimated remaining seconds of the outbound delay before it will be sent
	RemainingDelaySeconds *int64 `json:"remaining_delay_seconds,omitempty"`
	// returns true if no transaction outbound delay remains
	Completed bool `json:"completed"`
}

// OutboundSignedStage struct for OutboundSignedStage.
type OutboundSignedStage struct {
	// THORChain height for which the external outbound is scheduled
	ScheduledOutboundHeight *int64 `json:"scheduled_outbound_height,omitempty"`
	// THORChain blocks since the scheduled outbound height
	BlocksSinceScheduled *int64 `json:"blocks_since_scheduled,omitempty"`
	// returns true if an external transaction has been signed and broadcast (and observed in its mempool)
	Completed bool `json:"completed"`
}

// TxStagesResponse struct for TxStagesResponse.
type TxStagesResponse struct {
	InboundObserved            InboundObservedStage             `json:"inbound_observed"`
	InboundConfirmationCounted *InboundConfirmationCountedStage `json:"inbound_confirmation_counted,omitempty"`
	InboundFinalised           *InboundFinalisedStage           `json:"inbound_finalised,omitempty"`
	SwapStatus                 *SwapStatus                      `json:"swap_status,omitempty"`
	SwapFinalised              *SwapFinalisedStage              `json:"swap_finalised,omitempty"`
	OutboundDelay              *OutboundDelayStage              `json:"outbound_delay,omitempty"`
	OutboundSigned             *OutboundSignedStage             `json:"outbound_signed,omitempty"`
}

// Coin struct for Coin.
type Coin struct {
	Asset    string `json:"asset"`
	Amount   string `json:"amount"`
	Decimals *int64 `json:"decimals,omitempty"`
}

// Tx struct for Tx.
type Tx struct {
	Id          *string `json:"id,omitempty"`
	Chain       *string `json:"chain,omitempty"`
	FromAddress *string `json:"from_address,omitempty"`
	ToAddress   *string `json:"to_address,omitempty"`
	Coins       []Coin  `json:"coins"`
	Gas         []Coin  `json:"gas"`
	Memo        *string `json:"memo,omitempty"`
}

// ObservedTx struct for ObservedTx.
type ObservedTx struct {
	Tx             Tx      `json:"tx"`
	ObservedPubKey *string `json:"observed_pub_key,omitempty"`
	// the block height on the external source chain when the transaction was observed, not provided if chain is THOR
	ExternalObservedHeight *int64 `json:"external_observed_height,omitempty"`
	// the block height on the external source chain when confirmation counting will be complete, not provided if chain is THOR
	ExternalConfirmationDelayHeight *int64 `json:"external_confirmation_delay_height,omitempty"`
	// the outbound aggregator to use, will also match a suffix
	Aggregator *string `json:"aggregator,omitempty"`
	// the aggregator target asset provided to transferOutAndCall
	AggregatorTarget *string `json:"aggregator_target,omitempty"`
	// the aggregator target asset limit provided to transferOutAndCall
	AggregatorTargetLimit *string  `json:"aggregator_target_limit,omitempty"`
	Signers               []string `json:"signers,omitempty"`
	KeysignMs             *int64   `json:"keysign_ms,omitempty"`
	OutHashes             []string `json:"out_hashes,omitempty"`
	Status                *string  `json:"status,omitempty"`
}

// TxOutItem struct for TxOutItem.
type TxOutItem struct {
	Chain       string  `json:"chain"`
	ToAddress   string  `json:"to_address"`
	VaultPubKey *string `json:"vault_pub_key,omitempty"`
	Coin        Coin    `json:"coin"`
	Memo        *string `json:"memo,omitempty"`
	MaxGas      []Coin  `json:"max_gas"`
	GasRate     *int64  `json:"gas_rate,omitempty"`
	InHash      *string `json:"in_hash,omitempty"`
	OutHash     *string `json:"out_hash,omitempty"`
	Height      *int64  `json:"height,omitempty"`
	// clout spent in RUNE for the outbound
	CloutSpent *string `json:"clout_spent,omitempty"`
}

// TxDetailsResponse struct for TxDetailsResponse.
type TxDetailsResponse struct {
	TxId    *string      `json:"tx_id,omitempty"`
	Tx      ObservedTx   `json:"tx"`
	Txs     []ObservedTx `json:"txs"`
	Actions []TxOutItem  `json:"actions"`
	OutTxs  []Tx         `json:"out_txs"`
	// the thorchain height at which the inbound reached consensus
	ConsensusHeight *int64 `json:"consensus_height,omitempty"`
	// the thorchain height at which the outbound was finalised
	FinalisedHeight *int64 `json:"finalised_height,omitempty"`
	UpdatedVault    *bool  `json:"updated_vault,omitempty"`
	Reverted        *bool  `json:"reverted,omitempty"`
	// the thorchain height for which the outbound was scheduled
	OutboundHeight *int64 `json:"outbound_height,omitempty"`
}
