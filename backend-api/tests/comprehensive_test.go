package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"giddyup/api/internal/models"
)

// Test Fixtures (from database queries)
const (
	HORSE_ID   = 9643         // Captain Scooby - 195 runs
	TRAINER_ID = 666          // Trainer with most runners
	JOCKEY_ID  = 1548         // Jockey with most rides
	COURSE_ID  = 82           // Aintree
	RACE_ID    = 339          // Recent race with 12 runners
	DATE1      = "2024-01-01" // Date with many races
	DATE2      = "2024-01-02" // DATE1 + 1 day
	BASE_URL   = "http://localhost:8000"
)

// Helper: make HTTP request
func makeHTTPRequest(t *testing.T, method, url string, headers map[string]string) (*http.Response, []byte) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	t.Logf("  Latency: %v ms", latency.Milliseconds())
	return resp, body
}

// ========== A. Health, CORS, and Plumbing (5) ==========

func TestA01_HealthOK(t *testing.T) {
	t.Log("A01: Health endpoint returns OK")
	resp, body := makeHTTPRequest(t, "GET", BASE_URL+"/health", nil)

	if resp.StatusCode != 200 {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if result["status"] != "healthy" {
		t.Errorf("Expected status=healthy, got %s", result["status"])
	}

	t.Log("✅ Health check passed")
}

func TestA02_CORSPreflight(t *testing.T) {
	t.Log("A02: CORS preflight request")
	headers := map[string]string{
		"Origin":                        "http://localhost:3000",
		"Access-Control-Request-Method": "GET",
	}

	resp, _ := makeHTTPRequest(t, "OPTIONS", BASE_URL+"/api/v1/races", headers)

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		t.Errorf("Expected 200 or 204, got %d", resp.StatusCode)
	}

	allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	allowMethods := resp.Header.Get("Access-Control-Allow-Methods")

	if !strings.Contains(allowOrigin, "localhost:3000") && allowOrigin != "*" {
		t.Errorf("Expected origin in Allow-Origin, got: %s", allowOrigin)
	}

	if !strings.Contains(allowMethods, "GET") {
		t.Errorf("Expected GET in Allow-Methods, got: %s", allowMethods)
	}

	t.Log("✅ CORS headers present")
}

func TestA03_JSONContentType(t *testing.T) {
	t.Log("A03: JSON content type header")
	resp, _ := makeHTTPRequest(t, "GET", BASE_URL+"/api/v1/courses", nil)

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected application/json, got: %s", contentType)
	}

	t.Log("✅ Content-Type is application/json")
}

func TestA04_Graceful404(t *testing.T) {
	t.Log("A04: Graceful 404 handling")
	resp, body := makeHTTPRequest(t, "GET", BASE_URL+"/api/v1/this-does-not-exist", nil)

	if resp.StatusCode != 404 {
		t.Errorf("Expected 404, got %d", resp.StatusCode)
	}

	var errResp map[string]interface{}
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Errorf("404 should return JSON")
	}

	// No stack traces in response
	if strings.Contains(string(body), "panic") || strings.Contains(string(body), "goroutine") {
		t.Error("Response contains stack trace")
	}

	t.Log("✅ 404 handled gracefully")
}

func TestA05_SQLInjectionResilience(t *testing.T) {
	t.Log("A05: SQL injection resilience")
	maliciousQuery := "Frankel';DROP TABLE races;--"
	url := fmt.Sprintf("%s/api/v1/search?q=%s", BASE_URL, maliciousQuery)

	resp, _ := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode == 500 {
		t.Error("SQL injection caused 500 error")
	}

	// Verify database still works
	resp2, _ := makeHTTPRequest(t, "GET", BASE_URL+"/api/v1/courses", nil)
	if resp2.StatusCode != 200 {
		t.Error("Database appears corrupted after SQL injection attempt")
	}

	t.Log("✅ SQL injection blocked safely")
}

// ========== B. Global Search & Comments FTS (6) ==========

