# Sporting Life ↔ Betfair Matching Logic

**Problem**: fetch_all reports "Matched 0/51 races with Betfair data"

This document explains the complete matching logic to help debug the issue.

---

## 🔍 Overview

The matching process combines two data sources:
1. **Sporting Life API** - Race cards with jockey/trainer/owner info
2. **Betfair CSV Stitched Data** - Historical prices (BSP, PPWAP, etc.)

**Goal**: Match races and runners to merge the data into a complete dataset.

---

## 📊 Data Structures

### Sporting Life Race (from API)
```go
type Race struct {
    Date     string    // "2024-10-15"
    Region   string    // "GB" or "IRE"
    Course   string    // "Ascot" (human-readable name)
    RaceName string    // "British Stallion Studs EBF Maiden Stakes"
    OffTime  string    // "12:35:00" (HH:MM:SS format)
    Type     string    // "Flat", "Hurdle", "Chase"
    Runners  []Runner  // Array of runners
}
```

### Betfair Stitched Race (from CSV)
```go
type StitchedRace struct {
    Date      string    // "2024-10-15"
    EventName string    // "3m Hcap Hrd" or similar (abbreviated!)
    OffTime   string    // "12:35" (HH:MM format - NO SECONDS!)
    Venue     string    // "Ascot" (normalized course name)
    Runners   []StitchedRunner
}
```

**⚠️ KEY DIFFERENCES:**
1. **Time format**: SL has seconds ("12:35:00"), BF has minutes only ("12:35")
2. **Race name**: SL is full name, BF is abbreviated
3. **Field names**: SL uses `RaceName`, BF uses `EventName`

---

## 🔑 Matching Algorithm (in fetch_all)

### Step 1: Build Betfair Lookup Map

**Location**: `cmd/fetch_all/main.go` - `matchAndMerge()` function

```go
func matchAndMerge(slRaces []scraper.Race, bfRaces []scraper.StitchedRace) []scraper.Race {
    // Build Betfair lookup map
    bfMap := make(map[string]scraper.StitchedRace)
    
    for _, bfRace := range bfRaces {
        normName := scraper.NormalizeName(bfRace.EventName)  // ← Normalize race name
        normTime := normalizeTime(bfRace.OffTime)            // ← Keep as HH:MM
        
        // KEY FORMAT: "DATE|NORMALIZED_NAME|TIME"
        key := fmt.Sprintf("%s|%s|%s", bfRace.Date, normName, normTime)
        bfMap[key] = bfRace
    }
    
    // ...
}
```

**Example Betfair Key**:
```
"2024-10-15|3mhcaphrd|12:35"
```

### Step 2: Match Sporting Life Races

```go
for i := range slRaces {
    race := &slRaces[i]
    
    normName := scraper.NormalizeName(race.RaceName)  // ← Normalize race name
    normTime := normalizeTime(race.OffTime)           // ← Strip seconds!
    
    // KEY FORMAT: "DATE|NORMALIZED_NAME|TIME"
    key := fmt.Sprintf("%s|%s|%s", race.Date, normName, normTime)
    
    bfRace, found := bfMap[key]
    if !found {
        continue  // ← No match, skip this race
    }
    
    // Match found! Now merge runners...
}
```

**Example Sporting Life Key**:
```
"2024-10-15|britishstallionstudsebfmaidenstakes|12:35"
```

---

## 🧩 Key Normalization

### Function: `scraper.NormalizeName()`

**Location**: `internal/scraper/normalize.go`

```go
func NormalizeName(s string) string {
    // 1. Lowercase
    s = strings.ToLower(s)
    
    // 2. Remove special characters
    s = regexp.MustCompile(`[^a-z0-9\s]`).ReplaceAllString(s, "")
    
    // 3. Remove extra spaces
    s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
    
    // 4. Trim
    s = strings.TrimSpace(s)
    
    // 5. Remove all spaces
    s = strings.ReplaceAll(s, " ", "")
    
    return s
}
```

**Examples**:
- `"British Stallion Studs EBF Maiden Stakes"` → `"britishstallionstudsebfmaidenstakes"`
- `"3m Hcap Hrd"` → `"3mhcaphrd"`

---

## 🕒 Time Normalization

### Function: `normalizeTime()`

**Location**: `cmd/fetch_all/main.go`

