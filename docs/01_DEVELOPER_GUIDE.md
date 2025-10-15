# Developer Guide - GiddyUp Racing API

**Complete guide for backend developers working on the GiddyUp platform**

## Table of Contents

1. [Quick Start](#quick-start)
2. [Project Architecture](#project-architecture)
3. [Development Setup](#development-setup)
4. [Code Structure](#code-structure)
5. [Adding Features](#adding-features)
6. [Testing](#testing)
7. [Debugging](#debugging)
8. [Performance](#performance)

---

## 1. Quick Start

### Get Running in 15 Minutes

```bash
# 1. Clone and navigate
cd /home/smonaghan/GiddyUp

# 2. Start database
cd postgres
docker-compose up -d
cd ..

# 3. Restore data (2 minutes)
docker exec -i horse_racing psql -U postgres -d horse_db < postgres/db_backup.sql

# 4. Build API
cd backend-api
go build -o bin/api ./cmd/api/

# 5. Start server
./bin/api

# 6. Test it
curl http://localhost:8000/health
curl "http://localhost:8000/api/v1/courses" | jq
```

**Expected output**: `{"status":"healthy"}` and list of 89 courses

---

## 2. Project Architecture

### High-Level Overview

```
┌──────────────────┐
│   Client/UI      │
└────────┬─────────┘
         │ HTTP/JSON
         ▼
┌──────────────────────────────────────┐
│      API Server (Go/Gin)             │
│  ┌────────────┐  ┌────────────────┐  │
│  │  Handlers  │──│  Middleware    │  │
│  └──────┬─────┘  └────────────────┘  │
│         │                             │
│  ┌──────▼──────┐  ┌───────────────┐  │
│  │ Repository  │──│  Services     │  │
│  └──────┬──────┘  └───────────────┘  │
└─────────┼──────────────────────────────┘
          │ SQL
          ▼
┌──────────────────────────────────────┐
│    PostgreSQL 16                     │
│    • racing schema                   │
│    • 226K races                      │
│    • 2.2M runners                    │
│    • Partitioned by year             │
│    • Materialized views              │
└──────────────────────────────────────┘
```

### Directory Structure

```
backend-api/
├── cmd/                    # Command-line applications
│   ├── api/               # Main API server
│   ├── load_master/       # Bulk data loader
│   ├── backfill_dates/    # Date range backfiller
│   └── check_missing/     # Data gap detector
│
├── internal/              # Internal packages (not importable externally)
│   ├── config/           # Configuration management
│   ├── database/         # Database connection
│   ├── handlers/         # HTTP request handlers (controllers)
│   ├── middleware/       # HTTP middleware (CORS, logging, etc.)
│   ├── models/           # Data structures
│   ├── repository/       # Database queries (data layer)
│   ├── router/           # Route definitions
│   ├── scraper/          # Racing Post + Betfair scrapers
│   ├── services/         # Business logic (auto-update, etc.)
│   ├── stitcher/         # Data merging logic
│   └── logger/           # Logging utilities
│
├── bin/                   # Compiled binaries
├── logs/                  # Application logs
├── scripts/               # Test & demo scripts
└── tests/                 # Test files
```

### Data Flow

```
HTTP Request
    ↓
Router (routes.go)
    ↓
Middleware (CORS, validation)
    ↓
Handler (race.go, search.go, etc.)
    ↓
Repository (database queries)
    ↓
PostgreSQL
    ↓
Results back up the chain
    ↓
JSON Response
```

---

## 3. Development Setup

### Prerequisites

- **Go 1.21+** - `go version`
- **PostgreSQL 16** - via Docker
- **Docker** - for PostgreSQL container
- **Git** - for version control
- **curl/httpie** - for API testing
- **jq** - for JSON formatting (optional)

### Initial Setup

```bash
# 1. Install Go dependencies
cd /home/smonaghan/GiddyUp/backend-api
go mod download
go mod tidy

# 2. Build all tools
go build -o bin/api ./cmd/api/
go build -o bin/load_master ./cmd/load_master/
go build -o bin/backfill_dates ./cmd/backfill_dates/
go build -o bin/check_missing ./cmd/check_missing/

# 3. Verify builds
ls -lh bin/
# Should show: api, load_master, backfill_dates, check_missing

# 4. Start database
cd ../postgres
docker-compose up -d

# 5. Restore data
docker exec -i horse_racing psql -U postgres -d horse_db < db_backup.sql

# 6. Verify database
docker exec horse_racing psql -U postgres -d horse_db -c "
SELECT COUNT(*) FROM racing.races;
SELECT COUNT(*) FROM racing.runners;
SELECT COUNT(*) FROM racing.horses;
"

# Expected: ~226K races, ~2.2M runners, ~190K horses
```

### Environment Variables

Create `.env` file (or export in shell):

```bash
# Database
export DATABASE_HOST=localhost
export DATABASE_PORT=5432
export DATABASE_NAME=horse_db
export DATABASE_USER=postgres
export DATABASE_PASSWORD=password

# Server
export SERVER_PORT=8000
export SERVER_ENV=development
export CORS_ORIGINS=http://localhost:3000,http://localhost:5173

# Logging
export LOG_LEVEL=DEBUG          # DEBUG, INFO, WARN, ERROR
export LOG_DIR=logs             # Log file directory

# Auto-Update
export AUTO_UPDATE_ON_STARTUP=true
export DATA_DIR=/home/smonaghan/GiddyUp/data
```

### Start Development Server

```bash
cd /home/smonaghan/GiddyUp/backend-api

# With verbose logging (recommended for development)
LOG_LEVEL=DEBUG ./bin/api

# Or use the convenience script
./start_with_logging.sh
```

**Server will start on**: http://localhost:8000

---

## 4. Code Structure

### Handler Example

Handlers are HTTP controllers that process requests:

```go
// File: internal/handlers/race.go

func (h *RaceHandler) GetRace(c *gin.Context) {
    start := time.Now()
    
    // 1. Parse parameters
    raceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        logger.Warn("GetRace: invalid race ID: %v", err)
        c.JSON(400, gin.H{"error": "invalid race ID"})
        return
    }
    
    // 2. Log request
    logger.Info("→ GetRace: race_id=%d | IP: %s", raceID, c.ClientIP())
    
    // 3. Call repository
    race, err := h.repo.GetRaceByID(raceID)
    if err != nil {
        logger.Error("GetRace: repository error: %v", err)
        c.JSON(404, gin.H{"error": "race not found"})
        return
    }
    
    // 4. Log response
    duration := time.Since(start)
    logger.Info("← GetRace: %d runners | %v", len(race.Runners), duration)
    
    // 5. Return JSON
    c.JSON(200, race)
}
```

### Repository Example

Repositories handle database queries:

```go
// File: internal/repository/race.go

func (r *RaceRepository) GetRaceByID(raceID int64) (*models.RaceWithRunners, error) {
    // IMPORTANT: Always use racing. prefix for tables!
    query := `
        SELECT 
            r.race_id, r.race_key, r.race_date, r.region,
            c.course_name, r.off_time, r.race_name, r.race_type,
            r.class, r.dist_f, r.going, r.surface, r.ran
        FROM racing.races r
        LEFT JOIN racing.courses c ON c.course_id = r.course_id
        WHERE r.race_id = $1
    `
    
    var race models.RaceWithRunners
    if err := r.db.Get(&race, query, raceID); err != nil {
        return nil, fmt.Errorf("failed to get race: %w", err)
    }
    
    // Get runners
    runners, err := r.GetRaceRunners(raceID)
    if err != nil {
        return nil, err
    }
    race.Runners = runners
    
    return &race, nil
}
```

**Key Rules**:
1. ✅ Always use `racing.` schema prefix (`racing.races`, not `races`)
2. ✅ Use prepared statements (`$1`, `$2`) to prevent SQL injection
3. ✅ Return descriptive errors with context
4. ✅ Use LEFT JOIN for optional relationships
5. ✅ Always handle NULL values

### Model Example

Models define data structures:

```go
// File: internal/models/race.go

type Race struct {
    RaceID     int64     `json:"race_id" db:"race_id"`
    RaceKey    string    `json:"race_key" db:"race_key"`
    RaceDate   time.Time `json:"race_date" db:"race_date"`
    Region     string    `json:"region" db:"region"`
    CourseName *string   `json:"course_name,omitempty" db:"course_name"`
    OffTime    *string   `json:"off_time,omitempty" db:"off_time"`
    RaceName   string    `json:"race_name" db:"race_name"`
    RaceType   string    `json:"race_type" db:"race_type"`
    Class      *string   `json:"class,omitempty" db:"class"`
    DistanceF  *float64  `json:"distance_f,omitempty" db:"dist_f"`
    Going      *string   `json:"going,omitempty" db:"going"`
    Surface    *string   `json:"surface,omitempty" db:"surface"`
    Ran        int       `json:"ran" db:"ran"`
}

type RaceWithRunners struct {
    Race
    Runners []Runner `json:"runners"`
}
```

**Key Rules**:
1. ✅ Use pointers for nullable fields (`*string`, `*int64`)
2. ✅ Tag with `json` for API responses
3. ✅ Tag with `db` for database mapping
4. ✅ Use `omitempty` for optional fields

---

## 5. Adding Features

### Add a New Endpoint (Step-by-Step)

**Example**: Add `GET /api/v1/horses/{id}/siblings` endpoint

#### Step 1: Define Model

```go
// File: internal/models/horse.go

type Sibling struct {
    HorseID   int64   `json:"horse_id" db:"horse_id"`
    HorseName string  `json:"horse_name" db:"horse_name"`
    Sire      string  `json:"sire" db:"sire"`
    Dam       string  `json:"dam" db:"dam"`
    Runs      int     `json:"runs" db:"runs"`
    Wins      int     `json:"wins" db:"wins"`
}
```

#### Step 2: Add Repository Method

```go
// File: internal/repository/profile.go

func (r *ProfileRepository) GetHorseSiblings(horseID int64) ([]models.Sibling, error) {
    query := `
        SELECT DISTINCT
            h2.horse_id,
            h2.horse_name,
            b.sire,
            b.dam,
            COUNT(DISTINCT ru.race_id) AS runs,
            COUNT(DISTINCT ru.race_id) FILTER (WHERE ru.win_flag) AS wins
        FROM racing.horses h1
        JOIN racing.bloodlines b1 ON b1.blood_id = (
            SELECT blood_id FROM racing.runners WHERE horse_id = h1.horse_id LIMIT 1
        )
        JOIN racing.bloodlines b2 ON b2.dam = b1.dam AND b2.sire = b1.sire
        JOIN racing.horses h2 ON h2.horse_id IN (
            SELECT horse_id FROM racing.runners WHERE blood_id = b2.blood_id
        )
        LEFT JOIN racing.runners ru ON ru.horse_id = h2.horse_id
        WHERE h1.horse_id = $1
            AND h2.horse_id != $1
        GROUP BY h2.horse_id, h2.horse_name, b.sire, b.dam
        ORDER BY wins DESC, runs DESC
        LIMIT 20
    `
    
    var siblings []models.Sibling
    if err := r.db.Select(&siblings, query, horseID); err != nil {
        return nil, fmt.Errorf("failed to get siblings: %w", err)
    }
    
    return siblings, nil
}
```

#### Step 3: Add Handler

```go
// File: internal/handlers/profile.go

func (h *ProfileHandler) GetHorseSiblings(c *gin.Context) {
    start := time.Now()
    
    horseID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        logger.Warn("GetHorseSiblings: invalid horse ID: %v", err)
        c.JSON(400, gin.H{"error": "invalid horse ID"})
        return
    }
    
    logger.Info("→ GetHorseSiblings: horse_id=%d | IP: %s", horseID, c.ClientIP())
    
    siblings, err := h.repo.GetHorseSiblings(horseID)
    if err != nil {
        logger.Error("GetHorseSiblings: repository error: %v", err)
        c.JSON(500, gin.H{"error": "failed to get siblings"})
        return
    }
    
    duration := time.Since(start)
    logger.Info("← GetHorseSiblings: %d siblings | %v", len(siblings), duration)
    
    c.JSON(200, siblings)
}
```

#### Step 4: Register Route

```go
// File: internal/router/router.go

// In the profiles group:
profiles := v1.Group("/horses")
{
    profiles.GET("/:id/profile", profileHandler.GetHorseProfile)
    profiles.GET("/:id/siblings", profileHandler.GetHorseSiblings)  // ← Add this
}
```

#### Step 5: Test It

```bash
# Rebuild
go build -o bin/api ./cmd/api/

# Restart server
pkill api && ./bin/api &

# Test endpoint
curl "http://localhost:8000/api/v1/horses/1/siblings" | jq

# Check logs
tail -20 logs/server.log | grep GetHorseSiblings
```

---

## 6. Testing

### Run Test Suite

```bash
cd /home/smonaghan/GiddyUp/backend-api

# Full comprehensive test suite
go test -v ./tests/comprehensive_test.go

# Quick API verification
./test_quick.sh

# With test scripts
./scripts/run_comprehensive_tests.sh
```

### Write a New Test

```go
// File: tests/my_feature_test.go

package tests

import (
    "testing"
    "encoding/json"
)

func TestHorseSiblings(t *testing.T) {
    t.Log("Testing horse siblings endpoint")
    
    // Make request
    resp, body := makeHTTPRequest(t, "GET", 
        BASE_URL+"/api/v1/horses/1/siblings", nil)
    
    // Check status
    if resp.StatusCode != 200 {
        t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
    }
    
    // Parse response
    var siblings []models.Sibling
    if err := json.Unmarshal(body, &siblings); err != nil {
        t.Fatalf("Failed to parse: %v", err)
    }
    
    // Validate
    if len(siblings) == 0 {
        t.Error("Expected at least 1 sibling")
    }
    
    t.Logf("✅ Found %d siblings", len(siblings))
}
```

### Test Current API

```bash
# Run against live server
cd backend-api

# Start server if not running
./bin/api &

# Run tests
go test -v ./tests/comprehensive_test.go

# Expected: 32/33 PASS (97%)
```

---

## 7. Debugging

### Use Verbose Logging

```bash
# Start with DEBUG level
LOG_LEVEL=DEBUG ./bin/api

# Logs show:
# → Incoming requests with parameters
# ← Outgoing responses with timing
# ERROR Full error details with context
# DEBUG SQL queries with args

# Watch logs in real-time
tail -f logs/server.log
```

### Debug a Failing Test

```bash
# 1. Start server with logging
LOG_LEVEL=DEBUG ./bin/api > /tmp/debug.log 2>&1 &

# 2. Run specific test
go test -v ./tests/comprehensive_test.go -run TestC01_RacesOnDate

# 3. Check logs for errors
grep ERROR /tmp/debug.log
grep "GetRecentRaces" /tmp/debug.log

# 4. See SQL queries
grep "DEBUG: SQL:" /tmp/debug.log
```

### Common Issues & Solutions

**Issue**: `pq: relation "X" does not exist`
- **Cause**: Missing `racing.` schema prefix
- **Fix**: Add `racing.` to table name in SQL query
- **Example**: `FROM courses` → `FROM racing.courses`

**Issue**: `sql: no rows in result set`
- **Cause**: Query returned no data
- **Fix**: Check if data exists, handle gracefully
- **Example**: Return 404 instead of 500

**Issue**: `column reference "X" is ambiguous`
- **Cause**: Multiple tables have same column name
- **Fix**: Use table aliases (`r.race_date`, `ru.race_date`)

**Issue**: Test fails with 500 error
- **Fix**: Check `logs/server.log` for actual error
- **Pattern**: `grep ERROR logs/server.log | tail -20`

---

## 8. Performance

### Query Optimization

**Use Materialized Views** for expensive queries:

```sql
-- Horse profiles use mv_runner_base (fast!)
SELECT * FROM racing.mv_runner_base 
WHERE horse_id = $1
ORDER BY race_date DESC;
-- ~10-50ms vs 1-2s without view
```

**Add Indexes** for common queries:

```sql
-- For date range queries
CREATE INDEX idx_races_date ON racing.races (race_date);

-- For horse history
CREATE INDEX idx_runners_horse_date ON racing.runners (horse_id, race_date DESC);

-- For search
CREATE INDEX idx_horses_name_trgm ON racing.horses USING gin (horse_name gin_trgm_ops);
```

**Limit Result Sets**:

```go
// Always cap limits to prevent performance issues
if filters.Limit <= 0 {
    filters.Limit = 100  // default
}
if filters.Limit > 1000 {
    filters.Limit = 1000  // maximum
}
```

### Monitoring Performance

```bash
# Find slow queries in logs
grep -E "[0-9]+s\)" logs/server.log

# Average response time for an endpoint
grep "← GetRace:" logs/server.log | \
  grep -oE '[0-9]+ms' | \
  awk '{sum+=$1; count++} END {print "Average: " sum/count "ms"}'

# Count requests per endpoint
grep "→" logs/server.log | \
  awk '{print $4}' | \
  sort | uniq -c | sort -rn
```

---

## 9. Code Style & Standards

### Go Best Practices

```go
// ✅ Good: Descriptive errors
return nil, fmt.Errorf("failed to get race %d: %w", raceID, err)

// ❌ Bad: Generic errors
return nil, err

// ✅ Good: Validate input
if raceID <= 0 {
    return nil, errors.New("invalid race ID")
}

// ✅ Good: Use context for timeouts
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

// ✅ Good: Close resources
defer rows.Close()
defer tx.Rollback()  // safe even if committed
```

### SQL Best Practices

```sql
-- ✅ Good: Always use schema prefix
SELECT * FROM racing.races

-- ✅ Good: Use table aliases
SELECT r.*, c.course_name
FROM racing.races r
LEFT JOIN racing.courses c ON c.course_id = r.course_id

-- ✅ Good: Handle NULLs
COALESCE(r.class, 'Unknown')

-- ✅ Good: Use prepared statements
WHERE r.race_id = $1  -- Not: WHERE r.race_id = " + id

-- ✅ Good: Use indexes
WHERE r.race_date = $1  -- indexed!

-- ❌ Bad: Functions on indexed columns
WHERE EXTRACT(YEAR FROM r.race_date) = 2024  -- won't use index
```

### Logging Best Practices

```go
// ✅ Good: Log requests and responses
logger.Info("→ GetRace: race_id=%d | IP: %s", raceID, c.ClientIP())
logger.Info("← GetRace: %d runners | %v", len(runners), duration)

// ✅ Good: Log errors with context
logger.Error("GetRace: repository error for race_id=%d: %v", raceID, err)

// ✅ Good: Use appropriate levels
logger.Debug("...")  // Development details
logger.Info("...")   // Normal operations
logger.Warn("...")   // Potential issues
logger.Error("...")  // Actual errors
```

---

## 10. Common Development Tasks

### Task 1: Add a New Filter to Race Search

**Files to modify**: `models/race.go`, `handlers/race.go`, `repository/race.go`

```go
// 1. Add to model
type RaceFilters struct {
    // ... existing fields ...
    MinPrize *float64 `form:"min_prize"`  // ← Add this
}

// 2. Add to repository query
if filters.MinPrize != nil {
    argCount++
    query += fmt.Sprintf(" AND r.prize >= $%d", argCount)
    args = append(args, *filters.MinPrize)
}

// 3. Test it
curl "http://localhost:8000/api/v1/races/search?min_prize=10000&limit=10"
```

### Task 2: Add Caching

```go
// Use in-memory cache for hot queries
type CachedRepository struct {
    repo  *RaceRepository
    cache map[string]interface{}
    mu    sync.RWMutex
}

func (c *CachedRepository) GetRace(id int64) (*Race, error) {
    key := fmt.Sprintf("race:%d", id)
    
    // Check cache
    c.mu.RLock()
    if cached, ok := c.cache[key]; ok {
        c.mu.RUnlock()
        return cached.(*Race), nil
    }
    c.mu.RUnlock()
    
    // Query database
    race, err := c.repo.GetRaceByID(id)
    if err != nil {
        return nil, err
    }
    
    // Store in cache
    c.mu.Lock()
    c.cache[key] = race
    c.mu.Unlock()
    
    return race, nil
}
```

### Task 3: Add Pagination

```go
type PaginationParams struct {
    Page  int `form:"page"`   // 1-based page number
    Limit int `form:"limit"`  // Items per page
}

func (r *Repository) GetRacesPaginated(params PaginationParams) ([]Race, int, error) {
    if params.Page <= 0 {
        params.Page = 1
    }
    if params.Limit <= 0 {
        params.Limit = 50
    }
    
    offset := (params.Page - 1) * params.Limit
    
    // Get total count
    var total int
    r.db.Get(&total, "SELECT COUNT(*) FROM racing.races")
    
    // Get page
    query := "SELECT * FROM racing.races ORDER BY race_date DESC LIMIT $1 OFFSET $2"
    var races []Race
    r.db.Select(&races, query, params.Limit, offset)
    
    return races, total, nil
}
```

---

## 11. Deployment Checklist

### Before Deploying

- [ ] All tests passing (`go test ./...`)
- [ ] Code linted (`go fmt ./...`, `go vet ./...`)
- [ ] Dependencies updated (`go mod tidy`)
- [ ] Environment variables documented
- [ ] Logs configured (`LOG_LEVEL=INFO` for production)
- [ ] Database backup created
- [ ] Materialized views refreshed

### Production Build

```bash
# Build optimized binary
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-w -s" \
  -o bin/api-prod \
  ./cmd/api/

# Size should be ~25-30MB (smaller than dev build)
ls -lh bin/api-prod
```

### Health Checks

```bash
# API health
curl http://localhost:8000/health

# Database health
docker exec horse_racing psql -U postgres -d horse_db -c "SELECT 1;"

# Check data freshness
docker exec horse_racing psql -U postgres -d horse_db -c "
SELECT MAX(race_date) FROM racing.races;"
# Should be yesterday or today
```

---

## 12. Resources

### Documentation
- **API Reference**: `docs/02_API_DOCUMENTATION.md`
- **Database Schema**: `docs/03_DATABASE_GUIDE.md`
- **Deployment**: `docs/05_DEPLOYMENT_GUIDE.md`

### External Resources
- **Go Documentation**: https://golang.org/doc/
- **Gin Framework**: https://gin-gonic.com/docs/
- **PostgreSQL**: https://www.postgresql.org/docs/16/

### Internal Tools
- **Load Master**: Bulk data loader
- **Backfill Dates**: Date range backfiller
- **Check Missing**: Gap detector

See `backend-api/cmd/README.md` for CLI tool documentation.

---

## 13. Getting Help

### Check Logs First
```bash
# Recent errors
tail -100 logs/server.log | grep ERROR

# Specific endpoint
grep "GetRace" logs/server.log | tail -20

# Slow queries
grep -E "[0-9]{3,}ms" logs/server.log
```

### Common Questions

**Q: How do I add a new field to races?**
A: See section 5.1 "Add a New Filter"

**Q: Why is my query slow?**
A: Check section 8 "Performance" - likely missing index or materialized view

**Q: How do I debug a test failure?**
A: Section 7.2 "Debug a Failing Test"

**Q: What's the database schema?**
A: `docs/03_DATABASE_GUIDE.md` or `postgres/database.md`

---

**Status**: ✅ Ready for Development  
**Last Updated**: October 15, 2025  
**Version**: 1.0.0

