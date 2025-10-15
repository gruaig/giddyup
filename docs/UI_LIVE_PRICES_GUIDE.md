# UI Developer Guide: Today's Races & Live Prices

**Date:** October 15, 2025  
**For:** Frontend/UI Developers  
**Backend Version:** v2.0 (Live Prices)

## What's New

The backend now supports **today's races with live Betfair exchange prices** that update every 60 seconds. This enables real-time price displays for races happening today while maintaining the historical BSP (Betfair Starting Price) data for completed races.

### Key Changes

1. **Today's races available before results** - Race cards loaded at 6am
2. **Live prices** - Betfair exchange prices updated every 60 seconds
3. **Preliminary flag** - Distinguishes incomplete vs complete race data
4. **Non-destructive updates** - Prices accumulate without data loss
5. **UK/IRE races only** - No international races

## API Changes & New Features

### No Breaking Changes ✅

All existing endpoints work exactly as before. The changes are **additive only**:
- Existing race endpoints now include today's preliminary data
- New `prelim` flag indicates incomplete races
- Live prices available in existing price fields

### Modified Response Fields

#### Race Object

```typescript
interface Race {
  race_id: number;
  race_key: string;
  race_date: string;        // YYYY-MM-DD
  region: "GB" | "IRE";
  course_id: number;
  course_name: string;
  off_time: string;         // HH:MM
  race_name: string;
  race_type: "Flat" | "Hurdle" | "Chase" | "NH Flat";
  class: string | null;
  distance: string | null;
  going: string | null;
  surface: "Turf" | "AW";
  ran: number;              // 0 for today's races (prelim), >0 for complete
  prelim: boolean;          // NEW: true for today's races, false for complete results
  runners: Runner[];
}
```

#### Runner Object (Price Fields)

```typescript
interface Runner {
  runner_id: number;
  num: number;
  draw: number | null;
  horse_id: number;
  horse_name: string;
  jockey_id: number | null;
  jockey_name: string | null;
  trainer_id: number | null;
  trainer_name: string | null;
  age: number | null;
  weight_lbs: number | null;
  or: number | null;        // Official Rating
  rpr: number | null;       // Racing Post Rating
  
  // Position (only for complete races, prelim=false)
  position: string | null;  // e.g., "1", "2", "3", "PU", "F"
  
  // Live Prices (today's races, updated every 60s)
  win_ppwap: number | null;    // VWAP - Volume Weighted Average Price (BEST FOR DISPLAY)
  win_ppmax: number | null;    // Highest back price seen today
  win_ppmin: number | null;    // Lowest lay price seen today
  
  // Historical BSP (yesterday and older, final prices)
  win_bsp: number | null;      // Betfair Starting Price (official)
  place_bsp: number | null;    // Place Starting Price (if applicable)
  
  // Comment (only for complete races)
  comment: string | null;
}
```

### Key Price Fields Explained

| Field | When Available | Use Case | Example |
|-------|---------------|----------|---------|
| `win_ppwap` | Today (live), Yesterday (BSP period) | **Primary display price** | 3.45 |
| `win_ppmax` | Today only | Show price range/movement | 4.20 (was higher) |
| `win_ppmin` | Today only | Show price range/movement | 3.15 (was lower) |
| `win_bsp` | Yesterday+ (after results) | Official starting price | 3.50 |
| `place_bsp` | Yesterday+ (if applicable) | Place market price | 1.85 |

## Detecting Today's Races vs Historical

### Simple Check

```typescript
function isLiveRace(race: Race): boolean {
  return race.prelim === true && race.race_date === getCurrentDate();
}

function isHistoricalRace(race: Race): boolean {
  return race.prelim === false;
}

function isPreliminary(race: Race): boolean {
  return race.ran === 0 || race.prelim === true;
}
```

### Display Logic

```typescript
function getRaceStatus(race: Race): "upcoming" | "in-play" | "resulted" {
  const now = new Date();
  const raceTime = parseRaceDateTime(race.race_date, race.off_time);
  
  if (race.prelim === false || race.ran > 0) {
    return "resulted";  // Has official results
  }
  
  if (now < raceTime) {
    return "upcoming";  // Before off-time
  }
  
  return "in-play";     // After off-time, awaiting results
}
```

### ⚠️ IMPORTANT: In-Play Behavior

**Betfair removes pre-play prices when races go in-play** (after the off-time). This means:

- **Before off-time**: Live prices update every 60s ✅
- **After off-time (in-play)**: Prices disappear from API ⚠️
- **After results**: Official BSP available next day ✅

