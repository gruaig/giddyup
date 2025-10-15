package scraper

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// BetfairFetcher handles fetching historical BSP data from Betfair
type BetfairFetcher struct {
	client *http.Client
}

// NewBetfairFetcher creates a new Betfair fetcher
func NewBetfairFetcher() *BetfairFetcher {
	return &BetfairFetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FetchBSPForDate fetches Betfair BSP data for a specific date and region
// date format: YYYY-MM-DD
// region: "uk" or "ire"
func (bf *BetfairFetcher) FetchBSPForDate(date string, region string) ([]BetfairPrice, error) {
	// Convert date from YYYY-MM-DD to DDMMYYYY for Betfair
	dateStr, err := bf.formatDateForBetfair(date)
	if err != nil {
		return nil, err
	}

	log.Printf("[Betfair] Fetching BSP for %s (%s)...", date, region)

	// Fetch WIN prices
	winURL := fmt.Sprintf("https://promo.betfair.com/betfairsp/prices/dwbfprices%swin%s.csv", region, dateStr)
	winPrices, err := bf.fetchCSV(winURL, "win", region, date)
	if err != nil {
		log.Printf("[Betfair] Warning: Failed to fetch WIN prices: %v", err)
		winPrices = []BetfairPrice{}
	}

	// Fetch PLACE prices
	placeURL := fmt.Sprintf("https://promo.betfair.com/betfairsp/prices/dwbfprices%splace%s.csv", region, dateStr)
	placePrices, err := bf.fetchCSV(placeURL, "place", region, date)
	if err != nil {
		log.Printf("[Betfair] Warning: Failed to fetch PLACE prices: %v", err)
		placePrices = []BetfairPrice{}
	}

	// Merge WIN and PLACE prices
	merged := bf.mergePrices(winPrices, placePrices)

	log.Printf("[Betfair] Fetched %d prices for %s (%s)", len(merged), date, region)
	return merged, nil
}

// formatDateForBetfair converts YYYY-MM-DD to DDMMYYYY
func (bf *BetfairFetcher) formatDateForBetfair(date string) (string, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", err
	}
	return t.Format("02012006"), nil
}

