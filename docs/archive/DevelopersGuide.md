# Racing Post & Betfair Data Pipeline

A comprehensive data collection, processing, and storage system for UK & Irish horse racing data from Racing Post and Betfair.

## 🎯 Project Overview

This project collects historical and current horse racing data, stitches Racing Post results with Betfair market data, and stores it in a production-ready PostgreSQL database for analysis and UI consumption.

### Data Coverage
- **Racing Post**: Race results from 2006-2025 (GB & Ireland, Flat & Jumps)
- **Betfair**: Win/Place market data from 2007-2025
- **Combined**: Master dataset with matched races and runners

## 📊 Architecture

```
┌─────────────────┐
│  Racing Post    │──┐
│  Web Scraping   │  │
└─────────────────┘  │
                     │    ┌──────────────────┐    ┌──────────────┐
┌─────────────────┐  │    │  Master Data     │    │  PostgreSQL  │
│  Betfair API    │──┼───▶│  Stitcher        │───▶│  Database    │
│  Downloads      │  │    │  (Race Matching) │    │  (Racing)    │
└─────────────────┘  │    └──────────────────┘    └──────────────┘
                     │
┌─────────────────┐  │
│  Daily Updater  │──┘
│  (Automation)   │
└─────────────────┘
```

## 🗂️ Directory Structure

```
rpscrape/
├── data/                          # Raw data storage
│   ├── dates/                     # Racing Post data (by date)
│   │   ├── gb/{flat,jumps}/      # GB races by month
│   │   └── ire/{flat,jumps}/     # IRE races by month
│   ├── betfair_raw/               # Raw Betfair downloads
│   └── betfair_stitched/          # WIN+PLACE combined
│       ├── gb/{flat,jumps}/
│       └── ire/{flat,jumps}/
│
├── master/                        # Stitched master data (RP + Betfair)
│   ├── gb/{flat,jumps}/YYYY-MM/  # Monthly master files
│   │   ├── races_*.csv
│   │   ├── runners_*.csv
│   │   ├── unmatched_*.csv
│   │   └── manifest.json
│   └── ire/{flat,jumps}/YYYY-MM/
│
├── scripts/                       # Core scraping logic
│   ├── rpscrape.py               # Racing Post scraper
│   ├── models/                    # Data models
│   └── utils/                     # Helper functions
│
├── postgres/                      # Database setup
│   ├── init_clean.sql            # Database schema
│   ├── database.md               # Full schema docs
│   └── API_DOCUMENTATION.md      # Developer API guide
│
├── master_data_stitcher.py       # File-based stitching
├── master_data_stitcher_memory.py # In-memory stitching (fast)
├── load_master_to_postgres.py    # PostgreSQL loader
├── daily_updater.py              # Daily automation
├── betfair_backfill.py           # Betfair gap filling
└── scrape_monthly_simple.py      # Monthly RP scraper
```

## 🚀 Quick Start

### 1. Prerequisites
```bash
# Python 3.11+ recommended
python3 -m venv venv
source venv/bin/activate  # or activate.bat on Windows
pip install -r requirements.txt

# PostgreSQL (via Docker)
docker run -d --network=host --name=horse_racing \
  -e POSTGRES_PASSWORD=password \
  postgres:18.0-alpine3.22
```

### 2. Initialize Database
```bash
# Create schema
docker exec horse_racing dropdb -U postgres horse_db --if-exists
docker exec horse_racing createdb -U postgres horse_db
docker cp postgres/init_clean.sql horse_racing:/tmp/
docker exec horse_racing psql -U postgres -d horse_db -f /tmp/init_clean.sql
```

### 3. Load Historical Data
```bash
# Load all master data into PostgreSQL
python3 load_master_to_postgres.py

# Or load specific subset
python3 load_master_to_postgres.py --region gb --type flat --limit 50
```

## 📋 Core Scripts

### Data Collection

