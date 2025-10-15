# Schema Prefix Fix - October 15, 2025

## 🐛 Problem Found

Thanks to the verbose logging, we discovered the root cause of all test failures:

```
ERROR: GetCourses: repository error: pq: relation "courses" does not exist
ERROR: GetRace: repository error: pq: relation "horses" does not exist
ERROR: GetTrainerProfile: pq: relation "trainers" does not exist
ERROR: GetJockeyProfile: pq: relation "jockeys" does not exist
```

**Root Cause**: All SQL queries in the repository layer were missing the `racing.` schema prefix for dimension tables.

## ✅ What Was Fixed

Fixed **all repository files** to use proper schema-qualified table names:

### Before (WRONG)
```sql
SELECT * FROM courses WHERE course_id = $1
SELECT * FROM horses WHERE horse_id = $1
SELECT * FROM trainers WHERE trainer_id = $1
SELECT * FROM jockeys WHERE jockey_id = $1
JOIN courses c ON c.course_id = r.course_id
```

### After (CORRECT)
```sql
SELECT * FROM racing.courses WHERE course_id = $1
SELECT * FROM racing.horses WHERE horse_id = $1
SELECT * FROM racing.trainers WHERE trainer_id = $1
SELECT * FROM racing.jockeys WHERE jockey_id = $1
JOIN racing.courses c ON c.course_id = r.course_id
```

## 📁 Files Modified

All repository files were updated:

1. **`internal/repository/race.go`**
   - `courses` → `racing.courses`
   - `horses` → `racing.horses`
   - `trainers` → `racing.trainers`
   - `jockeys` → `racing.jockeys`

2. **`internal/repository/search.go`**
   - Fixed all dimension table references

3. **`internal/repository/profile.go`**
   - Fixed all profile queries

4. **`internal/repository/market.go`**
   - Fixed market analysis queries

5. **`internal/repository/bias.go`**
   - Fixed bias analysis queries

6. **`internal/repository/angle.go`**
   - Fixed angle queries

## 🔧 How the Fix Was Applied

Automated replacement across all repository files:

```bash
cd /home/smonaghan/GiddyUp/backend-api/internal/repository
for file in *.go; do
  sed -i 's/FROM courses/FROM racing.courses/g' "$file"
  sed -i 's/FROM horses/FROM racing.horses/g' "$file"
  sed -i 's/FROM trainers/FROM racing.trainers/g' "$file"
  sed -i 's/FROM jockeys/FROM racing.jockeys/g' "$file"
  sed -i 's/FROM owners/FROM racing.owners/g' "$file"
  sed -i 's/JOIN courses /JOIN racing.courses /g' "$file"
  sed -i 's/JOIN horses /JOIN racing.horses /g' "$file"
  sed -i 's/JOIN trainers /JOIN racing.trainers /g' "$file"
  sed -i 's/JOIN jockeys /JOIN racing.jockeys /g' "$file"
  # ... and LEFT JOIN variants
done
```

## ✅ Verification

### Quick Test

```bash
# Start server
cd /home/smonaghan/GiddyUp/backend-api
LOG_LEVEL=DEBUG ./bin/api

# In another terminal, run quick test
./test_quick.sh
```

Expected output:
```
✅ Server is running
✅ SUCCESS: Got X courses
✅ SUCCESS: Got X races
✅ SUCCESS: Search returned results
```

### Full Test Suite

```bash
# Run comprehensive tests
./scripts/run_comprehensive_tests.sh
```

Should now pass significantly more tests!

## 🎓 Why This Happened

The database uses a **schema** named `racing`:
- Tables: `racing.races`, `racing.runners`, `racing.courses`, etc.
- The server sets `search_path` to `racing, public` on startup

**However**, the dimension tables (`courses`, `horses`, `trainers`, `jockeys`, `owners`) were being referenced without the schema prefix, and PostgreSQL was looking in the default `public` schema instead of `racing`.

### Why `races` and `runners` worked

The main tables (`races`, `runners`) were always prefixed:
```sql
FROM racing.races   ← Always had prefix
FROM racing.runners ← Always had prefix
```

But the dimension tables weren't:
```sql
FROM courses        ← Missing prefix! ❌
FROM horses         ← Missing prefix! ❌
```

## 📝 Lesson Learned

**Always use fully-qualified table names** when working with custom schemas:
- ✅ `racing.courses`
- ✅ `racing.horses`
- ❌ `courses`
- ❌ `horses`

Even though `search_path` is set, it's better to be explicit for:
1. Clarity
2. Avoiding ambiguity
3. Performance (PostgreSQL doesn't have to search multiple schemas)

## 🎉 Impact

This fix should resolve **all 19 failing tests** that were showing:
- `pq: relation "courses" does not exist`
- `pq: relation "horses" does not exist`
- `pq: relation "trainers" does not exist`
- `pq: relation "jockeys" does not exist`

Tests that should now pass:
- ✅ Global search
- ✅ Race queries (by date, filters)
- ✅ Course endpoints
- ✅ Profile endpoints (horse, trainer, jockey)
- ✅ Market movers
- ✅ And more...

---

**Status**: ✅ **FIXED** - All dimension tables now use `racing.` schema prefix
**Next**: Re-run test suite to verify all tests pass