// fetchCSV fetches and parses a Betfair CSV file
func (bf *BetfairFetcher) fetchCSV(url string, market string, region string, date string) ([]BetfairPrice, error) {
	resp, err := bf.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// No data available for this date/market combination
		return []BetfairPrice{}, nil
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	reader := csv.NewReader(resp.Body)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	prices := []BetfairPrice{}
	for i, record := range records {
		// Skip header
		if i == 0 {
			continue
		}

		if len(record) < 10 {
			continue
		}

		price := BetfairPrice{
			Region: region,
			Date:   date,
		}

		// Parse CSV fields
		// Format: event_id,menu_hint,event_name,event_dt,selection_id,selection_name,win_lose,bsp,ppwap,morningwap,ppmax,ppmin,ipmax,ipmin,morningtradedvol,pptradedvol,iptradedvol
		
		// menu_hint contains course: "Hexham 11th Oct"
		if len(record) > 1 {
			menuHint := record[1]
			// Extract course name (before the date part)
			parts := strings.Split(menuHint, " ")
			if len(parts) > 0 {
				// Course is typically first 1-2 words
				// "Hexham 11th Oct" -> "Hexham"
				// "Market Rasen 11th Oct" -> "Market Rasen"
				if len(parts) >= 3 {
					price.Course = strings.Join(parts[:len(parts)-2], " ")
				} else if len(parts) > 0 {
					price.Course = parts[0]
				}
			}
		}
		
		// event_dt contains date and time: "11-10-2025 13:55"
		if len(record) > 3 {
			eventDt := record[3]
			parts := strings.Fields(eventDt)
			if len(parts) >= 2 {
				// Time is HH:MM
				price.OffTime = parts[1]
				// Note: We keep the date passed to FetchBSPForDate, don't override from event_dt
			}
		}
		
		// selection_name is the horse
		if len(record) > 5 {
			price.Horse = record[5] // SELECTION_NAME
		}

		// Parse prices based on market
		// CSV: event_id,menu_hint,event_name,event_dt,selection_id,selection_name,win_lose,bsp,ppwap,morningwap,ppmax,ppmin,ipmax,ipmin,morningtradedvol,pptradedvol,iptradedvol
		if market == "win" && len(record) > 7 {
			price.WinBSP = bf.parseFloat(record[7])  // bsp
			if len(record) > 8 {
				price.WinPPWAP = bf.parseFloat(record[8])  // ppwap
			}
			if len(record) > 9 {
				price.WinMorningWAP = bf.parseFloat(record[9])  // morningwap
			}
			if len(record) > 10 {
				price.WinPPMax = bf.parseFloat(record[10])  // ppmax
			}
			if len(record) > 11 {
				price.WinPPMin = bf.parseFloat(record[11])  // ppmin
			}
			if len(record) > 12 {
				price.WinIPMax = bf.parseFloat(record[12])  // ipmax
			}
			if len(record) > 13 {
				price.WinIPMin = bf.parseFloat(record[13])  // ipmin
			}
			if len(record) > 14 {
				price.WinMorningVol = bf.parseFloat(record[14])  // morningtradedvol
			}
			if len(record) > 15 {
				price.WinPreVol = bf.parseFloat(record[15])  // pptradedvol
			}
			if len(record) > 16 {
				price.WinIPVol = bf.parseFloat(record[16])  // iptradedvol
			}
		} else if market == "place" && len(record) > 7 {
			price.PlaceBSP = bf.parseFloat(record[6])
			if len(record) > 7 {
				price.PlacePPWAP = bf.parseFloat(record[7])
			}
			if len(record) > 8 {
				price.PlaceMorningWAP = bf.parseFloat(record[8])
			}
			if len(record) > 9 {
				price.PlacePPMax = bf.parseFloat(record[9])
			}
			if len(record) > 10 {
				price.PlacePPMin = bf.parseFloat(record[10])
			}
			if len(record) > 11 {
				price.PlaceIPMax = bf.parseFloat(record[11])
			}
			if len(record) > 12 {
				price.PlaceIPMin = bf.parseFloat(record[12])
			}
			if len(record) > 13 {
				price.PlaceMorningVol = bf.parseFloat(record[13])
			}
			if len(record) > 14 {
				price.PlacePreVol = bf.parseFloat(record[14])
			}
			if len(record) > 15 {
				price.PlaceIPVol = bf.parseFloat(record[15])
			}
		}

		prices = append(prices, price)
	}

	return prices, nil
}

// mergePrices merges WIN and PLACE prices for the same horse/race
func (bf *BetfairFetcher) mergePrices(winPrices, placePrices []BetfairPrice) []BetfairPrice {
	// Create map of WIN prices by horse/course/time
	winMap := make(map[string]*BetfairPrice)
	for i := range winPrices {
		key := fmt.Sprintf("%s|%s|%s",
			NormalizeName(winPrices[i].Horse),
			NormalizeCourseName(winPrices[i].Course),
			winPrices[i].OffTime)
		winMap[key] = &winPrices[i]
	}

	// Merge PLACE prices into WIN prices
	for _, placePrice := range placePrices {
		key := fmt.Sprintf("%s|%s|%s",
			NormalizeName(placePrice.Horse),
			NormalizeCourseName(placePrice.Course),
			placePrice.OffTime)

		if winPrice, exists := winMap[key]; exists {
			// Merge place data into existing win record
			winPrice.PlaceBSP = placePrice.PlaceBSP
			winPrice.PlacePPWAP = placePrice.PlacePPWAP
			winPrice.PlaceMorningWAP = placePrice.PlaceMorningWAP
			winPrice.PlacePPMax = placePrice.PlacePPMax
			winPrice.PlacePPMin = placePrice.PlacePPMin
			winPrice.PlaceIPMax = placePrice.PlaceIPMax
			winPrice.PlaceIPMin = placePrice.PlaceIPMin
			winPrice.PlaceMorningVol = placePrice.PlaceMorningVol
			winPrice.PlacePreVol = placePrice.PlacePreVol
			winPrice.PlaceIPVol = placePrice.PlaceIPVol
		} else {
			// No WIN price, add PLACE-only record
			winMap[key] = &placePrice
		}
	}

	// Convert map back to slice
	result := make([]BetfairPrice, 0, len(winMap))
	for _, price := range winMap {
		result = append(result, *price)
	}

	return result
}

// parseFloat safely parses a string to float64
func (bf *BetfairFetcher) parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0.0
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return val
}
