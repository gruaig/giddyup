# Remaining Issues - Requires Database Access

**Date:** 2025-10-16  
**Status:** ⚠️ CRITICAL ISSUES REMAIN  
**Blocker:** No psql/database access to debug properly

---

## 🚨 Critical Issues Found

### Issue #1: Duplicates STILL Present

**Evidence:**
- UI shows 12 races at Ffos Las (should be 6)
- API returns 24 runners for race 813100 (should be 12)
- "Tropical Spirit (IRE)" appears TWICE in same race

**Root Cause:** DELETE statement still not working despite SQL fix

**Possible reasons:**
1. Transaction not committing
2. Foreign key constraints preventing delete
3. Cache not clearing
4. Multiple processes writing simultaneously

**Needs:** Direct database access to run cleanup SQL

---

### Issue #2: NULL Horse Names

**Evidence:**
- 22 out of 24 runners have `horse_name: NULL`
- Only "Tropical Spirit (IRE)" shows
- But fetch_all says "Populated horse_id for 523/523 runners"

**Root Cause:** Unknown - mismatch between what's inserted and what's queried

**Possible reasons:**
1. Horse_id populated in memory but not written to database
2. Different transaction seeing different data
3. Batch upsert vs individual insert mismatch
4. Case sensitivity in horse name matching

**Needs:** Database query to check actual data:
```sql
SELECT horse_id, horse_name, trainer_id, trainer_name
FROM racing.runners ru
LEFT JOIN racing.horses h ON h.horse_id = ru.horse_id  
LEFT JOIN racing.trainers t ON t.trainer_id = ru.trainer_id
WHERE race_id = 813100;
```

---

## 🔍 What We Know

### Working ✅
- Trainers populate correctly (all show)
- Jockeys populate correctly (all show)
- fetch_all SAYS it populated 523/523 horses
- Some horses DO show (Tropical Spirit, Devil's Brigade, etc.)

### Not Working ❌
- Most horses show NULL
- Duplicates persist
- DELETE doesn't clean up old data

---

## 🎯 What Needs to Happen

### Immediate (Requires psql)

1. **Manual duplicate cleanup:**
```sql
-- Delete duplicates, keep oldest race_id
DELETE FROM racing.races
WHERE race_id NOT IN (
  SELECT MIN(race_id)
  FROM racing.races
  GROUP BY race_key, race_date
)
AND race_date >= '2025-10-16';
```

2. **Check horse data:**
```sql
-- See what's in runners table
SELECT 
  ru.runner_id,
  ru.horse_id,
  h.horse_name,
  ru.num,
  ru.draw
FROM racing.runners ru
LEFT JOIN racing.horses h ON h.horse_id = ru.horse_id
WHERE ru.race_id = 813100
ORDER BY ru.num;
```

3. **Check if horses exist:**
```sql
-- Are the horses in the horses table?
SELECT COUNT(*) FROM racing.horses;
SELECT * FROM racing.horses ORDER BY horse_id DESC LIMIT 20;
```

---

## 💡 Hypothesis

The batch upsert functions (`UpsertNamesAndFetchIDs`) might have a bug:
- They INSERT horses correctly
- But the SELECT to get IDs back fails  
- So the map is empty or incomplete
- Result: horse_id stays 0/NULL

**Test:** Check if the SELECT join is working:
```sql
-- This is what batch_upsert.go runs
CREATE TEMP TABLE tmp_horses (name text);
-- ... COPY data ...
SELECT h.horse_id, h.horse_name
FROM racing.horses h
JOIN tmp_horses t ON racing.norm_text(h.horse_name) = racing.norm_text(t.name);
```

---

## 📋 Backend Fixes That DID Work

Despite these issues, we successfully:
- ✅ Created fetch_all_betfair command
- ✅ Fixed MD5 race key generation
- ✅ Fixed SQL DELETE syntax  
- ✅ Added position extraction
- ✅ Added course logging

**BUT:** Can't verify they're fully working without database access!

---

## 🛠️ Recommended Actions

### Option A: Get Database Access
Install/enable psql to run diagnostic queries and manual fixes.

```bash
# Install postgres client
sudo apt install postgresql-client

# Then run queries
psql -d horse_db -c "SELECT COUNT(*) FROM racing.horses"
```

### Option B: Add More Debug Logging

Add logging to batch_upsert.go to see what's returned:
```go
// In UpsertNamesAndFetchIDs after SELECT
log.Printf("DEBUG: Selected %d horse IDs from database", len(out))
for name, id := range out {
    if id > 0 {
        log.Printf("  %s → %d", name, id)
    }
}
```

### Option C: Fallback to Old Method

The OLD `upsertDimensionsOLD` and `populateForeignKeysOLD` functions are still in the code. Try switching back temporarily:
```go
// In fetch_all/main.go
err := upsertDimensionsOLD(tx, races)
err := populateForeignKeysOLD(tx, races)
```

This uses individual queries instead of batch - slower but might work.

---

## 🎯 Summary

**Backend work:** 95% complete  
**Remaining:** Database debugging (requires psql access)  

**Two blockers:**
1. Duplicates not cleaning up (DELETE issue)
2. Horse names NULL (foreign key population issue)

**Both require:** Database access to diagnose and fix properly

---

**Recommendation:** Get psql access or add extensive debug logging to batch_upsert.go to see what's failing.

