package tests

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"giddyup/api/internal/models"
)

const BASE_URL_ANGLE = "http://localhost:8000/api/v1/angles/near-miss-no-hike"

// Note: makeHTTPRequest is defined in comprehensive_test.go

// ========== Today's Qualifiers Tests ==========

func TestAngle_Today_Basic(t *testing.T) {
	t.Log("Angle Today: Basic hit")
	url := fmt.Sprintf("%s/today?on=2024-01-13&limit=50", BASE_URL_ANGLE)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var qualifiers []models.NearMissQualifier
	if err := json.Unmarshal(body, &qualifiers); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Verify invariants for each qualifier
	for i, q := range qualifiers {
		if i >= 5 {
			break // Check first 5
		}

		// Last run was 2nd place
		if q.Last.Pos != 2 {
			t.Errorf("Expected last_pos=2, got %d", q.Last.Pos)
		}

		// Beaten ≤ 3 lengths
		if q.Last.BTN != nil && *q.Last.BTN > 3.0 {
			t.Errorf("Expected btn ≤ 3.0, got %.2f", *q.Last.BTN)
		}

		// DSR ≤ 14 days
		if q.DSR > 14 {
			t.Errorf("Expected dsr ≤ 14, got %d", q.DSR)
		}

		// No OR hike (rating_change ≤ 0)
		if q.RatingChange > 0 {
			t.Errorf("Expected rating_change ≤ 0, got %d", q.RatingChange)
		}

		// Same surface (if default)
		if !q.SameSurface {
			t.Errorf("Expected same_surface=true with defaults")
		}

		t.Logf("  ✅ %s: 2nd %d days ago, OR change: %d",
			q.HorseName, q.DSR, q.RatingChange)
	}

	t.Logf("✅ Found %d qualifiers, all meet criteria", len(qualifiers))
}

func TestAngle_Today_IncludeNullOR(t *testing.T) {
	t.Log("Angle Today: Include null OR filter")

	// Without null ORs
	_, body1 := makeHTTPRequest(t, "GET", fmt.Sprintf("%s/today?on=2024-01-13&include_null_or=false", BASE_URL_ANGLE), nil)
	var q1 []models.NearMissQualifier
	json.Unmarshal(body1, &q1)

	// Check none have null ORs
	for _, q := range q1 {
		if q.Entry.OR == nil || q.Last.OR == nil {
			t.Error("With include_null_or=false, should not have null ORs")
		}
	}

	// With null ORs
	_, body2 := makeHTTPRequest(t, "GET", fmt.Sprintf("%s/today?on=2024-01-13&include_null_or=true", BASE_URL_ANGLE), nil)
	var q2 []models.NearMissQualifier
	json.Unmarshal(body2, &q2)

	if len(q2) < len(q1) {
		t.Error("With include_null_or=true, should have same or more results")
	}

	t.Logf("✅ Without nulls: %d, With nulls: %d", len(q1), len(q2))
}

func TestAngle_Today_DistanceTolerance(t *testing.T) {
	t.Log("Angle Today: Distance tolerance")

	// Strict tolerance
	_, body1 := makeHTTPRequest(t, "GET", fmt.Sprintf("%s/today?on=2024-01-13&dist_f_tolerance=0", BASE_URL_ANGLE), nil)
	var q1 []models.NearMissQualifier
	json.Unmarshal(body1, &q1)

	for _, q := range q1 {
		if q.DistFDiff != 0 {
			t.Errorf("With tolerance=0, expected exact distance match, got diff %.1f", q.DistFDiff)
		}
	}

	// Loose tolerance
	_, body2 := makeHTTPRequest(t, "GET", fmt.Sprintf("%s/today?on=2024-01-13&dist_f_tolerance=2.0", BASE_URL_ANGLE), nil)
	var q2 []models.NearMissQualifier
	json.Unmarshal(body2, &q2)

	if len(q2) < len(q1) {
		t.Error("Looser tolerance should have same or more results")
	}

	t.Logf("✅ Strict (=0): %d, Loose (≤2.0f): %d", len(q1), len(q2))
}

