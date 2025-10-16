# API Update - October 15, 2025
## Today's Races + Live Prices Implementation

### 🎉 What's New

The backend now automatically loads **today's** and **tomorrow's** race cards on server startup, with live Betfair prices updating every 60 seconds.

---

## 📋 Summary of Changes

### 1. **Auto-Update Behavior**
- ✅ **Server always fetches today + tomorrow on startup** (force refresh)
- ✅ Races are marked as `prelim: true` until official results are available
- ✅ Historical data (Oct 11-14) is backfilled automatically if missing
- ✅ Live Betfair prices update every 60 seconds for today/tomorrow

### 2. **Data Sources**
- **Primary**: Sporting Life (fast, complete JSON data)
- **Fallback**: Racing Post (HTML scraping, slower but reliable)
- **Historical**: Always uses Racing Post results for past dates

### 3. **Database Changes**
New fields and tables:
- `races.prelim` (boolean) - indicates preliminary/pre-race data
- `live_prices` table - stores tick-by-tick Betfair price snapshots

---

## 🔌 API Endpoints (No Changes Required)

### Your existing endpoints work exactly the same:

#### **GET `/api/v1/races?date=2025-10-15`**
Returns today's races with all standard fields.

**Response Structure** (unchanged):
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
      "race_type": "Flat",
      "surface": "Turf",
      "region": "gb",
      "prelim": true,  // ⬅️ NEW: indicates pre-race data
      "ran": 0,        // ⬅️ 0 until race completes
      "runners": [
        {
          "runner_id": 789,
          "num": 1,
          "draw": 3,
          "horse": "Example Horse",
          "horse_id": 456,
          "jockey": "John Smith",
          "jockey_id": 789,
          "trainer": "Jane Doe",
          "trainer_id": 101,
          "age": 3,
          "weight": "9-0",
          "or": 75,
          "rpr": null,     // null for prelim races
          "form": "123",   // recent form (NEW from Sporting Life)
          "headgear": "b", // headgear symbols (NEW)
          "comment": "Improved last time", // runner commentary (NEW)
          // Live prices (populated by Betfair):
          "win_bsp": null,
          "win_ppwap": 4.5,    // ⬅️ Live VWAP (updates every 60s)
          "win_ppmax": 5.0,
          "win_ppmin": 4.0,
          "place_ppwap": 2.1,
          // ... other price fields
        }
      ]
    }
  ]
}
```

#### **GET `/api/v1/races/:race_id`**
Returns a single race with all runners and prices.

#### **GET `/api/v1/courses`**
Lists all courses (unchanged).

#### **GET `/api/v1/races?course=Nottingham`**
Filter by course (unchanged).

---

## 🔄 What Changed for UI

### **New Fields in API Response**

1. **`prelim` (boolean)** on `races` table
   - `true` = pre-race data (no results yet)
   - `false` = official results available
   - **UI Action**: Display "LIVE" or "UPCOMING" badge when `prelim: true`

2. **`form` (string)** on runners
   - Recent form summary (e.g., "1234" = 1st, 2nd, 3rd, 4th in last 4 races)
   - **UI Action**: Display next to horse name

3. **`headgear` (string)** on runners
   - Headgear symbols (e.g., "b" = blinkers, "v" = visor, "t" = tongue tie)
   - **UI Action**: Display as small icon/badge next to horse

4. **`comment` (string)** on runners
   - Expert commentary/notes about the runner
   - **UI Action**: Display in tooltip or expandable section

5. **Live Prices** (updating every 60s)
   - `win_ppwap`, `win_ppmax`, `win_ppmin` - updated in real-time
   - `place_ppwap`, `place_ppmax`, `place_ppmin` - place market prices
   - **UI Action**: 
     - Poll `/api/v1/races?date=today` every 60 seconds
     - Highlight price changes (green = better, red = worse)
     - Show timestamp of last update

---

## 📊 Recommended UI Changes

### 1. **Today's Races Page**
```javascript
// Poll for live prices every 60 seconds
useEffect(() => {
  const fetchTodaysRaces = async () => {
    const response = await fetch('/api/v1/races?date=2025-10-15');
    const data = await response.json();
    setRaces(data.races);
  };

  // Initial fetch
  fetchTodaysRaces();

  // Poll every 60 seconds for live price updates
  const interval = setInterval(fetchTodaysRaces, 60000);
  
  return () => clearInterval(interval);
}, []);
```

### 2. **Race Card Display**
```jsx
{race.prelim && race.ran === 0 && (
  <Badge color="green">LIVE</Badge>
)}

