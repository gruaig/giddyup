package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"giddyup/api/internal/models"
)

const (
	baseURL = "http://localhost:8000/api/v1"
)

// Helper function to make HTTP requests
func makeRequest(t *testing.T, method, url string) ([]byte, int) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request to %s: %v", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	return body, resp.StatusCode
}

// Test 1: Complete Horse Journey - Search, Select, Get Full Profile
func TestCompleteHorseJourney(t *testing.T) {
	t.Log("=== Testing Complete Horse Journey ===")

	// Step 1: Search for "Frankel"
	t.Log("Step 1: Searching for 'Frankel'...")
	body, status := makeRequest(t, "GET", fmt.Sprintf("%s/search?q=Frankel&limit=5", baseURL))
	if status != 200 {
		t.Fatalf("Search failed with status %d: %s", status, string(body))
	}

	var searchResults models.SearchResults
	if err := json.Unmarshal(body, &searchResults); err != nil {
		t.Fatalf("Failed to parse search results: %v", err)
	}

	if len(searchResults.Horses) == 0 {
		t.Fatal("No horses found for 'Frankel'")
	}

	horse := searchResults.Horses[0]
	t.Logf("✅ Found horse: %s (ID: %d, Score: %.2f)", horse.Name, horse.ID, horse.Score)

	// Verify search quality
	if horse.Score < 0.5 {
		t.Errorf("Expected high similarity score for 'Frankel', got %.2f", horse.Score)
	}

	// Step 2: Get complete horse profile
	t.Logf("Step 2: Getting profile for %s (ID: %d)...", horse.Name, horse.ID)
	body, status = makeRequest(t, "GET", fmt.Sprintf("%s/horses/%d/profile", baseURL, horse.ID))
	if status != 200 {
		t.Fatalf("Profile request failed with status %d: %s", status, string(body))
	}

	var profile models.HorseProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		t.Fatalf("Failed to parse horse profile: %v", err)
	}

	// Verify career summary
	t.Logf("✅ Career Summary:")
	t.Logf("   - Total Runs: %d", profile.CareerSummary.Runs)
	t.Logf("   - Wins: %d", profile.CareerSummary.Wins)
	t.Logf("   - Places: %d", profile.CareerSummary.Places)
	if profile.CareerSummary.PeakRPR != nil {
		t.Logf("   - Peak RPR: %d", *profile.CareerSummary.PeakRPR)
	}

	if profile.CareerSummary.Runs == 0 {
		t.Error("Expected career runs > 0")
	}

	// Verify recent form (last 3 runs with odds)
	t.Log("✅ Recent Form (Last 3 Runs):")
	for i, form := range profile.RecentForm {
		if i >= 3 {
			break
		}
		t.Logf("   Run %d: %s at %s", i+1, form.RaceDate, form.CourseName)
		t.Logf("      - Position: %v", form.PosRaw)
		if form.WinBSP != nil {
			t.Logf("      - BSP Odds: %.2f", *form.WinBSP)
		}
		if form.Dec != nil {
			t.Logf("      - SP Odds: %.2f", *form.Dec)
		}
		t.Logf("      - RPR: %v, OR: %v", form.RPR, form.OR)
	}

	if len(profile.RecentForm) == 0 {
		t.Error("Expected recent form data")
	}

	// Verify going splits
	t.Log("✅ Going Splits:")
	for _, split := range profile.GoingSplits {
		t.Logf("   - %s: %d runs, %d wins (%.1f%% SR)", split.Category, split.Runs, split.Wins, split.SR)
	}

	// Verify distance splits
	t.Log("✅ Distance Splits:")
	for _, split := range profile.DistanceSplits {
		t.Logf("   - %s: %d runs, %d wins (%.1f%% SR)", split.Category, split.Runs, split.Wins, split.SR)
	}

	// Verify course splits
	if len(profile.CourseSplits) > 0 {
		t.Log("✅ Course Splits (Top 3):")
		for i, split := range profile.CourseSplits {
			if i >= 3 {
				break
			}
			t.Logf("   - %s: %d runs, %d wins (%.1f%% SR)", split.Category, split.Runs, split.Wins, split.SR)
		}
	}

	t.Log("✅ Complete Horse Journey test PASSED")
}