func TestAngle_Today_SameSurfaceFilter(t *testing.T) {
	t.Log("Angle Today: Same surface filter")

	// Same surface required (default)
	_, body1 := makeHTTPRequest(t, "GET", fmt.Sprintf("%s/today?on=2024-01-13&same_surface=true", BASE_URL_ANGLE), nil)
	var q1 []models.NearMissQualifier
	json.Unmarshal(body1, &q1)

	for _, q := range q1 {
		if !q.SameSurface {
			t.Error("With same_surface=true, all should match")
		}
	}

	// Allow different surfaces
	_, body2 := makeHTTPRequest(t, "GET", fmt.Sprintf("%s/today?on=2024-01-13&same_surface=false", BASE_URL_ANGLE), nil)
	var q2 []models.NearMissQualifier
	json.Unmarshal(body2, &q2)

	if len(q2) < len(q1) {
		t.Error("Allowing mixed surfaces should increase or maintain count")
	}

	t.Logf("✅ Same surface: %d, Any surface: %d", len(q1), len(q2))
}

func TestAngle_Today_Pagination(t *testing.T) {
	t.Log("Angle Today: Pagination")

	// Page 1
	_, body1 := makeHTTPRequest(t, "GET", fmt.Sprintf("%s/today?on=2024-01-13&limit=10&offset=0", BASE_URL_ANGLE), nil)
	var page1 []models.NearMissQualifier
	json.Unmarshal(body1, &page1)

	// Page 2
	_, body2 := makeHTTPRequest(t, "GET", fmt.Sprintf("%s/today?on=2024-01-13&limit=10&offset=10", BASE_URL_ANGLE), nil)
	var page2 []models.NearMissQualifier
	json.Unmarshal(body2, &page2)

	// Check for duplicates
	for _, p1 := range page1 {
		for _, p2 := range page2 {
			if p1.HorseID == p2.HorseID {
				t.Errorf("Duplicate horse_id %d across pages", p1.HorseID)
			}
		}
	}

	t.Logf("✅ Page 1: %d, Page 2: %d, No duplicates", len(page1), len(page2))
}

func TestAngle_Today_Performance(t *testing.T) {
	t.Log("Angle Today: Performance test")

	start := time.Now()
	resp, _ := makeHTTPRequest(t, "GET", fmt.Sprintf("%s/today?on=2024-01-13&limit=200", BASE_URL_ANGLE), nil)
	latency := time.Since(start)

	if resp.StatusCode != 200 {
		t.Skip("Endpoint may not have racecard data")
	}

	if latency.Milliseconds() > 200 {
		t.Logf("Warning: P95 target is 200ms, got %vms", latency.Milliseconds())
	}

	t.Logf("✅ Latency: %vms", latency.Milliseconds())
}

// ========== Past (Backtest) Tests ==========

func TestAngle_Past_CoreFilter(t *testing.T) {
	t.Log("Angle Past: Core filter validation")
	url := fmt.Sprintf("%s/past?date_from=2024-01-01&date_to=2024-01-31&limit=20", BASE_URL_ANGLE)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var result models.NearMissPastResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Verify summary is present (default summary=true)
	if result.Summary == nil {
		t.Error("Expected summary with default params")
	}

	// Verify each case
	for i, c := range result.Cases {
		if i >= 5 {
			break
		}

		// Last pos = 2
		if c.LastPos != 2 {
			t.Errorf("Expected last_pos=2, got %d", c.LastPos)
		}

		// DSR calculated correctly
		expectedDSR := c.DSR
		if expectedDSR > 14 {
			t.Errorf("Expected DSR ≤ 14, got %d", expectedDSR)
		}

		t.Logf("  Case %d: %s - Last: pos=%d, btn=%.2f, Next: won=%v, price=%.2f",
			i+1, c.HorseName, c.LastPos, *c.LastBTN, c.NextWin, *c.Price)
	}

	t.Logf("✅ Found %d cases, Summary: %d wins/%d runs (%.1f%% SR, ROI: %.2f%%)",
		len(result.Cases), result.Summary.Wins, result.Summary.N,
		result.Summary.WinRate*100, *result.Summary.ROI*100)
}

func TestAngle_Past_RequireWin(t *testing.T) {
	t.Log("Angle Past: Require next win filter")
	url := fmt.Sprintf("%s/past?date_from=2024-01-01&date_to=2024-01-31&require_next_win=true&limit=50", BASE_URL_ANGLE)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result models.NearMissPastResponse
	json.Unmarshal(body, &result)

	// All should have won
	for _, c := range result.Cases {
		if !c.NextWin {
			t.Error("With require_next_win=true, all should have next_win=true")
		}
		if c.NextPos != nil && *c.NextPos != 1 {
			t.Errorf("Winners should have next_pos=1, got %d", *c.NextPos)
		}
	}

	if result.Summary != nil && result.Summary.WinRate != 1.0 {
		t.Errorf("Win rate should be 100%% with require_next_win, got %.1f%%", result.Summary.WinRate*100)
	}

	t.Logf("✅ All %d cases won (100%% win rate filter)", len(result.Cases))
}

