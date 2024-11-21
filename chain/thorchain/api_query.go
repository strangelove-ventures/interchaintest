package thorchain

// Api queries for thorchain
// Copied from thorchain's simulation test, may be replaced in the future for queries via chain binary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	sdkmath "cosmossdk.io/math"

	"github.com/strangelove-ventures/interchaintest/v8/chain/thorchain/common"
)

// Generic query for routes not yet supported here.
func (c *Thorchain) ApiQuery(ctx context.Context, path string, args ...string) (any, error) {
	url := fmt.Sprintf("%s/%s", c.GetAPIAddress(), path)
	var res any
	err := get(ctx, url, &res)
	return res, err
}

func (c *Thorchain) ApiGetNode(ctx context.Context, addr string) (OpenapiNode, error) {
	url := fmt.Sprintf("%s/thorchain/node/%s", c.GetAPIAddress(), addr)
	var node OpenapiNode
	err := get(ctx, url, &node)
	return node, err
}

func (c *Thorchain) ApiGetNodes(ctx context.Context) ([]OpenapiNode, error) {
	url := fmt.Sprintf("%s/thorchain/nodes", c.GetAPIAddress())
	var nodes []OpenapiNode
	err := get(ctx, url, &nodes)
	return nodes, err
}

func (c *Thorchain) APIGetBalances(ctx context.Context, addr string) (common.Coins, error) {
	url := fmt.Sprintf("%s/cosmos/bank/v1beta1/balances/%s", c.GetAPIAddress(), addr)
	var balances struct {
		Balances []struct {
			Denom  string `json:"denom"`
			Amount string `json:"amount"`
		} `json:"balances"`
	}
	err := get(ctx, url, &balances)
	if err != nil {
		return nil, err
	}

	// convert to common.Coins
	coins := make(common.Coins, 0, len(balances.Balances))
	for _, balance := range balances.Balances {
		var amount uint64
		amount, err = strconv.ParseUint(balance.Amount, 10, 64)
		if err != nil {
			return nil, err
		}
		var asset common.Asset
		asset, err = common.NewAsset(strings.ToUpper(balance.Denom))
		if err != nil {
			return nil, err
		}
		coins = append(coins, common.NewCoin(asset, sdkmath.NewUint(amount)))
	}

	return coins, nil
}

func (c *Thorchain) APIGetInboundAddress(ctx context.Context, chain string) (address string, router *string, err error) {
	url := fmt.Sprintf("%s/thorchain/inbound_addresses", c.GetAPIAddress())
	var inboundAddresses []InboundAddress
	err = get(ctx, url, &inboundAddresses)
	if err != nil {
		return "", nil, err
	}

	// find address for chain
	for _, inboundAddress := range inboundAddresses {
		if *inboundAddress.Chain == chain {
			if inboundAddress.Router != nil {
				router = new(string)
				*router = *inboundAddress.Router
			}
			return *inboundAddress.Address, router, nil
		}
	}

	return "", nil, fmt.Errorf("no inbound address found for chain %s", chain)
}

func (c *Thorchain) APIGetRouterAddress(ctx context.Context, chain string) (string, error) {
	url := fmt.Sprintf("%s/thorchain/inbound_addresses", c.GetAPIAddress())
	var inboundAddresses []InboundAddress
	err := get(ctx, url, &inboundAddresses)
	if err != nil {
		return "", err
	}

	// find address for chain
	for _, inboundAddress := range inboundAddresses {
		if *inboundAddress.Chain == chain {
			return *inboundAddress.Router, nil
		}
	}

	return "", fmt.Errorf("no inbound address found for chain %s", chain)
}

func (c *Thorchain) APIGetLiquidityProviders(ctx context.Context, asset common.Asset) ([]LiquidityProvider, error) {
	url := fmt.Sprintf("%s/thorchain/pool/%s/liquidity_providers", c.GetAPIAddress(), asset.String())
	var liquidityProviders []LiquidityProvider
	err := get(ctx, url, &liquidityProviders)
	return liquidityProviders, err
}

func (c *Thorchain) APIGetSavers(ctx context.Context, asset common.Asset) ([]Saver, error) {
	url := fmt.Sprintf("%s/thorchain/pool/%s/savers", c.GetAPIAddress(), asset.GetLayer1Asset().String())
	var savers []Saver
	err := get(ctx, url, &savers)
	return savers, err
}