```go
func normalizeTime(t string) string {
    // Normalize time to HH:MM format
    if len(t) >= 5 {
        return t[:5] // "12:35:00" -> "12:35"
    }
    return t
}
```

**Examples**:
- `"12:35:00"` → `"12:35"` ✅
- `"12:35"` → `"12:35"` ✅
- `"9:15:00"` → `"9:15"` ⚠️ (might need "09:15")

---

## 🐴 Runner Matching

Once races are matched, runners are matched by **horse name**:

```go
// Build Betfair runner map
bfRunnerMap := make(map[string]scraper.StitchedRunner)
for _, bfRunner := range bfRace.Runners {
    normHorse := scraper.NormalizeName(bfRunner.Horse)
    bfRunnerMap[normHorse] = bfRunner
}

// Merge into Sporting Life runners
for j := range race.Runners {
    runner := &race.Runners[j]
    normHorse := scraper.NormalizeName(runner.Horse)
    
    if bfRunner, found := bfRunnerMap[normHorse]; found {
        // Copy Betfair prices
        runner.WinBSP = parseFloat(bfRunner.WinBSP)
        runner.WinPPWAP = parseFloat(bfRunner.WinPPWAP)
        // ... etc
    }
}
```

---

## 🐛 Why Matching Might Fail

### Issue 1: Different Race Names ❌

**Sporting Life**:
```
"British Stallion Studs EBF Maiden Stakes (GBB Race)"
```

**Betfair**:
```
"Brit St Studs EBF Mdn Stks"
```

**Normalized**:
- SL: `"britishstallionstudsebfmaidenstakesgbbrace"`
- BF: `"britstsudsebfmdnstks"`

**Result**: NO MATCH ❌

### Issue 2: Time Padding ⏰

**Sporting Life**: `"9:15:00"`  
**Betfair**: `"09:15"`

After normalization:
- SL: `"9:15"`
- BF: `"09:15"`

**Result**: NO MATCH ❌

### Issue 3: Course Name Mismatch

**Sporting Life**: Uses `Course` field from API  
**Betfair**: Uses `Venue` field from CSV

Not currently used in matching key, but could cause issues if we add it.

### Issue 4: Missing Betfair Data

- Not all dates have Betfair CSV files
- CSV files might be incomplete
- Region mismatch (UK vs GB)

### Issue 5: Date Format Differences

**Both should be YYYY-MM-DD**, but verify!

---

## 🔬 Debugging Steps

### 1. Check What Keys Are Being Generated

Add logging to `matchAndMerge()`:

```go
// Log Betfair keys
for key := range bfMap {
    log.Printf("  BF Key: %s", key)
}

// Log Sporting Life keys
for i := range slRaces {
    race := &slRaces[i]
    normName := scraper.NormalizeName(race.RaceName)
    normTime := normalizeTime(race.OffTime)
    key := fmt.Sprintf("%s|%s|%s", race.Date, normName, normTime)
    log.Printf("  SL Key: %s", key)
}
```

### 2. Check Betfair CSV Files Exist

```bash
ls -la /home/smonaghan/GiddyUp/data/betfair_stitched/uk/win/2024-10-15.csv
ls -la /home/smonaghan/GiddyUp/data/betfair_stitched/ire/win/2024-10-15.csv
```

### 3. Inspect Betfair Stitched Data

```bash
head -20 /home/smonaghan/GiddyUp/data/betfair_stitched/uk/win/2024-10-15.csv
```

Check:
- Date format
- Event name format
- Time format
- Venue names

### 4. Check Betfair Stitcher Output

Look at the stitched data structure:

```go
type StitchedRace struct {
    Date      string
    EventName string  // ← Is this populated?
    OffTime   string  // ← Is this in HH:MM format?
    Venue     string
    Runners   []StitchedRunner
}
```

### 5. Verify Time Padding

Check if times need zero-padding:
```go
func normalizeTime(t string) string {
    if len(t) >= 5 {
        t = t[:5] // "12:35:00" -> "12:35"
    }
    
    // Add zero-padding if needed
    parts := strings.Split(t, ":")
    if len(parts) == 2 {
        h, _ := strconv.Atoi(parts[0])
        m, _ := strconv.Atoi(parts[1])
        return fmt.Sprintf("%02d:%02d", h, m)
    }
    
    return t
}
```

---

## 🔧 Potential Fixes

### Fix 1: Use Course + Time Instead of Race Name

Race names vary too much. Use more stable identifiers:

