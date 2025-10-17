package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

// Betfair credentials (from your tennis bot)
const (
	BETFAIR_USERNAME  = "colfish"
	BETFAIR_PASSWORD  = "Perlisagod$$1"
	BETFAIR_APP_KEY   = "Gs1Zut6sZQxncj6V"
	BETFAIR_API_URL   = "https://api.betfair.com/exchange/betting/json-rpc/v1"
	BETFAIR_LOGIN_URL = "https://identitysso.betfair.com/api/login"
)

func main() {
	targetDate := flag.String("date", time.Now().AddDate(0, 0, 1).Format("2006-01-02"), "Target date")
	continuous := flag.Bool("continuous", false, "Run continuously (every 30 mins)")
	intervalMins := flag.Int("interval", 30, "Update interval in minutes")
	flag.Parse()

	log.Println("🏇 Horse Racing Price Updater (Betfair)")
	log.Printf("📅 Target: %s", *targetDate)
	if *continuous {
		log.Printf("🔄 Mode: Continuous (every %d mins)", *intervalMins)
	} else {
		log.Println("⚡ Mode: Single run")
	}
	log.Println()

	// Connect to database
	dbURL := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", "password"),
		getEnv("DB_NAME", "horse_db"))

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("❌ Database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Database ping: %v", err)
	}
	log.Println("✅ Database connected")

	updater := &PriceUpdater{
		db:         db,
		targetDate: *targetDate,
	}

	// Set up graceful shutdown
	if *continuous {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigChan
			log.Println("\n⚠️  Shutdown signal received...")
			cancel()
		}()

		// Run continuous loop
		updater.RunContinuous(ctx, time.Duration(*intervalMins)*time.Minute)
	} else {
		// Single run
		if err := updater.UpdatePrices(); err != nil {
			log.Fatalf("❌ Update failed: %v", err)
		}
	}
}

type PriceUpdater struct {
	db         *sql.DB
	targetDate string
}

func (u *PriceUpdater) RunContinuous(ctx context.Context, interval time.Duration) {
	log.Println("🔄 Starting continuous updates...")
	log.Println("   Press Ctrl+C to stop")
	log.Println()

	// Run immediately
	if err := u.UpdatePrices(); err != nil {
		log.Printf("⚠️  Initial update failed: %v", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("✅ Stopped gracefully")
			return
		case <-ticker.C:
			// Re-login each cycle (like your tennis bot)
			if err := u.UpdatePrices(); err != nil {
				log.Printf("⚠️  Update failed: %v", err)
			}
		}
	}
}

