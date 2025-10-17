# How to Get Price Timestamps from the API

## 🎯 Quick Answer

The `price_updated_at` field is now included in every runner object from the races API.

---

## 📡 API Endpoints

### **1. Get Single Race with Runners**

```bash
GET http://localhost:8000/api/v1/races/{race_id}
```

**Example:**
```bash
curl http://localhost:8000/api/v1/races/811255
```

### **2. Get All Races for a Date**

```bash
GET http://localhost:8000/api/v1/races?date=2025-10-18
```

### **3. Get Meetings (includes runners)**

```bash
GET http://localhost:8000/api/v1/meetings?date=2025-10-18
```

---

## 📦 Response Format

### **JSON Structure:**

```json
{
  "race": {
    "race_id": 811255,
    "race_name": "Ascot Champions Day",
    "course_name": "Ascot",
    "off_time": "15:30:00",
    "race_date": "2025-10-18"
  },
  "runners": [
    {
      "runner_id": 12345,
      "horse_name": "Galileo Blue",
      "trainer_name": "A P O'Brien",
      "jockey_name": "R Moore",
      "win_ppwap": 5.60,
      "price_updated_at": "2025-10-17T19:28:45Z",  ← HERE!
      "dec": 5.50,
      "num": 3,
      "draw": 5
    }
  ]
}
```

---

## 🔍 Field Details

### **`price_updated_at`**

| Property | Value |
|----------|-------|
| **Type** | `string` (ISO 8601 timestamp) or `null` |
| **Format** | `"2025-10-17T19:28:45Z"` |
| **Timezone** | UTC (Z suffix) |
| **Null means** | Price never updated by live updater (using cached/BSP) |

---

## 💻 Frontend Usage

### **React Example:**

```tsx
interface Runner {
  horse_name: string;
  win_ppwap?: number;
  price_updated_at?: string;
}

function RunnerCard({ runner }: { runner: Runner }) {
  const { horse_name, win_ppwap, price_updated_at } = runner;
  
  // Calculate age in minutes
  const getAgeInMinutes = (timestamp: string | undefined): number | null => {
    if (!timestamp) return null;
    const updated = new Date(timestamp);
    return Math.floor((Date.now() - updated.getTime()) / 60000);
  };
  
  const ageMinutes = getAgeInMinutes(price_updated_at);
  
  return (
    <div>
      <h3>{horse_name}</h3>
      <div className="price">
        {win_ppwap ? win_ppwap.toFixed(2) : 'SP'}
      </div>
      
      {price_updated_at && (
        <div className="price-age">
          Updated: {ageMinutes !== null && ageMinutes < 60 
            ? `${ageMinutes}m ago`
            : new Date(price_updated_at).toLocaleTimeString()
          }
          
          {ageMinutes !== null && ageMinutes < 5 && (
            <span className="live-badge">LIVE</span>
          )}
        </div>
      )}
    </div>
  );
}
```

### **JavaScript/Fetch:**

```javascript
async function getRaceWithTimestamps(raceId) {
  const response = await fetch(`http://localhost:8000/api/v1/races/${raceId}`);
  const data = await response.json();
  
  // Access timestamp for each runner
  data.runners.forEach(runner => {
    console.log(`${runner.horse_name}:`);
    console.log(`  Price: ${runner.win_ppwap}`);
    console.log(`  Updated: ${runner.price_updated_at || 'Never'}`);
    
    if (runner.price_updated_at) {
      const ageMs = Date.now() - new Date(runner.price_updated_at).getTime();
      const ageMinutes = Math.floor(ageMs / 60000);
      console.log(`  Age: ${ageMinutes} minutes`);
    }
  });
}
```

### **Python Example:**

```python
import requests
from datetime import datetime, timezone

def get_race_with_timestamps(race_id):
    url = f"http://localhost:8000/api/v1/races/{race_id}"
    response = requests.get(url)
    data = response.json()
    
    for runner in data['runners']:
        horse = runner['horse_name']
        price = runner.get('win_ppwap', 'SP')
        timestamp = runner.get('price_updated_at')
        
        print(f"{horse}: {price}")
        
        if timestamp:
            updated = datetime.fromisoformat(timestamp.replace('Z', '+00:00'))
            age = datetime.now(timezone.utc) - updated
            minutes = int(age.total_seconds() / 60)
            print(f"  Updated: {minutes} minutes ago")
        else:
            print(f"  Updated: Never")

