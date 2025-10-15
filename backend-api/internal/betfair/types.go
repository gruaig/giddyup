package betfair

import "time"

// Core API types - minimal subset needed for live prices

type MarketFilter struct {
	EventTypeIds    []string   `json:"eventTypeIds,omitempty"`
	MarketCountries []string   `json:"marketCountries,omitempty"`
	MarketTypeCodes []string   `json:"marketTypeCodes,omitempty"`
	MarketStartTime *TimeRange `json:"marketStartTime,omitempty"`
}

type TimeRange struct {
	From *time.Time `json:"from,omitempty"`
	To   *time.Time `json:"to,omitempty"`
}

type MarketProjection string

const (
	ProjectionEvent             MarketProjection = "EVENT"
	ProjectionRunnerDescription MarketProjection = "RUNNER_DESCRIPTION"
	ProjectionMarketStartTime   MarketProjection = "MARKET_START_TIME"
)

type MarketSort string

const (
	SortFirstToStart MarketSort = "FIRST_TO_START"
)

// MarketCatalogue - metadata about a market
type MarketCatalogue struct {
	MarketID        string          `json:"marketId"`
	MarketName      string          `json:"marketName"`
	MarketStartTime *time.Time      `json:"marketStartTime,omitempty"`
	Event           *Event          `json:"event,omitempty"`
	Runners         []RunnerCatalog `json:"runners,omitempty"`
}

type Event struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	CountryCode string     `json:"countryCode,omitempty"`
	Venue       string     `json:"venue,omitempty"`
	OpenDate    *time.Time `json:"openDate,omitempty"`
}

type RunnerCatalog struct {
	SelectionID int64  `json:"selectionId"`
	RunnerName  string `json:"runnerName"`
	SortPriority int   `json:"sortPriority"`
}

// MarketBook - live prices and volumes
type MarketBook struct {
	MarketID     string       `json:"marketId"`
	Status       string       `json:"status,omitempty"`
	InPlay       bool         `json:"inplay,omitempty"`
	TotalMatched float64      `json:"totalMatched"`
	Runners      []RunnerBook `json:"runners,omitempty"`
}

type RunnerBook struct {
	SelectionID     int64            `json:"selectionId"`
	Status          string           `json:"status"`
	LastPriceTraded *float64         `json:"lastPriceTraded,omitempty"`
	TotalMatched    float64          `json:"totalMatched"`
	EX              *ExchangePrices  `json:"ex,omitempty"`
}

type ExchangePrices struct {
	AvailableToBack []PriceSize `json:"availableToBack,omitempty"`
	AvailableToLay  []PriceSize `json:"availableToLay,omitempty"`
	TradedVolume    []PriceSize `json:"tradedVolume,omitempty"`
}

type PriceSize struct {
	Price float64 `json:"price"`
	Size  float64 `json:"size"`
}

// JSON-RPC types
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int64       `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      int64       `json:"id"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}


