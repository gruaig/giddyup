# 12. Today/Tomorrow Convenience Endpoints

**Added:** October 17, 2025  
**Purpose:** Simplified frontend integration with automatic date calculation

---

## 🎯 **What They Do**

Instead of the frontend calculating today/tomorrow dates and calling `/api/v1/meetings?date=YYYY-MM-DD`, you can now use:

- **`GET /api/v1/today`** → Automatically returns today's meetings
- **`GET /api/v1/tomorrow`** → Automatically returns tomorrow's meetings

The backend calculates the dates using server time (UTC).

---

## 📡 **API Endpoints**

### **1. Today's Meetings**

```bash
GET http://localhost:8000/api/v1/today
```

**Response:** Same format as `/api/v1/meetings?date=2025-10-17`

```json
[
  {
    "course_name": "Catterick",
    "region": "GB",
    "races": [
      {
        "race_id": 811194,
        "race_name": "Go Racing In Yorkshire...",
        "off_time": "09:30:00",
        "runners": [
          {
            "horse_name": "Kitsune Power (IRE)",
            "win_ppwap": 6.6,
            "price_updated_at": "2025-10-17T20:35:40Z"
          }
        ]
      }
    ]
  }
]
```

---

### **2. Tomorrow's Meetings**

```bash
GET http://localhost:8000/api/v1/tomorrow
```

**Response:** Same format as `/api/v1/meetings?date=2025-10-18`

---

## 💻 **Frontend Usage**

### **Before (Manual Date Calculation):**

```typescript
// Frontend had to calculate dates
const today = new Date().toISOString().split('T')[0];
const response = await fetch(`/api/v1/meetings?date=${today}`);
```

### **After (Simpler):**

```typescript
// Backend calculates the date
const response = await fetch('/api/v1/today');
```

---

## 🔄 **Automatic Date Rollover**

At midnight (00:00:00 UTC), these endpoints automatically switch dates:

**Before midnight (23:59:59):**
- `/api/v1/today` → Returns Oct 17 races
- `/api/v1/tomorrow` → Returns Oct 18 races

**After midnight (00:00:01):**
- `/api/v1/today` → Returns Oct 18 races (auto-updated!)
- `/api/v1/tomorrow` → Returns Oct 19 races (auto-updated!)

**No frontend code changes needed!**

---

## 🎨 **React Component Example**

### **Simple Today's Races:**

```tsx
import { useEffect, useState } from 'react';

function TodaysRaces() {
  const [meetings, setMeetings] = useState([]);
  const [loading, setLoading] = useState(true);
  
  useEffect(() => {
    fetch('http://localhost:8000/api/v1/today')
      .then(res => res.json())
      .then(data => {
        setMeetings(data);
        setLoading(false);
      });
  }, []);
  
  if (loading) return <div>Loading...</div>;
  
  return (
    <div>
      <h1>Today's Racing</h1>
      {meetings.map(meeting => (
        <div key={meeting.course_name}>
          <h2>{meeting.course_name}</h2>
          <p>{meeting.races.length} races</p>
        </div>
      ))}
    </div>
  );
}
```

### **Tomorrow's Races:**

```tsx
function TomorrowsRaces() {
  const [meetings, setMeetings] = useState([]);
  
  useEffect(() => {
    fetch('http://localhost:8000/api/v1/tomorrow')
      .then(res => res.json())
      .then(setMeetings);
  }, []);
  
  return (
    <div>
      <h1>Tomorrow's Racing</h1>
      {meetings.map(meeting => (
        <MeetingCard key={meeting.course_name} meeting={meeting} />
      ))}
    </div>
  );
}
```

---

## 🧪 **Testing**

### **Command Line:**

```bash
# Today's races
curl http://localhost:8000/api/v1/today | jq '.[].course_name'

# Tomorrow's races  
curl http://localhost:8000/api/v1/tomorrow | jq '.[].course_name'
```

### **Verify Date Calculation:**

```bash
# Check what date the backend is using
curl -s http://localhost:8000/api/v1/today | \
  jq '.[0].races[0].race_date'
# Should return: "2025-10-17T00:00:00Z" (or current date)
```

---

## 📋 **Implementation Details**

### **Backend Logic:**

```go
func (h *RaceHandler) GetTodayMeetings(c *gin.Context) {
    today := time.Now().Format("2006-01-02")
    // Calls existing GetMeetings with today's date
}

func (h *RaceHandler) GetTomorrowMeetings(c *gin.Context) {
    tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
    // Calls existing GetMeetings with tomorrow's date
}
```

### **Timezone:**

- Uses **server timezone** (currently UTC)
- Consistent with autoupdate service
- Day rollover at midnight server time

---

## 🎁 **Benefits**

### **For Frontend:**
- ✅ No date calculation needed
- ✅ Simpler code
- ✅ Automatic date rollover
- ✅ Less code to maintain

### **For Backend:**
- ✅ Centralized date logic
- ✅ Consistent timezone handling
- ✅ Same date calculation as autoupdate
- ✅ Easy to add caching later

### **For Users:**
- ✅ Always see current day's races
- ✅ No stale data from wrong dates
- ✅ Seamless midnight transitions

---

## 🔗 **All Date-Based Endpoints**

| Endpoint | Purpose | Date Param |
|----------|---------|------------|
| `/api/v1/meetings?date=YYYY-MM-DD` | Specific date | Manual |
| `/api/v1/today` | Today's races | Auto |
| `/api/v1/tomorrow` | Tomorrow's races | Auto |
| `/api/v1/races?date=YYYY-MM-DD` | Races for date | Manual |

---

## 💡 **Frontend Migration**

### **Old Code:**

```typescript
const today = new Date().toISOString().split('T')[0];
const res = await fetch(`/api/v1/meetings?date=${today}`);
```

### **New Code:**

```typescript
const res = await fetch('/api/v1/today');
```

**Saves 1 line and eliminates timezone bugs!**

---

## 🚀 **Summary**

**Added:**
- ✅ `GET /api/v1/today` - Today's meetings
- ✅ `GET /api/v1/tomorrow` - Tomorrow's meetings

**Behavior:**
- ✅ Automatic date calculation
- ✅ Same response format as `/meetings`
- ✅ Midnight rollover handled

**Frontend Impact:**
- ✅ Simpler integration
- ✅ Less code
- ✅ More reliable

---

**Last Updated:** October 17, 2025  
**Status:** ✅ Production Ready  
**Endpoints:** `/api/v1/today`, `/api/v1/tomorrow`

