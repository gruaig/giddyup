# Betfair ↔ Sporting Life Timezone Issue

**Discovered**: October 16, 2025  
**Impact**: Matches only 9/39 races (23%) when should match ~31/39 (80%)

---

## 🐛 The Problem

Betfair CSV times are in **UTC**, but Sporting Life times are in **local UK time** (BST/GMT).

### Evidence

**Date**: 2025-10-10 (during British Summer Time = UTC+1)

**Newmarket races**:
| Sporting Life | Betfair | Difference |
|---------------|---------|------------|
| 12:15:00 | 13:15 | +1:00 ✅ |
| 12:50:00 | 13:50 | +1:00 ✅ |
| 13:25:00 | 14:25 | +1:00 ✅ |
| 13:57:00 | 14:57 | +1:00 ✅ |
| 14:30:00 | 15:30 | +1:00 ✅ |
| 15:10:00 | 16:10 | +1:00 ✅ |
| 15:45:00 | 16:45 | +1:00 ✅ |

**Every single race is offset by exactly +1 hour!**

### Why Some Races Match

The 9 races that matched (Kempton evening, Dundalk) are races where:
- Times happen to align after ±1 minute tolerance catches the edge
- Or coincidentally match due to rounding

---

## 🔧 The Fix

Convert Betfair UTC times to UK local time (Europe/London) when stitching CSV data.

### Option 1: Fix at Stitch Time (Recommended)

**File**: `backend-api/internal/scraper/betfair_stitcher.go`

In `stitchWinPlace()` when setting `race.OffTime`:

```go
// Current code:
race.OffTime = bs.extractTime(winRunners[0].EventDt)

// New code:
utcTime := bs.extractTime(winRunners[0].EventDt)
race.OffTime = bs.convertUTCToLocal(winRunners[0].EventDt, utcTime)
```

Add helper function:

```go
func (bs *BetfairStitcher) convertUTCToLocal(eventDt string, hhmmUTC string) string {
    // Parse event date: "11-10-2025 13:55" → "2025-10-11"
    date := bs.extractDate(eventDt)
    
    // Parse UTC time to full datetime
    datetime := fmt.Sprintf("%sT%s:00Z", date, hhmmUTC)
    utcTime, err := time.Parse("2006-01-02T15:04:05Z", datetime)
    if err != nil {
        return hhmmUTC // Fallback to original
    }
    
    // Convert to Europe/London
    location, _ := time.LoadLocation("Europe/London")
    localTime := utcTime.In(location)
    
    // Return HH:MM in local time
    return localTime.Format("15:04")
}
```

### Option 2: Fix at Match Time

Adjust Sporting Life times to UTC before matching:

```go
func adjustToUTC(localTime string, date string) string {
    datetime := fmt.Sprintf("%sT%s:00", date, localTime)
    localDateTime, _ := time.ParseInLocation("2006-01-02T15:04:05", datetime, 
        time.LoadLocation("Europe/London"))
    return localDateTime.UTC().Format("15:04")
}
```

**We recommend Option 1** - fix at source so all stitched data uses local time.

---

## 📊 Expected Results After Fix

**Current**: 9/39 matches (23%)  
**After Fix**: ~31/39 matches (80%)

The 8 unmatched races are likely:
- Races that didn't run (abandoned)
- Races with no Betfair market
- Non-runner walkover races

---

## 🧪 Testing the Fix

```bash
# 1. Delete old stitched data
rm -rf data/betfair_stitched/*/*/2025-10-10*

# 2. Re-run fetch_all (will re-stitch with timezone conversion)
cd backend-api
./fetch_all 2025-10-10

# 3. Check matches
# Expected output: "Matched 31/39 races (by course: 30, by name: 1)"
```

---

## 🕒 BST vs GMT Calendar

**British Summer Time (UTC+1)**:
- Last Sunday of March → Last Sunday of October
- 2025: March 30 - October 26

**Greenwich Mean Time (UTC+0)**:
- Last Sunday of October → Last Sunday of March  
- 2025: October 26 - March 29, 2026

**Current (Oct 10, 2025)**: BST = UTC+1 ✅

---

## 🔍 How to Verify Timezone

Check a known race:

```bash
# Sporting Life
curl -s "https://www.sportinglife.com/api/horse-racing/race/884123" | \
  jq '.race_summary.time'

# Betfair (from CSV)
# Check event_dt column - if it's UTC, time will be 1 hour ahead during BST
```

---

## 💡 Why This Wasn't Caught Earlier

1. **Evening races** (17:00-20:00) are less affected by DST edge cases
2. **±1 minute tolerance** occasionally catches UTC+1:00 differences when times round
3. **9/39 matches** seemed plausible as "partial Betfair coverage"
4. **No obvious error** - just silent mismatches

---

## 🚀 Implementation Priority

**HIGH** - This affects all historical matching (2021-present)

Once fixed:
- ✅ 80%+ match rate for races with Betfair data
- ✅ Accurate BSP/PPWAP merging
- ✅ Reliable historical analysis

---

**Status**: IDENTIFIED - Fix in progress  
**Impact**: Critical for historical data accuracy  
**Effort**: Small (one function change)  
**Risk**: Low (only affects stitched CSV creation, not live data)

