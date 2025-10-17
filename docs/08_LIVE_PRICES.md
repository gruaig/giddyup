# 08. Live Prices Integration

**Complete guide to Betfair live prices integration**

Last Updated: October 16, 2025

---

## 📋 Overview

GiddyUp integrates real-time Betfair prices for all runners, providing:

✅ **WIN Market Prices** - Best back/lay, traded volume, market depth  
✅ **PLACE Market Prices** - Place odds for each position  
✅ **Selection ID Matching** - Perfect Betfair market matching  
✅ **30-Second Updates** - Fresh prices every 30 seconds  
✅ **Fallback to BSP** - Historical prices from CSV files  

---

## 🔌 API Endpoints

### Get Live Prices for a Race

```http
GET /api/v1/races/{race_id}/prices/live
```

**Response:**
```json
{
  "race_id": 123456,
  "market_id": "1.234567890",
  "status": "OPEN",
  "total_matched": 123456.78,
  "last_updated": "2025-10-16T14:30:00Z",
  "runners": [
    {
      "selection_id": 12345678,
      "runner_name": "Silent Song",
      "status": "ACTIVE",
      "win_back_1": 3.50,
      "win_lay_1": 3.55,
      "win_traded_volume": 12345.67,
      "place_back_1": 1.45,
      "place_lay_1": 1.48,
      "last_price_traded": 3.50
    }
  ]
}
```

### Get All Live Races with Prices

```http
GET /api/v1/races/live
```

Returns all races with active Betfair markets.

---

## 🔧 Implementation Details

### 1. Selection ID Matching

**Perfect matching** via Betfair selection ID (no name normalization needed):

```sql
-- Stored in database
SELECT 
    runner_name,
    betfair_selection_id 
FROM racing.runners 
WHERE race_id = 123456;
```

**Frontend Usage:**
```typescript
// Match prices to runners
const runnerWithPrice = runner.betfair_selection_id 
    ? prices.find(p => p.selection_id === runner.betfair_selection_id)
    : null;
```

### 2. Price Update Flow

```
┌─────────────────┐
│  Betfair API-NG │ ← Fetch every 30s
└────────┬────────┘
         ↓
┌─────────────────┐
│  Live Prices    │
│  Service (Go)   │ ← Parse JSON, update cache
└────────┬────────┘
         ↓
┌─────────────────┐
│  In-Memory      │
│  Cache          │ ← Hot data, fast access
└────────┬────────┘
         ↓
┌─────────────────┐
│  API Endpoint   │ ← Serve to frontend
└─────────────────┘
```

### 3. Configuration

**Environment Variables** (in `settings.env`):

```bash
# Enable/disable live prices
ENABLE_LIVE_PRICES=true

# Betfair API credentials
BETFAIR_APP_KEY=your_app_key_here
BETFAIR_SESSION_TOKEN=your_session_token_here

# Update frequency (seconds)
LIVE_PRICE_UPDATE_INTERVAL=30
```

### 4. Fallback Strategy

If live prices unavailable:
1. Check BSP from historical CSV
2. Check PPWAP (Pre-Play Weighted Average Price)
3. Show "SP" (Starting Price placeholder)

**Database Fields:**
```sql
-- Historical prices from CSV
win_bsp     NUMERIC    -- Betfair Starting Price
win_ppwap   NUMERIC    -- Pre-Play WAP
place_bsp   NUMERIC    -- Place BSP
```

---

## 🎨 Frontend Integration

### React Example

```tsx
import { useEffect, useState } from 'react';

interface LivePrice {
  selection_id: number;
  win_back_1: number;
  win_lay_1: number;
  last_price_traded: number;
}

function RaceCard({ raceId }: { raceId: number }) {
  const [prices, setPrices] = useState<LivePrice[]>([]);

  useEffect(() => {
    const fetchPrices = async () => {
      const res = await fetch(`/api/v1/races/${raceId}/prices/live`);
      const data = await res.json();
      setPrices(data.runners);
    };

    // Initial fetch
    fetchPrices();

    // Poll every 30 seconds
    const interval = setInterval(fetchPrices, 30000);
    return () => clearInterval(interval);
  }, [raceId]);

  return (
    <div>
      {runners.map(runner => {
        const price = prices.find(p => 
          p.selection_id === runner.betfair_selection_id
        );
        
        return (
          <div key={runner.runner_id}>
            <h3>{runner.horse_name}</h3>
            {price ? (
              <div className="prices">
                <span className="back">{price.win_back_1.toFixed(2)}</span>
                <span className="lay">{price.win_lay_1.toFixed(2)}</span>
              </div>
            ) : (
              <span>SP</span>
            )}
          </div>
        );
      })}
    </div>
  );
}
```

### Price Display Component

```tsx
function PriceDisplay({ price }: { price: LivePrice }) {
  const isMoving = useIsPriceMoving(price.last_price_traded);
  
  return (
    <div className={`price ${isMoving ? 'flash' : ''}`}>
      <div className="back-price">
        <span className="odds">{price.win_back_1.toFixed(2)}</span>
        <span className="volume">£{formatVolume(price.win_traded_volume)}</span>
      </div>
      <div className="lay-price">
        <span className="odds">{price.win_lay_1.toFixed(2)}</span>
      </div>
    </div>
  );
}
```

---

## 📊 Data Structure

### Runner with Prices