// Test 2: Search Combinations
func TestSearchCombinations(t *testing.T) {
	t.Log("=== Testing Search Combinations ===")

	testCases := []struct {
		query          string
		expectedType   string
		minResults     int
		expectedEntity string
	}{
		{"Frankel", "horse", 1, "horse"},
		{"Enable", "horse", 1, "horse"},
		{"Ascot", "course", 1, "course"},
		{"Dettori", "jockey", 1, "jockey"},
		{"Gosden", "trainer", 1, "trainer"},
		{"Coolmore", "owner", 1, "owner"},
	}

	for _, tc := range testCases {
		t.Run(tc.query, func(t *testing.T) {
			body, status := makeRequest(t, "GET", fmt.Sprintf("%s/search?q=%s&limit=5", baseURL, tc.query))
			if status != 200 {
				t.Fatalf("Search failed for '%s' with status %d", tc.query, status)
			}

			var results models.SearchResults
			if err := json.Unmarshal(body, &results); err != nil {
				t.Fatalf("Failed to parse results: %v", err)
			}

			// Check appropriate result type
			var found int
			switch tc.expectedType {
			case "horse":
				found = len(results.Horses)
			case "course":
				found = len(results.Courses)
			case "jockey":
				found = len(results.Jockeys)
			case "trainer":
				found = len(results.Trainers)
			case "owner":
				found = len(results.Owners)
			}

			if found < tc.minResults {
				t.Errorf("Expected at least %d %s results for '%s', got %d", tc.minResults, tc.expectedType, tc.query, found)
			}

			t.Logf("✅ Search for '%s' returned %d %s results", tc.query, found, tc.expectedType)
		})
	}
}

// Test 3: Race Search with Filters
func TestRaceSearchWithFilters(t *testing.T) {
	t.Log("=== Testing Race Search with Filters ===")

	testCases := []struct {
		name     string
		url      string
		minRaces int
	}{
		{
			name:     "Recent races by date",
			url:      fmt.Sprintf("%s/races?date=2024-01-13&limit=10", baseURL),
			minRaces: 1,
		},
		{
			name:     "GB Flat races",
			url:      fmt.Sprintf("%s/races/search?date_from=2024-01-01&date_to=2024-01-31&region=GB&type=Flat&limit=10", baseURL),
			minRaces: 1,
		},
		{
			name:     "Class 1 races",
			url:      fmt.Sprintf("%s/races/search?date_from=2024-01-01&date_to=2024-12-31&class=1&limit=10", baseURL),
			minRaces: 1,
		},
		{
			name:     "Distance filtered (7-8f)",
			url:      fmt.Sprintf("%s/races/search?date_from=2024-01-01&date_to=2024-01-31&dist_min=7&dist_max=8&limit=10", baseURL),
			minRaces: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, status := makeRequest(t, "GET", tc.url)
			if status != 200 {
				t.Fatalf("Race search failed with status %d: %s", status, string(body))
			}

			var races []models.Race
			if err := json.Unmarshal(body, &races); err != nil {
				t.Fatalf("Failed to parse races: %v", err)
			}

			if len(races) < tc.minRaces {
				t.Errorf("Expected at least %d races, got %d", tc.minRaces, len(races))
			}

			if len(races) > 0 {
				t.Logf("✅ %s: Found %d races", tc.name, len(races))
				t.Logf("   Example: %s at %s on %s", races[0].RaceName, *races[0].CourseName, races[0].RaceDate)
			}
		})
	}
}

// Test 4: Race Details with Runners
func TestRaceDetailsWithRunners(t *testing.T) {
	t.Log("=== Testing Race Details with Runners ===")

	// First, get a race
	body, status := makeRequest(t, "GET", fmt.Sprintf("%s/races?date=2024-01-13&limit=1", baseURL))
	if status != 200 {
		t.Fatalf("Failed to get races: %d", status)
	}

	var races []models.Race
	if err := json.Unmarshal(body, &races); err != nil {
		t.Fatalf("Failed to parse races: %v", err)
	}

	if len(races) == 0 {
		t.Skip("No races found for 2024-01-13")
	}

	race := races[0]
	t.Logf("Testing race: %s (ID: %d)", race.RaceName, race.RaceID)

	// Get full race details with runners
	body, status = makeRequest(t, "GET", fmt.Sprintf("%s/races/%d", baseURL, race.RaceID))
	if status != 200 {
		t.Fatalf("Failed to get race details: %d", status)
	}

	var raceWithRunners models.RaceWithRunners
	if err := json.Unmarshal(body, &raceWithRunners); err != nil {
		t.Fatalf("Failed to parse race with runners: %v", err)
	}

	t.Logf("✅ Race: %s", raceWithRunners.Race.RaceName)
	t.Logf("   - Course: %s", *raceWithRunners.Race.CourseName)
	t.Logf("   - Date: %s", raceWithRunners.Race.RaceDate)
	t.Logf("   - Distance: %.1ff", *raceWithRunners.Race.DistF)
	t.Logf("   - Going: %s", *raceWithRunners.Race.Going)
	t.Logf("   - Runners: %d", len(raceWithRunners.Runners))

	// Verify runners have complete data
	if len(raceWithRunners.Runners) > 0 {
		t.Log("✅ Top 3 Runners:")
		for i, runner := range raceWithRunners.Runners {
			if i >= 3 {
				break
			}
			t.Logf("   %d. %s", i+1, *runner.HorseName)
			if runner.PosNum != nil {
				t.Logf("      - Position: %d", *runner.PosNum)
			}
			if runner.WinBSP != nil {
				t.Logf("      - BSP: %.2f", *runner.WinBSP)
			}
			if runner.Dec != nil {
				t.Logf("      - SP: %.2f", *runner.Dec)
			}
			t.Logf("      - Trainer: %s", *runner.TrainerName)
			t.Logf("      - Jockey: %s", *runner.JockeyName)
		}
	}
}

