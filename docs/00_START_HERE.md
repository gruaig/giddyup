# 🏇 GiddyUp - Start Here

**Welcome to GiddyUp!** Your comprehensive UK/IRE horse racing data platform.

---

## 📚 Quick Navigation

### Essential Guides (Read in Order)
1. **[00_START_HERE.md](./00_START_HERE.md)** ← You are here
2. **[01_DEVELOPER_GUIDE.md](./01_DEVELOPER_GUIDE.md)** - Setup, architecture, workflows
3. **[02_API_DOCUMENTATION.md](./02_API_DOCUMENTATION.md)** - REST API endpoints
4. **[03_DATABASE_GUIDE.md](./03_DATABASE_GUIDE.md)** - Schema, queries, optimization
5. **[04_FRONTEND_GUIDE.md](./04_FRONTEND_GUIDE.md)** - UI integration guide
6. **[05_DEPLOYMENT_GUIDE.md](./05_DEPLOYMENT_GUIDE.md)** - Production deployment
7. **[06_SPORTING_LIFE_API.md](./06_SPORTING_LIFE_API.md)** - Data source details ⭐ NEW

### Feature-Specific Docs
- **[features/AUTO_UPDATE.md](./features/AUTO_UPDATE.md)** - Automatic data updates
- **[UI_LIVE_PRICES_GUIDE.md](./UI_LIVE_PRICES_GUIDE.md)** - Live Betfair prices integration
- **[UI_DEVELOPER_README.md](./UI_DEVELOPER_README.md)** - Frontend developer handoff

### Status & History
- **[SPORTING_LIFE_COMPLETE.md](./SPORTING_LIFE_COMPLETE.md)** - Latest implementation (Oct 16, 2025)
- **[archive/](./archive/)** - Historical documentation

---

## 🚀 What is GiddyUp?

GiddyUp is a **complete horse racing data platform** that provides:

✅ **Comprehensive Race Data**
- All UK/IRE races (Flat, Hurdle, Chase, NH Flat)
- Complete runner information (jockey, trainer, owner, form, etc.)
- Historical data from 2021-present

✅ **Live Betfair Integration**
- Real-time market prices (WIN + PLACE)
- Betfair selection ID matching
- Best bookmaker odds

✅ **Rich API**
- RESTful endpoints
- Race listings, runner details, form analysis
- Horse/jockey/trainer profiles with statistics

✅ **Auto-Updating**
- Fetches today/tomorrow races on startup
- Parallel loading for speed
- Smart caching for performance

---

## 🎯 Architecture Overview

```
┌─────────────────────────────────────────────────────┐
│                   DATA SOURCES                       │
├─────────────────────────────────────────────────────┤
│  Sporting Life API   →   Race Data (jockey/trainer) │
│  Betfair API-NG     →   Live Prices + Selection IDs │
│  Betfair CSV        →   Historical BSP/PPWAP Data   │
└─────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────┐
│                 AUTO-UPDATE SERVICE                  │
├─────────────────────────────────────────────────────┤
│  • Fetches today/tomorrow in parallel               │
│  • Caches results locally                            │
│  • Matches Betfair markets by selection ID          │
│  • Updates live prices every 30s                     │
└─────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────┐
│                 POSTGRESQL DATABASE                  │
├─────────────────────────────────────────────────────┤
│  Schema: racing                                      │
│  Tables: courses, horses, jockeys, trainers,        │
│          owners, races, runners                      │
│  Indexes: Optimized for fast queries                │
└─────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────┐
│                    REST API (Go)                     │
├─────────────────────────────────────────────────────┤
│  Port: 8000                                          │
│  Endpoints: /api/v1/*                                │
│  Features: CORS, logging, error handling             │
└─────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────┐
│                 FRONTEND (Your UI)                   │
├─────────────────────────────────────────────────────┤
│  • Race cards with live prices                       │
│  • Horse profiles with form                          │
│  • Jockey/trainer statistics                         │
│  • Search and filtering                              │
└─────────────────────────────────────────────────────┘
```

---

## 🛠️ Tech Stack

### Backend
- **Language**: Go 1.21+
- **Framework**: Chi router
- **Database**: PostgreSQL 15+
- **External APIs**: Sporting Life, Betfair API-NG

### Data Sources
- **Primary**: Sporting Life API (racecards, runner details, odds)
- **Prices**: Betfair API-NG (live WIN/PLACE markets)
- **Historical**: Betfair CSV files (BSP, PPWAP, etc.)

### Tools
- **Migrations**: SQL scripts in `postgres/migrations/`
- **Caching**: JSON files in `/data/sportinglife/`
- **Logging**: Structured logs to `backend-api/logs/`

---

## 📦 Project Structure

```
GiddyUp/
├── backend-api/           # Go API server
│   ├── cmd/              # Executables (api, backfill_dates, etc.)
│   ├── internal/         # Core application code
│   │   ├── handlers/    # HTTP request handlers
│   │   ├── repository/  # Database queries
│   │   ├── scraper/     # Sporting Life scraper
│   │   ├── services/    # Business logic (auto-update)
│   │   └── betfair/     # Betfair integration
│   └── logs/            # Server logs
├── postgres/             # Database setup
│   ├── init.sql         # Initial schema
│   ├── migrations/      # Schema changes
│   └── README.md        # Database docs
├── data/                 # Cached data
│   ├── sportinglife/    # Race data cache
│   ├── betfair_stitched/ # Betfair CSV processed
│   └── master/          # Combined datasets
├── docs/                 # Documentation (you are here!)
└── scripts/             # Utility scripts (Python)
```

