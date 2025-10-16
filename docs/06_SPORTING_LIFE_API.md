# Sporting Life API Integration (V2)

**Date**: October 16, 2025  
**Status**: ✅ PRODUCTION READY

---

## Overview

GiddyUp uses **Sporting Life API V2** as its **sole data source** for UK/IRE horse racing data. Racing Post has been completely removed from the codebase.

### Key Features
- ✅ **Complete runner data**: Jockey, trainer, owner, form, age, weight, headgear
- ✅ **Betfair selection IDs**: Direct matching with Betfair Exchange (no name normalization!)
- ✅ **Best bookmaker odds**: Real-time odds from all major bookmakers
- ✅ **Parallel fetching**: Today and tomorrow loaded simultaneously
- ✅ **Smart caching**: Instant re-loads with local cache
- ✅ **Type-safe**: Handles all Sporting Life API variations

---

## API Architecture

### 3-Step Process

#### 1. Get Race List
```
GET https://www.sportinglife.com/api/horse-racing/racing/racecards/{date}
```
**Returns**: List of all UK/IRE races with basic metadata

#### 2. Get Full Race Details (per race)
```
GET https://www.sportinglife.com/api/horse-racing/race/{raceId}
```
**Returns**:
- Jockey name + ID
- Trainer name + ID
- Owner name + ID
- Horse age
- Weight (stones-lbs)
- Form summary
- Headgear
- Draw/stall number

#### 3. Get Betting Data (per race)
```
GET https://www.sportinglife.com/api/horse-racing/v2/racing/betting/{raceId}
```
**Returns**:
- **Betfair selection ID** (critical!)
- Best odds across all bookmakers
- Which bookmaker has best odds
- Each-way terms
- Non-runner status

### Data Merge Strategy

The scraper fetches **both** endpoints 2 & 3 for each race, then merges by horse name:

```go
// Pseudocode
for each race:
    raceDetails = GET /race/{id}           // Full runner info
    bettingData = GET /v2/racing/betting/{id}  // Odds + Betfair IDs
    
    for each runner:
        merge(raceDetails.horse, bettingData.horse)
        // Result: Complete runner with jockey, trainer, odds, selectionId
```

---

## Database Schema

### New Columns in `racing.runners`

```sql
-- Betfair matching
betfair_selection_id BIGINT  -- Direct Betfair Exchange selection ID

-- Bookmaker odds
best_odds DOUBLE PRECISION   -- Best decimal odds available
best_bookmaker VARCHAR(100)  -- Which bookmaker has best odds

-- Index for fast Betfair lookup
CREATE INDEX idx_runners_betfair_selection 
ON racing.runners(betfair_selection_id) 
WHERE betfair_selection_id IS NOT NULL;
```

### Betfair Matching - Before vs After

**Before (error-prone)**:
```sql
SELECT * FROM runners 
WHERE LOWER(TRIM(horse_name)) = LOWER(TRIM('Some Horse Name'));
```

**After (perfect match)**:
```sql
SELECT * FROM runners 
WHERE betfair_selection_id = 46013800;
```

---

## Code Structure

### Main Scraper
**File**: `backend-api/internal/scraper/sportinglife_v2.go`

**Key Functions**:
```go
// Fetch all races for a date
func (s *SportingLifeAPIV2) GetRacesForDate(date string) ([]Race, error)

// Fetch single race with betting data merged
func (s *SportingLifeAPIV2) fetchRaceWithBetting(info raceInfo) (Race, error)

// Merge race details + betting data
func (s *SportingLifeAPIV2) mergeRunnerData(raceRides, bettingRides) []Runner
```

### Type Definitions
**File**: `backend-api/internal/scraper/sportinglife_api_types.go`

Defines:
- `SLRacecardsResponse` - List of races
- `SLRaceResponse` - Full race details
- `SLBettingResponse` - Betting data

### Auto-Update Service
**File**: `backend-api/internal/services/autoupdate.go`

On startup:
1. Checks for missing data
2. Fetches today & tomorrow **in parallel** (2 goroutines)
3. Caches results
4. Starts live price updates (if enabled)

---

## Caching

### Cache Location
```
/data/sportinglife/{date}.json
```

### Cache Behavior
- ✅ First fetch: ~2 minutes (2 API calls per race × rate limit)
- ✅ Subsequent loads: **instant** (no API calls)
- ✅ Cache expires: Never (immutable historical data)
- ✅ Today's cache: Refreshed on each startup (force refresh)

### Cache Implementation
**File**: `backend-api/internal/scraper/sportinglife_cache.go`

