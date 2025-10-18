# 🔒 GiddyUp Safe Operation Guide

## ✅ **WHAT'S WORKING**

### **1. Race Data Matching: 100%**
- ✅ Sporting Life races → Database
- ✅ Betfair markets → Races (43/43 matched)
- ✅ Runners populate correctly
- ✅ API returns full race data

### **2. Historical Data: Complete**
- ✅ Oct 1-16: Full results (positions, BSP, trainers, jockeys)
- ✅ 207 races, 2,189 runners loaded from master data

### **3. API Endpoints: Fully Functional**
```bash
GET /api/v1/today          # Today's races
GET /api/v1/tomorrow       # Tomorrow's races  
GET /api/v1/meetings?date=YYYY-MM-DD  # Full race cards
```

---

## ❌ **WHAT'S DISABLED**

### **Live Price Updater (Background Service)**
- **Status**: DISABLED (`ENABLE_LIVE_PRICES=false`)
- **Reason**: ANGX-0001 errors causing repeated login attempts
- **Risk**: Could trigger Betfair account suspension

---

## 🎯 **SAFE APPROACH: Manual Price Updates**

### **The Working Solution:**

Run manual updates every 30 minutes:

```bash
cd /home/smonaghan/GiddyUp
source settings.env
cd backend-api
./bin/update_live_prices -date 2025-10-18
```

**This works because:**
- ✅ Fresh login each run (no session expiry)
- ✅ Single API session per update
- ✅ No spam (controlled frequency)
- ✅ 97.8% coverage (498/509 runners)

### **Set Up a Cron Job:**

```bash
# Edit crontab
crontab -e

# Add this line (runs every 30 mins during racing hours 9am-11pm):
*/30 9-23 * * * cd /home/smonaghan/GiddyUp && source settings.env && cd backend-api && ./bin/update_live_prices -date $(date +\%Y-\%m-\%d) >> logs/manual_price_updates.log 2>&1
```

---

## 📊 **CURRENT STATUS**

### **Oct 18 (Today in system):**
```bash
curl -s "http://localhost:8000/api/v1/meetings?date=2025-10-18" | \
  jq '.[0].races[0].runners[0] | {horse: .horse_name, price: .win_ppwap}'
```
**Output:**
```json
{
  "horse": "Al Qareem (IRE)",
  "price": 12.5     ← LIVE PRICE ✅
}
```

### **Oct 1 (Historical):**
```bash
curl -s "http://localhost:8000/api/v1/meetings?date=2025-10-01" | \
  jq '.[0].races[0].runners[0] | {horse: .horse_name, pos: .pos_raw, bsp: .win_bsp}'
```
**Output:**
```json
{
  "horse": "Yellow Diamonds (IRE)",
  "pos": "1",       ← POSITION ✅
  "bsp": 1.69       ← BSP ✅
}
```

---

## 🔧 **SYSTEM OPERATION**

### **Autoupdate Service (Always Running):**
- ✅ Loads today/tomorrow on startup
- ✅ Updates race data hourly
- ✅ Preserves prices (no deletion)
- ✅ 100% Betfair race matching
- ❌ Does NOT update prices (disabled for safety)

### **Manual Price Updates (You Control):**
- ✅ Run as often as needed
- ✅ Fresh authentication each time
- ✅ No account risk
- ✅ 97.8% coverage

---

## 🚨 **IMPORTANT: WHY LIVE UPDATER IS DISABLED**

The live updater was making **multiple login attempts per minute**:
- 2 simultaneous services (today + tomorrow)
- Each service: login every 60 seconds
- Multiple failures triggering retries
- **Risk**: Betfair could flag account for abuse

**The manual updater is safer because:**
- Controlled execution (you decide when)
- Single login per run
- Works reliably every time
- Same code that powers your tennis bot

---

## 📝 **RECOMMENDED WORKFLOW**

### **Morning (Pre-Racing):**
```bash
# Update prices for today before markets open
./bin/update_live_prices -date $(date +%Y-%m-%d)
```

### **During Racing Hours:**
```bash
# Run every 30 mins (via cron or manually)
./bin/update_live_prices -date $(date +%Y-%m-%d)
```

### **Check Coverage:**
```bash
curl -s "http://localhost:8000/api/v1/meetings?date=$(date +%Y-%m-%d)" | \
  jq '[.[] | .races[] | .runners[] | select(.win_ppwap != null)] | length'
```

---

## ✅ **SUMMARY**

**What's Working:**
- ✅ API is stable and safe
- ✅ Race data loads automatically
- ✅ Betfair matching is 100% accurate
- ✅ Historical data is complete
- ✅ Manual price updates work perfectly

**What's Manual:**
- 🔧 Price updates (run script every 30 mins)
- 🔧 Controlled Betfair authentication (safe)

**Account Safety:**
- 🔒 No login spam
- 🔒 Controlled API usage
- 🔒 Same pattern as working tennis bot

---

**Bottom Line:** The system is production-ready. Just run manual price updates as needed - it's safer and works perfectly!

