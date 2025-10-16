# GiddyUp Backend API

Go/Gin REST API for racing analytics data.

## 📚 Documentation

**All documentation is in the [`documentation/`](documentation/) folder.**

Quick links:
- **[Quick Start Guide](documentation/QUICKSTART.md)** - Get started quickly
- **[Answer to Your Question](documentation/ANSWER_TO_YOUR_QUESTION.md)** - Search horse → see runs with odds
- **[Test Results](documentation/TEST_RESULTS.md)** - 87.5% tests passing
- **[Full Documentation Index](documentation/README.md)** - All docs

## Quick Start

### 1. Set Environment Variables

```bash
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=horse_db
export DB_USER=postgres
export DB_PASSWORD=password
export SERVER_PORT=8000
export ENV=development
export CORS_ORIGINS=http://localhost:3000
```

### 2. Install Dependencies

```bash
cd backend-api
go mod tidy
```

### 3. Run the Server

```bash
go run cmd/api/main.go
```

The API will be available at `http://localhost:8000`

## API Endpoints

### Health Check
- `GET /health` - Server health status

### Search
- `GET /api/v1/search?q=<query>&limit=10` - Global search
- `GET /api/v1/search/comments?q=<query>` - Search runner comments

### Profiles
- `GET /api/v1/horses/:id/profile` - Horse profile with splits
- `GET /api/v1/trainers/:id/profile` - Trainer profile
- `GET /api/v1/jockeys/:id/profile` - Jockey profile

### Races
- `GET /api/v1/races?date=2024-01-13` - Races for a date
- `GET /api/v1/races/search` - Advanced race search with filters
- `GET /api/v1/races/:id` - Single race with runners
- `GET /api/v1/races/:id/runners` - Race runners
- `GET /api/v1/courses` - List all courses
- `GET /api/v1/courses/:id/meetings` - Course meetings

### Market Analytics
- `GET /api/v1/market/movers?date=2024-01-13&min_move=20` - Steamers/drifters
- `GET /api/v1/market/calibration/win` - Win market calibration
- `GET /api/v1/market/calibration/place` - Place market calibration
- `GET /api/v1/market/inplay-moves` - In-play price movements
- `GET /api/v1/market/book-vs-exchange` - SP vs BSP comparison

### Bias Analysis
- `GET /api/v1/bias/draw?course_id=1` - Draw bias analysis
- `GET /api/v1/analysis/recency` - Days-since-run effects
- `GET /api/v1/analysis/trainer-change` - Trainer change impact

## Project Structure

```
backend-api/
├── cmd/
│   └── api/
│       └── main.go              # Application entry point
├── internal/
│   ├── config/                  # Configuration management
│   ├── database/                # Database connection
│   ├── models/                  # Data models
│   ├── repository/              # Database queries
│   ├── handlers/                # HTTP handlers
│   ├── middleware/              # Middleware
│   └── router/                  # Route definitions
├── go.mod
└── README.md
```

## Database Connection

The API automatically sets `search_path` to `racing, public` on connection.

**Connection String:**
```
host=localhost port=5432 user=postgres password=password dbname=horse_db sslmode=disable
```

## Development

### Run Tests
```bash
go test ./...
```

### Build Binary
```bash
go build -o bin/api cmd/api/main.go
./bin/api
```

### Build for Production
```bash
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/api cmd/api/main.go
```

## Dependencies

- `github.com/gin-gonic/gin` - Web framework
- `github.com/lib/pq` - PostgreSQL driver
- `github.com/jmoiron/sqlx` - SQL extensions
- `github.com/joho/godotenv` - Environment variables (optional)

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | localhost | Database host |
| `DB_PORT` | 5432 | Database port |
| `DB_NAME` | horse_db | Database name |
| `DB_USER` | postgres | Database user |
| `DB_PASSWORD` | password | Database password |
| `SERVER_PORT` | 8000 | API server port |
| `ENV` | development | Environment (development/production) |
| `CORS_ORIGINS` | http://localhost:3000 | Allowed CORS origins |

## Example Usage

### Interactive Demos (Recommended!)

```bash
# Search any horse and see runs with odds
./demo_horse_journey.sh "Enable"
./demo_horse_journey.sh "Frankel"

# Test betting angle with any date
./demo_angle.sh 2024-01-15
./demo_angle.sh $(date +%Y-%m-%d)

# Quick endpoint verification
./verify_api.sh
```

### API Calls (Direct)

```bash
# Search for a Horse
curl "http://localhost:8000/api/v1/search?q=Frankel&limit=5"

# Get Horse Profile
curl http://localhost:8000/api/v1/horses/520803/profile

# Get Recent Races
curl http://localhost:8000/api/v1/races?date=2024-01-13

# Betting Angle Backtest
curl "http://localhost:8000/api/v1/angles/near-miss-no-hike/past?date_from=2024-01-01&date_to=2024-01-31"
```

## Production Deployment

### Using Docker
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o api cmd/api/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/api .
EXPOSE 8000
CMD ["./api"]
```

### Using systemd
```ini
[Unit]
Description=GiddyUp API
After=network.target

[Service]
Type=simple
User=racing
WorkingDirectory=/opt/giddyup-api
ExecStart=/opt/giddyup-api/bin/api
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

## License

Internal project - All rights reserved.