// Test 5: Trainer Profile
func TestTrainerProfile(t *testing.T) {
	t.Log("=== Testing Trainer Profile ===")

	// Search for a well-known trainer
	body, status := makeRequest(t, "GET", fmt.Sprintf("%s/search?q=Gosden", baseURL))
	if status != 200 {
		t.Fatalf("Search failed: %d", status)
	}

	var results models.SearchResults
	if err := json.Unmarshal(body, &results); err != nil {
		t.Fatalf("Failed to parse results: %v", err)
	}

	if len(results.Trainers) == 0 {
		t.Skip("No trainers found for 'Gosden'")
	}

	trainer := results.Trainers[0]
	t.Logf("Found trainer: %s (ID: %d)", trainer.Name, trainer.ID)

	// Get trainer profile
	body, status = makeRequest(t, "GET", fmt.Sprintf("%s/trainers/%d/profile", baseURL, trainer.ID))
	if status != 200 {
		t.Fatalf("Failed to get trainer profile: %d", status)
	}

	var profile models.TrainerProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		t.Fatalf("Failed to parse trainer profile: %v", err)
	}

	t.Logf("✅ Trainer: %s", profile.Trainer.TrainerName)

	// Verify rolling form
	t.Log("✅ Rolling Form:")
	for _, form := range profile.RollingForm {
		sr := 0.0
		if form.SR != nil {
			sr = *form.SR
		}
		t.Logf("   - %s: %d runs, %d wins (%.1f%% SR)", form.Period, form.Runs, form.Wins, sr)
	}

	// Verify course splits
	if len(profile.CourseSplits) > 0 {
		t.Log("✅ Top Course Splits:")
		for i, split := range profile.CourseSplits {
			if i >= 3 {
				break
			}
			t.Logf("   - %s: %d runs, %d wins (%.1f%% SR)", split.Category, split.Runs, split.Wins, split.SR)
		}
	}
}

// Test 6: Market Analytics - Steamers and Drifters
func TestMarketMovers(t *testing.T) {
	t.Log("=== Testing Market Movers ===")

	body, status := makeRequest(t, "GET", fmt.Sprintf("%s/market/movers?date=2024-01-13&min_move=10", baseURL))
	if status != 200 {
		t.Fatalf("Failed to get market movers: %d", status)
	}

	var movers []models.MarketMover
	if err := json.Unmarshal(body, &movers); err != nil {
		t.Fatalf("Failed to parse movers: %v", err)
	}

	t.Logf("✅ Found %d market movers", len(movers))

	if len(movers) > 0 {
		t.Log("Top 3 Movers:")
		for i, mover := range movers {
			if i >= 3 {
				break
			}
			moveType := "Drifter"
			if mover.MorningPrice != nil && mover.BSP != nil && *mover.BSP < *mover.MorningPrice {
				moveType = "Steamer"
			}
			t.Logf("   %d. %s - %s", i+1, mover.HorseName, moveType)
			if mover.MorningPrice != nil && mover.BSP != nil {
				t.Logf("      - Morning: %.2f -> BSP: %.2f", *mover.MorningPrice, *mover.BSP)
			}
			if mover.MovePct != nil {
				t.Logf("      - Move: %.1f%%", *mover.MovePct)
			}
			t.Logf("      - Result: Won=%v", mover.WinFlag)
		}
	}
}

