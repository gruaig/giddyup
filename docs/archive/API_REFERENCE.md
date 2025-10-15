## GiddyUp Racing Analytics API - Reference Guide

**Base URL:** `http://localhost:8000/api/v1`  
**Version:** 1.0  
**Date:** 2025-10-13

---

## 📋 Table of Contents

1. [Standard Response Format](#standard-response-format)
2. [Pagination & Sorting](#pagination--sorting)
3. [Error Handling](#error-handling)
4. [Endpoints](#endpoints)
   - [Search](#search)
   - [Profiles](#profiles)
   - [Races](#races)
   - [Market Analytics](#market-analytics)
   - [Betting Angles](#betting-angles)
5. [Performance Expectations](#performance-expectations)

---

## Standard Response Format

All endpoints return a consistent envelope:

```json
{
  "data": [...],
  "summary": { ... },
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 50,
    "total": 150,
    "request_id": "abc123-xyz789",
    "generated_at": "2025-10-13T18:45:00Z",
    "latency_ms": 125
  },
  "error": null
}
```

### Fields:

| Field | Type | Description |
|-------|------|-------------|
| `data` | array/object/null | Main response data |
| `summary` | object/null | Aggregate statistics (optional) |
| `meta` | object | Metadata about the response |
| `meta.limit` | int | Requested limit |
| `meta.offset` | int | Requested offset |
| `meta.returned` | int | Actual count returned |
| `meta.total` | int | Total available (if known) |
| `meta.request_id` | string | For debugging/tracing |
| `meta.generated_at` | string | ISO 8601 timestamp |
| `meta.latency_ms` | int | Server processing time |
| `error` | object/null | Error details (null on success) |

---

## Pagination & Sorting

### Common Parameters

| Parameter | Type | Default | Max | Description |
|-----------|------|---------|-----|-------------|
| `limit` | int | 50 | 1000 | Results per page |
| `offset` | int | 0 | - | Skip N results |
| `sort` | string | - | - | Comma-separated fields |

### Sorting

Use comma-separated field names. Prefix with `-` for descending order.

**Examples:**
- `sort=race_date` - Ascending by date
- `sort=-race_date` - Descending by date  
- `sort=course_id,race_date` - Multi-field sort

### Pagination Example

```bash
# Page 1
GET /api/v1/races?date=2024-01-01&limit=20&offset=0

# Page 2
GET /api/v1/races?date=2024-01-01&limit=20&offset=20

# Check meta.returned to know when to stop paginating
```

---

## Error Handling

### Error Response Format

```json
{
  "data": null,
  "meta": {
    "request_id": "abc123",
    "generated_at": "2025-10-13T18:45:00Z"
  },
  "error": {
    "code": "BAD_REQUEST",
    "message": "Invalid date format",
    "field": "date_from"
  }
}
```

### Error Codes

| HTTP | Code | Description |
|------|------|-------------|
| 400 | `BAD_REQUEST` | Invalid parameters |
| 400 | `VALIDATION_ERROR` | Field validation failed |
| 404 | `NOT_FOUND` | Resource not found |
| 500 | `INTERNAL_SERVER_ERROR` | Server error |

### Validation Error Example

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "limit must be between 1 and 1000",
    "field": "limit"
  }
}
```

---

## Endpoints

### Search

#### Global Search

**GET** `/api/v1/search`

Search across horses, trainers, jockeys, owners, and courses.

**Parameters:**

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `q` | string | ✅ | - | Search query (min 2 chars) |
| `limit` | int | ❌ | 10 | Results per category |

**Example:**
```bash
GET /api/v1/search?q=Frankel&limit=5
```

**Response:**
```json
{
  "data": {
    "horses": [
      {
        "id": 134020,
        "name": "Frankel (GB)",
        "score": 0.83,
        "type": "horse"
      }
    ],
    "trainers": [...],
    "jockeys": [...],
    "owners": [...],
    "courses": [...]
  },
  "meta": {
    "returned": 5,
    "generated_at": "2025-10-13T18:45:00Z",
    "latency_ms": 18
  }
}
```

---

#### Comment Search (FTS)

**GET** `/api/v1/search/comments`

Full-text search in race comments.

**Parameters:**

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `q` | string | ✅ | - | Search terms |
| `limit` | int | ❌ | 200 | Max results |
| `offset` | int | ❌ | 0 | Pagination offset |

**Example:**
```bash
GET /api/v1/search/comments?q=bumped%20at%20start&limit=10
```

**Response:**
```json
{
  "data": [
    {
      "runner_id": 12345,
      "race_id": 5678,
      "race_date": "2024-01-13",
      "comment": "Bumped at start, ran on well"
    }
  ],
  "meta": {
    "limit": 10,
    "offset": 0,
    "returned": 10,
    "latency_ms": 85
  }
}
```

**Performance:** < 300ms (with FTS index)

---

### Profiles

#### Horse Profile

**GET** `/api/v1/horses/:id/profile`

Complete horse profile with form, odds, and statistics.

**Parameters:**

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `months` | int | ❌ | 24 | Lookback period (months) |

**Example:**
```bash
GET /api/v1/horses/520803/profile?months=24
```

**Response:**
```json
{
  "data": {
    "horse": {
      "id": 520803,
      "name": "Enable (GB)",
      "country": "GB"
    },
    "career": {
      "runs": 14,
      "wins": 12,
      "places": 14,
      "strike_rate": 0.857,
      "peak_rpr": 128,
      "total_prize_money": 3127890.35
    },
    "recent_runs": [
      {
        "race_date": "2020-09-05",
        "course": "Kempton",
        "pos": 1,
        "bsp": 1.09,
        "sp": 1.07,
        "rpr": 121,
        "or": 128,
        "trainer": "John Gosden",
        "jockey": "Frankie Dettori",
        "dsr": 42
      }
    ],
    "going_splits": [...],
    "distance_splits": [...],
    "course_splits": [...]
  },
  "meta": {
    "returned": 1,
    "generated_at": "2025-10-13T18:45:00Z",
    "latency_ms": 450
  }
}
```

**Performance:** < 500ms (with mv_runner_base)

---

#### Trainer Profile

**GET** `/api/v1/trainers/:id/profile`

Trainer statistics and form analysis.

**Example:**
```bash
GET /api/v1/trainers/123/profile
```

**Performance:** < 500ms

---

#### Jockey Profile

**GET** `/api/v1/jockeys/:id/profile`

Jockey statistics and form analysis.

**Example:**
```bash
GET /api/v1/jockeys/456/profile
```

**Performance:** < 500ms

---

### Races

#### Races by Date

**GET** `/api/v1/races`

List all races on a specific date.

**Parameters:**

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `date` | string | ✅ | - | Date (YYYY-MM-DD) |
| `race_type` | string | ❌ | - | Flat, Hurdle, Chase, NH Flat |
| `limit` | int | ❌ | 100 | Max results |

**Example:**
```bash
GET /api/v1/races?date=2024-07-27&race_type=Flat
```

**Response:**
```json
{
  "data": [
    {
      "race_id": 991122,
      "race_date": "2024-07-27",
      "off_time": "14:10:00",
      "course": {
        "id": 2,
        "name": "Ascot"
      },
      "race_type": "Flat",
      "class": "Class 1",
      "dist_f": 10.0,
      "surface": "Turf",
      "going": "Good",
      "ran": 14
    }
  ],
  "meta": {
    "limit": 100,
    "returned": 7,
    "latency_ms": 12
  }
}
```

**Performance:** < 50ms

---

#### Advanced Race Search

**GET** `/api/v1/races/search`

Search races with multiple filters.

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `date_from` | string | Start date (YYYY-MM-DD) |
| `date_to` | string | End date (YYYY-MM-DD) |
| `course_id` | int | Filter by course |
| `race_type` | string | Flat, Hurdle, Chase, NH Flat |
| `class` | string | Class 1-7 |
| `surface` | string | Turf, AW |
| `going` | string | Heavy, Soft, Good, etc. |
| `dist_f_min` | float | Min distance (furlongs) |
| `dist_f_max` | float | Max distance (furlongs) |
| `limit` | int | Max results (default 100) |
| `offset` | int | Pagination offset |

**Example:**
```bash
GET /api/v1/races/search?date_from=2024-01-01&date_to=2024-01-31&class=Class%201&surface=Turf
```

**Performance:** < 100ms

---

#### Race Details with Runners

**GET** `/api/v1/races/:id`

Complete race card with all runners and results.

**Example:**
```bash
GET /api/v1/races/991122
```

**Response:**
```json
{
  "data": {
    "race": {
      "race_id": 991122,
      "race_date": "2024-07-27",
      "course": "Ascot",
      "race_type": "Flat",
      "class": "Class 1",
      "dist_f": 10.0
    },
    "runners": [
      {
        "runner_id": 88776,
        "horse_name": "Enable (GB)",
        "pos": 1,
        "draw": 5,
        "bsp": 1.42,
        "sp": 1.44,
        "trainer": "John Gosden",
        "jockey": "Frankie Dettori",
        "or": 128,
        "rpr": 124
      }
    ]
  },
  "meta": {
    "returned": 14,
    "latency_ms": 172
  }
}
```

**Performance:** < 300ms

---

### Market Analytics

#### Market Movers

**GET** `/api/v1/market/movers`

Horses with significant pre-race price movements.

**Parameters:**

| Name | Type | Default | Description |
|------|------|---------|-------------|
| `date` | string | - | Race date (YYYY-MM-DD) |
| `min_move` | float | 15.0 | Min price move (%) |
| `min_vol` | float | 0 | Min matched volume |
| `limit` | int | 200 | Max results |

**Example:**
```bash
GET /api/v1/market/movers?date=2024-07-27&min_move=20&min_vol=1000
```

**Response:**
```json
{
  "data": [
    {
      "runner_id": 12345,
      "horse_name": "Movin' On Up",
      "race_id": 5678,
      "course": "Ascot",
      "pre_min": 5.0,
      "pre_max": 8.5,
      "move_pct": 70.0,
      "bsp": 7.2,
      "drift_to_bsp_pct": 44.0
    }
  ],
  "meta": {
    "returned": 15,
    "latency_ms": 45
  }
}
```

**Performance:** < 100ms

---

#### Win Calibration

**GET** `/api/v1/market/calibration/win`

Checks if win probabilities match actual win rates.

**Parameters:**

| Name | Type | Default | Description |
|------|------|---------|-------------|
| `date_from` | string | - | Start date |
| `date_to` | string | - | End date |
| `bins` | int | 10 | Number of probability bins |
| `price` | string | bsp | Price source (bsp/dec/ppwap) |

**Example:**
```bash
GET /api/v1/market/calibration/win?date_from=2024-01-01&date_to=2024-03-31&bins=10&price=bsp
```

**Response:**
```json
{
  "data": [
    {
      "bin": 1,
      "n": 152,
      "bin_min": 0.0,
      "bin_max": 0.1,
      "mean_implied": 0.055,
      "actual_win_rate": 0.052,
      "calibration_error": -0.003
    }
  ],
  "summary": {
    "total_races": 1520,
    "rmse": 0.012,
    "mean_abs_error": 0.008
  },
  "meta": {
    "returned": 10,
    "latency_ms": 145
  }
}
```

**Performance:** < 200ms

---

### Betting Angles

#### Near-Miss-No-Hike: Today's Qualifiers

**GET** `/api/v1/angles/near-miss-no-hike/today`

Find horses declared to run today that match the angle.

**Parameters:**

| Name | Type | Default | Description |
|------|------|---------|-------------|
| `on` | string | today | Date to check (YYYY-MM-DD) |
| `race_type` | string | - | Filter by race type |
| `last_pos` | int | 2 | Last position required |
| `btn_max` | float | 3.0 | Max beaten distance (lengths) |
| `dsr_max` | int | 14 | Max days since last run |
| `or_delta_max` | int | 0 | Max OR increase allowed |
| `dist_f_tolerance` | float | 1.0 | Distance variance (furlongs) |
| `same_surface` | bool | true | Require same surface |
| `include_null_or` | bool | false | Include missing ORs |
| `limit` | int | 200 | Max results |
| `offset` | int | 0 | Pagination offset |

**Example:**
```bash
GET /api/v1/angles/near-miss-no-hike/today?race_type=Flat&dsr_max=7&btn_max=2.0
```

**Response:**
```json
{
  "data": [
    {
      "horse_id": 9643,
      "horse_name": "Captain Scooby (GB)",
      "entry": {
        "race_id": 1234,
        "date": "2025-10-13",
        "race_type": "Flat",
        "course_id": 52,
        "dist_f": 6.0,
        "surface": "AW",
        "or": 54
      },
      "last": {
        "race_id": 1200,
        "date": "2025-10-09",
        "pos": 2,
        "btn": 1.5,
        "or": 54,
        "dist_f": 6.0,
        "surface": "AW"
      },
      "dsr": 4,
      "rating_change": 0,
      "dist_f_diff": 0,
      "same_surface": true
    }
  ],
  "meta": {
    "returned": 1,
    "latency_ms": 95
  }
}
```

**Performance:** < 200ms

---

#### Near-Miss-No-Hike: Historical Backtest

**GET** `/api/v1/angles/near-miss-no-hike/past`

Backtest the angle on historical data with ROI calculation.

**Parameters:**

Same as `/today` plus:

| Name | Type | Default | Description |
|------|------|---------|-------------|
| `date_from` | string | - | Start date for last run |
| `date_to` | string | - | End date for last run |
| `require_next_win` | bool | false | Only show winners |
| `price_source` | string | bsp | bsp/dec/ppwap |
| `summary` | bool | true | Include aggregate stats |

**Example:**
```bash
GET /api/v1/angles/near-miss-no-hike/past?date_from=2024-01-01&date_to=2024-12-31&price_source=bsp
```

**Response:**
```json
{
  "data": [
    {
      "horse_id": 5450,
      "horse_name": "Tathmeen (IRE)",
      "last_race_id": 168,
      "last_date": "2024-01-15",
      "last_pos": 2,
      "last_btn": 0.3,
      "last_or": 49,
      "next_race_id": 335,
      "next_date": "2024-01-17",
      "next_pos": 4,
      "next_win": false,
      "dsr": 2,
      "rating_change": 0,
      "price": 3.58
    }
  ],
  "summary": {
    "n": 10,
    "wins": 4,
    "win_rate": 0.40,
    "roi": 0.069
  },
  "meta": {
    "returned": 10,
    "latency_ms": 83
  }
}
```

**Performance:** < 100ms (1 year), < 300ms (5 years)

---

## Performance Expectations

| Endpoint | Expected Latency | Notes |
|----------|------------------|-------|
| Search (global) | < 50ms | With trigram indexes |
| Search (comments) | < 300ms | With FTS index |
| Horse profile | < 500ms | With mv_runner_base |
| Trainer profile | < 500ms | With mv_runner_base |
| Jockey profile | < 500ms | With mv_runner_base |
| Races by date | < 50ms | Simple filter |
| Race details | < 300ms | Includes all runners |
| Market movers | < 100ms | Pre-aggregated |
| Calibration | < 200ms | 3-month window |
| Draw bias | < 400ms | With mv_draw_bias_flat |
| Angle (today) | < 200ms | Light query |
| Angle (backtest) | < 100ms (1yr) | Uses mv_last_next |

### Cold vs Warm

- **Cold:** First request after server start
- **Warm:** Subsequent requests (DB cache populated)

Performance targets are for **warm** queries. Cold queries may be 2-3x slower.

---

## Best Practices

### Date Formats

Always use `YYYY-MM-DD` format:
```bash
✅ date=2024-07-27
❌ date=07/27/2024
```

### Pagination

Check `meta.returned` to know when you've reached the end:
```javascript
if (response.meta.returned < response.meta.limit) {
  // No more results
}
```

### Error Handling

Always check for `error` field:
```javascript
if (response.error) {
  console.error(response.error.message);
  // Handle error based on response.error.code
}
```

### Rate Limiting

Current limit: **100 requests/minute** per IP

Exceeded limit returns:
```json
{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Too many requests, try again in 60 seconds"
  }
}
```

---

## Support

For issues or questions:
- Email: support@giddyup.racing
- Docs: `/backend-api/documentation/`
- API Status: `GET /health`

**Last Updated:** 2025-10-13

