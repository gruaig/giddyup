package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"giddyup/api/internal/betfair"

	"github.com/jmoiron/sqlx"
)

// LivePricesService handles fetching and updating live Betfair prices
type LivePricesService struct {
	db             *sqlx.DB
	bfClient       *betfair.Client
	matcher        *betfair.Matcher
	marketMappings map[string]*betfair.RaceMapping // marketID → race/runner mappings
	updateInterval time.Duration
}

// NewLivePricesService creates a new live prices service
func NewLivePricesService(db *sqlx.DB, bfClient *betfair.Client, updateInterval time.Duration) *LivePricesService {
	return &LivePricesService{
		db:             db,
		bfClient:       bfClient,
		matcher:        betfair.NewMatcher(bfClient),
		marketMappings: make(map[string]*betfair.RaceMapping),
		updateInterval: updateInterval,
	}
}

// SetMarketMappings sets the market to race/runner mappings
func (s *LivePricesService) SetMarketMappings(mappings map[string]*betfair.RaceMapping) {
	s.marketMappings = mappings
}

// Run starts the live prices update loop
func (s *LivePricesService) Run(ctx context.Context) error {
	if len(s.marketMappings) == 0 {
		return fmt.Errorf("no market mappings set - call SetMarketMappings first")
	}

	log.Printf("[LivePrices] Starting live prices service for %d markets", len(s.marketMappings))
	log.Printf("[LivePrices] Update interval: %v", s.updateInterval)

	ticker := time.NewTicker(s.updateInterval)
	defer ticker.Stop()

	// Run immediately on start
	if err := s.fetchAndUpdate(ctx); err != nil {
		log.Printf("[LivePrices] Warning: Initial fetch failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("[LivePrices] Stopping live prices service")
			return ctx.Err()
		case <-ticker.C:
			if err := s.fetchAndUpdate(ctx); err != nil {
				log.Printf("[LivePrices] Warning: Price fetch failed: %v", err)
			}
		}
	}
}

// fetchAndUpdate fetches current prices and updates database
func (s *LivePricesService) fetchAndUpdate(ctx context.Context) error {
	// Get all market IDs
	marketIDs := make([]string, 0, len(s.marketMappings))
	for marketID := range s.marketMappings {
		marketIDs = append(marketIDs, marketID)
	}

	// Fetch live market books
	marketBooks, err := s.bfClient.ListMarketBook(ctx, marketIDs)
	if err != nil {
		return fmt.Errorf("fetch market books: %w", err)
	}

	log.Printf("[LivePrices] Fetched %d market books", len(marketBooks))

	// Process each market
	ts := time.Now()
	totalUpdates := 0

	for _, book := range marketBooks {
		mapping, exists := s.marketMappings[book.MarketID]
		if !exists {
			continue
		}

		// Process each runner
		for _, runner := range book.Runners {
			runnerID, exists := mapping.Runners[runner.SelectionID]
			if !exists {
				continue
			}

			// Calculate prices
			backPrice, layPrice, vwap := extractPrices(runner)

			// Insert into live_prices table
			_, err := s.db.Exec(`
				INSERT INTO racing.live_prices (race_id, runner_id, ts, back_price, lay_price, vwap, traded_vol)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (runner_id, ts) DO UPDATE SET
					back_price = EXCLUDED.back_price,
					lay_price = EXCLUDED.lay_price,
					vwap = EXCLUDED.vwap,
					traded_vol = EXCLUDED.traded_vol
			`, mapping.RaceID, runnerID, ts, nullFloat(backPrice), nullFloat(layPrice), nullFloat(vwap), runner.TotalMatched)

			if err != nil {
				log.Printf("[LivePrices] Warning: Failed to insert price for runner %d: %v", runnerID, err)
				continue
			}

			totalUpdates++
		}
	}

	log.Printf("[LivePrices] ✓ Updated %d runner prices at %s", totalUpdates, ts.Format("15:04:05"))

	// Mirror latest prices to runners table (non-destructive)
	if err := s.mirrorLatestPrices(ts); err != nil {
		log.Printf("[LivePrices] Warning: Mirror to runners failed: %v", err)
	}

	return nil
}

// mirrorLatestPrices copies latest live prices to runners table (non-destructive, today only)
func (s *LivePricesService) mirrorLatestPrices(ts time.Time) error {
	// Update win_ppwap with latest VWAP for today's races only
	_, err := s.db.Exec(`
		WITH latest AS (
			SELECT DISTINCT ON (lp.runner_id)
				lp.runner_id,
				lp.vwap,
				lp.back_price,
				lp.lay_price
			FROM racing.live_prices lp
			JOIN racing.runners run ON run.runner_id = lp.runner_id
			JOIN racing.races r ON r.race_id = run.race_id
			WHERE r.race_date = CURRENT_DATE
			  AND lp.ts >= $1 - INTERVAL '5 minutes'
			ORDER BY lp.runner_id, lp.ts DESC
		)
		UPDATE racing.runners run
		SET 
			win_ppwap = COALESCE(latest.vwap, run.win_ppwap),
			win_ppmax = GREATEST(COALESCE(latest.back_price, 0), COALESCE(run.win_ppmax, 0)),
			win_ppmin = LEAST(
				CASE WHEN latest.lay_price > 0 THEN latest.lay_price ELSE 9999 END,
				CASE WHEN run.win_ppmin > 0 THEN run.win_ppmin ELSE 9999 END
			)
		FROM latest
		WHERE run.runner_id = latest.runner_id
	`, ts)

	return err
}

// extractPrices calculates best back, lay, and VWAP from runner book
func extractPrices(runner betfair.RunnerBook) (backPrice, layPrice, vwap float64) {
	if runner.EX == nil {
		return 0, 0, 0
	}

	// Best available back price (highest price punters can back at)
	if len(runner.EX.AvailableToBack) > 0 {
		backPrice = runner.EX.AvailableToBack[0].Price
	}

	// Best available lay price (lowest price punters can lay at)
	if len(runner.EX.AvailableToLay) > 0 {
		layPrice = runner.EX.AvailableToLay[0].Price
	}

	// VWAP = volume-weighted average from traded volume
	if len(runner.EX.TradedVolume) > 0 {
		var totalVol, weightedSum float64
		for _, tv := range runner.EX.TradedVolume {
			totalVol += tv.Size
			weightedSum += tv.Price * tv.Size
		}
		if totalVol > 0 {
			vwap = weightedSum / totalVol
		}
	}

	// Fallback: if no traded volume, use mid-point of back/lay
	if vwap == 0 && backPrice > 0 && layPrice > 0 {
		vwap = (backPrice + layPrice) / 2
	}

	return
}

// nullFloat returns nil for 0 values
func nullFloat(f float64) interface{} {
	if f == 0 {
		return nil
	}
	return f
}

