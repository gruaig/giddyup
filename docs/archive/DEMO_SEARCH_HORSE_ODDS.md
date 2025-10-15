# Demo: Search Horse & See Last 3 Runs with Odds

## Your Question:
> "Can I search for a horse called something and see its last 3 runs and the odds it was at?"

## Answer: YES! ✅

Here's how the API works:

---

## Step 1: Search for a Horse

```bash
curl "http://localhost:8000/api/v1/search?q=Captain%20Scooby&limit=5"
```

**Response:**
```json
{
  "horses": [
    {
      "id": 9643,
      "name": "Captain Scooby (GB)",
      "score": 0.83,
      "type": "horse"
    }
  ],
  "trainers": [],
  "jockeys": [],
  "owners": [],
  "courses": [],
  "total_results": 1
}
```

**Found Horse ID: 9643**

---

## Step 2: Get Horse Profile (includes last 20 runs with odds)

```bash
curl "http://localhost:8000/api/v1/horses/9643/profile"
```

**Response (showing last 3 runs):**
```json
{
  "horse": {
    "horse_id": 9643,
    "horse_name": "Captain Scooby (GB)"
  },
  "career_summary": {
    "runs": 195,
    "wins": 18,
    "places": 51,
    "total_prize": 84529.67,
    "avg_rpr": 55.57,
    "peak_rpr": 83
  },
  "recent_form": [
    {
      "race_date": "2018-01-11",
      "course_name": "Chelmsford (AW)",
      "race_name": "Bet toteWIN At betfred.com Handicap (Div II)",
      "race_type": "Flat",
      "going": "Standard",
      "dist_f": 6,
      "pos_num": 7,
      "pos_raw": "7",
      "btn": 0.3,
      "or": 46,
      "rpr": 40,
      "win_bsp": 19.00,          ← Betfair Starting Price
      "dec": 15.00,              ← Bookmaker SP
      "trainer_name": "Richard Guest",
      "jockey_name": "Ben Curtis",
      "days_since_run": 6
    },
    {
      "race_date": "2018-01-05",
      "course_name": "Southwell (AW)",
      "race_name": "Betway Best Odds Guaranteed Handicap",
      "race_type": "Flat",
      "going": "Standard",
      "dist_f": 6,
      "pos_num": 5,
      "pos_raw": "5",
      "btn": 1.5,
      "or": 45,
      "rpr": 34,
      "dec": 26.00,              ← Bookmaker SP only
      "trainer_name": "Richard Guest",
      "jockey_name": "Jason Hart",
      "days_since_run": 1
    },
    {
      "race_date": "2018-01-04",
      "course_name": "Chelmsford (AW)",
      "race_name": "Bet toteWIN At betfred.com Handicap",
      "race_type": "Flat",
      "going": "Standard",
      "dist_f": 5,
      "pos_num": 9,
      "pos_raw": "9",
      "btn": 3.25,
      "or": 46,
      "rpr": 26,
      "win_bsp": 25.10,          ← Betfair Starting Price
      "dec": 15.00,              ← Bookmaker SP
      "trainer_name": "Richard Guest",
      "jockey_name": "Georgia Cox",
      "days_since_run": 13
    }
  ],
  "going_splits": [...],
  "distance_splits": [...],
  "course_splits": [...]
}
```

---

## Information Available per Run

✅ **Race Details:**
- Date, course, race name
- Going, distance, race type

✅ **Position & Performance:**
- Finishing position (pos_num, pos_raw)
- Beaten distance (btn)
- Official Rating (OR)
- Racing Post Rating (RPR)

✅ **Odds (both types):**
- `win_bsp`: Betfair Starting Price (exchange)
- `dec`: Bookmaker Starting Price (decimal odds)

✅ **Connections:**
- Trainer name
- Jockey name
- Days since last run

✅ **Performance Splits:**
- Going performance (Heavy, Soft, Good, Firm, etc.)
- Distance performance (5-6f, 7-8f, 9-12f, 13f+)
- Course performance (top courses with strike rates)

---

## Full Data Points Available

The profile returns up to **20 most recent runs** with complete information.

Each run includes:
```
{
  "race_date": "YYYY-MM-DD",
  "course_name": "Course Name",
  "race_name": "Race Name",
  "race_type": "Flat/Hurdle/Chase",
  "going": "Going description",
  "dist_f": 6.0,                    // Distance in furlongs
  "pos_num": 7,                     // Numeric position
  "pos_raw": "7",                   // Raw position (could be "UR", "PU", etc.)
  "btn": 0.3,                       // Beaten distance (lengths)
  "or": 46,                         // Official Rating
  "rpr": 40,                        // Racing Post Rating
  "win_bsp": 19.00,                 // Betfair odds
  "dec": 15.00,                     // Bookmaker odds
  "trainer_name": "Trainer Name",
  "jockey_name": "Jockey Name",
  "days_since_run": 6               // Days since previous run
}
```

---

## Performance Stats Included

**Career Summary:**
- Total runs, wins, places
- Total prize money
- Average & peak RPR
- Average & peak OR

**Going Splits:**
```
Good To Soft: 35 runs, 7 wins (20% SR)
Soft: 33 runs, 6 wins (18.18% SR)
Standard: 62 runs, 3 wins (4.84% SR)
```

**Distance Splits:**
```
5-6f: 193 runs, 18 wins (9.33% SR, Avg RPR: 55.9)
7-8f: 2 runs, 0 wins (0% SR, Avg RPR: 24.5)
```

**Top Courses:**
```
Ayr: 8 runs, 2 wins (25% SR)
Nottingham: 10 runs, 2 wins (20% SR)
Chelmsford: 5 runs, 1 win (20% SR)
```

---

## Example API Calls

### Search by Name
```bash
# Exact match
curl "http://localhost:8000/api/v1/search?q=Enable"

# Partial match
curl "http://localhost:8000/api/v1/search?q=Frank"

# Fuzzy match (handles typos)
curl "http://localhost:8000/api/v1/search?q=Frankel"  # finds "Frankel (GB)"
```

### Get Profile
```bash
# By ID from search results
curl "http://localhost:8000/api/v1/horses/9643/profile"
```

---

## Summary

**YES!** You can:
1. Search for any horse by name (fuzzy matching with trigram similarity)
2. Get the horse ID from search results
3. Fetch complete profile including:
   - Last 20 runs (or specify limit)
   - Both Betfair (BSP) and Bookmaker (SP) odds for each run
   - Position, ratings, trainer, jockey
   - Days since previous run
   - Performance splits by going, distance, and course

The API provides **comprehensive racing analytics** with complete historical odds data from both Betfair and bookmakers!

---

**Note:** Profile queries currently take 30+ seconds due to complex joins. Performance optimization (indexes, caching) is next priority.