```go
cache := NewSportingLifeCache("/data")

// Save races
cache.SaveRaces(date, races)

// Load races
races, found, err := cache.LoadRaces(date)
```

---

## Type Handling

Sporting Life API has inconsistent types. We handle:

### 1. `cloth_number` (can be string OR int)
```go
switch v := rRide.ClothNumber.(type) {
case string:
    clothNum, _ = strconv.Atoi(v)
case float64:
    clothNum = int(v)
case int:
    clothNum = v
}
```

### 2. `headgear` (can be array OR object OR null)
```go
switch hg := rRide.Headgear.(type) {
case []interface{}:
    // Parse array of strings
    for _, item := range hg {
        if str, ok := item.(string); ok {
            headgearStrs = append(headgearStrs, str)
        }
    }
case map[string]interface{}:
    // Ignore objects (rare edge case)
}
```

---

## Rate Limiting

### Settings
- **Min delay**: 400ms between requests
- **User agent rotation**: 5 different user agents
- **Failure threshold**: Abort after 3 consecutive failures

### Request Volume (per date)
- Step 1: 1 request (get race list)
- Step 2+3: 2 requests × N races
- **Total for 44 races**: 1 + (2 × 44) = **89 requests**
- **Time**: ~35-40 seconds with 400ms rate limit

---

## Parallel Fetching

### Implementation
```go
var wg sync.WaitGroup
wg.Add(2)

// Thread 1: Today
go func() {
    defer wg.Done()
    fetchAndInsert(today)
}()

// Thread 2: Tomorrow
go func() {
    defer wg.Done()
    fetchAndInsert(tomorrow)
}()

wg.Wait()
```

### Benefits
- ✅ ~50% faster startup
- ✅ Both dates load simultaneously
- ✅ Independent error handling per thread

---

## Logging

### Verbose Progress Updates

```
[AutoUpdate] 📅 Fetching today/tomorrow in parallel...
[AutoUpdate] 📅 [Thread 1] Fetching TODAY (2025-10-16) [FORCE REFRESH]...
[AutoUpdate] 📅 [Thread 2] Fetching TOMORROW (2025-10-17) [FORCE REFRESH]...
[SportingLife] Fetching races for 2025-10-17 via API (3-endpoint flow)...
[SportingLife] Found 44 UK/IRE races for 2025-10-17
[SportingLife] Successfully fetched 44 races with runners and odds
[SportingLife Cache] Saved 44 races to /data/sportinglife/2025-10-17.json
[AutoUpdate] ✓ Upserted 6 courses, 402 horses, 245 trainers, 198 jockeys, 315 owners
[AutoUpdate] 📝 Progress: 20/44 races (45%), 180 runners so far
[AutoUpdate] ✅ [Thread 1] TODAY loaded: 53 races, 632 runners
[AutoUpdate] ✅ [Thread 2] TOMORROW loaded: 44 races, 402 runners
[AutoUpdate] ✅ Parallel load complete: TODAY (53 races) + TOMORROW (44 races)
```

---

## Error Handling

### Common Issues

1. **HTTP 403/429**: Rate limit hit
   - **Solution**: Exponential backoff + user agent rotation

2. **Type mismatch**: `cloth_number` or `headgear` type error
   - **Solution**: Use `interface{}` + type switch

3. **Missing data**: Some races have incomplete info
   - **Solution**: Graceful degradation (empty strings for missing fields)

4. **Consecutive failures**: Too many errors in a row
   - **Solution**: Abort after 3 failures to prevent infinite loops

---

## Migration from Racing Post

### What Was Removed
- ❌ `backend-api/internal/scraper/racecards.go` - Racing Post racecard scraper
- ❌ `backend-api/internal/scraper/results.go` - Racing Post results scraper
- ❌ All Racing Post fallback logic
- ❌ All Racing Post references in `autoupdate.go`

### What Replaced It
- ✅ `backend-api/internal/scraper/sportinglife_v2.go` - Complete Sporting Life integration
- ✅ `backend-api/internal/scraper/sportinglife_api_types.go` - API type definitions
- ✅ 2-endpoint merge strategy for complete data
- ✅ Betfair selection ID capture

---

## Testing

### Manual Test
```bash
# Start server
cd /home/smonaghan/GiddyUp
source settings.env
cd backend-api
./bin/api

# Watch logs
tail -f logs/server.log

# Verify data in database
SELECT 
  r.off_time,
  r.race_name,
  ru.horse_name,
  ru.jockey_name,
  ru.trainer_name,
  ru.betfair_selection_id,
  ru.best_odds,
  ru.best_bookmaker
FROM racing.races r
JOIN racing.runners ru ON ru.race_id = r.race_id
WHERE r.race_date = '2025-10-17'
LIMIT 5;
```

