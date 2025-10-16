# UI Update Required: Add Live Price Column

**Date:** 2025-10-16  
**Priority:** Medium  
**Effort:** ~30 minutes

---

## Problem

Race cards currently show `BSP` (Betfair Starting Price) column, but it's empty for today/tomorrow races because:
- **BSP** = Final price when race starts (only available AFTER the race finishes)
- For **upcoming races**, we have **live exchange prices** but they're not displayed

## Current UI (Missing Prices)

```
Pos  Horse                 Draw  RPR  OR  BSP  BTN  Trainer        Jockey
-    Tamather (GB)         5     -    60  -    -    James Owen     Paddy Bradley
-    Lady Lysandra (GB)    1     -    60  -    -    D Donovan      Hollie Doyle
-    American Flight (IRE) 4     -    60  -    -    G Boughey      Billy Loughnane
```

## Required UI (With Live Prices)

```
Pos  Horse                 Draw  Price  RPR  OR  BSP  BTN  Trainer        Jockey
-    Tamather (GB)         5     5.20   -    60  -    -    James Owen     Paddy Bradley
-    Lady Lysandra (GB)    1     3.45   -    60  -    -    D Donovan      Hollie Doyle
-    American Flight (IRE) 4     7.80   -    60  -    -    G Boughey      Billy Loughnane
```

---

## Implementation

### 1. Add "Price" Column

**New column order:**
```
Pos | Horse | Draw | Price | RPR | OR | BSP | BTN | Trainer | Jockey
                      ^^^^
                      NEW!
```

**Column header:** `"Price"` or `"Live Price"` or `"Current"`

### 2. Data Source

The data is **already in the API** - no backend changes needed!

**API Response** (from `/api/v1/races/today` or any race endpoint):
```json
{
  "runners": [
    {
      "runner_id": 12345,
      "horse_name": "Tamather",
      "num": 5,
      "draw": 5,
      "win_ppwap": 5.20,    // ← USE THIS for "Price" column
      "win_ppmax": 5.80,    // Highest price seen (optional)
      "win_ppmin": 4.90,    // Lowest price seen (optional)
      "win_bsp": null,      // BSP (only after race)
      "or": 60,
      "rpr": null
    }
  ]
}
```

### 3. Display Logic

**For Today/Tomorrow Races:**
```javascript
// Show live price in "Price" column
price = runner.win_ppwap || "-"

// BSP shows "-" (not available yet)
bsp = "-"
```

**For Historical Races (yesterday and older):**
```javascript
// No live price available
price = "-"

// Show BSP (final price)
bsp = runner.win_bsp || "-"
```

### 4. Formatting

**Option A: Decimal Odds (Recommended)**
```
5.20
3.45
11.50
```

**Option B: Fractional Odds**
```javascript
// Convert decimal to fractional
function toFractional(decimal) {
  if (!decimal) return "-";
  const fraction = decimal - 1;
  // e.g., 5.20 → "21/5" or "4.2/1"
  return `${(fraction * 5).toFixed(0)}/5`;
}
```

**Option C: With Dash if Missing**
```javascript
function formatPrice(price) {
  if (!price || price === 0) return "-";
  return price.toFixed(2);
}
```

### 5. Real-Time Updates

Prices auto-update every 60 seconds on the backend.

**Frontend polling strategy:**
```javascript
// Poll every 30-60 seconds to get latest prices
setInterval(() => {
  fetchTodaysRaces(); // Refresh race data
}, 30000); // 30 seconds

// The win_ppwap field will automatically have latest prices
```

---

## Code Example (React/TypeScript)

```typescript
interface Runner {
  runner_id: number;
  horse_name: string;
  num: number;
  draw: number;
  win_ppwap: number | null;  // Live price
  win_ppmax: number | null;  // Highest seen
  win_ppmin: number | null;  // Lowest seen
  win_bsp: number | null;    // BSP (final)
  or: number | null;
  rpr: number | null;
}

function RaceCard({ race, runners }: Props) {
  const isUpcoming = new Date(race.race_date) >= new Date().setHours(0,0,0,0);
  
  return (
    <table>
      <thead>
        <tr>
          <th>Pos</th>
          <th>Horse</th>
          <th>Draw</th>
          <th>Price</th>  {/* NEW COLUMN */}
          <th>RPR</th>
          <th>OR</th>
          <th>BSP</th>
          <th>Trainer</th>
          <th>Jockey</th>
        </tr>
      </thead>
      <tbody>
        {runners.map(runner => (
          <tr key={runner.runner_id}>
            <td>-</td>
            <td>{runner.horse_name}</td>
            <td>{runner.draw || "-"}</td>
            
            {/* NEW: Show live price for upcoming races */}
            <td className="price">
              {isUpcoming 
                ? (runner.win_ppwap?.toFixed(2) || "-")
                : "-"
              }
            </td>
            
            <td>{runner.rpr || "-"}</td>
            <td>{runner.or || "-"}</td>
            
            {/* BSP only for finished races */}
            <td>{runner.win_bsp?.toFixed(2) || "-"}</td>
            
            <td>{runner.trainer_name}</td>
            <td>{runner.jockey_name}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
```

