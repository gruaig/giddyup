# Quick Start Guide

## Start the API Server

```bash
cd /home/smonaghan/GiddyUp/backend-api

# Set required environment variables
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=horse_db
export DB_USER=postgres
export DB_PASSWORD=password
export SERVER_PORT=8000
export CORS_ORIGINS="http://localhost:3000"

# Run the server
go run cmd/api/main.go
```

Or use the compiled binary:

```bash
./bin/api
```

## Test the API

```bash
# Health check
curl http://localhost:8000/health

# Get all courses
curl http://localhost:8000/api/v1/courses | python3 -m json.tool

# Search for "Frankel"
curl "http://localhost:8000/api/v1/search?q=Frankel&limit=5" | python3 -m json.tool

# Search races
curl "http://localhost:8000/api/v1/races/search?date_from=2024-01-01&date_to=2024-01-02&limit=5" | python3 -m json.tool

# Get horse profile (horse_id: 134020 is Frankel)
curl http://localhost:8000/api/v1/horses/134020/profile | python3 -m json.tool

# Get market movers
curl "http://localhost:8000/api/v1/market/movers?date=2024-01-13&min_move=20" | python3 -m json.tool

# Draw bias analysis (course_id: 73 is Ascot)
curl "http://localhost:8000/api/v1/bias/draw?course_id=73&dist_min=5&dist_max=7" | python3 -m json.tool
```

## API Endpoints

### Base URL
`http://localhost:8000/api/v1`

### Available Endpoints

**Search**
- `GET /search?q=<query>&limit=10`
- `GET /search/comments?q=<query>`

**Profiles**
- `GET /horses/:id/profile`
- `GET /trainers/:id/profile`
- `GET /jockeys/:id/profile`

**Races**
- `GET /races?date=2024-01-13`
- `GET /races/search?date_from=...&date_to=...`
- `GET /races/:id`
- `GET /races/:id/runners`
- `GET /courses`
- `GET /courses/:id/meetings`

**Market Analytics**
- `GET /market/movers?date=...&min_move=20`
- `GET /market/calibration/win?date_from=...&date_to=...`
- `GET /market/calibration/place?date_from=...&date_to=...`
- `GET /market/inplay-moves?date_from=...&date_to=...`
- `GET /market/book-vs-exchange?date_from=...&date_to=...`

**Bias Analysis**
- `GET /bias/draw?course_id=...&dist_min=...&dist_max=...`
- `GET /analysis/recency?date_from=...&date_to=...`
- `GET /analysis/trainer-change?min_runs=5`

## Rebuild After Changes

```bash
cd /home/smonaghan/GiddyUp/backend-api

# Install/update dependencies
go mod tidy

# Build
go build -o bin/api cmd/api/main.go

# Run
./bin/api
```

## Production Build

```bash
# Build for Linux production
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/api-prod cmd/api/main.go
```

## Troubleshooting

### Port already in use
```bash
# Kill process on port 8000
lsof -ti:8000 | xargs kill
```

### Database connection issues
```bash
# Test database connection
docker exec horse_racing psql -U postgres -d horse_db -c "SET search_path TO racing; SELECT COUNT(*) FROM races;"
```

### Test database queries directly
```bash
go run test_db.go
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| DB_HOST | PostgreSQL host | localhost |
| DB_PORT | PostgreSQL port | 5432 |
| DB_NAME | Database name | horse_db |
| DB_USER | Database user | postgres |
| DB_PASSWORD | Database password | password |
| SERVER_PORT | API server port | 8000 |
| CORS_ORIGINS | Allowed CORS origins | http://localhost:3000 |
| ENV | Environment (development/production) | development |

## Next Steps

1. Test all endpoints with your frontend
2. Add authentication if needed
3. Set up proper environment variable management
4. Configure production deployment
5. Add monitoring and logging

## Support

See `README.md` for full documentation
See `IMPLEMENTATION_SUMMARY.md` for technical details

