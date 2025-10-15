# Developer Quick Reference

## 🚀 Getting Started

### Database Connection (CRITICAL!)

**All database connections MUST set search_path:**

```sql
SET search_path TO racing, public;
```

Without this, you won't see any tables!

---

## 📊 Database Stats (Oct 2025)

```
Database: horse_db
Schema:   racing
```

| Table | Row Count | Description |
|-------|-----------|-------------|
| `races` | 168,070 | Race metadata (partitioned 2007-2025) |
| `runners` | 1,610,337 | Runner-level data + Betfair markets |
| `horses` | 141,196 | Horse dimension |
| `trainers` | 3,659 | Trainer dimension |
| `jockeys` | 4,231 | Jockey dimension |
| `courses` | 89 | Course dimension |
| `owners` | - | Owner dimension |
| `bloodlines` | - | Sire/Dam dimension |

---

## 🔌 Connection Examples

### Python (psycopg2)
```python
import psycopg2

conn = psycopg2.connect(
    host='localhost',
    port=5432,
    database='horse_db',
    user='racing_api',
    password='your_password'
)

# CRITICAL: Set search_path
with conn.cursor() as cur:
    cur.execute("SET search_path TO racing, public;")

# Now you can query
with conn.cursor() as cur:
    cur.execute("SELECT COUNT(*) FROM races;")
    print(cur.fetchone())  # (168070,)
```

### Node.js (pg)
```javascript
const { Pool } = require('pg');

const pool = new Pool({
  host: 'localhost',
  port: 5432,
  database: 'horse_db',
  user: 'racing_api',
  password: 'your_password',
});

// Set search_path on every connection
pool.on('connect', (client) => {
  client.query('SET search_path TO racing, public;');
});

// Now you can query
const result = await pool.query('SELECT COUNT(*) FROM races;');
console.log(result.rows[0]);  // { count: '168070' }
```

### Direct psql
```bash
# Connect to database
psql -h localhost -U postgres -d horse_db

# Set search_path
horse_db=# SET search_path TO racing, public;
SET

# Now list tables
horse_db=# \dt
# You'll see: races, runners, horses, trainers, jockeys, etc.

# Verify data
horse_db=# SELECT COUNT(*) FROM races;
  count  
---------
 168070
```

---

## 🔍 Quick Queries

### Check Schema
```sql
-- List all tables in racing schema
SELECT tablename 
FROM pg_tables 
WHERE schemaname = 'racing' 
ORDER BY tablename;
```

### Verify Data
```sql
SET search_path TO racing, public;

-- Row counts
SELECT 
    'races' as table, COUNT(*)::text as count FROM races
UNION ALL
    SELECT 'runners', COUNT(*)::text FROM runners
UNION ALL
    SELECT 'horses', COUNT(*)::text FROM horses
UNION ALL
    SELECT 'trainers', COUNT(*)::text FROM trainers
UNION ALL
    SELECT 'jockeys', COUNT(*)::text FROM jockeys
UNION ALL
    SELECT 'courses', COUNT(*)::text FROM courses
ORDER BY 1;
```

### Sample Queries
```sql
SET search_path TO racing, public;

-- Recent races
SELECT race_date, course_id, race_name, ran 
FROM races 
ORDER BY race_date DESC 
LIMIT 10;

-- Top horses by runs
SELECT h.horse_name, COUNT(*) as runs
FROM runners ru
JOIN horses h ON h.horse_id = ru.horse_id
GROUP BY h.horse_id, h.horse_name
ORDER BY runs DESC
LIMIT 10;

-- Partition info
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'racing' 
  AND tablename LIKE 'races_20%'
ORDER BY tablename DESC
LIMIT 10;
```

---

## 📝 Common Issues & Solutions

### Issue: "Did not find any tables"
**Cause**: search_path not set  
**Fix**: Run `SET search_path TO racing, public;`

### Issue: "relation does not exist"
**Cause**: Missing schema prefix or search_path  
**Fix**: Either set search_path OR use `racing.table_name`

### Issue: Tables shown as empty
**Cause**: Connected to wrong database  
**Fix**: Connect to `horse_db` (not `postgres`)

### Issue: Permission denied
**Cause**: User doesn't have USAGE on schema  
**Fix**: 
```sql
GRANT USAGE ON SCHEMA racing TO your_user;
GRANT SELECT ON ALL TABLES IN SCHEMA racing TO your_user;
```

---

## 📚 Documentation Files

### For Backend Developer:
1. **`BACKEND_DEVELOPER_GUIDE.md`** ⭐ - Your main guide
   - All 30 API endpoints
   - Complete SQL queries
   - Security & caching

2. **`postgres/database.md`** - Complete schema DDL

3. **`postgres/API_DOCUMENTATION.md`** - Query examples

### For Frontend Developer:
1. **`FRONTEND_DEVELOPER_GUIDE.md`** ⭐ - Your main guide
   - All 30 UI components
   - TypeScript types
   - Chart examples

### Shared:
- **`README.md`** - Project overview
- **`QUICK_START.md`** - Setup guide

---

## ⚡ Performance Tips

1. **Always use search_path** - Avoids schema qualification overhead
2. **Use partitions** - Filter by `race_date` for partition pruning
3. **Leverage indexes** - All name columns have trigram (GIN) indexes
4. **Batch queries** - Use `WHERE id = ANY($1)` for multiple IDs
5. **Use CTEs** - PostgreSQL optimizes CTEs well

---

## 🎯 Next Steps

### Backend Developer:
```bash
# 1. Read the full guide
cat BACKEND_DEVELOPER_GUIDE.md

# 2. Test database connection
psql -h localhost -U postgres -d horse_db

# 3. Set search_path and verify
SET search_path TO racing, public;
\dt

# 4. Start building API
# (Choose framework and implement endpoints)
```

### Frontend Developer:
```bash
# 1. Read the full guide
cat FRONTEND_DEVELOPER_GUIDE.md

# 2. Coordinate with backend on API base URL

# 3. Set up project
npx create-next-app racing-ui --typescript --tailwind

# 4. Start building components
```

---

## 🔗 Quick Links

- Database: `localhost:5432/horse_db`
- Schema: `racing`
- Races: 168K (2007-2025)
- Runners: 1.6M
- All indexes: ✅ Created
- All partitions: ✅ Created
- Data quality: ✅ Validated

**Ready to build!** 🚀

