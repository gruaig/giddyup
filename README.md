# GiddyUp - Horse Racing Data Platform

A comprehensive Go-based platform for collecting, processing, and analyzing UK & Irish horse racing data with Betfair market prices.

## 🚀 Quick Start

```bash
# 1. Start PostgreSQL database
cd postgres
docker-compose up -d

# 2. Start API server with auto-update
cd backend-api
AUTO_UPDATE_ON_STARTUP=true ./bin/api

# 3. Access API
curl http://localhost:8000/health
```

## 📁 Project Structure

```
GiddyUp/
├── backend-api/          # Go API server & CLI tools
│   ├── cmd/             # Command-line applications
│   │   ├── api/         # Main API server
│   │   ├── load_master/ # Bulk data loader
│   │   ├── backfill_dates/ # Date range backfiller
│   │   └── check_missing/  # Data gap detector
│   ├── internal/        # Internal packages
│   │   ├── handlers/    # HTTP request handlers
│   │   ├── repository/  # Database queries
│   │   ├── scraper/     # Racing Post & Betfair scrapers
│   │   ├── services/    # Auto-update service
│   │   └── models/      # Data structures
│   └── scripts/         # Demo & test scripts
│
├── postgres/            # Database schema & migrations
│   ├── init.sql        # Schema definition
│   └── db_backup.sql   # Full database backup
│
├── data/               # Cached racing data
│   ├── master/         # Historical master dataset
│   ├── racingpost/     # Racing Post scraped data
│   └── betfair_stitched/ # Merged Betfair prices
│
├── docs/               # Documentation
│   ├── features/       # Feature guides
│   ├── api/           # API documentation
│   └── guides/        # Developer guides
│
└── scripts/           # Python maintenance scripts
```

## 🔧 Components

### Backend API
- **REST API** with 20+ endpoints for querying races, runners, horses, trainers, jockeys
- **Auto-update service** that backfills missing data on startup
- **Efficient set-based loader** for bulk historical data import

### CLI Tools
- **`load_master`** - Loads CSV master data into PostgreSQL (20 years in ~45 min)
- **`backfill_dates`** - Backfills specific date ranges from Racing Post + Betfair
- **`check_missing`** - Detects gaps in database vs expected Betfair data

### Data Pipeline
1. **Scrape** Racing Post for race results & runner details
2. **Fetch** Betfair BSP/PPWAP prices (WIN + PLACE)
3. **Stitch** Betfair data (merge WIN and PLACE markets)
4. **Match** Racing Post races with Betfair by course/time/horse
5. **Load** into PostgreSQL with idempotent upserts

## 📊 Database

**PostgreSQL 16** with optimized schema:
- **Races**: 400K+ races (2005-2025)
- **Runners**: 4.5M+ runners with full Betfair prices
- **Dimensions**: 100+ courses, 200K+ horses, 15K+ trainers, 20K+ jockeys
- **Partitioned by year** for fast queries
- **Indexed** on race_date, course, horse, trainer, jockey

## 🔥 Key Features

### ✨ Auto-Update Service
Automatically backfills missing dates when server starts:
- Finds last date in database
- Backfills from `last_date + 1` to `yesterday`
- Runs in background (non-blocking)
- Aggressive rate limiting (5-8s between races)

**Enable**: `AUTO_UPDATE_ON_STARTUP=true`

[Full documentation →](docs/features/AUTO_UPDATE.md)

### 📈 API Endpoints

**Races & Runners**
- `GET /api/v1/races?date=2025-10-14&course=Ascot` - Search races
- `GET /api/v1/races/{id}` - Get race details
- `GET /api/v1/races/{id}/runners` - Get all runners in a race

**Profiles**
- `GET /api/v1/horses/{id}` - Horse profile & history
- `GET /api/v1/trainers/{id}` - Trainer statistics
- `GET /api/v1/jockeys/{id}` - Jockey performance

**Search**
- `GET /api/v1/search/horses?q=Enable` - Search horses
- `GET /api/v1/search/trainers?q=Gosden` - Search trainers

**Angles & Bias**
- `GET /api/v1/angles` - Get profitable betting angles
- `GET /api/v1/bias/courses/{course}/draw` - Draw bias analysis

[Full API documentation →](docs/API_REFERENCE.md)

## 🛠️ Development

### Prerequisites
- **Go 1.21+**
- **PostgreSQL 16** (via Docker)
- **~2GB disk space** for data cache

### Build