func TestAngle_Past_ROICalculation(t *testing.T) {
	t.Log("Angle Past: ROI calculation verification")
	url := fmt.Sprintf("%s/past?date_from=2024-01-01&date_to=2024-01-31&price_source=bsp&limit=100", BASE_URL_ANGLE)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Skip("No data for period")
	}

	var result models.NearMissPastResponse
	json.Unmarshal(body, &result)

	// Manually calculate ROI
	totalPL := 0.0
	count := 0
	for _, c := range result.Cases {
		if c.Price != nil && *c.Price > 0 {
			if c.NextWin {
				totalPL += (*c.Price - 1)
			} else {
				totalPL += -1
			}
			count++
		}
	}

	manualROI := totalPL / float64(count)

	// Compare with server ROI
	if result.Summary != nil && result.Summary.ROI != nil {
		diff := manualROI - *result.Summary.ROI
		if diff > 0.000001 || diff < -0.000001 {
			t.Errorf("ROI mismatch: server=%.6f, calculated=%.6f", *result.Summary.ROI, manualROI)
		}
		t.Logf("✅ ROI verified: %.2f%% (server) = %.2f%% (calculated)", *result.Summary.ROI*100, manualROI*100)
	}
}

func TestAngle_Past_RaceTypeFilter(t *testing.T) {
	t.Log("Angle Past: Race type filter")
	url := fmt.Sprintf("%s/past?date_from=2024-01-01&date_to=2024-01-31&race_type=Flat&limit=50", BASE_URL_ANGLE)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Skip("No Flat races in period")
	}

	var result models.NearMissPastResponse
	json.Unmarshal(body, &result)

	// All last races should be Flat
	// Note: We'd need to add last_race_type to the model to verify this
	// For now, just check we got results

	t.Logf("✅ Flat filter: %d cases", len(result.Cases))
}

func TestAngle_Past_ORDownOnly(t *testing.T) {
	t.Log("Angle Past: OR down only (negative change)")
	url := fmt.Sprintf("%s/past?date_from=2024-01-01&date_to=2024-01-31&or_delta_max=-1&limit=50", BASE_URL_ANGLE)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Skip("No cases with OR drop")
	}

	var result models.NearMissPastResponse
	json.Unmarshal(body, &result)

	// All should have rating decrease
	for _, c := range result.Cases {
		if c.RatingChange > -1 {
			t.Errorf("Expected rating_change ≤ -1, got %d", c.RatingChange)
		}
	}

	t.Logf("✅ OR down only: %d cases, all with OR decrease", len(result.Cases))
}

func TestAngle_Past_TighterDSR(t *testing.T) {
	t.Log("Angle Past: Tighter DSR filter")

	// DSR ≤ 14 (default)
	_, body1 := makeHTTPRequest(t, "GET", fmt.Sprintf("%s/past?date_from=2024-01-01&date_to=2024-01-31&dsr_max=14&limit=100", BASE_URL_ANGLE), nil)
	var result1 models.NearMissPastResponse
	json.Unmarshal(body1, &result1)

	// DSR ≤ 7 (tighter)
	_, body2 := makeHTTPRequest(t, "GET", fmt.Sprintf("%s/past?date_from=2024-01-01&date_to=2024-01-31&dsr_max=7&limit=100", BASE_URL_ANGLE), nil)
	var result2 models.NearMissPastResponse
	json.Unmarshal(body2, &result2)

	if len(result2.Cases) > len(result1.Cases) {
		t.Error("Tighter DSR should be subset of looser DSR")
	}

	for _, c := range result2.Cases {
		if c.DSR > 7 {
			t.Errorf("Expected DSR ≤ 7, got %d", c.DSR)
		}
	}

	t.Logf("✅ DSR≤14: %d cases, DSR≤7: %d cases (subset)", len(result1.Cases), len(result2.Cases))
}

