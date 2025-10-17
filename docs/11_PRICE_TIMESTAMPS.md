# 11. Price Timestamps - Display Fresh Data

**Last Updated:** October 17, 2025  
**Feature:** Show users when Betfair prices were last updated

---

## 🎯 **What It Does**

Every time the price updater fetches fresh Betfair prices, it records a timestamp in `racing.runners.price_updated_at`.

The UI can display:
- "Prices updated 2 minutes ago" ✅
- "Last updated: 18:29" ✅
- "Stale prices (> 1 hour old)" ⚠️

---

## 📊 **Database Schema**

### **Column Added:**

```sql
ALTER TABLE racing.runners 
ADD COLUMN price_updated_at TIMESTAMP DEFAULT NULL;
```

**Type:** `TIMESTAMP WITHOUT TIME ZONE`  
**Default:** `NULL` (means never updated or using historical BSP)  
**Set by:** Price updater when updating `win_ppwap`

---

## 🔄 **How It Works**

### **Price Updater:**

```go
// When updating prices
UPDATE racing.runners
SET 
    win_ppwap = 5.30,
    price_updated_at = NOW()  ← Sets current timestamp
WHERE betfair_selection_id = 12345678;
```

### **Every 30 Minutes:**
- Price updater logs in to Betfair
- Fetches latest market prices
- Updates `win_ppwap` AND `price_updated_at`
- Timestamp shows exact moment of last update

---

## 📡 **API Response**

### **Before:**
```json
{
  "horse_name": "Galileo Blue",
  "win_ppwap": 5.60
}
```

### **After:**
```json
{
  "horse_name": "Galileo Blue",
  "win_ppwap": 5.60,
  "price_updated_at": "2025-10-18T18:29:54Z"
}
```

---

## 🎨 **UI Implementation**

### **React Example:**

```tsx
import { formatDistanceToNow } from 'date-fns';

function PriceDisplay({ runner }: { runner: Runner }) {
  const { win_ppwap, price_updated_at } = runner;
  
  if (!win_ppwap) {
    return <span>SP</span>;
  }
  
  // Calculate age of price
  const ageMinutes = price_updated_at 
    ? (Date.now() - new Date(price_updated_at).getTime()) / 60000
    : null;
  
  const isFresh = ageMinutes && ageMinutes < 5;
  const isStale = ageMinutes && ageMinutes > 60;
  
  return (
    <div className="price-container">
      <div className={`price ${isFresh ? 'fresh' : isStale ? 'stale' : ''}`}>
        {win_ppwap.toFixed(2)}
      </div>
      
      {price_updated_at && (
        <div className="price-age">
          {formatDistanceToNow(new Date(price_updated_at), { addSuffix: true })}
        </div>
      )}
    </div>
  );
}
```

### **CSS Styling:**

```css
.price.fresh {
  color: #22c55e; /* Green - recently updated */
  animation: pulse 1s ease-in-out;
}

.price.stale {
  color: #ef4444; /* Red - old price */
  opacity: 0.7;
}

.price-age {
  font-size: 0.75rem;
  color: #6b7280;
  margin-top: 2px;
}
```

---

## ⏰ **Freshness Indicators**

### **Recommended Thresholds:**

| Age | Status | Display | Color |
|-----|--------|---------|-------|
| < 5 min | ✅ Fresh | "Just now" | Green |
| 5-15 min | ✅ Current | "5 mins ago" | Normal |
| 15-60 min | ⚠️ Aging | "30 mins ago" | Orange |
| > 60 min | ❌ Stale | "1 hour ago" | Red |

### **Example Display:**

```
Runner: Galileo Blue
Price: 5.60
Last updated: 2 minutes ago ✅
```

```
Runner: Old Horse
Price: 12.50
Last updated: 1 hour ago ⚠️ (Stale)
```

---

## 🔍 **SQL Queries**

### **Get Freshness:**

```sql
SELECT 
    h.horse_name,
    ru.win_ppwap,
    ru.price_updated_at,
    EXTRACT(EPOCH FROM (NOW() - ru.price_updated_at))/60 as minutes_old,
    CASE 
        WHEN price_updated_at IS NULL THEN 'Never updated'
        WHEN EXTRACT(EPOCH FROM (NOW() - price_updated_at)) < 300 THEN 'Fresh'
        WHEN EXTRACT(EPOCH FROM (NOW() - price_updated_at)) < 3600 THEN 'Current'
        ELSE 'Stale'
    END as freshness
FROM racing.runners ru
LEFT JOIN racing.horses h ON h.horse_id = ru.horse_id
WHERE ru.race_date = '2025-10-18'
AND ru.win_ppwap IS NOT NULL
ORDER BY ru.price_updated_at DESC NULLS LAST;
```

### **Find Stale Prices:**

```sql
-- Prices older than 1 hour
SELECT COUNT(*) as stale_prices
FROM racing.runners
WHERE race_date = CURRENT_DATE
AND price_updated_at < NOW() - INTERVAL '1 hour';
```

---

## 🚨 **Alert Logic**

### **Backend Check:**