**What this means for your UI:**
```typescript
function PriceDisplay({ race, runner }: { race: Race; runner: Runner }) {
  const status = getRaceStatus(race);
  const hasPrice = runner.win_ppwap != null;
  
  if (status === "in-play" && !hasPrice) {
    return (
      <div className="flex items-center gap-2 text-gray-500">
        <ClockIcon size={16} />
        <span className="text-sm">In Play</span>
      </div>
    );
  }
  
  if (status === "upcoming" && hasPrice) {
    return <LivePrice runner={runner} />;  // Show live price
  }
  
  if (status === "resulted" && hasPrice) {
    return <HistoricalPrice runner={runner} />;  // Show BSP
  }
  
  return <span className="text-gray-400">-</span>;
}
```

## Price Display Strategy

### For Today's Races (Live)

Display the VWAP with optional range:

```tsx
function LivePrice({ runner, race }: { runner: Runner; race: Race }) {
  const status = getRaceStatus(race);
  
  // Handle in-play races (Betfair removes prices after off-time)
  if (status === "in-play" && !runner.win_ppwap) {
    return (
      <div className="flex items-center gap-1 text-sm text-gray-500">
        <span>In Play</span>
        <span className="text-xs">(Prices suspended)</span>
      </div>
    );
  }
  
  if (!runner.win_ppwap) {
    return <span className="text-gray-400">-</span>;
  }
  
  const hasMovement = runner.win_ppmax && runner.win_ppmin;
  const priceChange = hasMovement 
    ? ((runner.win_ppwap - runner.win_ppmin) / runner.win_ppmin * 100)
    : 0;
  
  return (
    <div className="flex items-center gap-2">
      <span className="font-bold text-lg">
        {runner.win_ppwap.toFixed(2)}
      </span>
      
      {hasMovement && (
        <span className={`text-xs ${priceChange > 0 ? 'text-green-600' : 'text-red-600'}`}>
          {priceChange > 0 ? '↗' : '↘'} {Math.abs(priceChange).toFixed(1)}%
        </span>
      )}
      
      <span className="text-xs text-gray-500">
        LIVE
      </span>
      
      {hasMovement && (
        <span className="text-xs text-gray-400">
          {runner.win_ppmin.toFixed(2)} - {runner.win_ppmax.toFixed(2)}
        </span>
      )}
    </div>
  );
}
```

### For Historical Races (BSP)

Display the official starting price:

```tsx
function HistoricalPrice({ runner }: { runner: Runner }) {
  const price = runner.win_bsp || runner.win_ppwap;
  
  if (!price) {
    return <span className="text-gray-400">-</span>;
  }
  
  return (
    <div className="flex items-center gap-2">
      <span className="font-medium">
        {price.toFixed(2)}
      </span>
      <span className="text-xs text-gray-500">
        BSP
      </span>
    </div>
  );
}
```

### Combined Component (Smart)

```tsx
function SmartPrice({ race, runner }: { race: Race; runner: Runner }) {
  const isToday = race.race_date === format(new Date(), 'yyyy-MM-dd');
  const isLive = race.prelim && isToday;
  
  if (isLive) {
    return <LivePrice runner={runner} />;
  }
  
  return <HistoricalPrice runner={runner} />;
}
```

## Real-Time Price Updates

### Polling Strategy (Recommended)

For today's races, poll the API every 60-90 seconds:

```typescript
function useLivePrices(raceId: number, enabled: boolean) {
  const [race, setRace] = useState<Race | null>(null);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  
  useEffect(() => {
    if (!enabled) return;
    
    const fetchPrices = async () => {
      try {
        const response = await fetch(`/api/v1/races/${raceId}`);
        const data = await response.json();
        setRace(data);
        setLastUpdate(new Date());
      } catch (error) {
        console.error('Failed to fetch live prices:', error);
      }
    };
    
    // Initial fetch
    fetchPrices();
    
    // Poll every 60 seconds (matches backend update frequency)
    const interval = setInterval(fetchPrices, 60000);
    
    return () => clearInterval(interval);
  }, [raceId, enabled]);
  
  return { race, lastUpdate };
}
```

### Usage Example

```tsx
function RaceCard({ raceId }: { raceId: number }) {
  const race = useRace(raceId);
  const isToday = race?.race_date === format(new Date(), 'yyyy-MM-dd');
  const { race: liveRace, lastUpdate } = useLivePrices(raceId, isToday && race?.prelim);
  
  const displayRace = liveRace || race;
  
  return (
    <div>
      <h2>{displayRace.race_name}</h2>
      <p>{displayRace.course_name} - {displayRace.off_time}</p>
      
      {isToday && lastUpdate && (
        <div className="text-xs text-gray-500">
          Last updated: {format(lastUpdate, 'HH:mm:ss')}
          <span className="ml-2 animate-pulse text-green-500">● LIVE</span>
        </div>
      )}
      
      <table>
        {displayRace.runners.map(runner => (
          <tr key={runner.runner_id}>
            <td>{runner.num}</td>
            <td>{runner.horse_name}</td>
            <td><SmartPrice race={displayRace} runner={runner} /></td>
          </tr>
        ))}
      </table>
    </div>
  );
}
```