// Test 7: Market Calibration
func TestMarketCalibration(t *testing.T) {
	t.Log("=== Testing Market Calibration ===")

	body, status := makeRequest(t, "GET", fmt.Sprintf("%s/market/calibration/win?date_from=2024-01-01&date_to=2024-01-31", baseURL))
	if status != 200 {
		t.Fatalf("Failed to get calibration: %d", status)
	}

	var bins []models.CalibrationBin
	if err := json.Unmarshal(body, &bins); err != nil {
		t.Fatalf("Failed to parse calibration: %v", err)
	}

	t.Logf("✅ Market Calibration Data:")
	t.Log("   Price Range | Runners | Wins | Actual SR | Implied SR | Edge")
	for _, bin := range bins {
		t.Logf("   %-11s | %7d | %4d | %8.2f%% | %9.2f%% | %+.2f%%",
			bin.PriceBin, bin.Runners, bin.Wins, bin.ActualSR, bin.ImpliedSR, bin.Edge)
	}

	if len(bins) == 0 {
		t.Error("Expected calibration bins data")
	}
}

// Test 8: Draw Bias Analysis
func TestDrawBias(t *testing.T) {
	t.Log("=== Testing Draw Bias Analysis ===")

	// Get Ascot course ID (73)
	body, status := makeRequest(t, "GET", fmt.Sprintf("%s/bias/draw?course_id=73&dist_min=5&dist_max=7", baseURL))
	if status != 200 {
		t.Fatalf("Failed to get draw bias: %d", status)
	}

	var bias []models.DrawBiasResult
	if err := json.Unmarshal(body, &bias); err != nil {
		t.Fatalf("Failed to parse draw bias: %v", err)
	}

	t.Logf("✅ Draw Bias Analysis (Ascot 5-7f):")
	t.Log("   Draw | Runs | Win Rate | Top3 Rate | Avg Position")
	for _, d := range bias {
		avgPos := "N/A"
		if d.AvgPosition != nil {
			avgPos = fmt.Sprintf("%.2f", *d.AvgPosition)
		}
		t.Logf("   %4d | %4d | %7.2f%% | %8.2f%% | %s",
			d.Draw, d.TotalRuns, d.WinRate, d.Top3Rate, avgPos)
	}
}

// Test 9: Comment Search
func TestCommentSearch(t *testing.T) {
	t.Log("=== Testing Comment Search ===")

	searchTerms := []string{"led", "prominent", "never dangerous"}

	for _, term := range searchTerms {
		t.Run(term, func(t *testing.T) {
			body, status := makeRequest(t, "GET", fmt.Sprintf("%s/search/comments?q=%s&limit=3", baseURL, term))
			if status != 200 {
				t.Fatalf("Comment search failed: %d", status)
			}

			var results []models.CommentSearchResult
			if err := json.Unmarshal(body, &results); err != nil {
				t.Fatalf("Failed to parse comment results: %v", err)
			}

			t.Logf("✅ Found %d results for '%s'", len(results), term)
			for i, result := range results {
				if i >= 2 {
					break
				}
				t.Logf("   - %s on %s: \"%s\"", result.HorseName, result.RaceDate, result.Comment)
			}
		})
	}
}

// Test 10: Courses and Meetings
func TestCoursesAndMeetings(t *testing.T) {
	t.Log("=== Testing Courses and Meetings ===")

	// Get all courses
	body, status := makeRequest(t, "GET", fmt.Sprintf("%s/courses", baseURL))
	if status != 200 {
		t.Fatalf("Failed to get courses: %d", status)
	}

	var courses []models.Course
	if err := json.Unmarshal(body, &courses); err != nil {
		t.Fatalf("Failed to parse courses: %v", err)
	}

	t.Logf("✅ Found %d courses", len(courses))

	if len(courses) == 0 {
		t.Fatal("Expected courses data")
	}

	// Get meetings for first course
	course := courses[0]
	t.Logf("Getting meetings for %s (ID: %d)", course.CourseName, course.CourseID)

	body, status = makeRequest(t, "GET", fmt.Sprintf("%s/courses/%d/meetings?date_from=2024-01-01&date_to=2024-12-31", baseURL, course.CourseID))
	if status != 200 {
		t.Fatalf("Failed to get meetings: %d", status)
	}

	var meetings []models.Meeting
	if err := json.Unmarshal(body, &meetings); err != nil {
		t.Fatalf("Failed to parse meetings: %v", err)
	}

	t.Logf("✅ Found %d meetings at %s", len(meetings), course.CourseName)
	if len(meetings) > 0 {
		t.Logf("   Latest meeting: %s (%d races)", meetings[0].RaceDate, meetings[0].RaceCount)
	}
}