func (u *PriceUpdater) UpdatePrices() error {
	startTime := time.Now()
	log.Printf("═══════════════════════════════════════════════════════════")
	log.Printf("🔄 Update cycle at %s", startTime.Format("15:04:05"))
	log.Printf("═══════════════════════════════════════════════════════════")

	// Step 1: Login to Betfair (fresh each time like tennis bot)
	log.Println("🔐 Logging in to Betfair...")
	sessionToken, err := loginInteractive()
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	log.Println("✅ Logged in")

	// Step 2: Discover horse racing markets for target date
	log.Printf("🔍 Discovering markets for %s...", u.targetDate)
	markets, err := fetchHorseRacingMarkets(sessionToken, u.targetDate)
	if err != nil {
		return fmt.Errorf("fetch markets: %w", err)
	}
	log.Printf("✅ Found %d horse racing markets", len(markets))

	if len(markets) == 0 {
		log.Println("⚠️  No markets found - might be too early")
		return nil
	}

	// Step 3: Get market IDs
	marketIDs := make([]string, 0, len(markets))
	for _, m := range markets {
		marketIDs = append(marketIDs, m.MarketID)
	}

	// Step 4: Fetch prices (market books) - in batches of 10 to avoid rate limits
	log.Printf("💰 Fetching prices for %d markets (in batches)...", len(marketIDs))

	var allMarketBooks []MarketBook
	batchSize := 10

	for i := 0; i < len(marketIDs); i += batchSize {
		end := i + batchSize
		if end > len(marketIDs) {
			end = len(marketIDs)
		}

		batch := marketIDs[i:end]
		books, err := fetchMarketBooks(sessionToken, batch)
		if err != nil {
			log.Printf("⚠️  Batch %d-%d failed: %v", i, end, err)
			continue // Skip failed batch, continue with others
		}

		allMarketBooks = append(allMarketBooks, books...)
		time.Sleep(200 * time.Millisecond) // Polite throttle between batches
	}

	log.Printf("✅ Got %d market books with prices", len(allMarketBooks))

	if len(allMarketBooks) == 0 {
		log.Println("⚠️  No market books returned - markets might not have prices yet")
		log.Println("   This is normal if > 12 hours before races")
		return nil
	}

	marketBooks := allMarketBooks

	// Step 5: Update database
	totalUpdates := 0
	for _, book := range marketBooks {
		for _, runner := range book.Runners {
			// Get best back price (like tennis bot: runner.ex.available_to_back[0].price)
			price := extractBestBackPrice(runner)
			if price == 0 {
				continue
			}

			// Update by selection ID with timestamp
			result, err := u.db.Exec(`
				UPDATE racing.runners
				SET 
					win_ppwap = $1,
					price_updated_at = NOW()
				WHERE betfair_selection_id = $2
				AND race_id IN (
					SELECT race_id FROM racing.races WHERE race_date = $3
				)
			`, price, runner.SelectionID, u.targetDate)

			if err != nil {
				continue
			}

			if rows, _ := result.RowsAffected(); rows > 0 {
				totalUpdates++
			}
		}
	}

	elapsed := time.Since(startTime)
	log.Printf("✅ Updated %d runners (took %v)", totalUpdates, elapsed.Round(time.Millisecond))

	// Show coverage
	u.showCoverage()
	log.Println()

	return nil
}

func (u *PriceUpdater) showCoverage() {
	var total, havePPWAP int
	var pct float64

	err := u.db.QueryRow(`
		SELECT 
			COUNT(*),
			COUNT(*) FILTER (WHERE win_ppwap IS NOT NULL),
			ROUND(100.0 * COUNT(*) FILTER (WHERE win_ppwap IS NOT NULL) / COUNT(*), 1)
		FROM racing.runners ru
		JOIN racing.races r ON r.race_id = ru.race_id
		WHERE r.race_date = $1
	`, u.targetDate).Scan(&total, &havePPWAP, &pct)

	if err != nil {
		return
	}

	log.Printf("📊 Coverage: %d/%d (%.1f%%)", havePPWAP, total, pct)
	if pct >= 80 {
		log.Println("   ✅ READY for betting script!")
	} else if pct >= 50 {
		log.Println("   ⚠️  PARTIAL - getting there...")
	} else {
		log.Println("   ❌ NOT READY - markets still developing")
	}
}

// ═══════════════════════════════════════════════════════════════════
// Betfair API calls (adapted from tennis bot)
// ═══════════════════════════════════════════════════════════════════

