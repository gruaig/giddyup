# Data Quality Issues Investigation

**Date:** 2025-10-16  
**Reported By:** User  
**Status:** Under Investigation

---

## Issues Identified

### 1. Duplicate Races (Today/Tomorrow)

**Symptom:** Multiple race entries appear for the same race in today/tomorrow's data.

**Example:** User reported seeing duplicate races in the UI.

**Suspected Causes:**
- Race key generation may not be consistent
- Sporting Life API returning the same race multiple times
- Auto-update service running multiple times

**Investigation Needed:**
```sql
-- Check for duplicates
SELECT race_date, COUNT(*) as total, COUNT(DISTINCT race_key) as unique_keys
FROM racing.races 
WHERE race_date IN ('2025-10-16', '2025-10-17')
GROUP BY race_date;

-- Find specific duplicates
SELECT r.race_name, r.off_time, c.course_name, COUNT(*) as count
FROM racing.races r
JOIN racing.courses c ON c.course_id = r.course_id
WHERE r.race_date = '2025-10-16'
GROUP BY r.race_name, r.off_time, c.course_name
HAVING COUNT(*) > 1;
```

---

### 2. Missing Race Results / Positions

**Symptom:** Historical races show "-" for position and winner data.

**Example from 2025-10-10:**
```
15:50 | 12 runners
Pos  Horse                      Draw  Form  Price  RPR  OR   BSP   BTN  Trainer  Jockey
-    Howd'yadoit                -     -     20.00  -    -    20.0  -    ...
-    Kansas (IRE)               -     -     2.65   -    -    2.65  -    ...
```

**Root Cause:** **Sporting Life scraper only fetches UPCOMING race cards**
- No results scraping implemented
- Positions (`pos_raw`) are never populated for past races
- BSP data comes from Betfair CSVs, but positions do not

**Missing Functionality:**
```
❌ NO results scraper exists
❌ NO position data import from any source
❌ NO "fetch results" command for historical races
```

**Recommended Solution:**
1. **Create results scraper** for Sporting Life results pages
2. **Or** use Racing Post results (if accessible)
3. **Or** use Betfair result data (has positions + distances beaten)
4. Update `fetch_all` to populate positions for past races

---

### 3. Missing Course Names in Horse Profiles

**Symptom:** Horse profile form showing "-" instead of course names.

**Example from horse #2131337:**
```
Date       Course   Dist  Going         Pos  BTN   RPR  OR   BSP   Trainer      Jockey
27/09/25   Newmarket 6f   Good To Firm  5    3L    96   101  2.86  A P OBrien   Tom Marquand
16/08/25   -        5f   Good          2    hd    95   92   1.63  A P OBrien   Ryan Moore
25/07/25   -        5f   Good To Firm  2    1.3L  87   89   1.51  A P OBrien   Wayne Lordan
```

**Root Cause:** **course_id is NULL or 0 for some historical races**

**Why This Happens:**
1. **Course not found during batch upsert**
   - Course name from source doesn't match database
   - Normalization fails (e.g., "Curragh" vs "The Curragh")
   - Foreign course not in `racing.courses` table

2. **Course lookup fails**
   ```go
   // From batch_upsert.go
   courseIDs, err := upsertCoursesAndFetchIDs(tx, courses)
   // If lookup fails → CourseID stays 0
   
   // From autoupdate.go
   nullInt64(race.CourseID)  // If 0 → becomes NULL in DB
   ```

3. **Profile query returns NULL**
   ```sql
   SELECT c.course_name  -- Returns NULL if course_id is NULL
   FROM racing.runners run
   LEFT JOIN racing.races r ON r.race_id = run.race_id
   LEFT JOIN racing.courses c ON c.course_id = r.course_id
   ```

**Investigation Needed:**
```sql
-- Find races with missing course_id
SELECT race_date, COUNT(*) as count
FROM racing.races
WHERE course_id IS NULL OR course_id = 0
GROUP BY race_date
ORDER BY race_date DESC
LIMIT 20;

-- Find which courses are missing
SELECT DISTINCT r.race_name, r.off_time
FROM racing.races r
WHERE r.course_id IS NULL OR r.course_id = 0
AND r.race_date >= '2025-07-01'
LIMIT 20;

-- Check courses table for normalization issues
SELECT course_id, course_name, region
FROM racing.courses
WHERE course_name ILIKE '%curragh%'
   OR course_name ILIKE '%naas%'
   OR course_name ILIKE '%leopardstown%';
```

**Potential Fixes:**
1. **Add missing courses to database**
2. **Improve course name normalization**
3. **Log failed course lookups** for debugging
4. **Fallback**: Store course name as string in races table (not ideal)

---

## Immediate Actions Required

### Priority 1: Results Scraping (CRITICAL)
- [ ] Implement results scraper for Sporting Life
- [ ] Or identify alternative results data source
- [ ] Update `fetch_all` to populate `pos_raw` for historical dates
- [ ] Backfill positions for existing historical data

### Priority 2: Course Name Debugging
- [ ] Query database to find races with NULL course_id
- [ ] Identify which course names are failing lookup
- [ ] Add missing courses to `racing.courses` table
- [ ] Improve course name normalization if needed
- [ ] Add debug logging to batch_upsert.go for failed lookups

### Priority 3: Duplicate Prevention
- [ ] Query database to confirm duplicates exist
- [ ] Check if issue is in Sporting Life API response
- [ ] Verify race_key generation is deterministic
- [ ] Add logging to show when duplicates are skipped

---

## Long-term Improvements

1. **Data Validation Dashboard**
   - Show data completeness by date
   - Highlight missing positions, course_id, etc.
   - Alert on duplicates

2. **Results Pipeline**
   - Automatic results fetch next morning
   - Backfill positions for yesterday's races
   - Quality checks

3. **Course Master Data**
   - Comprehensive course list (UK, IRE, international)
   - Aliases/variations handling
   - Region mapping

---

## Next Steps

1. **Run diagnostic queries** (need psql access or create Go diagnostic tool)
2. **Check Sporting Life for results pages** (do they exist?)
3. **Review Betfair data** (does it have positions?)
4. **Create results scraper** or import pipeline

---

**Status:** Awaiting database access to run diagnostic queries and confirm root causes.