#### `scrape_monthly_simple.py`
Monthly Racing Post scraper with parallel processing.
```bash
python3 scrape_monthly_simple.py
```
- Scrapes GB & Ireland (flat + jumps)
- Month-by-month chunking
- Resume capability
- Outputs to `data/dates/{region}/{type}/YYYY_MM_*.csv`

#### `betfair_backfill.py`
Download and stitch Betfair historical data.
```bash
python3 betfair_backfill.py [--dry-run]
```
- Auto-detects missing dates
- Downloads WIN and PLACE markets
- Combines into `betfair_stitched/`

### Data Processing

#### `master_data_stitcher.py`
Stitches Racing Post and Betfair data together.
```bash
python3 master_data_stitcher.py
```
- Matches races by time + horse names (Jaccard similarity)
- Generates stable race_key and runner_key
- Monthly partitioned output
- Idempotent (rerunnable)

**Features:**
- ✅ Name normalization (accents, punctuation, country codes)
- ✅ Time-based matching (±10 minutes)
- ✅ Jaccard similarity scoring (≥60% threshold)
- ✅ Validates data quality
- ✅ Tracks unmatched races

#### `master_data_stitcher_memory.py`
High-performance in-memory version (requires ~8GB RAM).
```bash
python3 master_data_stitcher_memory.py
```
- 3-5x faster than file-based version
- Same output format
- Use when you have ample RAM

### Database Loading

#### `load_master_to_postgres.py`
Loads master CSVs into PostgreSQL.
```bash
# Load all
python3 load_master_to_postgres.py

# Load specific month
python3 load_master_to_postgres.py --region gb --type flat --month 2024-01

# Initialize DB first
python3 load_master_to_postgres.py --init
```

**Features:**
- ✅ Automatic partition creation
- ✅ Dimension deduplication
- ✅ Idempotent upserts
- ✅ Data validation
- ✅ Handles sentinel 1.0 prices

### Automation

#### `daily_updater.py`
Automated daily data collection and stitching.
```bash
python3 daily_updater.py [--dry-run]
```
- Detects missing dates
- Downloads Betfair data
- Scrapes Racing Post
- Stitches and loads to PostgreSQL
- Ready for cron jobs

## 📊 Data Schema

### Master CSV Schema

#### `races_{region}_{type}_{YYYY-MM}.csv`
```
date, region, course, off, race_name, type, class, pattern,
rating_band, age_band, sex_rest, dist, dist_f, dist_m,
going, surface, ran, race_key
```

#### `runners_{region}_{type}_{YYYY-MM}.csv`
```
race_key, num, pos, draw, ovr_btn, btn, horse, age, sex, lbs, hg,
time, secs, dec, jockey, trainer, prize, prize_raw, or, rpr,
sire, dam, damsire, owner, comment,
win_bsp, win_ppwap, win_morningwap, win_ppmax, win_ppmin,
win_ipmax, win_ipmin, win_morning_vol, win_pre_vol, win_ip_vol, win_lose,
place_bsp, place_ppwap, place_morningwap, place_ppmax, place_ppmin,
place_ipmax, place_ipmin, place_morning_vol, place_pre_vol, place_ip_vol, place_win_lose,
runner_key, match_jaccard, match_time_diff_min, match_reason
```

### PostgreSQL Schema

See **`postgres/API_DOCUMENTATION.md`** for full database schema and API guide.

**Key Tables:**
- `racing.races` - Race metadata (partitioned by date)
- `racing.runners` - Runner-level facts with Betfair data
- `racing.horses` - Horse dimension
- `racing.trainers` - Trainer dimension
- `racing.jockeys` - Jockey dimension
- `racing.courses` - Course dimension

## 🔑 Key Concepts

### Race Matching Algorithm
1. **Time Window**: Match RP and Betfair races on same date within ±10 minutes
2. **Horse Set Similarity**: Calculate Jaccard similarity of normalized horse names
3. **Scoring Bonuses**: 
   - +0.5 if runner counts match
   - +0.5 if "handicap" hint matches
   - +0.5 if distance within 0.5f