// loginInteractive - Interactive login (like your tennis bot)
func loginInteractive() (string, error) {
	type LoginResponse struct {
		Token  string `json:"token"` // Betfair uses "token" not "sessionToken"
		Status string `json:"status"`
		Error  string `json:"error"`
	}

	// Use form-encoded (like tennis bot), not JSON
	formData := url.Values{}
	formData.Set("username", BETFAIR_USERNAME)
	formData.Set("password", BETFAIR_PASSWORD)

	req, err := http.NewRequest("POST", BETFAIR_LOGIN_URL, strings.NewReader(formData.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Application", BETFAIR_APP_KEY)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", err
	}

	if loginResp.Status != "SUCCESS" {
		return "", fmt.Errorf("login failed: %s", loginResp.Error)
	}

	if loginResp.Token == "" {
		return "", fmt.Errorf("no token in response")
	}

	return loginResp.Token, nil
}

// fetchHorseRacingMarkets - Get horse racing markets (event_type_id=7)
func fetchHorseRacingMarkets(sessionToken, targetDate string) ([]MarketCatalogue, error) {
	// Parse target date
	startDate, err := time.Parse("2006-01-02", targetDate)
	if err != nil {
		return nil, err
	}
	endDate := startDate.Add(24 * time.Hour)

	params := map[string]interface{}{
		"filter": map[string]interface{}{
			"eventTypeIds":    []string{"7"}, // 7 = Horse Racing (tennis was "2")
			"marketCountries": []string{"GB", "IE"},
			"marketTypeCodes": []string{"WIN"},
			"marketStartTime": map[string]string{
				"from": startDate.UTC().Format(time.RFC3339),
				"to":   endDate.UTC().Format(time.RFC3339),
			},
		},
		"marketProjection": []string{"EVENT", "RUNNER_DESCRIPTION"},
		"sort":             "FIRST_TO_START",
		"maxResults":       500,
	}

	var markets []MarketCatalogue
	if err := callBettingAPI(sessionToken, "listMarketCatalogue", params, &markets); err != nil {
		return nil, err
	}

	return markets, nil
}

// fetchMarketBooks - Get live prices (like tennis bot's list_market_book)
func fetchMarketBooks(sessionToken string, marketIDs []string) ([]MarketBook, error) {
	params := map[string]interface{}{
		"marketIds": marketIDs,
		"priceProjection": map[string]interface{}{
			"priceData": []string{"EX_BEST_OFFERS", "EX_TRADED"}, // Same as tennis bot
		},
	}

	var books []MarketBook
	if err := callBettingAPI(sessionToken, "listMarketBook", params, &books); err != nil {
		return nil, err
	}

	return books, nil
}

// callBettingAPI - Generic Betfair JSON-RPC call
func callBettingAPI(sessionToken, method string, params interface{}, result interface{}) error {
	reqPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "SportsAPING/v1.0/" + method,
		"params":  params,
		"id":      time.Now().UnixNano(),
	}

	reqBody, err := json.Marshal(reqPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", BETFAIR_API_URL, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Application", BETFAIR_APP_KEY)
	req.Header.Set("X-Authentication", sessionToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var rpcResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return err
	}

	if rpcResp.Error != nil {
		return fmt.Errorf("API error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	// Marshal result back to target type
	resultBytes, err := json.Marshal(rpcResp.Result)
	if err != nil {
		return err
	}

	return json.Unmarshal(resultBytes, result)
}

// extractBestBackPrice - Get best back price (like tennis bot: runner.ex.available_to_back[0].price)
func extractBestBackPrice(runner RunnerBook) float64 {
	if runner.EX != nil && len(runner.EX.AvailableToBack) > 0 {
		return runner.EX.AvailableToBack[0].Price
	}
	// Fallback to last traded
	if runner.LastPriceTraded != nil && *runner.LastPriceTraded > 0 {
		return *runner.LastPriceTraded
	}
	return 0
}

// ═══════════════════════════════════════════════════════════════════
// Types (matching Betfair API)
// ═══════════════════════════════════════════════════════════════════

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	ID      int64           `json:"id"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type MarketCatalogue struct {
	MarketID   string          `json:"marketId"`
	MarketName string          `json:"marketName"`
	Event      *Event          `json:"event,omitempty"`
	Runners    []RunnerCatalog `json:"runners,omitempty"`
}

type Event struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CountryCode string    `json:"countryCode"`
	Venue       string    `json:"venue"`
	OpenDate    time.Time `json:"openDate"`
}

type RunnerCatalog struct {
	SelectionID int64  `json:"selectionId"`
	RunnerName  string `json:"runnerName"`
}

type MarketBook struct {
	MarketID     string       `json:"marketId"`
	Status       string       `json:"status"`
	TotalMatched float64      `json:"totalMatched"`
	Runners      []RunnerBook `json:"runners"`
}

type RunnerBook struct {
	SelectionID     int64           `json:"selectionId"`
	Status          string          `json:"status"`
	LastPriceTraded *float64        `json:"lastPriceTraded,omitempty"`
	TotalMatched    float64         `json:"totalMatched"`
	EX              *ExchangePrices `json:"ex,omitempty"`
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

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
