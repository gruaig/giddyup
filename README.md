# 🏇 GiddyUp - UK/IRE Horse Racing Data Platform

**Comprehensive horse racing data API with live Betfair integration**

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791?style=flat&logo=postgresql)](https://www.postgresql.org/)
[![License](https://img.shields.io/badge/License-Proprietary-red)](./LICENSE)

---

## 🚀 What is GiddyUp?

GiddyUp provides **complete UK/IRE horse racing data** through a fast, reliable REST API:

- ✅ **All UK/IRE races** - Flat, Hurdle, Chase, NH Flat
- ✅ **Complete runner data** - Jockey, trainer, owner, form, odds
- ✅ **Live Betfair prices** - Real-time WIN/PLACE markets
- ✅ **Historical data** - 2021-present with Betfair BSP/PPWAP
- ✅ **Rich profiles** - Horse, jockey, trainer statistics
- ✅ **Auto-updating** - Today/tomorrow loaded automatically

---

## 📚 Documentation

**Start here** 👉 **[docs/00_START_HERE.md](./docs/00_START_HERE.md)**

### Essential Guides
1. **[Developer Guide](./docs/01_DEVELOPER_GUIDE.md)** - Setup, architecture, workflows
2. **[API Documentation](./docs/02_API_DOCUMENTATION.md)** - REST endpoints
3. **[Database Guide](./docs/03_DATABASE_GUIDE.md)** - Schema & queries
4. **[Frontend Guide](./docs/04_FRONTEND_GUIDE.md)** - UI integration
5. **[Deployment Guide](./docs/05_DEPLOYMENT_GUIDE.md)** - Production setup
6. **[Sporting Life API](./docs/06_SPORTING_LIFE_API.md)** - Data source details ⭐ NEW

### Features
- **[Auto-Update Service](./docs/features/AUTO_UPDATE.md)** - Automatic data updates
- **[Live Prices](./docs/UI_LIVE_PRICES_GUIDE.md)** - Betfair integration guide

---

## ⚡ Quick Start

### Prerequisites
- Go 1.21+
- PostgreSQL 15+
- Betfair API credentials (optional, for live prices)

### 1. Clone & Setup
```bash
git clone <repository>
cd GiddyUp

# Copy environment template
cp settings.env.example settings.env
# Edit settings.env with your credentials
```

### 2. Database
```bash
cd postgres
psql -U postgres < init.sql
```

### 3. Build & Run
```bash
cd backend-api
go build -o bin/api cmd/api/main.go
source ../settings.env
./bin/api
```

### 4. Test
```bash
# Health check
curl http://localhost:8000/health

# Today's races
curl http://localhost:8000/api/v1/races/today
```

---

## 🏗️ Architecture

```
Sporting Life API → Auto-Update → PostgreSQL → REST API → Your UI
                      Service         │
Betfair API-NG   →    ↓              │
                   Live Prices  ←─────┘
```

- **Data Source**: Sporting Life API (racecards, runner details, odds)
- **Live Prices**: Betfair API-NG (WIN/PLACE markets)
- **Database**: PostgreSQL (normalized schema with indexes)
- **API**: Go Chi router (RESTful endpoints)
- **Caching**: Local JSON files for fast re-loads

See [docs/01_DEVELOPER_GUIDE.md](./docs/01_DEVELOPER_GUIDE.md) for details.

---

## 📊 Key Features

### 1. **Complete Runner Data**
```json
{
  "horse": "Hidalgo De L'isle",
  "jockey": "Charlie Maggs",
  "trainer": "D McCain Jnr",
  "owner": "Mr T G Leslie",
  "age": 8,
  "weight": 161,
  "form": "1234",
  "headgear": "b, t",
  "betfair_selection_id": 46013800,
  "best_odds": 5.5,
  "best_bookmaker": "Betfair Sportsbook"
}
```

### 2. **Betfair Selection ID Matching**
- No more error-prone name normalization!
- Direct database lookup by `betfair_selection_id`
- Perfect matching with Betfair Exchange markets

### 3. **Parallel Data Loading**
- Today and tomorrow fetched simultaneously
- ~50% faster startup
- Independent error handling per thread

### 4. **Smart Caching**
- First load: ~40 seconds
- Cached loads: <1 second
- Automatic cache invalidation for today

---

## 🔌 API Endpoints

### Races
```
GET /api/v1/races/today              # Today's races with live prices
GET /api/v1/races/tomorrow           # Tomorrow's racecards
GET /api/v1/races/{date}             # Specific date (YYYY-MM-DD)
GET /api/v1/races/{id}               # Single race details
```

### Profiles
```
GET /api/v1/horses/{id}/profile      # Horse form & statistics
GET /api/v1/jockeys/{id}/profile     # Jockey statistics
GET /api/v1/trainers/{id}/profile    # Trainer statistics
```

### Search
```
GET /api/v1/search/horses?q=...      # Search horses
GET /api/v1/search/jockeys?q=...     # Search jockeys
GET /api/v1/search/trainers?q=...    # Search trainers
```

See [docs/02_API_DOCUMENTATION.md](./docs/02_API_DOCUMENTATION.md) for full API reference.

---

## 📦 Project Structure

```
GiddyUp/
├── backend-api/              # Go API server
│   ├── cmd/                 # Executables
│   ├── internal/            # Core application
│   │   ├── handlers/       # HTTP handlers
│   │   ├── scraper/        # Sporting Life integration
│   │   ├── services/       # Auto-update service
│   │   ├── betfair/        # Betfair integration
│   │   └── repository/     # Database queries
│   └── logs/               # Server logs
├── postgres/                # Database setup
│   ├── init.sql            # Schema
│   └── migrations/         # Schema changes
├── data/                    # Cached data
│   ├── sportinglife/       # Race data cache
│   └── betfair_stitched/   # Betfair CSV data
├── docs/                    # Documentation
└── scripts/                 # Python utilities
```

---

## 🛠️ Development

### Build
```bash
cd backend-api
go build -o bin/api cmd/api/main.go
```

### Run Tests
```bash
go test ./...
```

### Backfill Data
```bash
./bin/backfill_dates --start-date 2024-01-01 --end-date 2024-12-31
```

### Check Logs
```bash
tail -f backend-api/logs/server.log
```

---

## 🔧 Configuration

All configuration in `settings.env`:

```bash
# Database
DB_HOST=localhost
DB_PORT=5432
DB_NAME=horse_db
DB_USER=postgres
DB_PASSWORD=password

# Betfair (optional)
BETFAIR_APP_KEY=your_app_key
BETFAIR_SESSION_TOKEN=your_session_token
ENABLE_LIVE_PRICES=true

# Server
PORT=8000
LOG_LEVEL=info
```

---

## 📈 Performance

### Startup Time
- First load (no cache): ~40-50s per date
- Cached load: <1s
- Parallel today+tomorrow: ~50s total

### API Response Times
- Race list: <50ms
- Single race: <30ms
- Horse profile: <100ms

### Database
- ~100K horses, 50K jockeys, 40K trainers
- ~500K races, 5M+ runners
- Optimized indexes for fast queries

---

## 🐛 Troubleshooting

### Port Already in Use
```bash
pkill -f "bin/api"
# or
lsof -ti :8000 | xargs kill -9
```

### No Race Data
- Check logs: `tail -f backend-api/logs/server.log`
- Verify Sporting Life API is accessible
- Ensure database is running

### Live Prices Not Working
- Check Betfair credentials in `settings.env`
- Verify `ENABLE_LIVE_PRICES=true`
- See [docs/UI_LIVE_PRICES_GUIDE.md](./docs/UI_LIVE_PRICES_GUIDE.md)

---

## 📝 Recent Updates

### October 16, 2025 ⭐
- **Sporting Life API V2** implemented
- Racing Post completely removed
- 2-endpoint merge strategy (race details + betting data)
- Betfair selection IDs captured for perfect matching
- Parallel today/tomorrow fetching
- Improved type handling and caching

See [docs/SPORTING_LIFE_COMPLETE.md](./docs/SPORTING_LIFE_COMPLETE.md) for details.

---

## 📄 License

Proprietary - All rights reserved

---

## 🤝 Contributing

This is a private project. For questions or issues, contact the repository owner.

---

## 🎯 Next Steps

1. **New Developer?** Start with [docs/00_START_HERE.md](./docs/00_START_HERE.md)
2. **Building UI?** Read [docs/UI_DEVELOPER_README.md](./docs/UI_DEVELOPER_README.md)
3. **API Integration?** See [docs/02_API_DOCUMENTATION.md](./docs/02_API_DOCUMENTATION.md)
4. **Database Work?** Check [docs/03_DATABASE_GUIDE.md](./docs/03_DATABASE_GUIDE.md)

---

**Built with ❤️ for horse racing enthusiasts**