4. **Accept Threshold**: Jaccard ≥ 0.60 and total score ≥ 1.5

### Stable Keys
- **race_key**: MD5(date|region|course|off|race_name|type)
- **runner_key**: MD5(race_key|horse|num|draw)

These ensure unique, reproducible identifiers across reruns.

### Text Normalization
```python
normalize_text("Seán O'Brien (IRE)") → "sean obrien"
```
- Lowercase
- Strip accents
- Remove punctuation
- Remove country codes (GB), (IRE), (FR)
- Collapse whitespace

## 🔍 Data Quality

### Validation Checks
- ✅ Race count consistency between races and runners
- ✅ `ran` field matches actual runner count
- ✅ No duplicate race_key or runner_key
- ✅ No sentinel 1.0 prices in database
- ✅ All foreign keys valid

### Coverage Stats (Example)
```
Total Races: 95,425
Matched with Betfair: 95,425 (100%)
Unmatched: 59,953 (mostly pre-2007 or non-UK courses)
Total Runners: 1,200,000+
```

## ⚙️ Configuration

### Database Connection
```python
DB_CONFIG = {
    'host': 'localhost',
    'port': 5432,
    'database': 'horse_db',
    'user': 'postgres',
    'password': 'password'
}
```

### Data Directories
```python
RP_DATA_DIR = "/home/smonaghan/rpscrape/data/dates"
BF_DATA_DIR = "/home/smonaghan/rpscrape/data/betfair_stitched"
MASTER_DIR = "/home/smonaghan/rpscrape/master"
```

## 🛠️ Utilities

### Check Progress
```bash
./check_progress.sh
```
Shows scraping progress, active processes, and data coverage.

### Monitor Logs
```bash
tail -f logs/monthly_simple.log
tail -f logs/daily_updater.log
```

## 📈 Performance

### Scraping
- ~10-30 seconds per month (depends on VPN/rate limits)
- Parallel flat + jumps scraping
- Resume capability on failures

### Stitching
- File-based: ~5-10 minutes for full dataset
- In-memory: ~4-7 minutes with 16GB+ RAM
- Incremental: ~1-2 seconds per month

### Database Loading
- ~1-2 seconds per month
- ~15-30 minutes for full historical load
- Parallelizable by region/type

## 🚨 Troubleshooting

### HTTP 403/429 Errors
Racing Post blocks after too many requests. Solutions:
1. Use VPN rotation (see `scrape_racing_post_by_month.py`)
2. Reduce request rate
3. Add delays between requests

### Empty CSV Files
Check for:
- 238-byte files (header only)
- `_INCOMPLETE.csv` suffix
- Delete and re-scrape

### Database Connection Issues
```bash
# Restart PostgreSQL
docker restart horse_racing

# Check if running
docker ps | grep horse_racing
```

### Stitching Mismatches
Check `unmatched_*.csv` files for diagnostics:
- Low Jaccard scores → different horses
- Large time_diff → different race times
- No candidates → Racing Post data missing

## 📚 Further Reading

- **Database Schema**: `postgres/database.md`
- **API Guide**: `postgres/API_DOCUMENTATION.md`
- **PostgreSQL Setup**: `postgres/README.md`

## 🤝 Contributing

### Adding New Data Sources
1. Create scraper in `scripts/`
2. Output to standardized CSV format
3. Update stitcher to handle new fields
4. Add to `master_data_stitcher.py`

### Extending Database
1. Add new tables to `postgres/init_clean.sql`
2. Update `load_master_to_postgres.py`
3. Document in `postgres/database.md`

## 📝 License

Internal project - All rights reserved.

## 👥 Team

For questions, contact your development team lead.

---

**Last Updated**: 2025-10-13
**Version**: 1.0
**Data Coverage**: 2006-2025 (Racing Post), 2007-2025 (Betfair)