func (c *Thorchain) APIGetPools(ctx context.Context) ([]Pool, error) {
	url := fmt.Sprintf("%s/thorchain/pools", c.GetAPIAddress())
	var pools []Pool
	err := get(ctx, url, &pools)
	return pools, err
}

func (c *Thorchain) APIGetPool(ctx context.Context, asset common.Asset) (Pool, error) {
	url := fmt.Sprintf("%s/thorchain/pool/%s", c.GetAPIAddress(), asset.String())
	var pool Pool
	err := get(ctx, url, &pool)
	return pool, err
}

func (c *Thorchain) APIGetSwapQuote(ctx context.Context, from, to common.Asset, amount sdkmath.Uint) (QuoteSwapResponse, error) {
	baseURL := fmt.Sprintf("%s/thorchain/quote/swap", c.GetAPIAddress())
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return QuoteSwapResponse{}, err
	}
	params := url.Values{}
	params.Add("from_asset", from.String())
	params.Add("to_asset", to.String())
	params.Add("amount", amount.String())
	parsedURL.RawQuery = params.Encode()
	url := parsedURL.String()

	var quote QuoteSwapResponse
	err = get(ctx, url, &quote)
	return quote, err
}

func (c *Thorchain) APIGetSaverDepositQuote(ctx context.Context, asset common.Asset, amount sdkmath.Uint) (QuoteSaverDepositResponse, error) {
	baseURL := fmt.Sprintf("%s/thorchain/quote/saver/deposit", c.GetAPIAddress())
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return QuoteSaverDepositResponse{}, err
	}
	params := url.Values{}
	params.Add("asset", asset.String())
	params.Add("amount", amount.String())
	parsedURL.RawQuery = params.Encode()
	url := parsedURL.String()

	var quote QuoteSaverDepositResponse
	err = get(ctx, url, &quote)
	return quote, err
}

func (c *Thorchain) APIGetTxStages(ctx context.Context, txid string) (TxStagesResponse, error) {
	url := fmt.Sprintf("%s/thorchain/tx/stages/%s", c.GetAPIAddress(), txid)
	var stages TxStagesResponse
	err := get(ctx, url, &stages)
	return stages, err
}

func (c *Thorchain) APIGetTxDetails(ctx context.Context, txid string) (TxDetailsResponse, error) {
	url := fmt.Sprintf("%s/thorchain/tx/details/%s", c.GetAPIAddress(), txid)
	var details TxDetailsResponse
	err := get(ctx, url, &details)
	return details, err
}

func (c *Thorchain) APIGetMimirs(ctx context.Context) (map[string]int64, error) {
	url := fmt.Sprintf("%s/thorchain/mimir", c.GetAPIAddress())
	var mimirs map[string]int64
	err := get(ctx, url, &mimirs)
	return mimirs, err
}

////////////////////////////////////////////////////////////////////////////////////////
// Internal
////////////////////////////////////////////////////////////////////////////////////////

func get(ctx context.Context, url string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// extract error if the request failed
	type ErrorResponse struct {
		Error string `json:"error"`
	}
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	errResp := ErrorResponse{}
	err = json.Unmarshal(buf, &errResp)
	if err == nil && errResp.Error != "" {
		return errors.New(errResp.Error)
	}

	// decode response
	return json.Unmarshal(buf, target)
}

// ConvertAssetAmount converts the given coin to the target asset and returns the amount.
func (c *Thorchain) ConvertAssetAmount(ctx context.Context, coin Coin, asset string) (sdkmath.Uint, error) {
	pools, err := c.APIGetPools(ctx)
	if err != nil {
		return sdkmath.ZeroUint(), err
	}

	// find pools for the conversion rate
	var sourcePool, targetPool Pool
	for _, pool := range pools {
		if pool.Asset == coin.Asset {
			sourcePool = pool
		}
		if pool.Asset == asset {
			targetPool = pool
		}
	}

	// ensure we found both pools
	if sourcePool.Asset == "" {
		return sdkmath.ZeroUint(), fmt.Errorf("source asset not found")
	}
	if targetPool.Asset == "" {
		return sdkmath.ZeroUint(), fmt.Errorf("target asset not found")
	}

	// convert the amount
	converted := sdkmath.NewUintFromString(coin.Amount).
		Mul(sdkmath.NewUintFromString(sourcePool.BalanceRune)).
		Quo(sdkmath.NewUintFromString(sourcePool.BalanceAsset)).
		Mul(sdkmath.NewUintFromString(targetPool.BalanceAsset)).
		Quo(sdkmath.NewUintFromString(targetPool.BalanceRune))

	return converted, nil
}
