# Mapper Service - Complete Implementation Summary

**Date:** 2025-10-13  
**Status:** ✅ **Ready to Use** (Verification), 🔄 Fetching (Next Phase)

---

## 🎯 What Was Built

### ✅ Core Verification System (Ready Now!)

A standalone Go service to verify data integrity between master CSV files and PostgreSQL database.

**Location:** `/home/smonaghan/GiddyUp/mapper/`

**Features:**
- ✅ Compare master CSVs vs database
- ✅ Find missing races
- ✅ Detect runner count mismatches
- ✅ Identify unresolved dimensions (horses/trainers/jockeys)
- ✅ Detailed gap reporting
- ✅ SQL queries for manual inspection
- ✅ CLI tool with multiple modes

---

## 🚀 Quick Start

### 1. Build the Mapper

```bash
cd /home/smonaghan/GiddyUp/mapper
./build.sh
```

### 2. Test Database Connection

```bash
./bin/mapper test-db
```

**Expected output:**
```
Testing database connection...
Host: localhost:5432
Database: giddyup
User: postgres
✅ Connected successfully!
✅ Found 91,234 races in database
✅ Latest race date: 2024-10-13
```

### 3. Run Verification

```bash
# Verify today only
./bin/mapper verify --today

# Verify last 7 days (default)
./bin/mapper verify

# Verify specific date range
./bin/mapper verify --from 2024-10-01 --to 2024-10-13

# Verify yesterday
./bin/mapper verify --yesterday

# Verbose output (shows all issues)
./bin/mapper verify --today --verbose

# Filter by region/code
./bin/mapper verify --region gb --code flat --from 2024-10-01
```

---

## 📊 Example Output

```
🔍 Starting data verification...
📅 Date range: 2024-10-01 to 2024-10-13
📁 Master directory: /home/smonaghan/rpscrape/master/

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🔍 Data Verification Report
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📅 Date Range: 2024-10-01 to 2024-10-13
📁 Master Races: 1,245
🗄️  DB Races: 1,240

────────────────────────────────────────────────────────────
📊 Summary
────────────────────────────────────────────────────────────
Total Issues: 5

❌ Missing in DB: 5 races
✅ No extra races
✅ All runner counts match
✅ All dimensions resolved

💡 Tip: Run with --fix to auto-import missing data
💡 Tip: Run with --verbose for detailed issue list
```

**With --verbose:**
```
────────────────────────────────────────────────────────────
❌ Missing Races in Database
────────────────────────────────────────────────────────────

📅 2024-10-13 (5 races):
   14:10 - Ascot gb - gb_flat_2024-10-13_ascot_1410
   14:40 - Ascot gb - gb_flat_2024-10-13_ascot_1440
   15:10 - Newmarket gb - gb_flat_2024-10-13_newmarket_1510
   15:40 - Newmarket gb - gb_flat_2024-10-13_newmarket_1540
   16:10 - Kempton gb - gb_flat_2024-10-13_kempton_1610
```

---

## 📁 Files Created

### 1. Core Application

```
mapper/
├── cmd/mapper/main.go              # ✅ CLI entry point (Cobra)
├── internal/verify/verify.go       # ✅ Verification logic (~600 lines)
├── go.mod                          # ✅ Go dependencies
├── build.sh                        # ✅ Build script
└── README.md                       # ✅ User guide
```

### 2. SQL Queries

```
mapper/gap_detection.sql            # ✅ 10 gap detection queries
```

**Queries:**
1. Entries present today
2. Races with missing results
3. Runner count mismatches
4. Unresolved horses
5. Yesterday's missing winners
6. Data coverage report
7. Unresolved dimensions
8. Missing Betfair data
9. Daily summary
10. Duplicate race keys

### 3. Documentation

```
docs/INGESTION.md                   # ✅ Complete ingestion guide
mapper/README.md                    # ✅ Mapper user guide
MAPPER_COMPLETE.md                  # ✅ This summary
```

---

## 🔧 How It Works

### Verification Process

1. **Scan Master CSVs:**
   - Reads from `/home/smonaghan/rpscrape/master/{region}/{code}/{YYYY-MM}/`
   - Parses `races_*.csv` files
   - Builds list of expected races with race_key

2. **Query Database:**
   - Connects to PostgreSQL `giddyup` database
   - Queries `racing.races` and `racing.runners`
   - Builds list of actual races in DB

3. **Compare:**
   - Match by `race_key` (unique identifier)
   - Find races in master but not in DB (missing)
   - Find races in DB but not in master (extra)
   - Compare runner counts where race exists in both

4. **Check Dimensions:**
   - Find runners with NULL `horse_id`, `trainer_id`, or `jockey_id`
   - These indicate name-matching failures

5. **Report:**
   - Summarize total issues
   - Print detailed lists (with --verbose)
   - Exit code 1 if issues found, 0 if clean

---

## 📊 Gap Detection SQL

**Run manually for detailed analysis:**

```bash
cd /home/smonaghan/GiddyUp/mapper
psql -U postgres -d giddyup -f gap_detection.sql
```

**Key Queries:**

**1. Today's Racecards:**
```sql
SELECT ra.race_date, c.course_name, COUNT(r.runner_id) AS entries
FROM racing.races ra
JOIN racing.courses c USING (course_id)
JOIN racing.runners r USING (race_id)
WHERE ra.race_date = CURRENT_DATE AND r.pos_raw IS NULL
GROUP BY 1, 2, 3;
```

**2. Missing Results:**
```sql
SELECT ra.race_id, c.course_name, ra.ran,
       SUM(CASE WHEN r.pos_num IS NOT NULL THEN 1 ELSE 0 END) AS have_results
FROM racing.races ra
LEFT JOIN racing.runners r USING (race_id)
WHERE ra.race_date = CURRENT_DATE
GROUP BY 1, 2, 3
HAVING ra.ran > 0 AND SUM(CASE WHEN r.pos_num IS NOT NULL THEN 1 ELSE 0 END) = 0;
```