```bash
cd backend-api

# Build all tools
go build -o bin/api ./cmd/api/
go build -o bin/load_master ./cmd/load_master/
go build -o bin/backfill_dates ./cmd/backfill_dates/
go build -o bin/check_missing ./cmd/check_missing/
```

### Run Tests

```bash
cd backend-api
go test ./...

# Or use test scripts
./scripts/run_tests.sh
./scripts/verify_api.sh
```

### Load Historical Data

```bash
# Load 20 years of master data (takes ~45 minutes)
./bin/load_master \
  -dsn "host=localhost port=5432 user=postgres password=password dbname=horse_db sslmode=disable" \
  -master-dir /path/to/master/data

# Or restore from backup (takes ~2 minutes)
docker exec -i horse_racing psql -U postgres -d horse_db < postgres/db_backup.sql
```

## 📖 Documentation

### Features
- [Auto-Update Service](docs/features/AUTO_UPDATE.md) - Automatic data backfilling
- [Auto-Update Example Logs](docs/features/AUTO_UPDATE_EXAMPLE_LOGS.md) - Verbose log examples
- [Auto-Update Flow](docs/AUTO_UPDATE_FLOW_DIAGRAM.md) - System flow diagram

### Guides
- [Quick Start](docs/QUICKSTART.md) - Get up and running
- [Developer Guide](docs/BACKEND_DEVELOPER_GUIDE.md) - Development workflow
- [API Reference](docs/API_REFERENCE.md) - Complete API documentation

### Technical
- [Database Schema](postgres/database.md) - Table structure & indexes
- [Data Pipeline](docs/DATA_PIPELINE_GO_IMPLEMENTATION.md) - Scraping & loading
- [Project Status](docs/FINAL_STATUS.md) - Current state & roadmap

## 🎯 Use Cases

### Betting Analysis
- Find profitable angles (e.g., "trainers after layoff")
- Analyze draw bias at specific courses
- Track horse form trends over time

### Market Research
- Compare BSP vs pre-play prices
- Identify market inefficiencies
- Study odds movements

### Data Science
- Train machine learning models on 4.5M+ runners
- Predict race outcomes
- Analyze trainer/jockey performance patterns

## 🚨 Rate Limiting

The scrapers include **aggressive rate limiting** to avoid being blocked:
- **5-8s delay** between races (with random jitter)
- **15-30s pause** between dates
- **15+ rotating user agents**
- **Circuit breaker** (3 fails = 5 min pause)
- **Exponential backoff** on errors

⚠️ **Important**: Racing Post may block your IP if you scrape too aggressively. Use responsibly!

## 📊 Performance

### Load Times
- **Full historical load**: ~45 minutes (20 years, 768 months)
- **Database restore**: ~2 minutes (from backup)
- **Single date backfill**: ~2-3 minutes (avg 12 races/day)
- **API response time**: <50ms (typical query)

### Data Volume
- **Races**: 400K+ (2005-2025)
- **Runners**: 4.5M+
- **Database size**: ~2.5GB
- **Backup size**: ~920MB (compressed)

## 🔐 Environment Variables

```bash
# API Server
PORT=8000                    # API port (default: 8000)
DATABASE_URL=postgres://...  # PostgreSQL connection string
AUTO_UPDATE_ON_STARTUP=true  # Enable auto-update (default: false)
DATA_DIR=/path/to/data       # Data cache directory

# Auto-Update
AUTO_UPDATE_ON_STARTUP=true  # Enable background backfill
DATA_DIR=/home/smonaghan/GiddyUp/data  # Cache directory
```

## 🐛 Troubleshooting

### Server won't start
```bash
# Check database connection
docker ps | grep horse_racing
psql -h localhost -U postgres -d horse_db

# Check logs
tail -f backend-api/logs/*.log
```

### Missing data
```bash
# Check for gaps
./bin/check_missing -since 2025-01-01 -until 2025-12-31

# Backfill specific dates
./bin/backfill_dates -since 2025-10-01 -until 2025-10-14
```

### Rate limited by Racing Post
- Wait 1-2 hours
- Use a VPN or different IP
- Increase delays in `internal/scraper/results.go`

## 🤝 Contributing

This is a personal project, but suggestions and improvements are welcome!

## 📝 License

Private project - all rights reserved.

## 🙏 Acknowledgments

Data sources:
- **Racing Post** - Race results and runner details
- **Betfair** - BSP and pre-play market prices

---

**Last updated**: October 14, 2025
**Version**: 1.0.0
**Status**: ✅ Production ready