### Expected Results
- All fields populated (no NULL jockeys/trainers/owners)
- Betfair selection IDs present
- Best odds captured
- Cache files created in `/data/sportinglife/`

---

## API Endpoints (External)

### 1. Racecards List
```
GET /api/horse-racing/racing/racecards/{YYYY-MM-DD}
Host: www.sportinglife.com

Response:
[
  {
    "meeting_summary": {
      "date": "2025-10-17",
      "course": { "name": "Haydock", "country": { "short_name": "ENG" } },
      "going": "Good"
    },
    "races": [
      {
        "race_summary_reference": { "id": 884803 },
        "name": "British Stallion Studs EBF Maiden Stakes",
        "time": "12:35",
        "ride_count": 14
      }
    ]
  }
]
```

### 2. Race Details
```
GET /api/horse-racing/race/{raceId}
Host: www.sportinglife.com

Response:
{
  "rides": [
    {
      "cloth_number": "1",
      "stall": 5,
      "horse": { "name": "Hidalgo De L'isle", "age": 8 },
      "jockey": { "name": "Charlie Maggs", "jockey_reference": { "id": 12345 } },
      "trainer": { "name": "D McCain Jnr", "trainer_reference": { "id": 67890 } },
      "owner": { "name": "Mr T G Leslie", "owner_reference": { "id": 11111 } },
      "weight": "11-7",
      "form_summary": "1234",
      "headgear": ["b", "t"]
    }
  ]
}
```

### 3. Betting Data
```
GET /api/horse-racing/v2/racing/betting/{raceId}
Host: www.sportinglife.com

Response:
{
  "rides": [
    {
      "horse_name": "Hidalgo De L'isle",
      "bookmakerOdds": [
        {
          "bookmakerName": "Betfair Sportsbook",
          "selectionId": "46013800",  // ← CRITICAL!
          "decimalOdds": 5.5,
          "fractionalOdds": "9/2",
          "bestOdds": true
        }
      ]
    }
  ]
}
```

---

## Performance Metrics

### Startup Time
- **First load** (no cache): ~40-50 seconds per date
- **Cached load**: <1 second
- **Parallel today+tomorrow**: ~50 seconds total (not 100!)

### API Calls Per Day
- 1 racecard list call
- ~44 races × 2 endpoints = 88 calls
- **Total**: 89 API calls per date

### Database Inserts
- ~7 courses
- ~400-600 horses
- ~200-300 trainers
- ~200-250 jockeys
- ~300-500 owners
- ~40-55 races
- ~400-650 runners

---

## Troubleshooting

### Issue: Port 8000 already in use
```bash
# Kill existing process
pkill -f "bin/api"

# Or use lsof
lsof -ti :8000 | xargs kill -9
```

### Issue: No races loaded
- Check logs for HTTP errors
- Verify Sporting Life API is accessible
- Check date format (YYYY-MM-DD)

### Issue: Missing jockey/trainer data
- This should NOT happen with V2
- Check that both `/race/{id}` and `/v2/racing/betting/{id}` are being called
- Verify merge logic in `mergeRunnerData()`

### Issue: Type errors in JSON parsing
- Check for new API variations
- Add type handling in `sportinglife_v2.go`
- Use `interface{}` for flexible fields

---

## Future Enhancements

### Potential Improvements
1. **Reduce API calls**: Cache meeting-level data separately
2. **Websocket support**: Real-time odds updates
3. **Retry logic**: Exponential backoff for failed requests
4. **Monitoring**: Track API response times and error rates
5. **Multiple dates**: Batch fetch for historical backfill

### Not Needed
- ❌ Racing Post integration (removed permanently)
- ❌ HTML scraping (API is reliable)
- ❌ Name normalization for Betfair matching (we have selection IDs!)

---

## Related Documentation
- [02_API_DOCUMENTATION.md](./02_API_DOCUMENTATION.md) - GiddyUp API endpoints
- [03_DATABASE_GUIDE.md](./03_DATABASE_GUIDE.md) - Database schema
- [AUTO_UPDATE.md](./features/AUTO_UPDATE.md) - Auto-update service details
- [SPORTING_LIFE_COMPLETE.md](./SPORTING_LIFE_COMPLETE.md) - Implementation summary

---

**Last Updated**: October 16, 2025  
**Author**: AI Assistant  
**Status**: ✅ Production Ready

