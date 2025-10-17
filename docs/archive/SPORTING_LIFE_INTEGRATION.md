# Sporting Life Integration - Complete Implementation

## ✅ What's Implemented

### 1. Sporting Life Scraper
**File:** `backend-api/internal/scraper/sportinglife.go`

**Features:**
- ✅ Fetches all 36 UK/IRE races (vs 15 from Racing Post)
- ✅ Works for today, tomorrow
- ✅ Gets full runner data: horse, jockey, trainer, owner, weights, OR
- ✅ Includes commentary, form, headgear
- ✅ Anti-detection: 15 rotating user agents
- ✅ Rate limited: 400ms between requests
- ✅ Full browser headers to avoid bot detection

**How it works:**
1. Fetch main page (`/racecards/today`) - gets list of 36 races
2. Fetch each individual race page - gets runner details
3. Parse JSON from `__NEXT_DATA__` script tag
4. Convert to internal Race format

### 2. Data Sources Strategy

| Date Range | Source | Endpoint | Why |
|------------|--------|----------|-----|
| **Tomorrow** | Sporting Life | `/racecards/tomorrow` | Forward-looking, has Betfair markets |
| **Today** | Sporting Life | `/racecards/today` | Live races with full data |
| **Yesterday** | Racing Post | `/results/YYYY-MM-DD` | Official results + RPR ratings |
| **Older** | Racing Post | `/results/YYYY-MM-DD` | Historical results |

### 3. Configuration
**File:** `settings.env`

```bash
export USE_SPORTING_LIFE=true   # Use for today/tomorrow (default)
export USE_RACING_POST=false    # Fallback (disabled by default)
```

### 4. Auto-Update Flow

**On Server Startup:**
1. Backfill missing historical dates (Racing Post `/results`)
2. **Always** fetch TODAY (Sporting Life)
3. **Always** fetch TOMORROW (Sporting Life)  
4. Start live Betfair prices for both

**Midnight Rollover (Future):**
- Yesterday gets results backfill
- Today becomes yesterday
- Tomorrow becomes today
- New tomorrow is fetched

## 📊 Data Quality Comparison

| Field | Racing Post | Sporting Life |
|-------|-------------|---------------|
| Races Found (Today) | 15/36 ❌ | 36/36 ✅ |
| HTTP Requests | 37 | 37 (1+36) |
| Horse IDs | URL-based | Real IDs ✅ |
| Jockey IDs | URL-based | Real IDs ✅ |
| Trainer IDs | URL-based | Real IDs ✅ |
| Owner | ❌ | ✅ |
| Commentary | ❌ | ✅ |
| Form | ❌ | ✅ |
| Odds | ❌ | ✅ |
| Time Format | 12-hour | 24-hour ✅ |
| Starting Price | ❌ | ✅ |

## 🚀 Current Status

✅ Sporting Life scraper fully implemented
✅ Anti-detection measures in place
✅ Integrated into auto-update service
✅ Fetches today + tomorrow on startup
✅ Rate limited to be respectful

## 📝 Example Output

```
[SportingLife] Fetching race list for 2025-10-15...
[SportingLife] Found 36 UK/IRE races, fetching runner details...
[SportingLife] Fetching race 1/36: Nottingham
[SportingLife]    ✓ Got 11 runners for race 0
[SportingLife] Fetching race 2/36: Nottingham
[SportingLife]    ✓ Got 9 runners for race 0
...
[SportingLife] ✅ Fetched 36 races with full runner data
[AutoUpdate]   ✓ Upserted 295 horses, 198 trainers, 180 jockeys, 285 owners
[AutoUpdate] ✅ TODAY loaded: 36 races, 340 runners
[AutoUpdate] ✅ TOMORROW loaded: 36 races, 338 runners
```

## 🔧 Technical Details

### Anti-Detection Features
1. **User Agent Rotation**: 15 different browsers (Chrome, Firefox, Safari, Edge, Opera)
2. **Rate Limiting**: 400ms minimum between requests
3. **Browser Headers**: Accept-Language, DNT, Referer, Connection, etc.
4. **Randomization**: Different user agent per request
5. **Politeness**: Automatic delays prevent server overload

### Error Handling
- Graceful fallback if race page returns 404
- Continues scraping even if individual races fail
- Logs warnings for failed races
- Falls back to Racing Post if Sporting Life fails entirely

## 📅 Tomorrow's Races

- Automatically fetched on server startup
- Stored with `prelim=true`
- Betfair markets available 12-24h before
- UI can show tomorrow's card with live odds

## 🔄 Daily Cycle (Planned)

```
00:01 - Midnight Rollover:
  1. Backfill yesterday's results (was today)
  2. Delete old prelim races (>3 days)
  3. Fetch new today (was tomorrow)
  4. Fetch new tomorrow
  5. Start Betfair prices
```

## 🎯 Next Steps

- [x] Sporting Life scraper complete
- [x] Anti-detection added
- [x] Today + tomorrow integration
- [ ] Verify Betfair matching works
- [ ] Create rollover service for midnight
- [ ] Update UI documentation