### Batch Updates (Efficient)

For race list pages showing multiple races, fetch in batch:

```typescript
async function fetchTodaysRaces() {
  const today = format(new Date(), 'yyyy-MM-dd');
  const response = await fetch(
    `/api/v1/races?date_from=${today}&date_to=${today}&prelim=true`
  );
  return response.json();
}

// Poll every 60-90 seconds
setInterval(async () => {
  const races = await fetchTodaysRaces();
  updateRaceStore(races);
}, 60000);
```

## UI/UX Recommendations

### 1. Visual Indicators

**Live Badge**
```tsx
{race.prelim && (
  <span className="inline-flex items-center gap-1 px-2 py-1 rounded-full bg-red-100 text-red-700 text-xs font-medium">
    <span className="animate-pulse">●</span>
    LIVE PRICES
  </span>
)}
```

**Last Update Timestamp**
```tsx
<div className="text-xs text-gray-500">
  Updated {formatDistanceToNow(lastUpdate, { addSuffix: true })}
</div>
```

**Price Movement Indicator**
```tsx
{priceChange > 0 && <TrendingUp className="text-green-500" size={16} />}
{priceChange < 0 && <TrendingDown className="text-red-500" size={16} />}
```

### 2. Race Card Layout

**Today's Race Card (Pre-Race)**
```
┌─────────────────────────────────────────┐
│ 🔴 LIVE  Updated 2 mins ago             │
│ 3:30 Wetherby - Class 3 Hurdle (2m4f)  │
│ Going: Good to Soft                     │
├─────────────────────────────────────────┤
│ No. Horse           J/T           Price │
│  1  Thunder Bay     Smith/Jones   3.45  │
│                                   ↗ 2%  │
│  2  River Dance     Brown/White   5.20  │
│                                   ↘ 5%  │
│  3  Mountain High   Davis/Miller  8.50  │
└─────────────────────────────────────────┘
```

**Historical Race (With Results)**
```
┌─────────────────────────────────────────┐
│ ✓ RESULTED  Oct 14, 2025                │
│ 3:30 Wetherby - Class 3 Hurdle (2m4f)  │
│ Winner: Thunder Bay                     │
├─────────────────────────────────────────┤
│ Pos Horse           J/T          BSP    │
│  1  Thunder Bay     Smith/Jones  3.50   │
│  2  River Dance     Brown/White  5.10   │
│  3  Mountain High   Davis/Miller  8.20  │
└─────────────────────────────────────────┘
```

### 3. Price Chart (Optional Enhancement)

For today's races, you can fetch historical prices:

```typescript
async function fetchPriceHistory(runnerId: number) {
  const response = await fetch(`/api/v1/runners/${runnerId}/price-history`);
  return response.json();
}

// Note: Backend endpoint needs to be created for this
// Query: SELECT ts, vwap FROM racing.live_prices WHERE runner_id = $1 ORDER BY ts
```

Display as sparkline or line chart showing price movement throughout the day.

## API Endpoints Reference

### Get Today's Races

**Endpoint:** `GET /api/v1/races?date_from={today}&date_to={today}`

**Example Request:**
```bash
GET /api/v1/races?date_from=2025-10-15&date_to=2025-10-15
```

**Example Response:**
```json
[
  {
    "race_id": 12345,
    "race_key": "abc123...",
    "race_date": "2025-10-15",
    "region": "GB",
    "course_id": 40,
    "course_name": "Nottingham",
    "off_time": "13:35",
    "race_name": "British Stallion Studs EBF Maiden Stakes",
    "race_type": "Flat",
    "class": "5",
    "distance": "1m",
    "going": "Good",
    "surface": "Turf",
    "ran": 0,
    "prelim": true,
    "runners": [
      {
        "runner_id": 67890,
        "num": 1,
        "draw": 3,
        "horse_id": 123456,
        "horse_name": "Thunder Strike",
        "jockey_id": 789,
        "jockey_name": "Joe Smith",
        "trainer_id": 456,
        "trainer_name": "John Jones",
        "age": 3,
        "weight_lbs": 133,
        "or": 75,
        "rpr": 82,
        "position": null,
        "win_ppwap": 4.25,
        "win_ppmax": 4.80,
        "win_ppmin": 3.90,
        "win_bsp": null,
        "place_bsp": null,
        "comment": null
      }
    ]
  }
]
```

