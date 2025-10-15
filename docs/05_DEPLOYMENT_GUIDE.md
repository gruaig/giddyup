# Deployment Guide - GiddyUp API

**Production deployment and operations guide**

## Quick Deploy

```bash
# 1. Start database
docker-compose -f postgres/docker-compose.yml up -d

# 2. Restore data
docker exec -i horse_racing psql -U postgres -d horse_db < postgres/db_backup.sql

# 3. Start API with auto-update
cd backend-api
AUTO_UPDATE_ON_STARTUP=true ./bin/api
```

**Server will be ready in ~5 seconds** on http://localhost:8000

---

## Production Checklist

### Pre-Deployment

- [ ] All tests passing (32/33, 97%)
- [ ] Environment variables configured
- [ ] Database backup created
- [ ] Logs directory exists
- [ ] Port 8000 available (or configured)
- [ ] PostgreSQL 16 container running
- [ ] Network connectivity verified

### Post-Deployment

- [ ] Health check returns 200
- [ ] Sample API queries work
- [ ] Logs writing to `logs/server.log`
- [ ] Auto-update service starts (if enabled)
- [ ] Database connections stable

---

## Environment Configuration

### Required Variables

```bash
# Database Connection
export DATABASE_HOST=localhost
export DATABASE_PORT=5432
export DATABASE_NAME=horse_db
export DATABASE_USER=postgres
export DATABASE_PASSWORD=your_secure_password

# Server
export SERVER_PORT=8000
export SERVER_ENV=production
export CORS_ORIGINS=https://yourdomain.com

# Logging
export LOG_LEVEL=INFO          # INFO for production
export LOG_DIR=/var/log/giddyup

# Auto-Update (optional)
export AUTO_UPDATE_ON_STARTUP=true
export DATA_DIR=/home/smonaghan/GiddyUp/data
```

### Create systemd Service

```ini
# /etc/systemd/system/giddyup-api.service

[Unit]
Description=GiddyUp Racing API
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=smonaghan
WorkingDirectory=/home/smonaghan/GiddyUp/backend-api
Environment="DATABASE_HOST=localhost"
Environment="DATABASE_PORT=5432"
Environment="DATABASE_NAME=horse_db"
Environment="DATABASE_USER=postgres"
Environment="DATABASE_PASSWORD=password"
Environment="SERVER_PORT=8000"
Environment="LOG_LEVEL=INFO"
Environment="AUTO_UPDATE_ON_STARTUP=true"
Environment="DATA_DIR=/home/smonaghan/GiddyUp/data"
ExecStart=/home/smonaghan/GiddyUp/backend-api/bin/api
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

**Enable and start**:
```bash
sudo systemctl daemon-reload
sudo systemctl enable giddyup-api
sudo systemctl start giddyup-api

# Check status
sudo systemctl status giddyup-api

# View logs
sudo journalctl -u giddyup-api -f
```

---

## Database Setup

### PostgreSQL via Docker

**docker-compose.yml**:
```yaml
version: '3.8'

services:
  postgres:
    image: postgres:16
    container_name: horse_racing
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: password
      POSTGRES_DB: horse_db
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

volumes:
  postgres_data:
```

**Start**:
```bash
cd /home/smonaghan/GiddyUp/postgres
docker-compose up -d
```

### Initial Data Load

**Option 1: Restore from backup** (2 minutes)
```bash
docker exec -i horse_racing psql -U postgres -d horse_db < postgres/db_backup.sql

# Create materialized view
docker exec horse_racing psql -U postgres -d horse_db -c "
CREATE MATERIALIZED VIEW racing.mv_runner_base AS
SELECT 
    ru.runner_id, ru.race_id, ru.race_date, ru.horse_id,
    ru.trainer_id, ru.jockey_id, ru.pos_num, ru.rpr, ru.win_flag,
    r.course_id, r.race_type, r.class, r.dist_f, r.going
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
WHERE ru.pos_num IS NOT NULL;

