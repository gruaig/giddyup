package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"giddyup/api/internal/loader"
	"giddyup/api/internal/scraper"
	"giddyup/api/internal/stitcher"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// AdminHandler handles administrative endpoints
type AdminHandler struct {
	db *sqlx.DB
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(db *sqlx.DB) *AdminHandler {
	return &AdminHandler{
		db: db,
	}
}

// ScrapeYesterday scrapes yesterday's results, stitches, and loads
// POST /api/v1/admin/scrape/yesterday
func (h *AdminHandler) ScrapeYesterday(c *gin.Context) {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	// Scrape RP results
	rpScraper := scraper.NewResultsScraper()
	rpRaces, err := rpScraper.ScrapeDate(yesterday)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to scrape Racing Post",
			"details": err.Error(),
		})
		return
	}

	// Download and stitch Betfair data
	bfStitcher := scraper.NewBetfairStitcher("/home/smonaghan/GiddyUp/data")
	bfStitcher.StitchBetfairForDate(yesterday, "uk")
	bfStitcher.StitchBetfairForDate(yesterday, "ire")

	// Load stitched Betfair races
	bfRacesUK, _ := bfStitcher.LoadStitchedRacesForDate(yesterday, "uk")
	bfRacesIRE, _ := bfStitcher.LoadStitchedRacesForDate(yesterday, "ire")
	bfRaces := append(bfRacesUK, bfRacesIRE...)

	// Convert to BetfairPrice format for stitcher
	bfPrices := h.convertStitchedToPrices(bfRaces)

	// Stitch data
	stitcherInst := stitcher.New(rpRaces, bfPrices)
	masterRaces, masterRunners, err := stitcherInst.StitchData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to stitch data",
			"details": err.Error(),
		})
		return
	}

	// Load to database
	bulkLoader := loader.NewBulkLoader(h.db.DB)
	stats, err := bulkLoader.LoadRaces(masterRaces, masterRunners)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to load data",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"date":             yesterday,
		"races_scraped":    len(rpRaces),
		"betfair_stitched": len(bfRaces),
		"races_loaded":     stats.RacesLoaded,
		"runners_loaded":   stats.RunnersLoaded,
	})
}

// convertStitchedToPrices converts stitched races to BetfairPrice format
func (h *AdminHandler) convertStitchedToPrices(stitchedRaces []scraper.StitchedRace) []scraper.BetfairPrice {
	prices := []scraper.BetfairPrice{}
	
	for _, race := range stitchedRaces {
		for _, runner := range race.Runners {
			price := scraper.BetfairPrice{
				Date:    race.Date,
				OffTime: race.OffTime,
				Horse:   runner.Horse,
			}
			
			// Parse WIN prices
			if val, err := strconv.ParseFloat(runner.WinBSP, 64); err == nil {
				price.WinBSP = val
			}
			if val, err := strconv.ParseFloat(runner.WinPPWAP, 64); err == nil {
				price.WinPPWAP = val
			}
			if val, err := strconv.ParseFloat(runner.WinMorningWAP, 64); err == nil {
				price.WinMorningWAP = val
			}
			if val, err := strconv.ParseFloat(runner.WinPPMax, 64); err == nil {
				price.WinPPMax = val
			}
			if val, err := strconv.ParseFloat(runner.WinPPMin, 64); err == nil {
				price.WinPPMin = val
			}
			if val, err := strconv.ParseFloat(runner.WinIPMax, 64); err == nil {
				price.WinIPMax = val
			}
			if val, err := strconv.ParseFloat(runner.WinIPMin, 64); err == nil {
				price.WinIPMin = val
			}
			if val, err := strconv.ParseFloat(runner.WinMorningVol, 64); err == nil {
				price.WinMorningVol = val
			}
			if val, err := strconv.ParseFloat(runner.WinPreVol, 64); err == nil {
				price.WinPreVol = val
			}
			if val, err := strconv.ParseFloat(runner.WinIPVol, 64); err == nil {
				price.WinIPVol = val
			}
			
			// Parse PLACE prices
			if val, err := strconv.ParseFloat(runner.PlaceBSP, 64); err == nil {
				price.PlaceBSP = val
			}
			if val, err := strconv.ParseFloat(runner.PlacePPWAP, 64); err == nil {
				price.PlacePPWAP = val
			}
			if val, err := strconv.ParseFloat(runner.PlaceMorningWAP, 64); err == nil {
				price.PlaceMorningWAP = val
			}
			if val, err := strconv.ParseFloat(runner.PlacePPMax, 64); err == nil {
				price.PlacePPMax = val
			}
			if val, err := strconv.ParseFloat(runner.PlacePPMin, 64); err == nil {
				price.PlacePPMin = val
			}
			if val, err := strconv.ParseFloat(runner.PlaceIPMax, 64); err == nil {
				price.PlaceIPMax = val
			}
			if val, err := strconv.ParseFloat(runner.PlaceIPMin, 64); err == nil {
				price.PlaceIPMin = val
			}
			if val, err := strconv.ParseFloat(runner.PlaceMorningVol, 64); err == nil {
				price.PlaceMorningVol = val
			}
			if val, err := strconv.ParseFloat(runner.PlacePreVol, 64); err == nil {
				price.PlacePreVol = val
			}
			if val, err := strconv.ParseFloat(runner.PlaceIPVol, 64); err == nil {
				price.PlaceIPVol = val
			}
			
			prices = append(prices, price)
		}
	}
	
	return prices
}

