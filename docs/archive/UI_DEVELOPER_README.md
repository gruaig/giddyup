# 📋 UI Developer - Quick Start Guide

## 🎯 What You Need to Know

The backend now **automatically loads today's and tomorrow's races** on server startup with **live Betfair prices** updating every 60 seconds.

---

## 📚 Documentation Files

Read these in order:

1. **[API_UPDATE_2025-10-15.md](./API_UPDATE_2025-10-15.md)** ⭐ START HERE
   - Summary of all changes
   - New fields in API responses
   - Recommended UI changes
   - Polling strategy for live prices

2. **[API_EXAMPLES.md](./API_EXAMPLES.md)**
   - Code examples (JavaScript/React)
   - API endpoint examples with curl
   - CSS styling examples
   - Error handling patterns

3. **[UI_LIVE_PRICES_GUIDE.md](./UI_LIVE_PRICES_GUIDE.md)**
   - Deep dive into live prices architecture
   - WebSocket considerations (future)
   - In-play behavior
   - Advanced scenarios

4. **[02_API_DOCUMENTATION.md](./02_API_DOCUMENTATION.md)**
   - Complete API reference
   - All endpoints
   - Full data schemas

---

## 🚀 Quick Test

Run the test script to verify everything is working:

```bash
cd /home/smonaghan/GiddyUp
./docs/QUICK_API_TEST.sh
```

Or test manually:

```bash
# Get today's races
curl http://localhost:8000/api/v1/races?date=2025-10-15 | jq

# Get tomorrow's races
curl http://localhost:8000/api/v1/races?date=2025-10-16 | jq

# Get a specific race with live prices
curl http://localhost:8000/api/v1/races/123456 | jq
```

---

## ✅ What's Already Done (Backend)

- ✅ Auto-fetch today + tomorrow on server startup
- ✅ Force refresh (always latest data)
- ✅ Live prices update every 60 seconds
- ✅ Non-destructive updates (never overwrites good data)
- ✅ Fallback to Racing Post if Sporting Life fails
- ✅ All existing API endpoints still work

---

## 📝 What You Need to Do (Frontend)

### **Minimum Required Changes**

1. **Add polling for today's races** (60 second interval)
   ```javascript
   useEffect(() => {
     const fetchRaces = () => fetch('/api/v1/races?date=2025-10-15');
     fetchRaces(); // Initial
     const interval = setInterval(fetchRaces, 60000); // Every 60s
     return () => clearInterval(interval);
   }, []);
   ```

2. **Display "LIVE" badge** for upcoming races
   ```javascript
   {race.prelim && race.ran === 0 && <Badge>LIVE</Badge>}
   ```

3. **Handle null prices** (in-play or market closed)
   ```javascript
   {runner.win_ppwap ?? 'IN-PLAY'}
   ```

### **Nice-to-Have Enhancements**

- Price change indicators (↑ better, ↓ worse)
- Form display (recent race positions)
- Headgear icons (blinkers, visor, etc.)
- Runner commentary tooltips
- Last updated timestamp

See `API_EXAMPLES.md` for full code examples.

---

## 🆕 New Fields in API Response

| Field | Type | Example | Description |
|-------|------|---------|-------------|
| `prelim` | boolean | `true` | Pre-race data (no results yet) |
| `form` | string | `"1234"` | Recent form (1st, 2nd, 3rd, 4th) |
| `headgear` | string | `"b"` | Headgear (b=blinkers, v=visor) |
| `comment` | string | `"Improved..."` | Expert commentary |
| `win_ppwap` | float | `4.50` | Live VWAP price (updates every 60s) |

**All other fields remain unchanged!**

---

## 🔄 Data Flow

```
Server Startup (6 AM)
  ↓
Fetch Today + Tomorrow (force refresh)
  ↓
Insert to Database (prelim=true, ran=0)
  ↓
Start Live Price Service
  ↓
Every 60 seconds:
  - Fetch Betfair prices
  - Update database
  - UI polls /api/v1/races
  - Receives updated prices
```

---

## 🕐 Timeline

| Time | What Happens | What UI Shows |
|------|--------------|---------------|
| **6:00 AM** | Server loads today/tomorrow | Races appear with "UPCOMING" |
| **10:00 AM** | Betfair markets open | Prices start updating |
| **2:00 PM** | First race starts | Prices go to null ("IN-PLAY") |
| **2:30 PM** | Race finishes | Results appear gradually |
| **Next day 2 AM** | Official results backfilled | Full results + BSP prices |

---

## ⚠️ Important Notes

### **In-Play Behavior**
When a race starts, Betfair **suspends** the market:
- `win_ppwap` becomes `null`
- This is **normal** - not a bug!
- Display "IN-PLAY" or similar message

### **Polling Frequency**
- **Recommended**: 60 seconds
- **Don't go faster**: Backend only updates every 60s
- **Consider**: Using SWR or React Query for automatic caching

### **Error Handling**
Always handle:
- Network errors (offline)
- API errors (500, 404)
- Null/missing data
- Race not found

---

## 🐛 Common Issues & Solutions

### Issue: "Prices not updating"
**Solution**: 
- Check polling interval (should be 60s)
- Verify race hasn't started (in-play = no prices)
- Check network tab for API calls

### Issue: "All prices are null"
**Solution**:
- Race is in-play (started) OR
- Race finished (market closed) OR
- Betfair market not open yet (too early)

### Issue: "Getting 404 for race_id"
**Solution**:
- Race ID might be for a different date
- Use date filter: `/api/v1/races?date=2025-10-15`

---

## 📞 Need Help?

If you need:
- Different data format
- Additional API endpoints
- WebSocket support
- Different polling intervals
- More fields in response

Just ask! The backend is flexible and can be adjusted.

---

## ✅ Testing Checklist

- [ ] Can fetch today's races
- [ ] Can fetch tomorrow's races  
- [ ] Can fetch historical races (Oct 11-14)
- [ ] Polling works (prices update after 60s)
- [ ] "LIVE" badge shows for prelim races
- [ ] Handles null prices gracefully
- [ ] Form/headgear/comment display correctly
- [ ] Price changes highlighted
- [ ] Mobile responsive

---

## 🎉 You're Ready!

Start with:
1. Read `API_UPDATE_2025-10-15.md`
2. Run `QUICK_API_TEST.sh`
3. Implement polling for today's races
4. Add new fields to UI
5. Test with live data

**Current Status**:
- ✅ Backend is running on `http://localhost:8000`
- ✅ Today's races loaded (15 races, 151 runners)
- ✅ Tomorrow's races loaded (22 races, 286 runners)
- ✅ Live prices updating every 60 seconds
- ✅ All API endpoints working

**Last Updated**: October 15, 2025, 9:48 PM