CREATE INDEX idx_mv_runner_base_horse ON racing.mv_runner_base (horse_id, race_date DESC);
CREATE INDEX idx_mv_runner_base_race ON racing.mv_runner_base (race_id);
CREATE INDEX idx_mv_runner_base_trainer ON racing.mv_runner_base (trainer_id, race_date DESC);
CREATE INDEX idx_mv_runner_base_jockey ON racing.mv_runner_base (jockey_id, race_date DESC);
"
```

**Option 2: Load from CSV** (45 minutes)
```bash
cd backend-api
./bin/load_master -v
```

---

## Auto-Update Service

### Enable Automatic Backfilling

```bash
# Start API with auto-update enabled
AUTO_UPDATE_ON_STARTUP=true ./bin/api
```

**What it does**:
1. Waits 5 seconds after server starts
2. Queries `MAX(race_date)` from database
3. Backfills from `last_date + 1` to `yesterday`
4. Runs in background (non-blocking)
5. Logs verbose progress

**Expected logs**:
```
[AutoUpdate] 🔍 Checking for missing data...
[AutoUpdate] 📅 Backfilling 3 days (2025-10-12 to 2025-10-14)...
[AutoUpdate] Processing 2025-10-12...
[AutoUpdate]   [1/4] Scraping Racing Post...
[AutoUpdate]   [2/4] Fetching Betfair data...
[AutoUpdate]   [3/4] Matching...
[AutoUpdate]   [4/4] Inserting to database...
[AutoUpdate] ✅ 2025-10-12: 43 races, 476 runners
[AutoUpdate] ⏸️  Pausing 23s before next date...
```

**Performance**: ~2-3 minutes per day

See `features/AUTO_UPDATE.md` for complete documentation.

---

## Monitoring

### Health Checks

```bash
# API health
curl http://localhost:8000/health
# Expected: {"status":"healthy"}

# Database health
docker exec horse_racing psql -U postgres -d horse_db -c "SELECT 1;"
# Expected: 1 row

# Data freshness
docker exec horse_racing psql -U postgres -d horse_db -c "
SELECT MAX(race_date) FROM racing.races;"
# Expected: Yesterday or today
```

### Logging

**Log Location**: `logs/server.log`

**Monitor errors**:
```bash
# Watch for errors
tail -f logs/server.log | grep ERROR

# Count errors per hour
grep ERROR logs/server.log | grep "$(date +%Y-%m-%d)" | wc -l

# Find slow queries (>1s)
grep -E "[0-9]+s\)" logs/server.log
```

### Metrics to Track

1. **Request Rate**: Requests per minute
2. **Error Rate**: Errors per minute
3. **Response Time**: P50, P95, P99
4. **Database Connections**: Active connections
5. **Data Freshness**: Hours since last update

---

## Backup & Recovery

### Daily Backup

```bash
#!/bin/bash
# /home/smonaghan/scripts/backup-giddyup.sh

DATE=$(date +%Y%m%d)
BACKUP_DIR=/home/smonaghan/backups
mkdir -p $BACKUP_DIR

# Create backup
docker exec horse_racing pg_dump -U postgres horse_db > $BACKUP_DIR/giddyup_$DATE.sql

# Compress
gzip $BACKUP_DIR/giddyup_$DATE.sql

# Keep last 7 days
find $BACKUP_DIR -name "giddyup_*.sql.gz" -mtime +7 -delete

echo "✅ Backup complete: giddyup_$DATE.sql.gz"
```

**Cron schedule** (daily at 3 AM):
```cron
0 3 * * * /home/smonaghan/scripts/backup-giddyup.sh
```

### Restore from Backup

```bash
# 1. Stop API
sudo systemctl stop giddyup-api

# 2. Drop database
docker exec horse_racing psql -U postgres -c "DROP DATABASE horse_db;"
docker exec horse_racing psql -U postgres -c "CREATE DATABASE horse_db;"

# 3. Restore
gunzip -c backups/giddyup_20251015.sql.gz | \
  docker exec -i horse_racing psql -U postgres -d horse_db

# 4. Recreate materialized view (if not in backup)
docker exec horse_racing psql -U postgres -d horse_db -c "
CREATE MATERIALIZED VIEW racing.mv_runner_base AS ...
"

# 5. Start API
sudo systemctl start giddyup-api