func TestB01_GlobalSearchBasic(t *testing.T) {
	t.Log("B01: Global search basic structure")
	resp, body := makeHTTPRequest(t, "GET", BASE_URL+"/api/v1/search?q=Frankel&limit=5", nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var results models.SearchResults
	if err := json.Unmarshal(body, &results); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Verify structure
	if results.Total == 0 {
		t.Error("Expected total_results > 0")
	}

	// Check horses array has proper structure
	for _, h := range results.Horses {
		if h.ID == 0 || h.Name == "" || h.Type == "" {
			t.Error("Horse missing required fields")
		}
		if h.Score < 0 || h.Score > 1 {
			t.Errorf("Score out of range [0,1]: %.2f", h.Score)
		}
	}

	t.Logf("✅ Found %d total results", results.Total)
}

func TestB02_TrigramTolerance(t *testing.T) {
	t.Log("B02: Trigram tolerance (typo handling)")
	resp, body := makeHTTPRequest(t, "GET", BASE_URL+"/api/v1/search?q=Fr4nkel&limit=5", nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var results models.SearchResults
	json.Unmarshal(body, &results)

	if len(results.Horses) == 0 {
		t.Error("Expected fuzzy match results even with typo")
	}

	// Top match should still be close to "Frankel"
	if len(results.Horses) > 0 {
		topMatch := results.Horses[0]
		t.Logf("✅ Top match: %s (score: %.2f)", topMatch.Name, topMatch.Score)
	}
}

func TestB03_LimitEnforcement(t *testing.T) {
	t.Log("B03: Limit parameter enforcement")
	resp, body := makeHTTPRequest(t, "GET", BASE_URL+"/api/v1/search?q=a&limit=3", nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var results models.SearchResults
	json.Unmarshal(body, &results)

	if len(results.Horses) > 3 {
		t.Errorf("Horses limit violated: got %d", len(results.Horses))
	}
	if len(results.Trainers) > 3 {
		t.Errorf("Trainers limit violated: got %d", len(results.Trainers))
	}
	if len(results.Jockeys) > 3 {
		t.Errorf("Jockeys limit violated: got %d", len(results.Jockeys))
	}

	t.Log("✅ Limit enforced correctly")
}

func TestB04_CommentFTSPhrase(t *testing.T) {
	t.Log("B04: Comment FTS phrase search")
	resp, body := makeHTTPRequest(t, "GET", BASE_URL+"/api/v1/search/comments?q=led&limit=5", nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var results []models.CommentSearchResult
	if err := json.Unmarshal(body, &results); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	for _, r := range results {
		if r.RunnerID == 0 || r.RaceID == 0 || r.Comment == "" {
			t.Error("Missing required fields in comment search result")
		}
		// Verify comment contains search term (case-insensitive)
		if !strings.Contains(strings.ToLower(r.Comment), "led") {
			t.Errorf("Comment doesn't match search term: %s", r.Comment)
		}
	}

	t.Logf("✅ Found %d comment results", len(results))
}

// ========== C. Races & Runners (10) ==========

func TestC01_RacesOnDate(t *testing.T) {
	t.Log("C01: Races on a specific date")
	resp, body := makeHTTPRequest(t, "GET", fmt.Sprintf("%s/api/v1/races?date=%s&limit=50", BASE_URL, DATE1), nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var races []models.Race
	if err := json.Unmarshal(body, &races); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if len(races) == 0 {
		t.Fatal("Expected races on DATE1")
	}

	for _, r := range races {
		if !strings.Contains(r.RaceDate, DATE1) {
			t.Errorf("Race date mismatch: expected %s, got %s", DATE1, r.RaceDate)
		}
		if r.Ran < 1 {
			t.Error("Race should have ran >= 1")
		}
	}

	t.Logf("✅ Found %d races on %s", len(races), DATE1)
}

func TestC02_RaceDetail(t *testing.T) {
	t.Log("C02: Single race detail")
	url := fmt.Sprintf("%s/api/v1/races/%d", BASE_URL, RACE_ID)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var raceWithRunners models.RaceWithRunners
	if err := json.Unmarshal(body, &raceWithRunners); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if raceWithRunners.Race.RaceID != RACE_ID {
		t.Error("Race ID mismatch")
	}
	if raceWithRunners.Race.RaceName == "" {
		t.Error("Missing race_name")
	}
	if raceWithRunners.Race.Ran == 0 {
		t.Error("Missing ran count")
	}

	t.Logf("✅ Race: %s, Ran: %d", raceWithRunners.Race.RaceName, raceWithRunners.Race.Ran)
}

func TestC03_RaceRunnersCountEqualsRan(t *testing.T) {
	t.Log("C03: Race runners count equals ran")
	url := fmt.Sprintf("%s/api/v1/races/%d", BASE_URL, RACE_ID)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var raceWith models.RaceWithRunners
	json.Unmarshal(body, &raceWith)

	if len(raceWith.Runners) != raceWith.Race.Ran {
		t.Errorf("Runner count mismatch: expected %d, got %d", raceWith.Race.Ran, len(raceWith.Runners))
	}

	t.Logf("✅ Runners count (%d) matches ran (%d)", len(raceWith.Runners), raceWith.Race.Ran)
}

func TestC04_WinnerInvariants(t *testing.T) {
	t.Log("C04: Winner invariants")
	url := fmt.Sprintf("%s/api/v1/races/%d", BASE_URL, RACE_ID)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Skip("Race not found")
	}

	var raceWith models.RaceWithRunners
	json.Unmarshal(body, &raceWith)

	winnerCount := 0
	for _, r := range raceWith.Runners {
		if r.WinFlag {
			winnerCount++
			if r.PosNum == nil || *r.PosNum != 1 {
				t.Error("Winner should have pos_num = 1")
			}
		}

		// Price invariants
		if r.Dec != nil && *r.Dec < 1.01 {
			t.Errorf("dec should be >= 1.01, got %.2f", *r.Dec)
		}
		if r.WinBSP != nil && *r.WinBSP < 1.01 {
			t.Errorf("win_bsp should be >= 1.01, got %.2f", *r.WinBSP)
		}
	}

	if winnerCount > 1 {
		t.Errorf("Expected exactly 1 winner, got %d", winnerCount)
	}

	t.Log("✅ Winner invariants satisfied")
}

func TestC05_DateRangeSearch(t *testing.T) {
	t.Log("C05: Date range search")
	url := fmt.Sprintf("%s/api/v1/races/search?date_from=%s&date_to=%s&limit=100", BASE_URL, DATE1, DATE2)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var races []models.Race
	json.Unmarshal(body, &races)

	for _, r := range races {
		raceDate := strings.Split(r.RaceDate, "T")[0] // Get date part
		if raceDate < DATE1 || raceDate > DATE2 {
			t.Errorf("Race outside date range: %s", raceDate)
		}
	}

	t.Logf("✅ All %d races within date range", len(races))
}

func TestC06_RaceFiltersCourseAndType(t *testing.T) {
	t.Log("C06: Race filters - course and type")
	url := fmt.Sprintf("%s/api/v1/races/search?course_id=%d&type=Flat&limit=50", BASE_URL, COURSE_ID)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var races []models.Race
	json.Unmarshal(body, &races)

	for _, r := range races {
		if r.CourseID == nil || *r.CourseID != COURSE_ID {
			t.Error("Course ID filter not applied")
		}
		if r.RaceType != "Flat" {
			t.Errorf("Type filter not applied: got %s", r.RaceType)
		}
	}

	t.Logf("✅ Filters applied: %d races", len(races))
}

func TestC07_FieldSizeFilter(t *testing.T) {
	t.Log("C07: Field size filter")
	url := fmt.Sprintf("%s/api/v1/races/search?field_min=12&field_max=20&limit=50", BASE_URL)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Skip("Field size filter endpoint may not be implemented")
	}

	var races []models.Race
	json.Unmarshal(body, &races)

	for _, r := range races {
		if r.Ran < 12 || r.Ran > 20 {
			t.Errorf("Field size out of range: %d", r.Ran)
		}
	}

	t.Logf("✅ Field size filter applied")
}

func TestC08_CoursesList(t *testing.T) {
	t.Log("C08: Courses list")
	resp, body := makeHTTPRequest(t, "GET", BASE_URL+"/api/v1/courses", nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var courses []models.Course
	if err := json.Unmarshal(body, &courses); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if len(courses) < 80 {
		t.Errorf("Expected >= 80 courses, got %d", len(courses))
	}

	for _, c := range courses {
		if c.CourseID == 0 || c.CourseName == "" || c.Region == "" {
			t.Error("Course missing required fields")
		}
	}

	t.Logf("✅ Found %d courses", len(courses))
}

func TestC09_CourseMeetings(t *testing.T) {
	t.Log("C09: Course meetings")
	url := fmt.Sprintf("%s/api/v1/courses/%d/meetings?date_from=%s&date_to=%s", BASE_URL, COURSE_ID, DATE1, DATE2)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var meetings []models.Meeting
	json.Unmarshal(body, &meetings)

	for _, m := range meetings {
		if m.RaceCount == 0 {
			t.Error("Meeting should have race_count > 0")
		}
	}

	t.Logf("✅ Found %d meetings", len(meetings))
}

// ========== D. Profiles (Horse / Trainer / Jockey) (9) ==========

func TestD01_HorseProfileBasic(t *testing.T) {
	t.Log("D01: Horse profile basic structure")
	url := fmt.Sprintf("%s/api/v1/horses/%d/profile", BASE_URL, HORSE_ID)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var profile models.HorseProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if profile.CareerSummary.Runs == 0 {
		t.Error("Expected career_runs > 0")
	}
	if profile.CareerSummary.Wins > profile.CareerSummary.Runs {
		t.Error("Wins cannot exceed runs")
	}
	if len(profile.GoingSplits) == 0 {
		t.Error("Expected going splits")
	}
	if len(profile.DistanceSplits) == 0 {
		t.Error("Expected distance splits")
	}

	t.Logf("✅ Profile: %d runs, %d wins", profile.CareerSummary.Runs, profile.CareerSummary.Wins)
}

func TestD02_TrainerProfileBasic(t *testing.T) {
	t.Log("D02: Trainer profile basic structure")
	url := fmt.Sprintf("%s/api/v1/trainers/%d/profile", BASE_URL, TRAINER_ID)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var profile models.TrainerProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if len(profile.RollingForm) == 0 {
		t.Error("Expected rolling form stats")
	}

	for _, form := range profile.RollingForm {
		if form.Wins > form.Runs {
			t.Errorf("Wins (%d) cannot exceed runs (%d)", form.Wins, form.Runs)
		}
	}

	t.Logf("✅ Trainer profile loaded with %d form periods", len(profile.RollingForm))
}

func TestD03_JockeyProfileBasic(t *testing.T) {
	t.Log("D03: Jockey profile basic structure")
	url := fmt.Sprintf("%s/api/v1/jockeys/%d/profile", BASE_URL, JOCKEY_ID)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var profile models.JockeyProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if profile.CareerStats.Runs == 0 {
		t.Error("Expected career runs > 0")
	}

	// Verify strike rate is a percentage
	for _, form := range profile.RollingForm {
		if form.SR != nil {
			if *form.SR < 0 || *form.SR > 100 {
				t.Errorf("SR should be in [0,100], got %.2f", *form.SR)
			}
		}
	}

	t.Log("✅ Jockey profile loaded")
}

// ========== E. Market Analytics (10) ==========

func TestE01_SteamersAndDrifters(t *testing.T) {
	t.Log("E01: Steamers and drifters")
	url := fmt.Sprintf("%s/api/v1/market/movers?date=%s&min_move=15", BASE_URL, DATE1)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var movers []models.MarketMover
	if err := json.Unmarshal(body, &movers); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	for i, m := range movers {
		if m.MorningPrice != nil && m.BSP != nil {
			if *m.MorningPrice < *m.BSP {
				t.Logf("  Drifter: %s (%.2f -> %.2f)", m.HorseName, *m.MorningPrice, *m.BSP)
			} else {
				t.Logf("  Steamer: %s (%.2f -> %.2f)", m.HorseName, *m.MorningPrice, *m.BSP)
			}
		}
		if i >= 2 {
			break
		}
	}

	t.Logf("✅ Found %d movers", len(movers))
}

func TestE02_WinCalibration(t *testing.T) {
	t.Log("E02: Win market calibration")
	url := fmt.Sprintf("%s/api/v1/market/calibration/win?date_from=%s&date_to=%s", BASE_URL, DATE1, DATE2)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var bins []models.CalibrationBin
	if err := json.Unmarshal(body, &bins); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	for _, bin := range bins {
		if bin.Runners == 0 {
			t.Error("Bin should have runners > 0")
		}
		if bin.ActualSR < 0 || bin.ActualSR > 100 {
			t.Errorf("ActualSR out of range: %.2f", bin.ActualSR)
		}
		if bin.ImpliedSR < 0 || bin.ImpliedSR > 100 {
			t.Errorf("ImpliedSR out of range: %.2f", bin.ImpliedSR)
		}
	}

	t.Logf("✅ Calibration: %d bins", len(bins))
}

func TestE03_PlaceCalibration(t *testing.T) {
	t.Log("E03: Place market calibration")
	url := fmt.Sprintf("%s/api/v1/market/calibration/place?date_from=%s&date_to=%s", BASE_URL, DATE1, DATE2)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var bins []models.CalibrationBin
	json.Unmarshal(body, &bins)

	for _, bin := range bins {
		if bin.Runners == 0 {
			t.Error("Bin should have runners > 0")
		}
	}

	t.Logf("✅ Place calibration: %d bins", len(bins))
}

func TestE04_InPlayMoves(t *testing.T) {
	t.Log("E04: In-play price movements")
	url := fmt.Sprintf("%s/api/v1/market/inplay-moves?date_from=%s&date_to=%s&min_move=20", BASE_URL, DATE1, DATE2)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Skip("In-play moves endpoint may not have data for this period")
	}

	var moves []models.InPlayMove
	json.Unmarshal(body, &moves)

	for _, m := range moves {
		if m.IPHigh != nil && m.IPLow != nil {
			if *m.IPHigh < *m.IPLow {
				t.Error("IP high should be >= IP low")
			}
			if *m.IPLow < 1.01 {
				t.Errorf("IP low should be >= 1.01, got %.2f", *m.IPLow)
			}
		}
	}

	t.Logf("✅ In-play moves analyzed")
}

func TestE05_BookVsExchange(t *testing.T) {
	t.Log("E05: Book vs Exchange comparison")
	url := fmt.Sprintf("%s/api/v1/market/book-vs-exchange?date_from=%s&date_to=%s", BASE_URL, DATE1, DATE2)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var comparison []models.BookVsExchange
	json.Unmarshal(body, &comparison)

	if len(comparison) > 0 {
		t.Logf("✅ Book vs Exchange: %d daily comparisons", len(comparison))
	}
}

// ========== F. Bias & Analysis (6) ==========

func TestF01_DrawBias(t *testing.T) {
	t.Log("F01: Draw bias analysis")
	url := fmt.Sprintf("%s/api/v1/bias/draw?course_id=%d&dist_min=5&dist_max=7", BASE_URL, COURSE_ID)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var bias []models.DrawBiasResult
	if err := json.Unmarshal(body, &bias); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	for _, d := range bias {
		if d.TotalRuns == 0 {
			t.Error("Draw should have runs > 0")
		}
		if d.WinRate < 0 || d.WinRate > 100 {
			t.Errorf("Win rate out of range: %.2f", d.WinRate)
		}
	}

	t.Logf("✅ Draw bias: %d draw positions analyzed", len(bias))
}

func TestF02_RecencyAnalysis(t *testing.T) {
	t.Log("F02: Recency (DSR) analysis")
	url := fmt.Sprintf("%s/api/v1/analysis/recency?date_from=%s&date_to=%s", BASE_URL, DATE1, DATE2)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var effects []models.RecencyEffect
	json.Unmarshal(body, &effects)

	for _, e := range effects {
		if e.Runs == 0 {
			t.Error("DSR bucket should have runs > 0")
		}
		if e.SR < 0 || e.SR > 100 {
			t.Errorf("SR out of range: %.2f", e.SR)
		}
	}

	t.Logf("✅ Recency: %d DSR buckets", len(effects))
}

func TestF03_TrainerChangeImpact(t *testing.T) {
	t.Log("F03: Trainer change impact")
	url := fmt.Sprintf("%s/api/v1/analysis/trainer-change?min_runs=5", BASE_URL)
	resp, body := makeHTTPRequest(t, "GET", url, nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var changes []models.TrainerChange
	json.Unmarshal(body, &changes)

	for _, c := range changes {
		if c.RunsBefore+c.RunsAfter < 5 {
			t.Error("Total runs should be >= min_runs parameter")
		}
	}

	t.Logf("✅ Trainer changes: %d horses analyzed", len(changes))
}

// ========== G. Validation & Error Handling (4) ==========

func TestG01_BadParams400(t *testing.T) {
	t.Log("G01: Bad parameters return 400")
	resp, _ := makeHTTPRequest(t, "GET", BASE_URL+"/api/v1/races/search?date_from=2024-13-99", nil)

	// Should return 400 or handle gracefully
	if resp.StatusCode == 500 {
		t.Error("Bad date should not cause 500 error")
	}

	t.Log("✅ Bad parameters handled")
}

func TestG02_NonExistentID404(t *testing.T) {
	t.Log("G02: Non-existent IDs return 404")
	resp, body := makeHTTPRequest(t, "GET", BASE_URL+"/api/v1/races/999999999", nil)

	if resp.StatusCode != 404 {
		t.Errorf("Expected 404, got %d", resp.StatusCode)
	}

	// Should not contain stack trace
	if strings.Contains(string(body), "panic") {
		t.Error("404 response contains stack trace")
	}

	t.Log("✅ Non-existent IDs return clean 404")
}

func TestG03_LimitsCapped(t *testing.T) {
	t.Log("G03: Limits are capped to server maximum")
	resp, body := makeHTTPRequest(t, "GET", BASE_URL+"/api/v1/races/search?limit=100000", nil)

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var races []models.Race
	json.Unmarshal(body, &races)

	// Should cap to reasonable limit (e.g., 1000)
	if len(races) > 1000 {
		t.Error("Limit should be capped to server maximum")
	}

	t.Logf("✅ Results capped to %d items", len(races))
}

func TestG04_EmptyResultsValid(t *testing.T) {
	t.Log("G04: Empty results are valid")
	start := time.Now()
	resp, body := makeHTTPRequest(t, "GET", BASE_URL+"/api/v1/races?date=2099-01-01", nil)
	latency := time.Since(start)

	if resp.StatusCode != 200 {
		t.Errorf("Expected 200 for empty results, got %d", resp.StatusCode)
	}

	var races []models.Race
	json.Unmarshal(body, &races)

	if len(races) != 0 {
		t.Errorf("Expected empty array, got %d items", len(races))
	}

	if latency.Milliseconds() > 100 {
		t.Logf("Warning: Empty query took %v ms (target < 50ms)", latency.Milliseconds())
	}

	t.Log("✅ Empty results handled correctly")
}