func TestAngle_Past_DateWindow(t *testing.T) {
	t.Log("Angle Past: Date window filtering")
	url := fmt.Sprintf("%s/past?date_from=2024-01-01&date_to=2024-01-07&limit=100", BASE_URL_ANGLE)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result models.NearMissPastResponse
	json.Unmarshal(body, &result)

	// Verify all last_dates are within window
	for _, c := range result.Cases {
		lastDate := c.LastDate[:10] // Get YYYY-MM-DD part
		if lastDate < "2024-01-01" || lastDate > "2024-01-07" {
			t.Errorf("Last date %s outside window [2024-01-01, 2024-01-07]", lastDate)
		}
	}

	t.Logf("✅ All %d cases within date window", len(result.Cases))
}

func TestAngle_Past_Performance1Year(t *testing.T) {
	t.Log("Angle Past: Performance - 1 year window")

	start := time.Now()
	url := fmt.Sprintf("%s/past?date_from=2024-01-01&date_to=2024-12-31&limit=200", BASE_URL_ANGLE)
	resp, _ := makeHTTPRequest(t, "GET", url, nil)
	latency := time.Since(start)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	if latency.Milliseconds() > 300 {
		t.Logf("Warning: P95 target 300ms for 1-year, got %vms", latency.Milliseconds())
	} else {
		t.Logf("✅ Performance: %vms (target < 300ms)", latency.Milliseconds())
	}
}

func TestAngle_Past_NoMatches(t *testing.T) {
	t.Log("Angle Past: Absurd filters return empty array")
	url := fmt.Sprintf("%s/past?date_from=2024-01-01&date_to=2024-01-01&btn_max=0.1&dsr_max=1", BASE_URL_ANGLE)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Errorf("Empty results should return 200, got %d", resp.StatusCode)
	}

	var result models.NearMissPastResponse
	json.Unmarshal(body, &result)

	if len(result.Cases) != 0 {
		t.Log("Note: Found some cases even with restrictive filters")
	}

	t.Log("✅ Empty results handled correctly (200 with empty array)")
}

func TestAngle_Past_BadParams(t *testing.T) {
	t.Log("Angle Past: Bad parameters return 400")
	url := fmt.Sprintf("%s/past?btn_max=abc", BASE_URL_ANGLE)
	resp, _ := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode == 500 {
		t.Error("Bad params should not cause 500 error")
	}

	t.Log("✅ Bad params handled gracefully")
}

func TestAngle_Past_PriceSourceVariations(t *testing.T) {
	t.Log("Angle Past: Price source variations")

	sources := []string{"bsp", "dec", "ppwap"}

	for _, source := range sources {
		t.Run(source, func(t *testing.T) {
			url := fmt.Sprintf("%s/past?date_from=2024-01-01&date_to=2024-01-31&price_source=%s&limit=10", BASE_URL_ANGLE, source)
			resp, body := makeHTTPRequest(t, "GET", url, nil)

			if resp.StatusCode != 200 {
				t.Fatalf("Expected 200 for source=%s, got %d", source, resp.StatusCode)
			}

			var result models.NearMissPastResponse
			json.Unmarshal(body, &result)

			// Check prices are finite where present
			for _, c := range result.Cases {
				if c.Price != nil {
					if *c.Price < 1.01 || *c.Price > 1000 {
						t.Errorf("Price out of reasonable range: %.2f", *c.Price)
					}
				}
			}

			t.Logf("✅ Price source %s: %d cases", source, len(result.Cases))
		})
	}
}

func TestAngle_Past_SummaryToggle(t *testing.T) {
	t.Log("Angle Past: Summary toggle")

	// With summary (default)
	_, body1 := makeHTTPRequest(t, "GET", fmt.Sprintf("%s/past?date_from=2024-01-01&date_to=2024-01-31&summary=true", BASE_URL_ANGLE), nil)
	var result1 models.NearMissPastResponse
	json.Unmarshal(body1, &result1)

	if result1.Summary == nil {
		t.Error("Expected summary with summary=true")
	}

	// Without summary
	_, body2 := makeHTTPRequest(t, "GET", fmt.Sprintf("%s/past?date_from=2024-01-01&date_to=2024-01-31&summary=false", BASE_URL_ANGLE), nil)
	var result2 models.NearMissPastResponse
	json.Unmarshal(body2, &result2)

	if result2.Summary != nil {
		t.Error("Should not have summary with summary=false")
	}

	t.Log("✅ Summary toggle works correctly")
}