```go
// Instead of: DATE|RACENAME|TIME
// Use: DATE|COURSE|TIME

key := fmt.Sprintf("%s|%s|%s", 
    bfRace.Date, 
    scraper.NormalizeName(bfRace.Venue),  // Course name
    normTime,
)
```

### Fix 2: Add Time Tolerance

Allow ±1 minute matching:

```go
// Try exact time first
if bfRace, found := bfMap[key]; found {
    // Matched!
}

// Try ±1 minute
for offset := -1; offset <= 1; offset++ {
    // Parse time, add offset, try again
}
```

### Fix 3: Better Race Name Normalization

Strip common words that differ:
```go
func normalizeRaceName(s string) string {
    // Remove common prefixes/suffixes
    replacements := map[string]string{
        "handicap": "hcap",
        "stakes": "stks",
        "maiden": "mdn",
        "hurdle": "hrd",
        "chase": "chs",
        // ... etc
    }
    
    for old, new := range replacements {
        s = strings.ReplaceAll(s, old, new)
    }
    
    return scraper.NormalizeName(s)
}
```

### Fix 4: Use Betfair Selection ID

If Sporting Life provides `betfair_selection_id`, use that for matching runners directly:

```go
// Instead of matching by horse name
if bfRunner, found := bfRunnerMap[runner.BetfairSelectionID]; found {
    // Perfect match!
}
```

---

## 📝 Current Matching Flow

```
┌─────────────────────┐
│  Sporting Life API  │
│  51 races           │
└──────────┬──────────┘
           │
           │ For each race:
           │ • Normalize race name
           │ • Normalize time (strip seconds)
           │ • Build key: DATE|NAME|TIME
           │
           v
    ┌──────────────┐
    │  Lookup Key  │
    │              │
    │ "2024-10-15| │
    │  british...| │
    │  12:35"      │
    └──────┬───────┘
           │
           │ Check if exists in...
           v
┌─────────────────────┐
│  Betfair Map        │
│  (from CSV files)   │
│                     │
│  Key: DATE|NAME|TIME│
│  Val: StitchedRace  │
└──────────┬──────────┘
           │
           ├─→ FOUND? ✅ Merge runners
           │
           └─→ NOT FOUND? ❌ Skip race
```

---

## 🚨 Most Likely Issue

**The race names don't match after normalization!**

Sporting Life uses full formal names:
- `"British Stallion Studs EBF Maiden Stakes (GBB Race)"`

Betfair uses abbreviated names:
- `"Brit St Studs EBF Mdn Stks"`

After normalization, these are completely different strings.

**Solution**: Use **Course + Time** matching instead of **Race Name + Time**.

---

## 🛠️ Quick Fix to Test

Add debug logging to see what's happening:

```go
// In cmd/fetch_all/main.go - matchAndMerge()

log.Println("\n=== BETFAIR KEYS ===")
for key := range bfMap {
    log.Printf("BF: %s", key)
}

log.Println("\n=== SPORTING LIFE KEYS ===")
for i := range slRaces {
    race := &slRaces[i]
    normName := scraper.NormalizeName(race.RaceName)
    normTime := normalizeTime(race.OffTime)
    key := fmt.Sprintf("%s|%s|%s", race.Date, normName, normTime)
    log.Printf("SL: %s (course: %s, time: %s)", key, race.Course, race.OffTime)
}
```

This will show you exactly what keys are being compared and why they're not matching.

---

## 📖 Related Files

- `cmd/fetch_all/main.go` - Main matching logic (line 145+)
- `internal/scraper/normalize.go` - Name normalization
- `internal/scraper/betfair_stitcher.go` - Betfair CSV processing
- `internal/scraper/models.go` - Data structures

---

## ✅ Recommended Solution

**Switch to Course + Time matching** instead of Race Name + Time:

```go
// Betfair key
key := fmt.Sprintf("%s|%s|%s", 
    bfRace.Date,
    scraper.NormalizeName(bfRace.Venue),  // Course name
    normTime,
)

// Sporting Life key  
key := fmt.Sprintf("%s|%s|%s",
    race.Date,
    scraper.NormalizeName(race.Course),   // Course name
    normTime,
)
```

Course names are more stable than race names!

---

**Created**: October 16, 2025  
**Issue**: 0/51 races matched  
**Root Cause**: Race name normalization mismatch  
**Fix**: Use course + time instead of race name + time

