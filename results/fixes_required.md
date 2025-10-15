# Required Fixes for Backend API

## Summary
**Current Status:** 15/33 tests passing (45.5%)
**Target Status:** 28-30/33 tests passing (85-90%)
**Estimated Time:** 1-2 hours

---

## Fix #1: PostgreSQL Shared Memory Configuration ⚠️ CRITICAL

### Problem
```
pq: could not resize shared memory segment "/PostgreSQL.XXXXXXXX" to 4194304 bytes: No space left on device
```

### Affected Tests (10+)
- All Profile endpoints (Horse, Trainer, Jockey)
- Comment FTS search (some queries)

### Current Settings Check
```bash
psql -U postgres -d horse_db -c "SHOW shared_buffers;"
psql -U postgres -d horse_db -c "SHOW work_mem;"
psql -U postgres -d horse_db -c "SHOW temp_buffers;"
```

### Solution
```bash
# Edit PostgreSQL config
sudo vi /etc/postgresql/*/main/postgresql.conf

# Update these values:
shared_buffers = 256MB          # was likely 128MB or lower
work_mem = 8MB                  # was likely 4MB
temp_buffers = 16MB             # was likely 8MB
effective_cache_size = 1GB      # for query planner

# Restart PostgreSQL
sudo systemctl restart postgresql

# Or if using Docker:
docker restart postgres_container
```

**Impact:** Will fix 10+ failing tests immediately

---

## Fix #2: SQL ROUND Function Error ⚠️ HIGH

### Problem
```
pq: function round(double precision, integer) does not exist
```

### Affected Tests
- Market Movers
- Win Calibration
- Place Calibration

### Root Cause
PostgreSQL's `ROUND(value, digits)` requires NUMERIC type, not DOUBLE PRECISION.

### Location
`/home/smonaghan/GiddyUp/backend-api/internal/repository/market.go`

### Fix Pattern
```sql
-- WRONG:
ROUND(win_ppmax, 2)
ROUND(move_pct, 2)

-- CORRECT:
ROUND(win_ppmax::numeric, 2)
ROUND(move_pct::numeric, 2)
ROUND(CAST(win_ppmax AS numeric), 2)
```

### Search & Replace Strategy
```bash
cd /home/smonaghan/GiddyUp/backend-api/internal/repository
# Find all ROUND calls in market.go
grep -n "ROUND(" market.go

# For each ROUND call with a double precision column, add ::numeric cast
```

**Impact:** Will fix 3-4 failing tests

---

## Fix #3: Angle Model Mapping Error ⚠️ HIGH

### Problem
```
missing destination name next_race_id in *[]models.NearMissQualifier
```

### Affected Tests
- All angle/today tests (6 tests timing out)

### Root Cause
The SQL query returns `next_race_id` column but the Go struct doesn't have this field.

### Location
`/home/smonaghan/GiddyUp/backend-api/internal/models/angle.go`

### Solution
Add the missing field to the struct:

```go
// In NearMissQualifier struct (or nested Entry struct)
type NearMissQualifier struct {
    HorseID      int     `json:"horse_id" db:"horse_id"`
    HorseName    string  `json:"horse_name" db:"horse_name"`
    
    Entry struct {
        RaceID    int     `json:"race_id" db:"race_id"`
        NextRaceID int    `json:"next_race_id" db:"next_race_id"` // ADD THIS
        Date      string  `json:"date" db:"date"`
        // ... other fields
    } `json:"entry"`
    
    // ... rest of struct
}
```

**Alternative:** If `next_race_id` is not needed in the response, remove it from the SQL SELECT.

**Impact:** Will fix 6+ angle tests

---

## Fix #4: Limit Validation ⚠️ MEDIUM

### Problem
- No server-side limit capping
- Test requested 100,000 items, got 100,000 items
- Should cap at 1000

### Solution
Add validation middleware:

```go
// internal/middleware/validation.go
func ValidatePagination() gin.HandlerFunc {
    return func(c *gin.Context) {
        limit := c.DefaultQuery("limit", "50")
        limitInt, _ := strconv.Atoi(limit)
        
        if limitInt > 1000 {
            c.Set("limit", 1000)
        } else if limitInt < 1 {
            c.Set("limit", 1)
        }
        
        c.Next()
    }
}
```