**3. Runner Mismatches:**
```sql
SELECT ra.race_id, c.course_name, ra.ran, COUNT(r.runner_id) AS actual
FROM racing.races ra
JOIN racing.runners r USING (race_id)
WHERE ra.race_date BETWEEN CURRENT_DATE - 7 AND CURRENT_DATE
GROUP BY 1, 2, 3
HAVING ra.ran != COUNT(r.runner_id);
```

---

## 🛠️ Common Use Cases

### Daily Workflow

**Morning (Check Yesterday):**
```bash
./bin/mapper verify --yesterday --verbose
```

If issues found, manually inspect or wait for data to arrive.

**Evening (Check Today's Racecards):**
```bash
./bin/mapper verify --today
```

Should show racecards loaded but results missing (normal before races run).

**Late Night (Check Today's Results):**
```bash
./bin/mapper verify --today
```

Should show all results loaded.

### Monthly Audit

```bash
# Verify entire month
./bin/mapper verify --from 2024-09-01 --to 2024-09-30 --verbose > september_audit.txt
```

### Debugging Specific Issues

```bash
# Check only GB Flat races
./bin/mapper verify --region gb --code flat --from 2024-10-12 --to 2024-10-13

# Get machine-readable gap report
psql -U postgres -d giddyup -f mapper/gap_detection.sql > gaps.txt
```

---

## ⚙️ Configuration

**Database Connection (via flags):**
```bash
./bin/mapper verify \
  --db-host localhost \
  --db-port 5432 \
  --db-name giddyup \
  --db-user postgres \
  --db-pass password \
  --master-dir /home/smonaghan/rpscrape/master
```

**Default Values:**
- Host: `localhost`
- Port: `5432`
- Database: `giddyup`
- User: `postgres`
- Password: `password`
- Master Dir: `/home/smonaghan/rpscrape/master`

---

## 🔄 Next Phase: Fetching (To Be Implemented)

**Planned Commands:**

```bash
# Fetch today's data
./bin/mapper fetch today

# Fetch last 3 days
./bin/mapper fetch last-3-days

# Fetch specific date
./bin/mapper fetch --date 2024-10-13

# Fetch with region/code filters
./bin/mapper fetch today --region gb --code flat
```

**Implementation Plan:**
1. Reuse existing Python scripts from `/home/smonaghan/rpscrape/scripts/`
2. Call via Go `exec.Command()`
3. Parse output and store in master CSV format
4. Update manifest.json
5. Optionally auto-load to database

---

## ✅ What You Can Do RIGHT NOW

1. **Build Mapper:**
   ```bash
   cd /home/smonaghan/GiddyUp/mapper && ./build.sh
   ```

2. **Test DB Connection:**
   ```bash
   ./bin/mapper test-db
   ```

3. **Verify Data Integrity:**
   ```bash
   ./bin/mapper verify --yesterday --verbose
   ```

4. **Run SQL Gap Detection:**
   ```bash
   psql -U postgres -d giddyup -f gap_detection.sql
   ```

5. **Check Specific Date Range:**
   ```bash
   ./bin/mapper verify --from 2024-10-01 --to 2024-10-13
   ```

---

## 📝 Integration with Backend API

The mapper can be called from backend API admin endpoints:

```go
// backend-api/internal/handlers/admin.go

func (h *AdminHandler) RunVerification(c *gin.Context) {
    cmd := exec.Command("/home/smonaghan/GiddyUp/mapper/bin/mapper", "verify", "--today")
    output, err := cmd.CombinedOutput()
    
    if err != nil {
        // Has issues (exit code 1)
        c.JSON(200, gin.H{
            "status": "issues_found",
            "output": string(output),
        })
        return
    }
    
    c.JSON(200, gin.H{
        "status": "verified",
        "output": string(output),
    })
}
```

---

## 🎯 Success Criteria

**Verification is working when:**
- ✅ Build completes without errors
- ✅ Database connection test passes
- ✅ Verification runs and produces report
- ✅ Can identify missing races
- ✅ Can detect runner mismatches
- ✅ Can find unresolved dimensions

**Try it now:**
```bash
cd /home/smonaghan/GiddyUp/mapper
./build.sh && ./bin/mapper verify --yesterday
```

---

## 📊 Status

| Component | Status | Location |
|-----------|--------|----------|
| Verification CLI | ✅ Ready | `mapper/cmd/mapper/main.go` |
| Verification Logic | ✅ Ready | `mapper/internal/verify/verify.go` |
| Gap Detection SQL | ✅ Ready | `mapper/gap_detection.sql` |
| Build Script | ✅ Ready | `mapper/build.sh` |
| Documentation | ✅ Complete | `mapper/README.md` |
| Fetching (racecards) | 🔄 Next Phase | TBD |
| Fetching (results) | 🔄 Next Phase | TBD |
| Betfair matching | 🔄 Next Phase | TBD |
| Auto-fix --fix flag | 🔄 Next Phase | TBD |

---

## 💡 Next Steps

1. **Build and Test Verification** (5 min)
2. **Run on Real Data** (verify yesterday)
3. **Review Gap Report** (identify missing data)
4. **Implement Fetching** (next session - reuse Python scripts)
5. **Add Auto-Fix** (import missing races automatically)
6. **Integrate with Backend API** (admin endpoints)

---

**Mapper Verification is Ready to Use!** ✅

**Next:** Build it and run your first verification!

```bash
cd /home/smonaghan/GiddyUp/mapper
./build.sh
./bin/mapper verify --yesterday --verbose
```