```go
// Check if prices need refresh
func (s *Service) CheckPriceFreshness(date string) (bool, error) {
    var oldestUpdate time.Time
    err := s.db.QueryRow(`
        SELECT MIN(price_updated_at)
        FROM racing.runners ru
        JOIN racing.races r ON r.race_id = ru.race_id
        WHERE r.race_date = $1
        AND price_updated_at IS NOT NULL
    `, date).Scan(&oldestUpdate)
    
    if err != nil {
        return false, err
    }
    
    age := time.Since(oldestUpdate)
    return age > 1*time.Hour, nil // Stale if > 1 hour old
}
```

### **Frontend Alert:**

```tsx
function PriceAlert({ runners }: { runners: Runner[] }) {
  const oldestPrice = runners
    .filter(r => r.price_updated_at)
    .map(r => new Date(r.price_updated_at!))
    .sort((a, b) => a.getTime() - b.getTime())[0];
  
  if (!oldestPrice) return null;
  
  const minutesOld = (Date.now() - oldestPrice.getTime()) / 60000;
  
  if (minutesOld > 60) {
    return (
      <Alert variant="warning">
        ⚠️ Prices are {Math.floor(minutesOld / 60)} hours old.
        Refresh recommended.
      </Alert>
    );
  }
  
  return (
    <div className="price-freshness">
      ✅ Prices updated {Math.floor(minutesOld)} minutes ago
    </div>
  );
}
```

---

## 📋 **Use Cases**

### **1. Live Race Card**

```tsx
<RaceCard>
  <Header>
    <Title>Catterick 14:30</Title>
    <PriceFreshness>
      Prices updated 3 mins ago ✅
    </PriceFreshness>
  </Header>
  
  {runners.map(runner => (
    <RunnerRow key={runner.runner_id}>
      <Horse>{runner.horse_name}</Horse>
      <Price value={runner.win_ppwap} 
             lastUpdated={runner.price_updated_at} />
    </RunnerRow>
  ))}
</RaceCard>
```

### **2. Betting Decision Page**

```tsx
function BettingPage() {
  const { price_updated_at } = runner;
  const ageMinutes = getAgeInMinutes(price_updated_at);
  
  return (
    <>
      <PriceBox>
        <span className="odds">{win_ppwap}</span>
        {ageMinutes < 5 && <Badge variant="success">LIVE</Badge>}
        {ageMinutes > 30 && <Badge variant="warning">Updating...</Badge>}
      </PriceBox>
      
      <Timestamp>
        Last updated: {formatRelative(price_updated_at)}
      </Timestamp>
    </>
  );
}
```

### **3. Historical View**

For finished races:
```tsx
// If price_updated_at is NULL, price is from BSP (historical)
if (!price_updated_at) {
  return <span title="Historical BSP">BSP: {win_bsp}</span>;
}

// If updated during race day, show time
return (
  <span title={`Updated ${price_updated_at}`}>
    {win_ppwap} (as of {format(price_updated_at, 'HH:mm')})
  </span>
);
```

---

## 🧪 **Testing**

### **Verify Timestamps Updating:**

```sql
-- Check that timestamps are being set
SELECT 
    COUNT(*) as total_with_prices,
    COUNT(*) FILTER (WHERE price_updated_at IS NOT NULL) as with_timestamp,
    MAX(price_updated_at) as most_recent_update,
    EXTRACT(EPOCH FROM (NOW() - MAX(price_updated_at)))/60 as minutes_since_update
FROM racing.runners
WHERE race_date = '2025-10-18'
AND win_ppwap IS NOT NULL;
```

**Expected:**
- `with_timestamp` should equal `total_with_prices`
- `minutes_since_update` should be < 30 (if updater running)

---

## 📊 **Sample Data**

```
Horse: Bollin Neil (GB)
Price: 3.00
Updated: 2025-10-17T19:28:18Z
Age: 2 minutes ago ✅

Horse: Galileo Blue  
Price: 5.60
Updated: 2025-10-17T19:28:18Z
Age: 2 minutes ago ✅
```

---

## 💡 **Best Practices**

### **1. Always Show Age for Live Prices**

```tsx
{price_updated_at ? (
  <TimeAgo date={price_updated_at} />
) : (
  <span>Historical price</span>
)}
```

### **2. Warn on Stale Prices**

```tsx
const age = getAgeInMinutes(price_updated_at);

if (age > 60) {
  return <Warning>Price may be outdated</Warning>;
}
```

### **3. Differentiate Live vs Historical**

```tsx
// Live prices (today/tomorrow)
if (race_date >= today && price_updated_at) {
  return `LIVE (${formatAge(price_updated_at)})`;
}

// Historical prices (BSP)
if (win_bsp && !price_updated_at) {
  return `BSP (final)`;
}
```

---

## 🚀 **Summary**

**Added:**
- ✅ `price_updated_at` column to `racing.runners`
- ✅ Price updater sets timestamp on every update
- ✅ API returns timestamp in responses
- ✅ UI can display "Last updated: X mins ago"

**Benefits:**
- Users see price freshness
- Can identify stale prices
- Builds trust in data quality
- Professional betting interface

---

**Last Updated:** October 17, 2025  
**Status:** ✅ Production Ready  
**Location:** `racing.runners.price_updated_at`