---

## Optional Enhancements (Nice to Have)

### A. Price Range (Show volatility)

```typescript
function PriceCell({ runner }: { runner: Runner }) {
  if (!runner.win_ppwap) return <td>-</td>;
  
  const hasRange = runner.win_ppmin && runner.win_ppmax;
  
  return (
    <td>
      <div className="price-current">{runner.win_ppwap.toFixed(2)}</div>
      {hasRange && (
        <div className="price-range text-muted text-xs">
          {runner.win_ppmin.toFixed(2)} - {runner.win_ppmax.toFixed(2)}
        </div>
      )}
    </td>
  );
}
```

**Displays as:**
```
5.20
4.90 - 5.80
```

### B. Price Movement Indicator

Store previous price in state and compare:

```typescript
const [prevPrices, setPrevPrices] = useState<Map<number, number>>(new Map());

function getPriceChange(runnerId: number, currentPrice: number) {
  const prev = prevPrices.get(runnerId);
  if (!prev) return null;
  
  if (currentPrice > prev) return "up";
  if (currentPrice < prev) return "down";
  return "stable";
}

// Display with arrows
<td className={`price price-${movement}`}>
  {price.toFixed(2)} {movement === "up" ? "↑" : movement === "down" ? "↓" : ""}
</td>
```

### C. Color Coding by Odds

```typescript
function getPriceColor(price: number) {
  if (price < 3) return "text-green-600";      // Favorite
  if (price < 6) return "text-blue-600";       // Mid-range
  if (price < 10) return "text-orange-600";    // Outsider
  return "text-gray-600";                       // Long shot
}

<td className={getPriceColor(runner.win_ppwap)}>
  {runner.win_ppwap.toFixed(2)}
</td>
```

---

## Testing Checklist

- [ ] Price column appears between Draw and RPR
- [ ] Today's races show live prices from `win_ppwap`
- [ ] Historical races show "-" for Price
- [ ] BSP shows "-" for upcoming races
- [ ] BSP shows value for completed races
- [ ] Prices update when polling API (every 30-60s)
- [ ] Missing prices display as "-" gracefully
- [ ] Decimal formatting shows 2 decimal places (e.g., "5.20")

---

## API Endpoints (No Changes Needed)

All existing endpoints already return the required fields:

```
GET /api/v1/races/today
GET /api/v1/races/tomorrow
GET /api/v1/races/{date}
GET /api/v1/races/{race_id}
```

**Response includes:**
```json
{
  "runners": [
    {
      "win_ppwap": 5.20,  // Already exists!
      "win_ppmax": 5.80,  // Already exists!
      "win_ppmin": 4.90,  // Already exists!
      "win_bsp": null     // Already exists!
    }
  ]
}
```

---

## Summary

1. **Add "Price" column** between "Draw" and "RPR"
2. **Display `win_ppwap`** from API response (no backend changes needed)
3. **Format as decimal** (e.g., "5.20")
4. **Show "-"** if null or zero
5. **Keep polling** every 30-60 seconds (prices auto-update on backend)

**Effort:** ~30 minutes  
**Backend changes:** None (data already available)  
**Testing:** Verify on today's races page

---

## Questions?

- **"Where does the data come from?"** → Betfair Exchange API, updated every 60 seconds automatically
- **"Do I need new API endpoints?"** → No, all existing endpoints already return this data
- **"What about historical races?"** → Show "-" for Price, show `win_bsp` for BSP column
- **"How often should I poll?"** → Every 30-60 seconds is fine
- **"Can I show both back and lay prices?"** → Yes, but those would need to be added to API response (not currently exposed)

---

**Ready to implement!** 🚀

