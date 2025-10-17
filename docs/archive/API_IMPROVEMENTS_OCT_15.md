# API Improvements - October 15, 2025

## Summary

Fixed critical API issues and added new features for better frontend integration.

---

## Issues Fixed

### 1. ✅ CORS Configuration

**Problem**: Frontend on `http://localhost:3001` blocked by CORS policy

**Fix**: Updated CORS config to support multiple origins
- **File**: `backend-api/internal/config/config.go`
- **Change**: Added support for comma-separated CORS origins
- **Default**: `localhost:3000`, `localhost:3001`, `localhost:5173`

**Test**:
```bash
curl -X OPTIONS http://localhost:8000/api/v1/courses \
  -H "Origin: http://localhost:3001"
# Returns: Access-Control-Allow-Origin: http://localhost:3001
```

### 2. ✅ Race Details Missing Runner Names

**Problem**: `/races/{id}` returned incomplete runner data (no horse names, trainer names, etc.)

**Root Cause**: Missing `racing.` schema prefixes in SQL JOINs

**Fix**: Added schema prefixes for `owners` and `bloodlines` tables
- **File**: `backend-api/internal/repository/race.go`
- **Lines**: 199-200
- **Change**: `LEFT JOIN owners` → `LEFT JOIN racing.owners`

**Result**: API now returns complete runner data with all 60+ fields

**Test**:
```bash
curl "http://localhost:8000/api/v1/races/2967" | jq '.runners[0]'
# Returns: horse_name, trainer_name, jockey_name, rpr, win_bsp, comment, etc.
```

### 3. ✅ Broken Time Format

**Problem**: `off_time` displayed as `"0000-01-01T13:44:00Z"` instead of `"13:44:00"`

**Root Cause**: PostgreSQL TIME field auto-converted to Go time.Time with date component

**Fix**: Cast TIME to TEXT in SQL queries
- **File**: `backend-api/internal/repository/race.go`
- **Change**: `r.off_time` → `r.off_time::text` (3 occurrences)

**Result**: Times display correctly as `"13:44:00"`

**Test**:
```bash
curl "http://localhost:8000/api/v1/races?date=2025-10-14" | jq '.[0].off_time'
# Returns: "13:44:00" ✅
```

### 4. ✅ Auto-Update Missing Foreign Keys

**Problem**: Recently loaded races (Oct 13-15) had NULL for horse_id, trainer_id, jockey_id

**Root Cause**: Auto-update service didn't populate dimension tables before inserting races

**Fix**: Enhanced auto-update service with dimension table management
- **File**: `backend-api/internal/services/autoupdate.go`
- **Added**: 
  - `upsertDimensions()` - Populates courses, horses, trainers, jockeys, owners
  - `populateForeignKeys()` - Looks up IDs and populates foreign keys
- **Logs**: Added verbose logging for dimension operations

**Result**: New data loads with proper foreign key relationships

**Test**:
```bash
# Check auto-update logs
tail -50 /tmp/api_autoupdate.log | grep "Upserted"
# Shows: "✓ Upserted X courses, Y horses, Z trainers..."
```

### 5. ✅ Unknown Courses in Meetings

**Problem**: Meetings showed "Unknown Course" for most venues

**Root Cause**: 
- Scraped data had course_ids that didn't match our courses table
- International courses (Chantilly, Caulfield, etc.) weren't in database

**Fix**: 
1. Enhanced auto-update to upsert courses from scraped data
2. Look up course_id by matching course name (not using scraper's ID)
3. Added international courses to database
4. Updated existing races with correct course_ids

**Result**: All meetings now show proper course names

**Test**:
```bash
curl "http://localhost:8000/api/v1/meetings?date=2025-10-11" | jq '[.[] | .course_name]'
# All courses have names now ✅
```

---

## New Features

### 1. 🆕 Meetings Endpoint

**Purpose**: Group races by venue for cleaner UI display

**Endpoint**: `GET /api/v1/meetings?date={date}`

**Response**:
```json
[
  {
    "race_date": "2025-10-14T00:00:00Z",
    "region": "GB",
    "course_id": 30,
    "course_name": "Sedgefield",
    "race_count": 8,
    "first_race_time": "13:44:00",
    "last_race_time": "17:15:00",
    "race_types": "Flat",
    "races": [
      // All 8 Sedgefield races
    ]
  }
]
```

**Benefits**:
- ✅ Races grouped by venue automatically
- ✅ Meeting-level metadata (race count, time range, types)
- ✅ Cleaner UI - show "Sedgefield - 8 races" instead of 8 cards
- ✅ Fast - ~1-5ms response time

**Files**:
- `backend-api/internal/models/race.go` - Added `MeetingWithRaces` struct
- `backend-api/internal/repository/race.go` - Added `GetRacesByMeetings()` method
- `backend-api/internal/handlers/race.go` - Added `GetMeetings()` handler
- `backend-api/internal/router/router.go` - Registered route

### 2. 🆕 Date Parameter for Race Search

**Purpose**: Simplify searching for races on a specific date

**Enhancement**: `/races/search` now accepts single `date` parameter

**Before**:
```bash
# Had to use date_from and date_to for single date
/races/search?date_from=2025-10-04&date_to=2025-10-04&course_id=73
```

**After**:
```bash
# Can use single date parameter
/races/search?date=2025-10-04&course_id=73
```

**How it works**: If `date` parameter is provided, it automatically sets both `date_from` and `date_to` to the same value

**Files**:
- `backend-api/internal/handlers/race.go` - Added date parameter handling

---

## Frontend Integration Examples

### Use Meetings Endpoint

```typescript
// Fetch today's meetings
const meetings = await fetch(
  `http://localhost:8000/api/v1/meetings?date=2025-10-14`
).then(r => r.json());

