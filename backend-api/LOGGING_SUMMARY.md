# Verbose Logging Implementation Summary

## ✅ What Was Added

### 1. File Logging System
- **Dual output**: Logs go to **both** terminal and `logs/server.log`
- **Automatic directory creation**: `logs/` folder created if it doesn't exist
- **Configurable location**: Set `LOG_DIR` environment variable to change location

### 2. Enhanced Logger Functions
**File**: `internal/logger/logger.go`

- `InitializeFileLogging(logDir)` - Sets up file logging
- `CloseLogFile()` - Cleanup on shutdown
- Writes to both stdout/stderr AND log file simultaneously

### 3. Verbose Handler Logging
**File**: `internal/handlers/race.go` (example - apply pattern to other handlers)

Each handler now logs:
- **Request received**: `→ FunctionName: params | IP: x.x.x.x`
- **Response sent**: `← FunctionName: results | duration`
- **Errors**: Full error details with context

**Pattern**:
```go
func (h *Handler) SomeEndpoint(c *gin.Context) {
    start := time.Now()
    logger.Info("→ SomeEndpoint: param1=%v, param2=%v | IP: %s", p1, p2, c.ClientIP())
    
    result, err := h.repo.DoSomething(params)
    if err != nil {
        logger.Error("SomeEndpoint: repository error: %v | Params: %+v", err, params)
        c.JSON(500, gin.H{"error": "failed"})
        return
    }
    
    duration := time.Since(start)
    logger.Info("← SomeEndpoint: %d results | %v", len(result), duration)
    c.JSON(200, result)
}
```

### 4. Startup Script
**File**: `start_with_logging.sh`

Convenient script to start server with verbose logging enabled:
```bash
./start_with_logging.sh
```

Sets:
- `LOG_LEVEL=DEBUG` (maximum verbosity)
- `AUTO_UPDATE_ON_STARTUP=true` (optional)
- `DATA_DIR` to default location

### 5. Documentation
**File**: `LOGGING_GUIDE.md`

Complete guide covering:
- How to start with logging
- Log levels (DEBUG, INFO, WARN, ERROR)
- Log format and structure
- Troubleshooting
- Log analysis examples
- Best practices

## 🚀 How to Use

### Quick Start
```bash
cd /home/smonaghan/GiddyUp/backend-api

# Option 1: Use the script (easiest)
./start_with_logging.sh

# Option 2: Manual
LOG_LEVEL=DEBUG ./bin/api

# Option 3: With auto-update
LOG_LEVEL=DEBUG AUTO_UPDATE_ON_STARTUP=true ./bin/api
```

### View Logs
```bash
# In terminal - see logs in real-time as they happen
# (Already visible if you started server without backgrounding)

# In another terminal - tail the log file
tail -f logs/server.log

# Filter for errors only
tail -f logs/server.log | grep ERROR

# Filter for specific endpoint
tail -f logs/server.log | grep "GetRace"
```

## 📊 Log Examples

### Server Startup
```
[2025-10-14 23:00:00.000] INFO:  📝 Logging initialized - writing to: logs/server.log
[2025-10-14 23:00:00.001] INFO:  === GiddyUp API Starting ===
[2025-10-14 23:00:00.002] INFO:  Environment: development
[2025-10-14 23:00:00.003] INFO:  Server Port: 8000
[2025-10-14 23:00:00.004] INFO:  Log Level: DEBUG
[2025-10-14 23:00:00.050] INFO:  ✅ Database connection established
[2025-10-14 23:00:00.051] INFO:  ✅ GiddyUp API is running on http://localhost:8000
```

### API Request (INFO level)
```
[2025-10-14 23:01:15.123] INFO:  → GetRecentRaces: date=2024-01-01, limit=50 | IP: 127.0.0.1
[2025-10-14 23:01:15.234] INFO:  ← GetRecentRaces: 12 races on 2024-01-01 | 111.234ms
```

### Error Logging
```
[2025-10-14 23:01:15.123] INFO:  → GetRace: race_id=339 | IP: 127.0.0.1
[2025-10-14 23:01:15.234] ERROR: GetRace: repository error for race_id=339: sql: no rows in result set
```

**Shows exactly what went wrong and with what parameters!**