### Get Single Race

**Endpoint:** `GET /api/v1/races/:race_id`

Same structure as above, single race object.

### Get Races by Date Range

**Endpoint:** `GET /api/v1/races?date_from={start}&date_to={end}`

**Combines:**
- Historical races (prelim=false, with BSP)
- Today's races (prelim=true, with live prices)

**Filter by completion:**
```bash
# Only complete races
GET /api/v1/races?date_from=2025-10-01&date_to=2025-10-15&prelim=false

# Only preliminary (today's)
GET /api/v1/races?date_from=2025-10-15&date_to=2025-10-15&prelim=true
```

### Get Race by Course/Time

**Endpoint:** `GET /api/v1/races/:course_id/meetings?date_from={date}&date_to={date}`

Returns all races at a course on a specific date (includes today's preliminary races).

## Frontend Implementation Examples

### React Component: Today's Races Page

```typescript
import { useState, useEffect } from 'react';
import { format } from 'date-fns';

interface TodaysRacesProps {}

export function TodaysRaces({}: TodaysRacesProps) {
  const [races, setRaces] = useState<Race[]>([]);
  const [loading, setLoading] = useState(true);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);

  useEffect(() => {
    const fetchRaces = async () => {
      try {
        const today = format(new Date(), 'yyyy-MM-dd');
        const response = await fetch(
          `/api/v1/races?date_from=${today}&date_to=${today}&prelim=true`
        );
        const data = await response.json();
        setRaces(data);
        setLastUpdate(new Date());
        setLoading(false);
      } catch (error) {
        console.error('Failed to fetch races:', error);
      }
    };

    // Initial fetch
    fetchRaces();

    // Poll every 60 seconds for price updates
    const interval = setInterval(fetchRaces, 60000);

    return () => clearInterval(interval);
  }, []);

  if (loading) {
    return <div>Loading today's races...</div>;
  }

  // Group by course
  const racesByCourse = races.reduce((acc, race) => {
    const course = race.course_name;
    if (!acc[course]) acc[course] = [];
    acc[course].push(race);
    return acc;
  }, {} as Record<string, Race[]>);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Today's Racing</h1>
        <div className="flex items-center gap-2 text-sm text-gray-600">
          <span className="animate-pulse text-red-500">●</span>
          Live Prices
          {lastUpdate && (
            <span className="text-gray-400">
              Updated {format(lastUpdate, 'HH:mm:ss')}
            </span>
          )}
        </div>
      </div>

      {Object.entries(racesByCourse).map(([course, courseRaces]) => (
        <div key={course} className="bg-white rounded-lg shadow p-6">
          <h2 className="text-xl font-semibold mb-4">{course}</h2>
          <div className="space-y-3">
            {courseRaces.map(race => (
              <RaceCardCompact key={race.race_id} race={race} />
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
```

### Race Card Component

```typescript
function RaceCardCompact({ race }: { race: Race }) {
  const status = getRaceStatus(race);
  
  return (
    <div className="border rounded p-4 hover:shadow-md transition-shadow">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-3">
          <span className="text-2xl font-bold">{race.off_time}</span>
          <div>
            <h3 className="font-medium">{race.race_name}</h3>
            <p className="text-sm text-gray-600">
              {race.race_type} • {race.distance} • {race.going}
            </p>
          </div>
        </div>
        
        <StatusBadge status={status} />
      </div>

      {/* Runners Table */}
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b">
            <th className="text-left py-2">No.</th>
            <th className="text-left">Horse</th>
            <th className="text-left">Jockey</th>
            <th className="text-right">Price</th>
          </tr>
        </thead>
        <tbody>
          {race.runners.slice(0, 5).map(runner => (
            <tr key={runner.runner_id} className="border-b hover:bg-gray-50">
              <td className="py-2">{runner.num}</td>
              <td className="font-medium">{runner.horse_name}</td>
              <td className="text-gray-600">{runner.jockey_name}</td>
              <td className="text-right">
                <SmartPrice race={race} runner={runner} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      
      {race.runners.length > 5 && (
        <button className="mt-2 text-blue-600 text-sm">
          View all {race.runners.length} runners →
        </button>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const styles = {
    upcoming: 'bg-blue-100 text-blue-700',
    'in-play': 'bg-green-100 text-green-700',
    resulted: 'bg-gray-100 text-gray-700',
  };
  
  return (
    <span className={`px-3 py-1 rounded-full text-xs font-medium ${styles[status]}`}>
      {status === 'in-play' && <span className="animate-pulse mr-1">●</span>}
      {status.toUpperCase().replace('-', ' ')}
    </span>
  );
}
```

## Data Freshness

### Update Frequencies

| Data Type | Update Frequency | Source | Notes |
|-----------|-----------------|--------|-------|
| Today's race structure | Once at startup (6am) | Racing Post racecards | |
| Live prices (pre-play) | Every 60 seconds | Betfair API (30-60s delayed) | **Only before off-time** |
| In-play prices | ❌ Not available | Betfair suspends | Prices disappear during race |
| Historical results | Once per day | Racing Post results + Betfair BSP | Next morning |

### ⚠️ In-Play Price Gap

**Timeline:**
1. **Pre-race** (6am - off_time): Live prices update every 60s ✅
2. **In-play** (off_time - finish): **NO PRICES** - Betfair suspends market ⚠️
3. **Post-race** (finish - next morning): Waiting for results ⏳
4. **Next day**: Official BSP and results available ✅

**Handle this in your UI:**
```typescript
const racePhase = getRacePhase(race);

switch (racePhase) {
  case 'pre-play':
    return <LivePrice runner={runner} />;
  case 'in-play':
    return <InPlayIndicator />;  // "Race in progress"
  case 'awaiting-results':
    return <AwaitingResults />;  // "Results pending"
  case 'resulted':
    return <OfficialPrice runner={runner} />;
}
```

### Polling Guidelines

**For race list pages:**
- Poll every 60-90 seconds
- Only poll during racing hours (6am-11pm)
- Stop polling after last race completes

**For individual race cards:**
- Poll every 60 seconds while race is upcoming/in-play
- Stop when race is resulted
- Cache aggressively for historical races

**Optimization:**
```typescript
function shouldPoll(race: Race): boolean {
  if (race.prelim === false) return false;  // Don't poll complete races
  
  const now = new Date();
  const raceTime = parseRaceDateTime(race.race_date, race.off_time);
  const hoursSinceRace = (now.getTime() - raceTime.getTime()) / (1000 * 60 * 60);
  
  if (hoursSinceRace > 2) return false;  // Stop 2 hours after race
  if (now.getHours() < 6 || now.getHours() >= 23) return false;  // Outside racing hours
  
  return true;
}
```

## Displaying Price Movements

### Color Coding

```typescript
function getPriceColor(current: number, max: number, min: number): string {
  const range = max - min;
  if (range === 0) return 'text-gray-700';
  
  const position = (current - min) / range;
  
  if (position > 0.7) return 'text-green-600';  // Near high (good for backers)
  if (position < 0.3) return 'text-red-600';    // Near low (shortening)
  return 'text-gray-700';                        // Mid-range
}
```

### Price Movement Component

```tsx
function PriceMovement({ runner }: { runner: Runner }) {
  if (!runner.win_ppwap || !runner.win_ppmax || !runner.win_ppmin) {
    return <span className="text-gray-400">-</span>;
  }
  
  const current = runner.win_ppwap;
  const high = runner.win_ppmax;
  const low = runner.win_ppmin;
  const range = high - low;
  
  if (range < 0.1) {
    // Stable price
    return (
      <div className="flex items-center gap-2">
        <span className="font-bold">{current.toFixed(2)}</span>
        <span className="text-xs text-gray-500">STABLE</span>
      </div>
    );
  }
  
  const fromHigh = ((current - low) / range * 100).toFixed(0);
  
  return (
    <div className="space-y-1">
      <div className="flex items-center gap-2">
        <span className="font-bold text-lg">{current.toFixed(2)}</span>
        <span className="text-xs text-gray-400">
          L: {low.toFixed(2)} H: {high.toFixed(2)}
        </span>
      </div>
      
      {/* Visual range indicator */}
      <div className="w-24 h-1.5 bg-gray-200 rounded-full overflow-hidden">
        <div 
          className="h-full bg-blue-500"
          style={{ width: `${fromHigh}%` }}
        />
      </div>
    </div>
  );
}
```

## Error Handling

### Handle Missing Prices

```typescript
function PriceDisplay({ runner }: { runner: Runner }) {
  const hasLivePrice = runner.win_ppwap != null;
  const hasBSP = runner.win_bsp != null;
  
  if (!hasLivePrice && !hasBSP) {
    return (
      <div className="text-sm text-gray-400">
        <span>Price unavailable</span>
        <InfoIcon className="inline ml-1" size={14} />
      </div>
    );
  }
  
  // ... render price
}
```

### Handle API Errors

```typescript
function useLivePrices(raceId: number, enabled: boolean) {
  const [error, setError] = useState<string | null>(null);
  const [retryCount, setRetryCount] = useState(0);
  
  const fetchPrices = async () => {
    try {
      const response = await fetch(`/api/v1/races/${raceId}`);
      
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }
      
      const data = await response.json();
      setRace(data);
      setError(null);
      setRetryCount(0);
    } catch (err) {
      setError(err.message);
      setRetryCount(prev => prev + 1);
      
      // Exponential backoff
      if (retryCount < 5) {
        setTimeout(fetchPrices, Math.min(1000 * Math.pow(2, retryCount), 30000));
      }
    }
  };
  
  // ... rest of hook
}
```

## Testing Checklist for UI

### Display Tests

- [ ] Today's races show with prelim=true badge
- [ ] Live prices display in win_ppwap field
- [ ] Price range shows (min/max) for movement tracking
- [ ] Last update timestamp displays and updates
- [ ] Historical races show BSP instead of live prices
- [ ] Races sort by off_time correctly
- [ ] Course grouping works (if implemented)

### Interaction Tests

- [ ] Polling starts automatically for today's races
- [ ] Polling stops for historical races
- [ ] Price updates reflect within 60-90 seconds
- [ ] Race transitions from "upcoming" → "in-play" → "resulted"
- [ ] Clicking race opens detailed view
- [ ] All prices display with 2 decimal places

### Responsive Tests

- [ ] Mobile: Race cards stack vertically
- [ ] Mobile: Price displays remain readable
- [ ] Mobile: Live indicator visible
- [ ] Tablet: Optimal layout for card grid
- [ ] Desktop: Multi-column layout works

### Edge Cases

- [ ] No races today (empty state)
- [ ] Race with no prices (show "-" or "N/A")
- [ ] API timeout (show error, retry)
- [ ] Race cancelled/abandoned (handle gracefully)
- [ ] Non-runner (check runner.status if added)

## State Management Examples

### React Context

```typescript
// contexts/LivePricesContext.tsx
interface LivePricesContextType {
  races: Race[];
  lastUpdate: Date | null;
  isPolling: boolean;
  startPolling: () => void;
  stopPolling: () => void;
}

export const LivePricesProvider = ({ children }) => {
  const [races, setRaces] = useState<Race[]>([]);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const [isPolling, setIsPolling] = useState(false);
  
  const fetchRaces = async () => {
    const today = format(new Date(), 'yyyy-MM-dd');
    const response = await fetch(
      `/api/v1/races?date_from=${today}&date_to=${today}`
    );
    const data = await response.json();
    setRaces(data);
    setLastUpdate(new Date());
  };
  
  const startPolling = () => {
    if (isPolling) return;
    setIsPolling(true);
    fetchRaces();
    
    const interval = setInterval(fetchRaces, 60000);
    // Store interval ID for cleanup
  };
  
  const stopPolling = () => {
    setIsPolling(false);
    // Clear interval
  };
  
  return (
    <LivePricesContext.Provider value={{ races, lastUpdate, isPolling, startPolling, stopPolling }}>
      {children}
    </LivePricesContext.Provider>
  );
};
```

### Usage

```typescript
function RacePage() {
  const { races, lastUpdate, startPolling, stopPolling } = useLivePrices();
  
  useEffect(() => {
    startPolling();
    return () => stopPolling();
  }, []);
  
  return <div>{/* ... */}</div>;
}
```

## Accessibility Considerations

### Screen Reader Announcements

```tsx
<div aria-live="polite" aria-atomic="true" className="sr-only">
  {lastUpdate && `Prices updated at ${format(lastUpdate, 'HH:mm:ss')}`}
</div>
```

### Price Change Announcements

```tsx
{priceChanged && (
  <span className="sr-only">
    Price changed from {oldPrice} to {newPrice}
  </span>
)}
```

### Keyboard Navigation

- Ensure race cards are keyboard navigable
- Tab through runners logically
- Enter/Space to expand detailed view

## Performance Optimization

### Memoization

```typescript
const PriceDisplay = memo(({ runner }: { runner: Runner }) => {
  // Component implementation
}, (prev, next) => {
  // Only re-render if prices actually changed
  return prev.runner.win_ppwap === next.runner.win_ppwap &&
         prev.runner.win_ppmax === next.runner.win_ppmax &&
         prev.runner.win_ppmin === next.runner.win_ppmin;
});
```

### Virtual Scrolling

For pages with many races (>20), use virtual scrolling:

```typescript
import { useVirtualizer } from '@tanstack/react-virtual';

function VirtualRaceList({ races }: { races: Race[] }) {
  const parentRef = useRef<HTMLDivElement>(null);
  
  const virtualizer = useVirtualizer({
    count: races.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 200, // Estimated race card height
  });
  
  return (
    <div ref={parentRef} className="h-screen overflow-auto">
      <div style={{ height: `${virtualizer.getTotalSize()}px` }}>
        {virtualizer.getVirtualItems().map(item => (
          <div key={item.key} style={{ height: `${item.size}px`, transform: `translateY(${item.start}px)` }}>
            <RaceCardCompact race={races[item.index]} />
          </div>
        ))}
      </div>
    </div>
  );
}
```

## Mobile-Specific Considerations

### Compact Price Display

```tsx
// Mobile: Show only current price + indicator
function MobilePriceDisplay({ runner }: { runner: Runner }) {
  if (!runner.win_ppwap) return <span>-</span>;
  
  const movement = runner.win_ppmax && runner.win_ppmin
    ? (runner.win_ppwap - runner.win_ppmin) / runner.win_ppmin
    : 0;
  
  return (
    <div className="flex flex-col items-end">
      <span className="text-lg font-bold">{runner.win_ppwap.toFixed(2)}</span>
      {Math.abs(movement) > 0.01 && (
        <span className={`text-xs ${movement > 0 ? 'text-green-600' : 'text-red-600'}`}>
          {movement > 0 ? '↗' : '↘'} {(Math.abs(movement) * 100).toFixed(0)}%
        </span>
      )}
    </div>
  );
}
```

### Swipeable Race Cards

```tsx
import { useSwipeable } from 'react-swipeable';

function SwipeableRaceCards({ races }: { races: Race[] }) {
  const [currentIndex, setCurrentIndex] = useState(0);
  
  const handlers = useSwipeable({
    onSwipedLeft: () => setCurrentIndex(i => Math.min(i + 1, races.length - 1)),
    onSwipedRight: () => setCurrentIndex(i => Math.max(i - 1, 0)),
  });
  
  return (
    <div {...handlers} className="touch-pan-y">
      <RaceCardCompact race={races[currentIndex]} />
      <div className="flex justify-center gap-1 mt-4">
        {races.map((_, i) => (
          <div 
            key={i}
            className={`h-2 w-2 rounded-full ${i === currentIndex ? 'bg-blue-500' : 'bg-gray-300'}`}
          />
        ))}
      </div>
    </div>
  );
}
```

## WebSocket Alternative (Future)

If you need sub-second updates, consider WebSockets:

### Backend (Future Enhancement)

```go
// internal/handlers/websocket.go
func (h *WebSocketHandler) HandleLivePrices(c *gin.Context) {
  // Upgrade to WebSocket
  // Push price updates as they arrive
}
```

### Frontend

```typescript
function useLivePricesWebSocket(raceId: number) {
  useEffect(() => {
    const ws = new WebSocket(`ws://localhost:8080/ws/live-prices/${raceId}`);
    
    ws.onmessage = (event) => {
      const update = JSON.parse(event.data);
      updateRunnerPrice(update.runner_id, update.price);
    };
    
    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
      // Fallback to polling
    };
    
    return () => ws.close();
  }, [raceId]);
}
```

## API Response Time

### Expected Latencies

- `/api/v1/races` (today, 15 races): **< 100ms**
- `/api/v1/races/:id` (single race): **< 50ms**
- `/api/v1/races` (date range, 7 days): **< 500ms**

### Caching Headers

Backend should set appropriate cache headers:

```
# For today's races (prelim=true)
Cache-Control: max-age=30, must-revalidate

