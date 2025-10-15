package betfair

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	BettingAPIURL = "https://api.betfair.com/exchange/betting/json-rpc/v1"
)

// Client handles Betfair REST API calls
type Client struct {
	appKey     string
	sessionKey string
	httpClient *http.Client
}

// NewClient creates a new Betfair API client
func NewClient(appKey, sessionKey string) *Client {
	return &Client{
		appKey:     appKey,
		sessionKey: sessionKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// UpdateSessionKey updates the session token (for re-auth)
func (c *Client) UpdateSessionKey(sessionKey string) {
	c.sessionKey = sessionKey
}

// ListMarketCatalogue fetches market metadata for discovery
func (c *Client) ListMarketCatalogue(ctx context.Context, filter MarketFilter, projection []MarketProjection, sort MarketSort, maxResults int) ([]MarketCatalogue, error) {
	params := map[string]interface{}{
		"filter":           filter,
		"marketProjection": projection,
		"sort":             sort,
		"maxResults":       maxResults,
		"locale":           "en",
	}

	resp, err := c.makeBettingAPIRequest(ctx, "listMarketCatalogue", params)
	if err != nil {
		return nil, err
	}

	var results []MarketCatalogue
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}

	if err := json.Unmarshal(resultBytes, &results); err != nil {
		return nil, fmt.Errorf("unmarshal market catalogue: %w", err)
	}

	return results, nil
}

// ListMarketBook fetches live prices for specified markets
func (c *Client) ListMarketBook(ctx context.Context, marketIDs []string) ([]MarketBook, error) {
	params := map[string]interface{}{
		"marketIds": marketIDs,
		"priceProjection": map[string]interface{}{
			"priceData": []string{"EX_BEST_OFFERS", "EX_TRADED"},
		},
	}

	resp, err := c.makeBettingAPIRequest(ctx, "listMarketBook", params)
	if err != nil {
		return nil, err
	}

	var results []MarketBook
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}

	if err := json.Unmarshal(resultBytes, &results); err != nil {
		return nil, fmt.Errorf("unmarshal market book: %w", err)
	}

	return results, nil
}

// makeBettingAPIRequest sends a JSON-RPC request to Betfair Betting API
func (c *Client) makeBettingAPIRequest(ctx context.Context, method string, params interface{}) (*JSONRPCResponse, error) {
	requestPayload := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  fmt.Sprintf("SportsAPING/v1.0/%s", method),
		Params:  params,
		ID:      time.Now().UnixNano(),
	}

	jsonData, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", BettingAPIURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Application", c.appKey)
	req.Header.Set("X-Authentication", c.sessionKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var rpcResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("API error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return &rpcResp, nil
}

