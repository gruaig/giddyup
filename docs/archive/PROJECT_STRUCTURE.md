# GiddyUp Backend API - Project Structure

```
backend-api/
│
├── cmd/
│   └── api/
│       └── main.go                      # Application entry point
│
├── internal/                            # Internal packages (not importable)
│   ├── config/
│   │   └── config.go                    # Environment configuration
│   ├── database/
│   │   └── postgres.go                  # DB connection with search_path
│   ├── logger/
│   │   └── logger.go                    # Comprehensive logging system
│   ├── models/                          # Data models
│   │   ├── common.go                    # Shared types (StatsSplit, TrendPoint)
│   │   ├── course.go                    # Course, Meeting models
│   │   ├── horse.go                     # Horse, HorseProfile models
│   │   ├── trainer.go                   # Trainer, TrainerProfile models
│   │   ├── jockey.go                    # Jockey, JockeyProfile models
│   │   ├── race.go                      # Race, RaceFilters models
│   │   ├── runner.go                    # Runner model (all Betfair fields)
│   │   ├── search.go                    # SearchResults models
│   │   ├── market.go                    # Market analytics models
│   │   └── bias.go                      # Bias analysis models
│   ├── repository/                      # Database query layer
│   │   ├── search.go                    # Global search, comment FTS
│   │   ├── profile.go                   # Horse/Trainer/Jockey profiles
│   │   ├── race.go                      # Race search, details, runners
│   │   ├── market.go                    # Market movers, calibration
│   │   └── bias.go                      # Draw bias, recency, trainer change
│   ├── handlers/                        # HTTP request handlers
│   │   ├── search.go                    # Search endpoints
│   │   ├── profile.go                   # Profile endpoints
│   │   ├── race.go                      # Race endpoints
│   │   ├── market.go                    # Market endpoints
│   │   └── bias.go                      # Bias endpoints
│   ├── middleware/                      # HTTP middleware
│   │   ├── cors.go                      # CORS handling
│   │   └── error.go                     # Error recovery & logging
│   └── router/
│       └── router.go                    # Route configuration
│
├── tests/                               # Test suites
│   ├── comprehensive_test.go            # 24 comprehensive tests
│   └── e2e_test.go                      # End-to-end journey tests
│
├── documentation/                       # 📚 All documentation
│   ├── README.md                        # Documentation index
│   ├── QUICKSTART.md                    # Quick start guide
│   ├── ANSWER_TO_YOUR_QUESTION.md       # Main user question answered
│   ├── DEMO_SEARCH_HORSE_ODDS.md        # Detailed demo
│   ├── IMPLEMENTATION_SUMMARY.md        # Technical architecture
│   ├── STATUS.md                        # Current status
│   ├── TEST_RESULTS.md                  # Test outcomes
│   └── FINAL_SUMMARY.md                 # Executive summary
│
├── bin/                                 # Compiled binaries (gitignored)
│   └── api                              # Main API binary (29MB)
│
├── scripts/                             # Helper scripts
│   ├── start_server.sh                  # Start API server
│   ├── verify_api.sh                    # Quick verification
│   ├── demo_horse_journey.sh            # Interactive demo
│   ├── test_working_endpoints.sh        # Test working endpoints
│   ├── run_tests.sh                     # Run basic tests
│   └── run_comprehensive_tests.sh       # Run full test suite
│
├── sql/                                 # SQL utilities
│   ├── optimize_db.sql                  # Performance optimization
│   └── get_test_fixtures.sql            # Get test data IDs
│
├── test_db.go                           # Database connection test
├── go.mod                               # Go module definition
├── go.sum                               # Dependency checksums
├── README.md                            # Main README
└── PROJECT_STRUCTURE.md                 # This file

```

---

## 📦 Key Components

### Core Application (`cmd/api/`)
- **main.go**: Application entry point, server initialization, graceful shutdown

### Business Logic (`internal/`)
- **models/**: 10 files defining all data structures
- **repository/**: 5 files with all database queries
- **handlers/**: 5 files handling HTTP requests
- **middleware/**: CORS, error recovery, request logging
- **router/**: Route registration and grouping

### Infrastructure
- **config/**: Environment-based configuration
- **database/**: PostgreSQL connection with search_path
- **logger/**: Color-coded logging with DEBUG/INFO/WARN/ERROR levels

---

## 📊 Statistics

| Category | Count |
|----------|-------|
| Go Files | 20 |
| Lines of Code | ~2,500 |
| API Endpoints | 19 |
| Test Files | 2 |
| Test Cases | 34 |
| Documentation Files | 8 |
| Helper Scripts | 6 |
| Dependencies | 3 core |

---

## 🎯 Main Entry Points

### For Users
1. Start here: `documentation/README.md`
2. Quick start: `documentation/QUICKSTART.md`
3. See demo: `./demo_horse_journey.sh`

### For Developers
1. Main README: `README.md`
2. Code entry point: `cmd/api/main.go`
3. Add endpoints: `internal/handlers/` + `internal/router/router.go`
4. Add queries: `internal/repository/`

### For Testers
1. Run tests: `./run_comprehensive_tests.sh`
2. Test results: `documentation/TEST_RESULTS.md`
3. Verify API: `./verify_api.sh`

---

## 🚀 Quick Commands

```bash
# Development
go run cmd/api/main.go                    # Run in dev mode
go build -o bin/api cmd/api/main.go       # Build binary
go test ./...                             # Run all tests

# Production
./start_server.sh                         # Start with management
./verify_api.sh                           # Verify endpoints
tail -f /tmp/giddyup-api.log              # Monitor logs

# Testing
./run_comprehensive_tests.sh              # Full test suite
./demo_horse_journey.sh                   # Interactive demo
go test -v ./tests/comprehensive_test.go  # Detailed test output
```

---

## 🗂️ File Organization Principles

### `/cmd`
- Application entry points
- Main packages
- Thin layer (just initialization)

### `/internal`
- All application code
- Cannot be imported by other projects
- Organized by layer (models, repos, handlers)

### `/tests`
- Integration and E2E tests
- Separate from unit tests
- Use real database

### `/documentation`
- All markdown documentation
- User guides and technical docs
- Indexed by README

### Root scripts (`.sh`)
- Development helpers
- Testing utilities
- Demo scripts

---

**This structure follows Go best practices and provides clear separation of concerns.**