# For historical races (prelim=false)
Cache-Control: max-age=86400, immutable
```

Frontend caching strategy:
```typescript
const cacheKey = race.prelim 
  ? `race-live-${race.race_id}-${Math.floor(Date.now() / 60000)}`  // 1-min cache
  : `race-historical-${race.race_id}`;                               // Permanent cache
```

## Common Gotchas

### 1. Comparing Decimal Prices

**❌ Wrong:**
```typescript
if (price1 === price2) { ... }
```

**✅ Correct:**
```typescript
if (Math.abs(price1 - price2) < 0.01) { ... }
```

### 2. Null vs Zero Prices

**❌ Wrong:**
```typescript
if (!runner.win_ppwap) {
  // This catches both null AND 0, but 0 is invalid for prices anyway
}
```

**✅ Correct:**
```typescript
if (runner.win_ppwap === null || runner.win_ppwap === undefined) {
  return <span>No price</span>;
}

if (runner.win_ppwap < 1.01) {
  return <span>Invalid price</span>;
}
```

### 3. Time Zone Handling

All times in the API are **local UK/IRE time** (no UTC conversion needed for display):
- `off_time`: "13:35" means 1:35 PM local time
- Display as-is, no timezone conversion required

### 4. Prelim Flag vs Ran Count

Both indicate preliminary status:
```typescript
const isPreliminary = race.prelim === true || race.ran === 0;
```

Use `prelim` flag preferentially (more explicit).

## Sample API Queries

### Get All Today's Races with Prices

```bash
curl -X GET "http://localhost:8080/api/v1/races?date_from=2025-10-15&date_to=2025-10-15" \
  -H "Accept: application/json"