```typescript
interface RunnerWithPrices {
  // Runner info
  runner_id: number;
  horse_name: string;
  jockey_name: string;
  trainer_name: string;
  
  // Betfair matching
  betfair_selection_id: number | null;
  
  // Live prices (if available)
  live_prices?: {
    win_back_1: number;
    win_lay_1: number;
    win_traded_volume: number;
    place_back_1: number;
    place_lay_1: number;
    last_price_traded: number;
  };
  
  // Fallback historical prices
  win_bsp: number | null;
  win_ppwap: number | null;
}
```

---

## 🐛 Troubleshooting

### Prices Not Updating

**Check 1: Betfair API Credentials**
```bash
# Verify credentials are set
echo $BETFAIR_APP_KEY
echo $BETFAIR_SESSION_TOKEN

# Test API connection
curl -H "X-Application: $BETFAIR_APP_KEY" \
     -H "X-Authentication: $BETFAIR_SESSION_TOKEN" \
     https://api.betfair.com/exchange/betting/json-rpc/v1 \
     -d '{"jsonrpc":"2.0","method":"SportsAPING/v1.0/listMarketCatalogue"}'
```

**Check 2: Service Running**
```bash
# Check logs
tail -f backend-api/logs/server.log | grep "live_price"

# Should see:
# "Starting live price updates (30s interval)"
# "Updated prices for race 123456: 12 runners"
```

**Check 3: Market Status**
```bash
# Verify market is OPEN
curl http://localhost:8000/api/v1/races/123456/prices/live | jq '.status'

# Should return: "OPEN" or "ACTIVE"
```

### Selection IDs Missing

**Problem:** `betfair_selection_id` is NULL for some runners

**Solution:** Re-scrape the race to fetch selection IDs:
```bash
# Via admin endpoint
curl -X POST http://localhost:8000/api/v1/admin/scrape/date \
  -H "Content-Type: application/json" \
  -d '{"date": "2025-10-16"}'
```

### High API Usage

**Problem:** Hitting Betfair API limits

**Solutions:**
1. Increase `LIVE_PRICE_UPDATE_INTERVAL` (e.g., 60 seconds)
2. Only fetch prices for "OPEN" markets (don't poll finished races)
3. Use market streaming API for high-volume clients

---

## 🚀 Performance Tips

### 1. Cache Aggressively

```typescript
// Cache prices for 30s
const PRICE_CACHE_TTL = 30000;

const cachedPrices = useMemo(() => {
  return prices; // React will memoize
}, [prices.map(p => p.last_price_traded).join(',')]);
```

### 2. WebSocket Alternative

For real-time updates, consider Betfair's Stream API:
```go
// backend-api/internal/betfair/stream.go
func StreamMarketPrices(marketIds []string) {
    // Subscribe to market changes
    // Push updates via WebSocket to frontend
}
```

### 3. Debounce UI Updates

```typescript
const debouncedPrices = useDebounce(prices, 500);
// Prevents UI flashing on rapid updates
```

---

## 📈 Analytics & Monitoring

### Track Price Movements

```sql
-- Log price changes for analysis
CREATE TABLE racing.price_movements (
    movement_id SERIAL PRIMARY KEY,
    runner_id BIGINT REFERENCES racing.runners(runner_id),
    timestamp TIMESTAMP DEFAULT NOW(),
    old_price NUMERIC(5,2),
    new_price NUMERIC(5,2),
    price_change NUMERIC(5,2),
    volume_change NUMERIC(10,2)
);
```

### Monitor API Health

```bash
# Check update success rate
grep "Updated prices" logs/server.log | wc -l

# Check errors
grep "ERROR.*live_price" logs/server.log
```

---

## 🔒 Security

### API Key Protection

```go
// Never expose Betfair credentials to frontend!
// All API calls go through backend proxy

func (h *LivePriceHandler) GetPrices(c *gin.Context) {
    // Backend makes Betfair API call with credentials
    prices := h.betfairClient.FetchPrices(raceID)
    
    // Return sanitized data to frontend
    c.JSON(http.StatusOK, prices)
}
```

### Rate Limiting

```go
// Prevent abuse
var limiter = rate.NewLimiter(10, 100) // 10 req/s, burst 100

func (h *Handler) RateLimited(c *gin.Context) {
    if !limiter.Allow() {
        c.JSON(429, gin.H{"error": "rate limit exceeded"})
        return
    }
    // ... handle request
}
```

---

## ✅ Testing

### Manual Test

```bash
# Get today's races
RACE_ID=$(curl -s http://localhost:8000/api/v1/races/today | \
  jq -r '.[0].races[0].race_id')

# Get live prices
curl "http://localhost:8000/api/v1/races/${RACE_ID}/prices/live" | jq
```

### Expected Output

```json
{
  "market_id": "1.234567890",
  "status": "OPEN",
  "runners": [
    {
      "selection_id": 12345,
      "win_back_1": 3.50,
      "win_lay_1": 3.55
    }
  ]
}
```

---

## 📚 Related Documentation

- [02_API_DOCUMENTATION.md](02_API_DOCUMENTATION.md) - Full API reference
- [06_SPORTING_LIFE_API.md](06_SPORTING_LIFE_API.md) - How selection IDs are fetched
- [09_AUTO_UPDATE.md](09_AUTO_UPDATE.md) - Background update service

---

**Last Updated:** October 16, 2025  
**Status:** ✅ Production Ready  
**Live Prices:** ✅ Fully Functional