---

## ⚡ Quick Start

### 1. Prerequisites
```bash
# Install Go 1.21+
go version

# Install PostgreSQL 15+
psql --version

# Set environment variables
cp settings.env.example settings.env
# Edit settings.env with your credentials
```

### 2. Database Setup
```bash
cd postgres
psql -U postgres < init.sql
```

### 3. Build and Run API
```bash
cd backend-api
go build -o bin/api cmd/api/main.go
source ../settings.env
./bin/api
```

### 4. Test API
```bash
# Health check
curl http://localhost:8000/health

# Get today's races
curl http://localhost:8000/api/v1/races/today

# Get a horse profile
curl http://localhost:8000/api/v1/horses/123456/profile
```

---

## 🔑 Key Concepts

### 1. **Sporting Life API V2**
- **PRIMARY DATA SOURCE** (Racing Post removed!)
- Fetches from 2 endpoints per race:
  1. `/race/{id}` → Jockey, trainer, owner, form
  2. `/v2/racing/betting/{id}` → Odds, Betfair selection ID
- See [06_SPORTING_LIFE_API.md](./06_SPORTING_LIFE_API.md)

### 2. **Betfair Matching**
- Uses **selection ID** for perfect matching (no name normalization!)
- Stored in `racing.runners.betfair_selection_id`
- Enables direct lookup for live prices

### 3. **Auto-Update Service**
- Runs on startup and daily
- Fetches today + tomorrow **in parallel**
- Caches locally for fast re-loads
- See [features/AUTO_UPDATE.md](./features/AUTO_UPDATE.md)

### 4. **Database Schema**
- Normalized design (courses, horses, jockeys, trainers, owners, races, runners)
- Foreign keys for data integrity
- Indexes for performance
- See [03_DATABASE_GUIDE.md](./03_DATABASE_GUIDE.md)

---

## 📖 Common Tasks

### Add a New API Endpoint
1. Define handler in `internal/handlers/`
2. Add route in `internal/router/router.go`
3. Add repository query in `internal/repository/`
4. Document in [02_API_DOCUMENTATION.md](./02_API_DOCUMENTATION.md)

### Backfill Historical Data
```bash
cd backend-api
./bin/backfill_dates --start-date 2024-01-01 --end-date 2024-12-31
```

### Update Database Schema
1. Create migration file in `postgres/migrations/`
2. Number sequentially (e.g., `011_add_column.sql`)
3. Apply: `psql -U postgres -d horse_db < migrations/011_add_column.sql`

### Debug Auto-Update Issues
```bash
# Watch logs in real-time
tail -f backend-api/logs/server.log

# Check for errors
grep ERROR backend-api/logs/server.log

# Verify cache
ls -lh data/sportinglife/
```

---

## 🐛 Troubleshooting

### API Won't Start
```bash
# Check if port 8000 is in use
lsof -ti :8000

# Kill existing process
pkill -f "bin/api"
```

### No Race Data Loading
- Check `logs/server.log` for errors
- Verify Sporting Life API is accessible
- Ensure database credentials are correct

### Betfair Prices Not Updating
- Check Betfair API credentials in `settings.env`
- Verify `ENABLE_LIVE_PRICES=true`
- See [UI_LIVE_PRICES_GUIDE.md](./UI_LIVE_PRICES_GUIDE.md)

---

## 📞 Support & Contact

### Documentation
- Full guides in `docs/` directory
- API examples in [02_API_DOCUMENTATION.md](./02_API_DOCUMENTATION.md)
- Frontend guide in [UI_DEVELOPER_README.md](./UI_DEVELOPER_README.md)

### Code Comments
- All functions have doc comments
- Complex logic explained inline
- See `internal/` directories

---

## 🎉 Recent Updates

### October 16, 2025 - Sporting Life V2 ⭐
- ✅ Racing Post completely removed
- ✅ 2-endpoint merge strategy implemented
- ✅ Betfair selection IDs captured
- ✅ Parallel today/tomorrow fetching
- ✅ Improved type handling and error recovery
- 📄 See [SPORTING_LIFE_COMPLETE.md](./SPORTING_LIFE_COMPLETE.md)

### Previous Milestones
- Database optimization (Oct 14, 2025)
- Live prices integration (Oct 13, 2025)
- Auto-update service (Oct 12, 2025)

---

## 🚦 Next Steps

1. **For Developers**: Read [01_DEVELOPER_GUIDE.md](./01_DEVELOPER_GUIDE.md)
2. **For Frontend**: Read [UI_DEVELOPER_README.md](./UI_DEVELOPER_README.md)
3. **For API Users**: Read [02_API_DOCUMENTATION.md](./02_API_DOCUMENTATION.md)
4. **For DB Work**: Read [03_DATABASE_GUIDE.md](./03_DATABASE_GUIDE.md)

---

**Welcome aboard! Happy coding! 🏇🚀**