// ScrapeDate scrapes a specific date
// POST /api/v1/admin/scrape/date
// Body: {"date": "2025-10-13"}
func (h *AdminHandler) ScrapeDate(c *gin.Context) {
	var req struct {
		Date string `json:"date" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}

	// Scrape RP results
	rpScraper := scraper.NewResultsScraper()
	rpRaces, err := rpScraper.ScrapeDate(req.Date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to scrape Racing Post",
			"details": err.Error(),
		})
		return
	}

	// Download and stitch Betfair data
	bfStitcher := scraper.NewBetfairStitcher("/home/smonaghan/GiddyUp/data")
	bfStitcher.StitchBetfairForDate(req.Date, "uk")
	bfStitcher.StitchBetfairForDate(req.Date, "ire")

	// Load stitched Betfair races
	bfRacesUK, _ := bfStitcher.LoadStitchedRacesForDate(req.Date, "uk")
	bfRacesIRE, _ := bfStitcher.LoadStitchedRacesForDate(req.Date, "ire")
	bfRaces := append(bfRacesUK, bfRacesIRE...)

	// Convert to BetfairPrice format for stitcher
	bfPrices := h.convertStitchedToPrices(bfRaces)

	// Stitch data (match RP + BF)
	stitcherInst := stitcher.New(rpRaces, bfPrices)
	masterRaces, masterRunners, err := stitcherInst.StitchData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to stitch data",
			"details": err.Error(),
		})
		return
	}

	// Save master CSVs (skip database for now)
	masterWriter := loader.NewMasterWriter("/home/smonaghan/GiddyUp/data")
	err = masterWriter.SaveMasterData(req.Date, masterRaces, masterRunners)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to save master CSVs",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"date":             req.Date,
		"races_scraped":    len(rpRaces),
		"betfair_stitched": len(bfRaces),
		"races_matched":    len(masterRaces),
		"runners_created":  len(masterRunners),
		"master_csv_saved": true,
	})
}

// GetUpdateStatus returns the status of data updates
// GET /api/v1/admin/status
func (h *AdminHandler) GetUpdateStatus(c *gin.Context) {
	query := `
		SELECT 
			update_date::text,
			status,
			races_loaded,
			runners_loaded,
			completed_at::text
		FROM racing.data_updates
		WHERE update_type = 'daily'
		ORDER BY update_date DESC
		LIMIT 10
	`

	rows, err := h.db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to query update status",
		})
		return
	}
	defer rows.Close()

	updates := []map[string]interface{}{}
	for rows.Next() {
		var date, status, completedAt sql.NullString
		var racesLoaded, runnersLoaded sql.NullInt64

		rows.Scan(&date, &status, &racesLoaded, &runnersLoaded, &completedAt)

		update := map[string]interface{}{
			"date":           date.String,
			"status":         status.String,
			"races_loaded":   racesLoaded.Int64,
			"runners_loaded": runnersLoaded.Int64,
			"completed_at":   completedAt.String,
		}
		updates = append(updates, update)
	}

	c.JSON(http.StatusOK, gin.H{
		"updates": updates,
	})
}

// DetectGaps detects missing dates in the database
// GET /api/v1/admin/gaps
func (h *AdminHandler) DetectGaps(c *gin.Context) {
	// Get all dates with completed updates
	query := `
		SELECT DISTINCT update_date::text
		FROM racing.data_updates
		WHERE update_type = 'daily'
		  AND status = 'completed'
		  AND update_date >= CURRENT_DATE - INTERVAL '30 days'
		ORDER BY update_date
	`

	rows, err := h.db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to query dates",
		})
		return
	}
	defer rows.Close()

	updatedDates := make(map[string]bool)
	for rows.Next() {
		var date string
		rows.Scan(&date)
		updatedDates[date] = true
	}

	// Find missing dates in last 30 days
	missing := []string{}
	current := time.Now().AddDate(0, 0, -30)
	yesterday := time.Now().AddDate(0, 0, -1)

	for current.Before(yesterday) || current.Equal(yesterday) {
		dateStr := current.Format("2006-01-02")
		if !updatedDates[dateStr] {
			missing = append(missing, dateStr)
		}
		current = current.AddDate(0, 0, 1)
	}

	c.JSON(http.StatusOK, gin.H{
		"missing_dates": missing,
		"count":         len(missing),
	})
}