Register in router:
```go
api.Use(middleware.ValidatePagination())
```

---

## Fix #5: 404 Error Response Format ⚠️ MEDIUM

### Problem
404 responses not returning JSON (returning HTML or plain text)

### Solution
Update error middleware to always return JSON:

```go
// internal/middleware/error.go
func ErrorHandler() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()
        
        if len(c.Errors) > 0 {
            c.JSON(c.Writer.Status(), gin.H{
                "error": c.Errors.Last().Error(),
            })
        }
    }
}

// Add custom 404 handler
router.NoRoute(func(c *gin.Context) {
    c.JSON(404, gin.H{
        "error": "endpoint not found",
        "path": c.Request.URL.Path,
    })
})
```

---

## Fix #6: Bad Parameter Handling ⚠️ LOW

### Problem
Bad date formats (e.g., "2024-13-99") cause 500 errors instead of 400

### Solution
Add parameter validation before database queries:

```go
func ValidateDate(dateStr string) error {
    _, err := time.Parse("2006-01-02", dateStr)
    return err
}

// In handler:
if dateFrom := c.Query("date_from"); dateFrom != "" {
    if err := ValidateDate(dateFrom); err != nil {
        c.JSON(400, gin.H{"error": "invalid date_from format, use YYYY-MM-DD"})
        return
    }
}
```

---

## Fix #7: Comment Search Multi-word Phrases ⚠️ LOW

### Problem
"never dangerous" (with space) fails with 400 error

### Possible Causes
- URL encoding issue
- FTS query parser doesn't handle phrases
- Need to use `websearch_to_tsquery` instead of `plainto_tsquery`

### Solution
```sql
-- Current (probably):
WHERE comment_fts @@ plainto_tsquery('english', $1)

-- Better for phrases:
WHERE comment_fts @@ websearch_to_tsquery('english', $1)
```

---

## Implementation Order

### Phase 1: Database Configuration (30 min)
1. Check PostgreSQL settings
2. Increase shared_buffers, work_mem
3. Restart PostgreSQL
4. Re-run tests

**Expected Result:** 10+ tests start passing

### Phase 2: Code Fixes (45 min)
1. Fix ROUND function in market.go (15 min)
2. Fix angle model mapping (10 min)
3. Add limit validation (10 min)
4. Fix 404 handler (10 min)

**Expected Result:** 3-4 more tests pass

### Phase 3: Optimizations (1-2 days)
1. Run production_hardening.sql
2. Optimize comment search
3. Add validation middleware

**Expected Result:** Near 100% pass rate, excellent performance

---

## Files to Edit

1. `/etc/postgresql/*/main/postgresql.conf` - Memory settings
2. `/home/smonaghan/GiddyUp/backend-api/internal/repository/market.go` - ROUND fixes
3. `/home/smonaghan/GiddyUp/backend-api/internal/models/angle.go` - Add NextRaceID field
4. `/home/smonaghan/GiddyUp/backend-api/internal/middleware/error.go` - 404 handler
5. `/home/smonaghan/GiddyUp/backend-api/internal/middleware/validation.go` - Limit validation
6. `/home/smonaghan/GiddyUp/backend-api/internal/router/router.go` - Register middleware

---

## Expected Test Results After Fixes

| Section | Current | After Phase 1 | After Phase 2 |
|---------|---------|---------------|---------------|
| A: Health | 4/5 (80%) | 5/5 (100%) | 5/5 (100%) |
| B: Search | 2/4 (50%) | 3/4 (75%) | 4/4 (100%) |
| C: Races | 9/9 (100%) | 9/9 (100%) | 9/9 (100%) |
| D: Profiles | 0/3 (0%) | 3/3 (100%) | 3/3 (100%) |
| E: Market | 1/5 (20%) | 1/5 (20%) | 4/5 (80%) |
| F: Bias | 2/3 (67%) | 2/3 (67%) | 3/3 (100%) |
| G: Validation | 2/4 (50%) | 2/4 (50%) | 4/4 (100%) |
| **TOTAL** | **15/33 (45%)** | **25/33 (76%)** | **32/33 (97%)** |