```

### Get Specific Race

```bash
curl -X GET "http://localhost:8080/api/v1/races/12345" \
  -H "Accept: application/json"
```

### Filter by Course

```bash
curl -X GET "http://localhost:8080/api/v1/races/40/meetings?date_from=2025-10-15&date_to=2025-10-15" \
  -H "Accept: application/json"
```

## Quick Start for UI Development

### 1. Setup

```bash
# Clone repository
git clone <repo-url>
cd GiddyUp

# Load backend config
source settings.env

# Start backend (terminal 1)
cd backend-api
./bin/api
```

### 2. Verify Data

```bash
# Check today's races loaded (terminal 2)
curl -s http://localhost:8080/api/v1/races?date_from=$(date +%Y-%m-%d) | jq 'length'

# Should return > 0 if races are available
```

### 3. Build UI

```typescript
// App.tsx or equivalent
import { TodaysRaces } from './components/TodaysRaces';

function App() {
  return (
    <div>
      <nav>{/* ... */}</nav>
      <main>
        <TodaysRaces />
      </main>
    </div>
  );
}
```

### 4. Test Live Updates

1. Open browser DevTools → Network tab
2. Navigate to today's races page
3. Observe API calls every 60 seconds
4. Watch prices update in UI

## TypeScript Definitions

```typescript
// types/racing.ts

