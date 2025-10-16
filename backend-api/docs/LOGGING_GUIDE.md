# Server Logging Guide

## Overview

The GiddyUp API now has comprehensive logging that writes to **both** your terminal and `logs/server.log`.

## Quick Start

### Start with Verbose Logging

```bash
cd /home/smonaghan/GiddyUp/backend-api

# Option 1: Use the script
./start_with_logging.sh

# Option 2: Manual with DEBUG level
LOG_LEVEL=DEBUG ./bin/api

# Option 3: Manual with auto-update and DEBUG
LOG_LEVEL=DEBUG AUTO_UPDATE_ON_STARTUP=true ./bin/api
```

### View Logs in Real-Time

```bash
# In another terminal, tail the log file
tail -f /home/smonaghan/GiddyUp/backend-api/logs/server.log

# Or with grep to filter
tail -f logs/server.log | grep ERROR
tail -f logs/server.log | grep "GetRace"
```

## Log Levels

Set with `LOG_LEVEL` environment variable:

| Level | What You See |
|-------|--------------|
| `DEBUG` | Everything - requests, SQL, details (very verbose) |
| `INFO` | Standard logs - requests, responses, errors (default) |
| `WARN` | Warnings and errors only |
| `ERROR` | Errors only |

**Example**:
```bash
# Maximum verbosity (recommended for debugging)
LOG_LEVEL=DEBUG ./bin/api

# Normal operation
LOG_LEVEL=INFO ./bin/api

# Quiet - only errors
LOG_LEVEL=ERROR ./bin/api
```

## Log Output

### Dual Output
Logs are written to **two places simultaneously**:
1. **Terminal** (stdout/stderr) - see logs in real-time
2. **File** (`logs/server.log`) - persistent log file

### Log File Location

Default: `logs/server.log`

Change with `LOG_DIR`:
```bash
LOG_DIR=/var/log/giddyup ./bin/api
# Logs will be written to: /var/log/giddyup/server.log
```

## What Gets Logged

### Server Startup
```
[2025-10-14 23:00:00.000] INFO:  📝 Logging initialized - writing to: logs/server.log
[2025-10-14 23:00:00.001] INFO:  === GiddyUp API Starting ===
[2025-10-14 23:00:00.002] INFO:  Environment: development
[2025-10-14 23:00:00.003] INFO:  Server Port: 8000
[2025-10-14 23:00:00.004] INFO:  Log Level: DEBUG
[2025-10-14 23:00:00.010] INFO:  ✅ Database connection established
```

### HTTP Requests (INFO level)
```
[2025-10-14 23:01:15.123] INFO:  → GetRecentRaces: date=2024-01-01, limit=50 | IP: 127.0.0.1
[2025-10-14 23:01:15.234] INFO:  ← GetRecentRaces: 12 races on 2024-01-01 | 111.234ms
```

**Format**: `→` = incoming request, `←` = response sent

### Errors (ERROR level)
```
[2025-10-14 23:01:15.123] ERROR: GetRecentRaces: repository error for date=2024-01-01: pq: relation "racing.races" does not exist
[2025-10-14 23:01:15.124] ERROR: GetRace: repository error for race_id=339: sql: no rows in result set
```

**Shows**:
- Function name
- Parameters
- Actual error message from database/code

### SQL Queries (DEBUG level only)
```
[2025-10-14 23:01:15.123] DEBUG: SQL: SELECT * FROM racing.races WHERE race_date = $1 | Args: [2024-01-01] | Duration: 45.123ms
```

### Auto-Update Service
```
[2025-10-14 23:00:05.000] [AutoUpdate] 🔍 Checking for missing data...
[2025-10-14 23:00:05.100] [AutoUpdate] 📅 Backfilling 3 days (2025-10-12 to 2025-10-14)...
[2025-10-14 23:00:05.100] [AutoUpdate] Processing 2025-10-12...
[2025-10-14 23:00:05.101] [AutoUpdate]   [1/4] Scraping Racing Post for 2025-10-12...
[2025-10-14 23:00:42.456] [AutoUpdate]   ✓ Got 12 races from Racing Post
[2025-10-14 23:00:42.457] [AutoUpdate]   [2/4] Fetching Betfair data...
```

## Log Format

### Structure
```
[TIMESTAMP] LEVEL:  MESSAGE
[2025-10-14 23:01:15.123] INFO:  → GetRace: race_id=339 | IP: 127.0.0.1
```

**Parts**:
- **Timestamp**: `YYYY-MM-DD HH:MM:SS.mmm` (millisecond precision)
- **Level**: `DEBUG`, `INFO`, `WARN`, `ERROR`
- **Message**: Contextual information

### Request/Response Pattern
```
[timestamp] INFO:  → FunctionName: params | IP: x.x.x.x     ← Request received
[timestamp] INFO:  ← FunctionName: results | duration       ← Response sent
```

Or if error:
```
[timestamp] INFO:  → FunctionName: params | IP: x.x.x.x
[timestamp] ERROR: FunctionName: error details
```

## Use Cases

### 1. Debugging Failed Tests

When tests fail, check logs to see the actual error:

```bash
# Start server with DEBUG logging
LOG_LEVEL=DEBUG ./bin/api > /tmp/test-run.log 2>&1 &

# Run tests
./scripts/run_comprehensive_tests.sh

# Check logs for errors
grep ERROR /tmp/test-run.log
grep "repository error" /tmp/test-run.log
```