{race.prelim === false && race.ran > 0 && (
  <Badge color="blue">RESULTS</Badge>
)}
```

### 3. **Runner Display with New Fields**
```jsx
<div className="runner">
  <span className="horse-name">{runner.horse}</span>
  
  {/* NEW: Form */}
  {runner.form && (
    <span className="form">{runner.form}</span>
  )}
  
  {/* NEW: Headgear */}
  {runner.headgear && (
    <span className="headgear" title="Headgear: {runner.headgear}">
      {headgearIcon(runner.headgear)}
    </span>
  )}
  
  {/* NEW: Commentary */}
  {runner.comment && (
    <Tooltip content={runner.comment}>
      <InfoIcon />
    </Tooltip>
  )}
  
  {/* Live Price (updates every 60s) */}
  <PriceDisplay 
    price={runner.win_ppwap} 
    prevPrice={prevPrices[runner.runner_id]}
    isLive={race.prelim}
  />
</div>
```

### 4. **Price Change Indicator**
```jsx
const PriceDisplay = ({ price, prevPrice, isLive }) => {
  const change = price && prevPrice ? price - prevPrice : 0;
  
  return (
    <div className={`price ${change > 0 ? 'worse' : change < 0 ? 'better' : ''}`}>
      {price ? price.toFixed(2) : 'N/A'}
      {isLive && <span className="live-indicator">●</span>}
      {change !== 0 && (
        <span className="change">{change > 0 ? '↑' : '↓'}</span>
      )}
    </div>
  );
};
```

---

## 🕐 Data Freshness Timeline

| Time | What Happens |
|------|--------------|
| **6:00 AM** | Server fetches today's + tomorrow's racecards (force refresh) |
| **6:00 AM - 11:00 PM** | Live Betfair prices update every 60 seconds |
| **After each race** | Prices go to `null` when market closes (race goes in-play) |
| **Next day 2:00 AM** | Yesterday's results are backfilled with official data |

---

## ⚠️ Important Notes for UI

### **Handling In-Play Races**
When a race goes "in-play" (starts), Betfair **suspends** the WIN market:
- `win_ppwap`, `win_ppmax`, `win_ppmin` will become `null`
- This is **normal behavior** - not a bug!

**UI Should**:
```jsx
{runner.win_ppwap === null && race.prelim ? (
  <Badge color="orange">IN-PLAY</Badge>
) : (
  <PriceDisplay price={runner.win_ppwap} />
)}
```

### **Tomorrow's Races**
- Available at `/api/v1/races?date=2025-10-16`
- Same structure as today
- Live prices available if Betfair markets are open (usually from ~10 AM day before)

### **Historical Races**
- Any date before today: `/api/v1/races?date=2025-10-11`
- Will have `prelim: false` and `ran > 0`
- Prices are **final** (BSP, VWAP, etc.)

---

## 🐛 Troubleshooting

### Q: "Why are some prices `null`?"
**A**: Three reasons:
1. Race hasn't opened on Betfair yet (too far in future)
2. Race is currently in-play (markets suspended)
3. Race finished (markets closed)

### Q: "Why does polling return the same data?"
**A**: Betfair updates every 60s. If you poll faster, you'll see duplicates.

### Q: "How do I know if prices are live?"
**A**: Check `race.prelim === true` AND `race.ran === 0` AND `win_ppwap !== null`

### Q: "What if I want more frequent updates?"
**A**: Backend interval is configurable in `settings.env`:
```bash
LIVE_PRICE_INTERVAL=30  # Change to 30 seconds
```
But be mindful of rate limits on Betfair API.

---

## 📈 Performance Considerations

- **Cache responses** on the frontend for 30-60 seconds
- **Don't poll faster than 60s** - backend only updates every 60s
- **Use WebSockets** (future enhancement) for real-time push updates

---

## 🚀 Testing Checklist

- [ ] Fetch today's races: `GET /api/v1/races?date=2025-10-15`
- [ ] Verify `prelim: true` for upcoming races
- [ ] Check `form`, `headgear`, `comment` fields are populated
- [ ] Prices update after 60 seconds (poll twice, 60s apart)
- [ ] Tomorrow's races accessible: `GET /api/v1/races?date=2025-10-16`
- [ ] Historical races have `prelim: false`: `GET /api/v1/races?date=2025-10-11`

---

## 📝 Summary

### **What You Need to Do**:
1. ✅ **No breaking changes** - all existing endpoints work as before
2. ✅ Add polling (every 60s) for today's races to get live price updates
3. ✅ Display new fields: `form`, `headgear`, `comment`
4. ✅ Show "LIVE" badge when `prelim: true && ran === 0`
5. ✅ Handle `null` prices gracefully (in-play or not available)
6. ✅ Show price change indicators (up/down arrows)

### **What the Backend Handles**:
1. ✅ Auto-fetch today + tomorrow on server startup
2. ✅ Force refresh today/tomorrow (always latest data)
3. ✅ Update Betfair prices every 60 seconds
4. ✅ Backfill historical results automatically
5. ✅ Non-destructive updates (never overwrite good data with nulls)

---

## 📞 Questions?

If you need:
- Different polling intervals
- WebSocket support for real-time updates
- Additional fields in the API response
- Filtering/sorting options

Let me know and I'll implement!

---

**Last Updated**: October 15, 2025  
**API Version**: v1  
**Backend Status**: ✅ Live and Running