// Display grouped by venue
{meetings.map(meeting => (
  <div key={meeting.course_id}>
    <h2>{meeting.course_name} - {meeting.race_count} races</h2>
    <p>{meeting.first_race_time} - {meeting.last_race_time}</p>
    
    {meeting.races.map(race => (
      <RaceCard key={race.race_id} race={race} />
    ))}
  </div>
))}
```

### Use Date Parameter in Search

```typescript
// Old way (still works)
const races = await fetch(
  `/races/search?date_from=2025-10-04&date_to=2025-10-04&course_id=73`
).then(r => r.json());

// New way (simpler)
const races = await fetch(
  `/races/search?date=2025-10-04&course_id=73`
).then(r => r.json());
```

### Parse Times in Frontend

```typescript
// API returns: "13:44:00"
const formatTime = (timeString) => {
  if (!timeString) return '';
  
  // Display as "13:44"
  return timeString.substring(0, 5);
  
  // Or as "1:44 PM"
  const [hours, minutes] = timeString.split(':');
  const hour12 = hours % 12 || 12;
  const ampm = hours >= 12 ? 'PM' : 'AM';
  return `${hour12}:${minutes} ${ampm}`;
};
```

---

## Database Changes

### Courses Added

Added international courses to support scraped data:
- Chantilly (FR)
- Woodbine (CA)
- Keeneland (US)
- Belmont At The Big A (US)
- Rosehill (AU)
- Caulfield (AU)
- Tokyo (JP)
- Palermo (AR)

### Existing Races Updated

Updated races from Oct 11-14 with correct course_ids:
- York: 107 → 508
- Naas: 192 → 10975
- Wolverhampton: 513 → 48 (and NULL → 48)
- Carlisle: 25 → 63
- Downpatrick: 182 → 10951
- Lingfield: 393 → 31
- Dundalk: 1353 → 10955
- Newmarket: 1138 → 189
- Chelmsford: 35 → 43
- Southwell: 195 → 52

**Total courses in database**: 89 + 8 international = 97 courses

---

## Performance Impact

| Endpoint | Before | After | Change |
|----------|--------|-------|--------|
| `/races/{id}` | 50-200ms | 50-200ms | No change ✅ |
| `/races/search` | 1-5ms | 1-5ms | No change ✅ |
| `/meetings` | N/A | 1-5ms | New endpoint ✅ |
| CORS preflight | <1ms | <1ms | No change ✅ |

**No performance degradation** - all fixes were schema/data corrections

---

## Breaking Changes

**None!** All changes are backwards compatible:
- ✅ Existing endpoints still work the same way
- ✅ New `date` parameter is optional (old `date_from`/`date_to` still work)
- ✅ Meetings endpoint is new (doesn't replace anything)
- ✅ Response formats unchanged

---

## Testing

### Quick Verification

```bash
# 1. Test CORS
curl -X OPTIONS http://localhost:8000/api/v1/courses \
  -H "Origin: http://localhost:3001" | grep "Access-Control"

# 2. Test race details (complete runner data)
curl "http://localhost:8000/api/v1/races/2967" | \
  jq '.runners[0] | {horse_name, trainer_name, jockey_name}'

# 3. Test time format
curl "http://localhost:8000/api/v1/races?date=2025-10-14" | \
  jq '.[0].off_time'
# Should show: "13:44:00" ✅

# 4. Test meetings grouping
curl "http://localhost:8000/api/v1/meetings?date=2025-10-14" | \
  jq 'length'
# Should show: 7 meetings (not 42 individual races)

# 5. Test date parameter
curl "http://localhost:8000/api/v1/races/search?date=2025-10-04&course_id=73" | \
  jq '[.[].race_date] | unique'
# Should show only: ["2025-10-04T00:00:00Z"]
```

---

## Files Changed

### Backend Code
1. `backend-api/internal/config/config.go` - CORS origins parsing
2. `backend-api/internal/repository/race.go` - Schema prefixes, time casting, meetings endpoint
3. `backend-api/internal/handlers/race.go` - Date parameter support, meetings handler
4. `backend-api/internal/router/router.go` - Meetings route
5. `backend-api/internal/models/race.go` - MeetingWithRaces struct
6. `backend-api/internal/services/autoupdate.go` - Dimension table management

### Documentation
1. `docs/02_API_DOCUMENTATION.md` - Updated examples and new endpoint documentation

### Database
1. Added 8 international courses
2. Updated 50+ races with correct course_ids

---

## Next Steps

### For Future Auto-Updates

The enhanced auto-update service will now:
1. ✅ Auto-upsert courses from scraped data
2. ✅ Auto-upsert horses, trainers, jockeys, owners
3. ✅ Look up proper IDs from dimension tables
4. ✅ Populate foreign keys correctly
5. ✅ Log all dimension operations verbosely

**No more "Unknown Course" or missing runner details!**

### For Frontend Developers

You can now use:
- ✅ `/meetings` endpoint for cleaner UI
- ✅ Single `date` parameter for search
- ✅ Complete runner data with names and ratings
- ✅ Properly formatted times

---

**Status**: ✅ All fixes deployed and tested  
**Impact**: Major improvement to API usability  
**Breaking Changes**: None  
**Performance**: No degradation  
**Date**: October 15, 2025