# 6. Verify
curl http://localhost:8000/health
```

---

## Scaling

### Vertical Scaling

**Increase database resources**:
```yaml
# docker-compose.yml
services:
  postgres:
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 8G
```

**Optimize PostgreSQL**:
```ini
# postgres.conf
shared_buffers = 2GB
effective_cache_size = 6GB
maintenance_work_mem = 512MB
max_connections = 200
```

### Horizontal Scaling

**Read Replicas** for analytics:
1. Set up streaming replication
2. Route analytical queries to replica
3. Route writes to primary

**API Instances**:
1. Run multiple API instances
2. Use nginx/HAProxy for load balancing
3. All instances connect to same database

---

## Troubleshooting

### Server Won't Start

**Check logs**:
```bash
tail -50 logs/server.log
# or
sudo journalctl -u giddyup-api -n 50
```

**Common issues**:
- Port already in use: Change `SERVER_PORT`
- Database unreachable: Check `docker ps | grep horse_racing`
- Missing env vars: Check systemd service file

### Slow Performance

**Check database**:
```sql
-- Find slow queries
SELECT 
    query, 
    calls, 
    mean_exec_time,
    max_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;
```

**Solutions**:
- Add indexes
- Refresh materialized views
- Vacuum tables
- Check for table bloat

### Database Connection Issues

```bash
# Check connections
docker exec horse_racing psql -U postgres -d horse_db -c "
SELECT count(*) FROM pg_stat_activity WHERE datname = 'horse_db';"

# Check for locks
docker exec horse_racing psql -U postgres -d horse_db -c "
SELECT * FROM pg_locks WHERE NOT granted;"
```

---

## Maintenance Schedule

### Daily (Automated)
- ✅ Auto-update backfills yesterday's data
- ✅ Logs rotated (if using logrotate)
- ✅ Database backup created

### Weekly
```bash
# Refresh materialized views
docker exec horse_racing psql -U postgres -d horse_db -c "
REFRESH MATERIALIZED VIEW racing.mv_runner_base;"

# Vacuum and analyze
docker exec horse_racing psql -U postgres -d horse_db -c "
VACUUM ANALYZE racing.races;
VACUUM ANALYZE racing.runners;"
```

### Monthly
```bash
# Check database size
docker exec horse_racing psql -U postgres -d horse_db -c "
SELECT pg_size_pretty(pg_database_size('horse_db'));"

# Check for partition maintenance needs
# Archive old backups
find /home/smonaghan/backups -name "*.sql.gz" -mtime +30 -delete
```

---

## Security

### Production Hardening

1. **Use read-only database user for API**:
```sql
CREATE USER racing_api WITH PASSWORD 'secure_password';
GRANT USAGE ON SCHEMA racing TO racing_api;
GRANT SELECT ON ALL TABLES IN SCHEMA racing TO racing_api;
```

2. **Enable SSL**:
```bash
export DATABASE_SSLMODE=require
```

3. **Firewall rules**:
```bash
# Only allow API server to access PostgreSQL
sudo ufw allow from <api-server-ip> to any port 5432
```

4. **Rate limiting**:
- Implement nginx rate limiting
- Add API key authentication

---

## Disaster Recovery

### Scenario 1: Database Corruption

```bash
# 1. Stop API
sudo systemctl stop giddyup-api

# 2. Restore from latest backup
gunzip -c backups/giddyup_latest.sql.gz | \
  docker exec -i horse_racing psql -U postgres -d horse_db

# 3. Verify
docker exec horse_racing psql -U postgres -d horse_db -c "
SELECT COUNT(*) FROM racing.races;"

# 4. Restart API
sudo systemctl start giddyup-api
```

### Scenario 2: API Server Crash

```bash
# Check status
sudo systemctl status giddyup-api

# View crash logs
sudo journalctl -u giddyup-api -n 100

# Restart
sudo systemctl restart giddyup-api
```

### Scenario 3: Data Loss

```bash
# Check what data exists
docker exec horse_racing psql -U postgres -d horse_db -c "
SELECT MIN(race_date), MAX(race_date), COUNT(*) FROM racing.races;"

# Use check_missing to find gaps
cd backend-api
./bin/check_missing -since 2024-01-01 -until 2025-10-15

# Backfill missing dates
./bin/backfill_dates -since 2025-10-01 -until 2025-10-14 -dry-run=false
```

---

**Status**: ✅ Production Ready  
**Last Updated**: October 15, 2025  
**Version**: 1.0.0

