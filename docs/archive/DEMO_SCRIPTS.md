# Demo Scripts - Usage Guide

All demo scripts accept parameters for flexible testing.

---

## 🐎 Horse Journey Demo

**Script:** `./demo_horse_journey.sh [horse_name]`

Shows complete search → profile → odds journey for any horse.

### Usage Examples:

```bash
# Default (Captain Scooby - 195 runs)
./demo_horse_journey.sh

# Enable (12 wins from 14 runs, 85.7% SR)
./demo_horse_journey.sh "Enable"

# Frankel (undefeated champion, 14 wins from 14 runs!)
./demo_horse_journey.sh "Frankel"

# Any horse name
./demo_horse_journey.sh "Sea The Stars"
./demo_horse_journey.sh "Kauto Star"
```

### What It Shows:
- ✅ Search results with similarity scores
- ✅ Career summary (runs, wins, strike rate, prize money)
- ✅ Last 3 runs with **both Betfair SP and Bookmaker SP**
- ✅ Going performance splits
- ✅ Distance performance splits
- ✅ Top courses

### Example Output (Frankel):
```
Step 1: Searching for 'Frankel'...
✅ Found: Frankel (GB) (ID: 134020, Score: 0.73)

Career Summary:
  Total Runs: 14
  Wins: 14
  Strike Rate: 100.0%  ← Undefeated!
  Peak RPR: 143
  
Last 3 Runs:
  1. 2012-10-20 - Ascot
     Position: 1
     Betfair SP: 1.29
     Bookmaker SP: 1.18
     
Going Performance:
  Good: 6 runs, 6 wins (100% SR)
  Good To Firm: 3 runs, 3 wins (100% SR)
```

---

## 🎯 Betting Angle Demo

**Script:** `./demo_angle.sh [date]`

Shows near-miss-no-hike angle backtest for any date/month.

### Usage Examples:

```bash
# Default (2024-01-15 and January backtest)
./demo_angle.sh

# Specific date
./demo_angle.sh 2024-01-20
./demo_angle.sh 2024-02-15

# Today's date
./demo_angle.sh $(date +%Y-%m-%d)

# Test different months
./demo_angle.sh 2024-03-10
./demo_angle.sh 2024-06-15
```

### What It Shows:
- ✅ Backtest summary (SR, ROI)
- ✅ Sample qualifying cases
- ✅ Detailed P/L breakdown
- ✅ Today's qualifiers (if racecard data exists)

### Example Output (January 2024):
```
BACKTEST MODE: January 2024
Summary:
  Total Qualifiers: 10
  Winners: 4
  Strike Rate: 40.0%
  ROI: +6.90%
  
Sample Case:
  Admirable Lad (GB)
  Last Run: 2024-01-13 - Pos 2, Beaten 1.5L
  Next Run: 2024-01-17 - Pos 1, Won: True
  BSP: 3.63
  P/L: +2.63 units
```

---

## ✅ API Verification

**Script:** `./verify_api.sh`

Quick health check of all major endpoints.

### Usage:
```bash
./verify_api.sh
```

### Tests:
- Health endpoint
- Courses list
- Global search
- Race search
- Race details with runners

---

## 🧪 Start Server

**Script:** `./start_server.sh`

Starts the API server with proper cleanup.

### Usage:
```bash
./start_server.sh
```

### Features:
- Kills existing processes on port 8000
- Starts server in background
- Verifies startup
- Shows log location

**Output:**
```
✅ Server is running on http://localhost:8000
   Health: http://localhost:8000/health
   API: http://localhost:8000/api/v1/
   Logs: tail -f /tmp/giddyup-api.log
```

---

## 🧪 Test Suites

### Comprehensive Tests
**Script:** `./run_comprehensive_tests.sh`

Runs all 24 core API tests.

```bash
./run_comprehensive_tests.sh
```

### Angle Tests
**Script:** Run with go test

```bash
go test -v ./tests/angle_test.go
```

---

## 📊 Quick Comparisons

### Famous Horses Data:

**Frankel:**
- 14 runs, 14 wins (100% SR)
- Peak RPR: 143
- Prize: £3.0M
- Retired undefeated

**Enable:**
- 14 runs, 12 wins (85.7% SR)
- Peak RPR: 128
- Prize: £3.1M
- Multiple Arc winner

**Captain Scooby:**
- 195 runs, 18 wins (9.2% SR)
- Peak RPR: 83
- Prize: £84.5K
- Hardest working horse in database!

---

## 🎯 Real-World Usage

### Find Horses Similar to Frankel
```bash
./demo_horse_journey.sh "Frankel"
# Then manually search for horses with similar:
# - Going: Good/Good To Firm
# - Distance: 8-10f
# - RPR: 130+
```

### Test Angle on Different Periods
```bash
# Test each month of 2024
for month in {01..12}; do
  ./demo_angle.sh 2024-$month-15
done
```

### Search for Trainer's Horses
```bash
# Search for trainer first
curl "http://localhost:8000/api/v1/search?q=Gosden"

# Then get trainer profile
curl "http://localhost:8000/api/v1/trainers/TRAINER_ID/profile"
```

---

## 💡 Tips

### Horse Name Matching:
- Fuzzy matching handles typos: "Fr4nkel" → Frankel
- Partial names work: "Enable" finds "Enable (GB)"
- Multi-word names: "Sea The Stars" (use quotes)

### Date Formats:
- Always use YYYY-MM-DD format
- `$(date +%Y-%m-%d)` for today
- Script calculates month automatically for backtests

### Performance:
- First request may be slower (cold cache)
- Subsequent requests are faster
- Profile queries: ~1 second with indexes

---

## 🚀 All Demo Scripts

```bash
cd /home/smonaghan/GiddyUp/backend-api

# 1. Horse journey (with horse name)
./demo_horse_journey.sh "Enable"

# 2. Betting angle (with date)
./demo_angle.sh 2024-01-15

# 3. Verify endpoints
./verify_api.sh

# 4. Start server
./start_server.sh

# 5. Run tests
./run_comprehensive_tests.sh
```

---

**All scripts are parameterized and ready for flexible testing!** ✅