export interface Race {
  race_id: number;
  race_key: string;
  race_date: string;
  region: "GB" | "IRE";
  course_id: number;
  course_name: string;
  off_time: string;
  race_name: string;
  race_type: "Flat" | "Hurdle" | "Chase" | "NH Flat";
  class: string | null;
  pattern: string | null;
  age_band: string | null;
  rating_band: string | null;
  sex_rest: string | null;
  distance: string | null;
  distance_f: number | null;
  distance_m: number | null;
  going: string | null;
  surface: "Turf" | "AW";
  ran: number;
  prelim: boolean;
  runners: Runner[];
}

export interface Runner {
  runner_id: number;
  num: number;
  position: string | null;
  draw: number | null;
  horse_id: number;
  horse_name: string;
  age: number | null;
  sex: string | null;
  weight_lbs: number | null;
  jockey_id: number | null;
  jockey_name: string | null;
  trainer_id: number | null;
  trainer_name: string | null;
  owner_id: number | null;
  owner_name: string | null;
  or: number | null;
  rpr: number | null;
  ts: number | null;
  
  // Prices
  win_ppwap: number | null;    // VWAP (live or historical pre-play)
  win_ppmax: number | null;    // Max back price (today only)
  win_ppmin: number | null;    // Min lay price (today only)
  win_bsp: number | null;      // Starting Price (historical)
  place_bsp: number | null;    // Place SP (historical)
  
