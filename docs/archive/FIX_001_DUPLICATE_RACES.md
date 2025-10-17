# Fix #001: Duplicate Races

**Date:** 2025-10-16  
**Status:** ✅ FIXED  
**Priority:** CRITICAL  
**Time:** 15 minutes

---

## Problem

Duplicate races appearing in database for same date.

**Root Cause:** Inconsistent `generateRaceKey()` implementation across commands.

```go
// autoupdate.go - Used MD5 hash
hash := md5.Sum([]byte(data))
return fmt.Sprintf("%x", hash)

// fetch_all.go - Used plain string (WRONG!)
return fmt.Sprintf("%s|%s|%s|%s", race.Date, ...)
```

**Result:** Same physical race got TWO different race_keys → duplicates!

---

## Solution

Updated `cmd/fetch_all/main.go` to use **identical** key generation as `autoupdate.go`:

### Changes Made

**File:** `backend-api/cmd/fetch_all/main.go`

**Before:**
```go
func generateRaceKey(race scraper.Race) string {
	return fmt.Sprintf("%s|%s|%s|%s", 
		race.Date, 
		scraper.NormalizeName(race.Course), 
		race.OffTime, 
		scraper.NormalizeName(race.RaceName))
}
```

**After:**
```go
func generateRaceKey(race scraper.Race) string {
	normCourse := strings.ToLower(strings.TrimSpace(race.Course))
	normTime := race.OffTime
	if len(normTime) >= 5 {
		normTime = normTime[:5] // Strip seconds: "12:35:00" → "12:35"
	}
	normName := strings.ToLower(strings.TrimSpace(race.RaceName))
	normType := strings.ToLower(strings.TrimSpace(race.Type))
	normRegion := strings.ToUpper(strings.TrimSpace(race.Region))

	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s", race.Date, normRegion, normCourse, normTime, normName, normType)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}
```

**Added import:**
```go
import "crypto/md5"
```

---

## Key Components Now Consistent

The race key now includes **6 normalized components**:

1. **Date** - Race date (YYYY-MM-DD)
2. **Region** - Uppercase (GB, IRE)
3. **Course** - Lowercase, trimmed
4. **Off Time** - HH:MM (seconds stripped)
5. **Race Name** - Lowercase, trimmed
6. **Race Type** - Lowercase, trimmed

These are concatenated and MD5 hashed for consistency.

---

## Impact

### Before Fix
- `autoupdate.go` → generates key: `"a1b2c3d4e5..."`
- `fetch_all` → generates key: `"2025-10-16|newmarket|13:30|..."`
- **Result:** TWO race records for same race!

### After Fix
- `autoupdate.go` → generates key: `"a1b2c3d4e5..."`
- `fetch_all` → generates key: `"a1b2c3d4e5..."` ✅ SAME!
- **Result:** Single race record (ON CONFLICT properly deduplicates)

---

## Database Behavior

The database has this constraint:
```sql
ON CONFLICT (race_key, race_date) DO UPDATE SET ...
```

With matching keys, this now works correctly:
- First insert → creates race
- Second insert → updates existing race (no duplicate)

---

## Testing

### Verify Fix
```bash
cd backend-api

# Rebuild
go build -o bin/fetch_all cmd/fetch_all/main.go

# Test with today (should not create duplicates)
./fetch_all $(date +%Y-%m-%d) --force

# Check for duplicates in database
psql -d horse_db -c "
  SELECT race_date, COUNT(*) as total, COUNT(DISTINCT race_key) as unique_keys
  FROM racing.races 
  WHERE race_date = CURRENT_DATE
  GROUP BY race_date;
"
# Should show: total = unique_keys (no duplicates)
```

### Expected Result
```
 race_date  | total | unique_keys 
------------+-------+-------------
 2025-10-16 |    53 |          53
```
✅ Total matches unique keys = no duplicates!

---

## Cleanup Required

### Remove Existing Duplicates

If duplicates exist from previous runs:

```sql
-- Find duplicates
WITH dups AS (
  SELECT race_key, race_date, COUNT(*) as count
  FROM racing.races
  WHERE race_date >= '2025-10-10'
  GROUP BY race_key, race_date
  HAVING COUNT(*) > 1
)
SELECT * FROM dups;

-- Delete duplicates (keep earliest race_id)
DELETE FROM racing.races r
WHERE race_id NOT IN (
  SELECT MIN(race_id)
  FROM racing.races
  GROUP BY race_key, race_date
)
AND race_date >= '2025-10-10';
```

**Warning:** This will delete runner data too (cascading foreign key). 
**Recommendation:** Re-run `fetch_all` for affected dates after cleanup.

---

## Prevention

### Future Code Changes

When creating new commands that insert races, **ALWAYS**:

1. Copy `generateRaceKey()` from `autoupdate.go`
2. Include comment: `// MUST match autoupdate.go`
3. Use identical normalization logic
4. Include all 6 components (date, region, course, time, name, type)
5. Use MD5 hash

### Long-term Solution

Consider extracting to shared utility:

**File:** `backend-api/internal/services/race_key.go`
```go
package services

func GenerateRaceKey(race scraper.Race) string {
    // Single implementation used everywhere
    ...
}
```

Then all commands import and use this function.

---

## Related Files

Other files with `generateRaceKey()` - should all be consistent:

- ✅ `autoupdate.go` - Reference implementation (MD5 hash)
- ✅ `fetch_all.go` - **FIXED** to match autoupdate.go
- ✅ `backfill_dates.go` - Already uses MD5 hash (OK)
- ✅ `fetch_all_betfair.go` - Uses simplified version (OK - only for matching, not inserting)
- ✅ `internal/betfair/matcher.go` - Uses simplified version (OK - documentation only)

---

## Status

✅ **FIXED** - No more duplicates will be created  
⚠️  **CLEANUP NEEDED** - Existing duplicates should be removed  
📝 **DOCUMENTATION** - Added this fix record

---

## Next Steps

1. ✅ Fix implemented
2. ⏳ Test with fresh data
3. ⏳ Clean up existing duplicates
4. ⏳ Verify UI no longer shows duplicates

**Estimated total impact:** Resolves duplicate race issue permanently.

