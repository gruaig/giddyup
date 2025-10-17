# 🚨 URGENT: Data Quality Issues Found

**Date:** 2025-10-16  
**Status:** ACTION REQUIRED

---

## 🔴 Issue #1: Duplicate Races (CRITICAL)

### Root Cause: Inconsistent Race Key Generation

**Problem:** Different `generateRaceKey()` functions across codebase!

```go
// autoupdate.go (line 488) - Uses MD5 hash
func generateRaceKey(race scraper.Race) string {
    data := fmt.Sprintf("%s|%s|%s|%s|%s|%s", race.Date, normRegion, normCourse, normTime, normName, normType)
    hash := md5.Sum([]byte(data))
    return fmt.Sprintf("%x", hash)  // ← MD5 hash!
}

// fetch_all.go (line 481) - Uses plain string!
func generateRaceKey(race scraper.Race) string {
    return fmt.Sprintf("%s|%s|%s|%s", race.Date, scraper.NormalizeName(race.Course), race.OffTime, scraper.NormalizeName(race.RaceName))
    // ← Plain string! Different format!
}
```

**Result:** Same race gets TWO different keys → duplicates in database!

### Fix Required

**Option A: Use MD5 everywhere (recommended)**
- Update `fetch_all.go` to use the same MD5 hash as `autoupdate.go`
- Add `region` and `race_type` to key generation
- Ensures consistency across all commands

**Option B: Deduplicate on insert**
- Add better conflict handling in database
- May still have issues

---

## 🟠 Issue #2: Missing Race Results / Positions

### Root Cause: NO Results Scraper Exists!

**What We Have:**
- ✅ Sporting Life scraper for **upcoming race cards**
- ✅ Betfair CSV for **historical BSP prices**
- ✅ Betfair API-NG for **live prices**

**What We're Missing:**
- ❌ **NO results scraper** for past races
- ❌ **NO position data** import
- ❌ **NO winner information** for historical races

**Example from 2025-10-10:**
```
All positions show "-" because:
- Sporting Life API only returns upcoming racecards
- No results page scraping implemented
- Betfair CSV has prices but NOT positions
```

### Why This Matters

**UI Impact:**
- Horse profiles show incomplete form (no positions)
- Historical race pages missing winners
- Analysis tools can't calculate win/place rates

**Missing Fields:**
- `pos_raw` (position: "1", "2", "3", etc.)
- `btn` (beaten by: "3L", "hd", "nk", etc.)
- Winner details
- Official distances

### Fix Required

**Option A: Scrape Sporting Life Results** (recommended)
```
URL pattern: https://www.sportinglife.com/racing/results/YYYY-MM-DD
```
- Create `results.go` scraper
- Parse results pages for positions
- Update existing races with position data

**Option B: Use Betfair Result Data**
- Betfair has result data in separate endpoints
- Includes positions + distances beaten
- May require additional API calls

**Option C: Manual CSV Import**
- Import results from external source
- One-time backfill
- Not sustainable

---

## 🟡 Issue #3: Missing Course Names in Profiles

### Root Cause: course_id is NULL for Some Races

**Why This Happens:**

1. **Course Lookup Fails**
   ```go
   // From batch_upsert.go
   courseIDs, err := upsertCoursesAndFetchIDs(tx, courses)
   // If lookup fails → CourseID stays 0
   ```

2. **Normalization Mismatch**
   - Database has "Curragh"
   - Sporting Life returns "The Curragh" 
   - Normalization fails → course_id = NULL

3. **Foreign Courses Missing**
   - French courses not in `racing.courses`
   - German courses not in `racing.courses`
   - Returns NULL for unknown courses

**Profile Query Impact:**
```sql
SELECT c.course_name  -- Returns NULL if course_id is NULL!
FROM racing.runners run
LEFT JOIN racing.races r ON r.race_id = run.race_id
LEFT JOIN racing.courses c ON c.course_id = r.course_id
WHERE run.horse_id = 2131337
```

### Fix Required

**Option A: Add Logging for Failed Lookups**
```go
// In batch_upsert.go
if id, ok := courseIDs[courseName]; !ok {
    log.Printf("⚠️  Course lookup failed: %s (region: %s)", courseName, region)
}
```

**Option B: Store Course Name in Races Table**
- Add `course_name_raw` column
- Fallback if `course_id` is NULL
- Quick fix but denormalized

**Option C: Improve Course Normalization**
- Better `NormalizeName()` function
- Handle "The X" vs "X" variations
- Add course aliases table

---

## 🎯 Priority Action Plan

### 🔥 Immediate (Today)

1. **Fix Duplicate Race Keys** ← CRITICAL
   - [ ] Update `fetch_all.go` generateRaceKey() to match `autoupdate.go`
   - [ ] Add MD5 hashing
   - [ ] Include all 6 components (date, region, course, time, name, type)
   - [ ] Test: Run fetch_all and verify no duplicates

2. **Add Course Lookup Logging**
   - [ ] Add debug logging to `batch_upsert.go`
   - [ ] Run fetch_all for Oct 10-16
   - [ ] Capture which courses are failing lookup

### 📅 This Week

3. **Implement Results Scraper**
   - [ ] Research Sporting Life results page structure
   - [ ] Create `results.go` scraper
   - [ ] Parse positions, BTN, winner details
   - [ ] Create `fetch_results <date>` command
   - [ ] Backfill Oct 10-15 results

4. **Fix Course Matching**
   - [ ] Review failed course lookups from logs
   - [ ] Add missing courses to database
   - [ ] Improve normalization if needed
   - [ ] Re-run historical imports

---

## 📊 Diagnostic Queries Needed

**To run when psql is available:**

```sql
-- Check for duplicate races
SELECT race_date, COUNT(*) as total, COUNT(DISTINCT race_key) as unique
FROM racing.races 
WHERE race_date BETWEEN '2025-10-10' AND '2025-10-17'
GROUP BY race_date;

-- Find races with missing course_id
SELECT race_date, COUNT(*) 
FROM racing.races
WHERE course_id IS NULL OR course_id = 0
GROUP BY race_date
ORDER BY race_date DESC;

-- Check position data completeness
SELECT race_date,
       COUNT(*) as total_runners,
       COUNT(CASE WHEN pos_raw IS NOT NULL THEN 1 END) as with_position,
       ROUND(100.0 * COUNT(CASE WHEN pos_raw IS NOT NULL THEN 1 END) / COUNT(*), 2) as pct_complete
FROM racing.runners
WHERE race_date BETWEEN '2025-10-01' AND '2025-10-16'
GROUP BY race_date
ORDER BY race_date DESC;
```

---

## 📝 Summary

| Issue | Severity | Impact | Fix Complexity | ETA |
|-------|----------|--------|----------------|-----|
| Duplicate races | 🔴 CRITICAL | Data corruption | Easy (1 line change) | 30 min |
| Missing positions | 🟠 HIGH | UI incomplete | Medium (new scraper) | 4-6 hours |
| Missing courses | 🟡 MEDIUM | Profile gaps | Easy (add logging) | 1 hour |

**Total estimated effort:** 1 day to fix all issues

---

## ✅ Next Steps

1. **You:** Approve priority and approach
2. **Me:** Fix duplicate race key issue (30 min)
3. **Me:** Add course lookup logging (1 hour)
4. **Me:** Build results scraper (4-6 hours)
5. **You:** Run backfill for Oct 10-16
6. **Both:** Verify data quality improved

Ready to start with fixing the duplicate race key issue?