# Usage
get_race_with_timestamps(811255)
```

---

## 🎨 Display Examples

### **1. Simple Age Display:**

```
Galileo Blue
Price: 5.60
Updated: 2 mins ago ✅
```

### **2. Colored Badges:**

```css
.price-fresh { color: #22c55e; } /* < 5 mins */
.price-current { color: #3b82f6; } /* 5-30 mins */
.price-aging { color: #f59e0b; } /* 30-60 mins */
.price-stale { color: #ef4444; } /* > 60 mins */
```

```tsx
<span className={
  ageMinutes < 5 ? 'price-fresh' :
  ageMinutes < 30 ? 'price-current' :
  ageMinutes < 60 ? 'price-aging' : 'price-stale'
}>
  {win_ppwap}
</span>
```

### **3. Timestamp Display:**

```
Last updated: 19:28:45
2 minutes ago ✅
```

### **4. Global Freshness:**

```tsx
function RaceCard({ race, runners }) {
  // Find oldest price
  const oldestUpdate = runners
    .filter(r => r.price_updated_at)
    .map(r => new Date(r.price_updated_at))
    .sort((a, b) => a - b)[0];
  
  const ageMinutes = oldestUpdate 
    ? Math.floor((Date.now() - oldestUpdate.getTime()) / 60000)
    : null;
  
  return (
    <div>
      <h2>{race.race_name}</h2>
      {ageMinutes !== null && (
        <div className="race-freshness">
          {ageMinutes < 5 
            ? '✅ Live prices' 
            : `⏱️ Prices ${ageMinutes}m old`}
        </div>
      )}
      {/* runners */}
    </div>
  );
}
```

---

## 🔄 When is it Updated?

### **Live Price Updater:**

The `update_live_prices` service updates this field every time it fetches new prices:

```go
UPDATE racing.runners
SET 
    win_ppwap = 5.60,
    price_updated_at = NOW()  ← Sets current timestamp
WHERE betfair_selection_id = 12345678;
```

### **Update Frequency:**

- **Continuous mode:** Every 30 minutes
- **One-shot mode:** On-demand via command

### **Check if Updater is Running:**

```bash
ps aux | grep update_live_prices
```

### **Manual Update:**

```bash
cd /home/smonaghan/GiddyUp/backend-api
source ../settings.env
./bin/update_live_prices --date=2025-10-18
```

---

## 🧪 Testing

### **1. Check Database:**

```sql
SELECT 
    h.horse_name,
    ru.win_ppwap,
    ru.price_updated_at,
    EXTRACT(EPOCH FROM (NOW() - ru.price_updated_at))/60 as minutes_ago
FROM racing.runners ru
LEFT JOIN racing.horses h ON h.horse_id = ru.horse_id
WHERE ru.race_date = '2025-10-18'
AND ru.price_updated_at IS NOT NULL
ORDER BY ru.price_updated_at DESC
LIMIT 10;
```

### **2. Test API Response:**

```bash
# Get race with timestamps
curl -s http://localhost:8000/api/v1/races/811255 | \
  jq '.runners[] | {horse: .horse_name, price: .win_ppwap, updated: .price_updated_at}'
```

### **3. Check Freshness:**

```bash
curl -s http://localhost:8000/api/v1/races/811255 | \
  python3 -c "
import json, sys
from datetime import datetime, timezone

data = json.load(sys.stdin)
for r in data['runners']:
    if r.get('price_updated_at'):
        ts = datetime.fromisoformat(r['price_updated_at'].replace('Z', '+00:00'))
        age = (datetime.now(timezone.utc) - ts).total_seconds() / 60
        print(f\"{r['horse_name']}: {int(age)} minutes ago\")
"
```

---

## 📊 Summary

| What | Where | Format |
|------|-------|--------|
| **Field name** | `price_updated_at` | String (ISO 8601) |
| **API endpoint** | `/api/v1/races/{id}` | JSON response |
| **Updated by** | `update_live_prices` | Every 30 mins |
| **Null means** | Never updated | Use BSP/cached |
| **Use for** | Display freshness | "2 mins ago" |

---

**Last Updated:** October 17, 2025  
**Status:** ✅ Live in Production  
**API Version:** v1

