# API Examples - Live Racing Data

## Quick Start for UI Developers

### 1. Get Today's Races
```bash
curl http://localhost:8000/api/v1/races?date=2025-10-15 | jq
```

**Response**:
```json
{
  "races": [
    {
      "race_id": 123456,
      "date": "2025-10-15",
      "course": "Nottingham",
      "race_name": "British Stallion Studs EBF Maiden Stakes",
      "off_time": "14:25",
      "distance": "1m",
      "going": "Good To Soft",
      "class": 4,
      "prelim": true,
      "ran": 0,
      "runners": [ /* ... */ ]
    }
  ]
}
```

### 2. Get Single Race with Live Prices
```bash
curl http://localhost:8000/api/v1/races/123456 | jq
```

**Response includes live prices** (updated every 60s):
```json
{
  "race_id": 123456,
  "runners": [
    {
      "horse": "Example Horse",
      "form": "1234",
      "headgear": "b",
      "comment": "Improved last time",
      "win_ppwap": 4.50,
      "win_ppmax": 5.00,
      "win_ppmin": 4.00,
      "place_ppwap": 2.10
    }
  ]
}
```

### 3. Poll for Price Updates (React Example)
```javascript
import { useEffect, useState } from 'react';

function TodaysRaces() {
  const [races, setRaces] = useState([]);
  const [lastUpdate, setLastUpdate] = useState(null);

  useEffect(() => {
    const fetchRaces = async () => {
      const res = await fetch('/api/v1/races?date=2025-10-15');
      const data = await res.json();
      setRaces(data.races);
      setLastUpdate(new Date());
    };

    // Initial fetch
    fetchRaces();

    // Poll every 60 seconds for live prices
    const interval = setInterval(fetchRaces, 60000);

    return () => clearInterval(interval);
  }, []);

  return (
    <div>
      <h1>Today's Races</h1>
      <p>Last updated: {lastUpdate?.toLocaleTimeString()}</p>
      {races.map(race => (
        <RaceCard key={race.race_id} race={race} />
      ))}
    </div>
  );
}
```

### 4. Display Live Prices with Change Indicator
```javascript
function PriceCell({ runner, prevPrice }) {
  const currentPrice = runner.win_ppwap;
  const change = prevPrice ? currentPrice - prevPrice : 0;

  return (
    <div className="price-cell">
      <span className={change > 0 ? 'price-worse' : change < 0 ? 'price-better' : ''}>
        {currentPrice ? currentPrice.toFixed(2) : 'N/A'}
      </span>
      {change !== 0 && (
        <span className="change-arrow">
          {change > 0 ? '↑' : '↓'}
        </span>
      )}
    </div>
  );
}
```

### 5. Filter by Course
```bash
curl http://localhost:8000/api/v1/races?course=Nottingham | jq
```

### 6. Get Tomorrow's Races
```bash
curl http://localhost:8000/api/v1/races?date=2025-10-16 | jq
```

### 7. Get Historical Results
```bash
curl http://localhost:8000/api/v1/races?date=2025-10-11 | jq
```

---

## Field Definitions

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `prelim` | boolean | True if pre-race data (no results yet) | `true` |
| `ran` | integer | Number of runners that finished | `0` (prelim), `12` (results) |
| `form` | string | Recent form (1=1st, 2=2nd, etc.) | `"1234"` |
| `headgear` | string | Headgear symbols (b/v/t/h/p) | `"b"` (blinkers) |
| `comment` | string | Expert commentary | `"Improved last time"` |
| `win_ppwap` | float | Live VWAP price (WIN market) | `4.50` |
| `win_ppmax` | float | Max pre-play price | `5.00` |
| `win_ppmin` | float | Min pre-play price | `4.00` |
| `place_ppwap` | float | PLACE market VWAP | `2.10` |

---

## Headgear Symbols

| Symbol | Meaning |
|--------|---------|
| `b` | Blinkers |
| `v` | Visor |
| `t` | Tongue Tie |
| `h` | Hood |
| `p` | Cheek Pieces |
| `e` | Eyeshield |

---

## CSS Styling Examples

```css
/* Live badge */
.badge-live {
  background: #22c55e;
  color: white;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
}

/* Price change indicators */
.price-better {
  color: #22c55e; /* Green - price improved (went down) */
  animation: pulse-green 0.5s;
}

.price-worse {
  color: #ef4444; /* Red - price worsened (went up) */
  animation: pulse-red 0.5s;
}

/* Live indicator dot */
.live-indicator {
  color: #22c55e;
  font-size: 20px;
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}
```

---

## Error Handling

```javascript
async function fetchRaces(date) {
  try {
    const res = await fetch(`/api/v1/races?date=${date}`);
    
    if (!res.ok) {
      throw new Error(`API error: ${res.status}`);
    }
    
    const data = await res.json();
    return data.races;
    
  } catch (error) {
    console.error('Failed to fetch races:', error);
    // Show user-friendly error message
    return [];
  }
}
```

---

## Testing Live Prices

Run the test script:
```bash
./docs/QUICK_API_TEST.sh
```

Or manually test price updates:
```bash
# Get initial price
curl -s http://localhost:8000/api/v1/races/123456 | jq '.runners[0].win_ppwap'

# Wait 60 seconds
sleep 60

# Get updated price
curl -s http://localhost:8000/api/v1/races/123456 | jq '.runners[0].win_ppwap'
```

---

## Performance Tips

1. **Cache responses** for 30-60 seconds on frontend
2. **Don't poll faster than 60s** - backend updates every 60s
3. **Use React Query** or SWR for automatic caching/revalidation:

```javascript
import useSWR from 'swr';

function TodaysRaces() {
  const { data, error } = useSWR(
    '/api/v1/races?date=2025-10-15',
    fetcher,
    { refreshInterval: 60000 } // Refresh every 60s
  );

  if (error) return <div>Error loading races</div>;
  if (!data) return <div>Loading...</div>;

  return <RaceList races={data.races} />;
}
```