### 2. Monitor API Usage

```bash
# Start server
./bin/api

# In another terminal, watch requests
tail -f logs/server.log | grep "→"

# See only slow requests (>100ms)
tail -f logs/server.log | grep -E "[0-9]{3,}ms"
```

### 3. Debug Specific Endpoint

```bash
# Filter logs for specific function
tail -f logs/server.log | grep "GetRace:"

# Or specific race ID
tail -f logs/server.log | grep "race_id=339"
```

### 4. Track Auto-Update Progress

```bash
# Watch auto-update in real-time
tail -f logs/server.log | grep AutoUpdate
```

## Log File Management

### Rotation

The log file (`server.log`) appends indefinitely. To rotate:

```bash
# Manual rotation
cd /home/smonaghan/GiddyUp/backend-api/logs
mv server.log server.log.$(date +%Y%m%d_%H%M%S)

# Server will create new server.log automatically on next write
```

### Cleanup Old Logs

```bash
# Delete logs older than 7 days
find /home/smonaghan/GiddyUp/backend-api/logs -name "*.log" -mtime +7 -delete

# Archive old logs
cd /home/smonaghan/GiddyUp/backend-api/logs
tar -czf archive_$(date +%Y%m).tar.gz *.log.*
rm *.log.*
```

### Size Monitoring

```bash
# Check log file size
ls -lh logs/server.log

# If it gets too big (>100MB), rotate it
if [ $(stat -f%z logs/server.log) -gt 104857600 ]; then
    mv logs/server.log logs/server.log.old
fi
```

## Troubleshooting

### "No such file or directory: logs/server.log"

Directory doesn't exist. Server will create it automatically, but if you want to pre-create:

```bash
mkdir -p /home/smonaghan/GiddyUp/backend-api/logs
```

### "Permission denied: logs/server.log"

Log file isn't writable:

```bash
chmod 644 /home/smonaghan/GiddyUp/backend-api/logs/server.log
```

### Logs not showing up

Check that logging is initialized:

```bash
# Look for this in startup logs:
grep "Logging initialized" logs/server.log

# If missing, server didn't initialize file logging
```

### Too verbose / log file growing too fast

Use a higher log level:

```bash
# Only errors (much quieter)
LOG_LEVEL=ERROR ./bin/api

# Or standard INFO level
LOG_LEVEL=INFO ./bin/api
```

## Log Analysis

### Common Queries

```bash
cd /home/smonaghan/GiddyUp/backend-api

# Count errors
grep -c ERROR logs/server.log

# Show all unique error messages
grep ERROR logs/server.log | sort | uniq

# Find slow queries (>1s)
grep "Duration: [0-9]\+s" logs/server.log

# Top 10 most called endpoints
grep "→" logs/server.log | awk '{print $4}' | sort | uniq -c | sort -rn | head -10

# Requests from a specific IP
grep "IP: 192.168.1.100" logs/server.log

# Errors in the last hour
find logs/server.log -mmin -60 -exec grep ERROR {} \;
```

### Performance Analysis

```bash
# Find average response time for GetRace
grep "← GetRace:" logs/server.log | \
  grep -oE '[0-9]+(\.[0-9]+)?ms' | \
  awk '{s+=$1; c++} END {print "Average: " s/c "ms"}'

# Find requests taking >500ms
grep "←" logs/server.log | grep -E "[0-9]{3,}ms" | grep -vE "[0-9]{1,2}ms"
```

## Best Practices

1. **Use DEBUG for development**: `LOG_LEVEL=DEBUG`
2. **Use INFO for production**: `LOG_LEVEL=INFO`
3. **Rotate logs regularly**: Don't let `server.log` grow indefinitely
4. **Monitor error rate**: `grep -c ERROR logs/server.log`
5. **Archive old logs**: Compress and store logs for historical analysis

## Examples

### Full debugging session

```bash
# Terminal 1: Start server with verbose logging
cd /home/smonaghan/GiddyUp/backend-api
LOG_LEVEL=DEBUG ./bin/api

# Terminal 2: Watch logs
tail -f logs/server.log

# Terminal 3: Make API call
curl "http://localhost:8000/api/v1/races?date=2024-01-01"

# Terminal 2 will show:
# → GetRecentRaces: date=2024-01-01, limit=50 | IP: 127.0.0.1
# DEBUG: SQL: SELECT ... | Args: [2024-01-01] | Duration: 45ms
# ← GetRecentRaces: 12 races on 2024-01-01 | 47ms
```

### Production monitoring

```bash
# Start server (INFO level)
LOG_LEVEL=INFO ./bin/api > /dev/null 2>&1 &

# Monitor errors
watch -n 60 'grep ERROR /home/smonaghan/GiddyUp/backend-api/logs/server.log | tail -20'

# Or with alerting
while true; do
    count=$(grep ERROR logs/server.log | wc -l)
    if [ $count -gt 100 ]; then
        echo "ALERT: $count errors in log!"
    fi
    sleep 300
done
```

---

**Quick Reference**:
- Logs location: `logs/server.log`
- Start with logging: `./start_with_logging.sh`
- Debug mode: `LOG_LEVEL=DEBUG ./bin/api`
- View logs: `tail -f logs/server.log`
- Find errors: `grep ERROR logs/server.log`