### DEBUG Level (very verbose)
```
[2025-10-14 23:01:15.123] INFO:  → GetRecentRaces: date=2024-01-01, limit=50 | IP: 127.0.0.1
[2025-10-14 23:01:15.124] DEBUG: GetRecentRaces: filters={Date:2024-01-01 Limit:50}
[2025-10-14 23:01:15.125] DEBUG: SQL: SELECT * FROM racing.races WHERE race_date = $1 LIMIT $2 | Args: [2024-01-01 50] | Duration: 45.123ms
[2025-10-14 23:01:15.234] INFO:  ← GetRecentRaces: 12 races on 2024-01-01 | 111.234ms
```

## 🎯 Debugging Failed Tests

Now when tests fail, you can see **exactly** what the error is:

### Before (no logs)
```
--- FAIL: TestC01_RacesOnDate (0.00s)
    comprehensive_test.go:271: Expected 200, got 500: {"error":"failed to get races"}
```

### After (with logs)
Check `logs/server.log`:
```
[2025-10-14 23:01:15.123] INFO:  → GetRecentRaces: date=2024-01-01, limit=50 | IP: 127.0.0.1
[2025-10-14 23:01:15.234] ERROR: GetRecentRaces: repository error for date=2024-01-01: pq: column "race_key" does not exist
```

**Now you know**: The `race_key` column is missing from the database!

## 📁 Files Modified

1. **`internal/logger/logger.go`**
   - Added `InitializeFileLogging()` function
   - Added `CloseLogFile()` function
   - Dual output (stdout + file)

2. **`cmd/api/main.go`**
   - Call `logger.InitializeFileLogging()` on startup
   - Call `defer logger.CloseLogFile()` for cleanup
   - Log the configured log level

3. **`internal/handlers/race.go`**
   - Added request/response logging to all endpoints
   - Added timing measurements
   - Added error context logging
   - Pattern: `→` incoming, `←` outgoing

4. **`start_with_logging.sh`** (new)
   - Convenient startup script

5. **`LOGGING_GUIDE.md`** (new)
   - Complete documentation

6. **`LOGGING_SUMMARY.md`** (this file)
   - Implementation summary

## 🔧 Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `INFO` | `DEBUG`, `INFO`, `WARN`, `ERROR` |
| `LOG_DIR` | `logs` | Directory for `server.log` |

## 💡 Next Steps

### Apply Logging Pattern to Other Handlers

The pattern used in `race.go` should be applied to:
- `internal/handlers/search.go`
- `internal/handlers/profile.go`
- `internal/handlers/market.go`
- `internal/handlers/angle.go`
- `internal/handlers/bias.go`
- etc.

**Pattern to follow**:
```go
func (h *Handler) SomeFunction(c *gin.Context) {
    start := time.Now()
    logger.Info("→ SomeFunction: params | IP: %s", c.ClientIP())
    
    // ... do work ...
    
    if err != nil {
        logger.Error("SomeFunction: error details: %v", err)
        // return error response
    }
    
    duration := time.Since(start)
    logger.Info("← SomeFunction: results | %v", duration)
    // return success response
}
```

### Add Repository Logging

For even more detail, add logging to repository functions:
```go
func (r *Repo) GetSomething(id int) (*Thing, error) {
    logger.Debug("Repository: GetSomething(id=%d)", id)
    // ... SQL query ...
    if err != nil {
        logger.Error("Repository: GetSomething failed for id=%d: %v", id, err)
        return nil, err
    }
    return result, nil
}
```

## ✅ Benefits

1. **Dual Output**: See logs in terminal AND save to file
2. **Debugging**: See exact errors with full context
3. **Performance**: Track request duration
4. **Audit**: Know who called what and when (IP addresses)
5. **Troubleshooting**: Grep logs to find issues
6. **Monitoring**: Track error rates over time

## 🎉 Summary

You now have:
- ✅ File logging to `logs/server.log`
- ✅ Verbose request/response logging
- ✅ Error logging with full context
- ✅ Performance timing for all requests
- ✅ Easy startup script
- ✅ Complete documentation

**To debug failed tests**: Just run `LOG_LEVEL=DEBUG ./bin/api` and check `logs/server.log` for the actual errors!