  comment: string | null;
}

export interface LivePriceUpdate {
  runner_id: number;
  timestamp: string;
  back_price: number;
  lay_price: number;
  vwap: number;
  traded_volume: number;
}
```

## Summary for UI Developer

### What You Need to Know

1. **Today's Data Available**: Races appear in API from 6am onwards with `prelim: true`

2. **Price Field to Use**: Always use `win_ppwap` for display
   - For today: Live VWAP (updated every 60s)
   - For history: Historical VWAP or BSP

3. **Polling Required**: Poll API every 60 seconds for today's races only

4. **Price Range**: Use `win_ppmax` and `win_ppmin` to show movement

5. **Status Indicator**: Show "LIVE" badge for `prelim: true` races

6. **No Breaking Changes**: All existing functionality still works

### Priority UI Tasks

**High Priority:**
1. Display today's races with live prices
2. Implement 60-second polling
3. Show last update timestamp
4. Add "LIVE" indicator

**Medium Priority:**
1. Price movement indicators (↗↘)
2. Price range display (min-max)
3. Mobile-optimized layout
4. Error handling for missing prices

**Low Priority:**
1. Price charts/sparklines
2. WebSocket implementation
3. Push notifications for price movements
4. Advanced filtering (by course, time, etc.)

---

**Questions?** Check `/docs/02_API_DOCUMENTATION.md` for full API reference or `/docs/features/TODAYS_RACES_LIVE_PRICES.md` for backend implementation details.

**Support:** Backend API health check: `http://localhost:8080/health`

