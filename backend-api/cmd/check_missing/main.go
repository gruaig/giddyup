package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

var (
	dbConnStr  = flag.String("db", "host=localhost port=5432 dbname=horse_db user=postgres password=password sslmode=disable", "Postgres conn string")
	searchPath = flag.String("search-path", "racing, public", "Postgres search_path")
	bfRoot     = flag.String("betfair-root", "/home/smonaghan/rpscrape/data/betfair_stitched", "betfair_stitched root")
	sinceStr   = flag.String("since", "2006-01-01", "YYYY-MM-DD (inclusive)")
	untilStr   = flag.String("until", "", "YYYY-MM-DD (inclusive; default=today)")
	dryRun     = flag.Bool("dry-run", true, "don’t trigger anything, just report")
	showSample = flag.Bool("show-sample", true, "print per-bucket samples")
	debugKeys  = flag.Bool("debug-keys", false, "print a few expected/loaded keys to diagnose mismatches")
	maxSamples = flag.Int("max-samples", 10, "dates to show per bucket")

	// NEW outputs:
	listMissingRaces = flag.Bool("list-missing-races", false, "Print missing races (date,region,type,HH:MM)")
	listMissingDays  = flag.Bool("list-missing-days", false, "Print missing dates (unique)")
)

// ---- Key types ----

type dayKey struct {
	Date     string // YYYY-MM-DD
	Region   string // GB|IRE
	RaceType string // Flat|Hurdle/Chase
}
type raceKey struct {
	Date     string
	Region   string
	RaceType string
	OffHHMM  string // HH:MM (24h)
}

func dayKeyString(d dayKey) string { return d.Date + "|" + d.Region + "|" + d.RaceType }
func raceKeyString(r raceKey) string {
	return r.Date + "|" + r.Region + "|" + r.RaceType + "|" + r.OffHHMM
}

func mustParseDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		log.Fatalf("bad date %q: %v", s, err)
	}
	return t
}

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	since := mustParseDate(*sinceStr)
	until := time.Now().Truncate(24 * time.Hour)
	if strings.TrimSpace(*untilStr) != "" {
		until = mustParseDate(*untilStr)
	}
	if until.Before(since) {
		log.Fatalf("until (%s) < since (%s)", until.Format("2006-01-02"), since.Format("2006-01-02"))
	}

	log.Printf("🔍 Scanning betfair_root=%s range=%s..%s", *bfRoot, since.Format("2006-01-02"), until.Format("2006-01-02"))

	// Build race-level expectations from Betfair files (date/region/type/off_time)
	expRaces := scanBetfairRaceTimes(*bfRoot, since, until)
	log.Printf("📅 Expected races from Betfair: %d", len(expRaces))

	db := mustOpenDB(*dbConnStr, *searchPath)
	defer db.Close()

	// What race-level entries do we already have in DB (with any Betfair data)?
	loadedRaces := mustFetchLoadedRaceTimes(context.Background(), db, since, until)
	log.Printf("💾 Loaded races in DB (with BF data): %d", len(loadedRaces))

	// Diff at race-level
	missingRaces := map[string]raceKey{}
	for k, v := range expRaces {
		if _, ok := loadedRaces[k]; !ok {
			missingRaces[k] = v
		}
	}
	log.Printf("❌ Missing races: %d", len(missingRaces))

	// Also compute missing days (useful to bulk re-scrape)
	missingDays := uniqueMissingDaysFromRaces(missingRaces)
	log.Printf("❌ Missing days (from race diff): %d", len(missingDays))

	if *debugKeys {
		printRaceKeyDebug(expRaces, loadedRaces, missingRaces)
	}

	if *showSample {
		printRaceBucketSamples(missingRaces, *maxSamples)
	}

	// Output modes:
	if *listMissingRaces {
		dates := make([]string, 0, len(missingRaces))
		for _, v := range missingRaces {
			dates = append(dates, fmt.Sprintf("%s,%s,%s,%s", v.Date, v.Region, v.RaceType, v.OffHHMM))
		}
		sort.Strings(dates)
		for _, line := range dates {
			fmt.Println(line)
		}
		log.Printf("📄 Printed %d missing races.", len(dates))
		return
	}
	if *listMissingDays {
		for _, d := range missingDays {
			fmt.Println(d)
		}
		log.Printf("📄 Printed %d missing days.", len(missingDays))
		return
	}

	if *dryRun {
		log.Printf("\nDone (dry-run=%v).", *dryRun)
		return
	}
}

// ---------------- Betfair calendar from filenames + CSV (race-level) ----------------

// Example BF file name: gb_flat_2013-09-24_1430.csv
var bfFileRe = regexp.MustCompile(`(?i)^(gb|ire)_(flat|jumps)_(\d{4}-\d{2}-\d{2})_[0-2]\d[0-5]\d\.csv$`)

func bucketTypeFromBF(dirType string) string {
	switch strings.ToLower(dirType) {
	case "flat":
		return "Flat"
	case "jumps":
		return "Hurdle/Chase"
	default:
		return strings.Title(dirType)
	}
}

func scanBetfairRaceTimes(root string, since, until time.Time) map[string]raceKey {
	out := make(map[string]raceKey, 2048)

	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		m := bfFileRe.FindStringSubmatch(d.Name())
		if len(m) != 4 {
			return nil
		}
		date := m[3]
		dt, err := time.Parse("2006-01-02", date)
		if err != nil || dt.Before(since) || dt.After(until) {
			return nil
		}
		region := strings.ToUpper(m[1])                  // GB|IRE
		rtype := bucketTypeFromBF(strings.ToLower(m[2])) // Flat|Hurdle/Chase

		// Open CSV and pull distinct OFF times (HH:MM)
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		reader := csv.NewReader(bufio.NewReader(f))
		reader.FieldsPerRecord = -1

		header, err := reader.Read()
		if err != nil {
			return nil
		}
		offIdx := indexOf(header, "off")
		dateIdx := indexOf(header, "date")
		if offIdx < 0 || dateIdx < 0 {
			return nil
		}

		seenOff := map[string]bool{}
		for {
			rec, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil || len(rec) <= offIdx || len(rec) <= dateIdx {
				continue
			}
			// Respect row date (some stitched sets can mix)
			rowDate := strings.TrimSpace(rec[dateIdx])
			if rowDate != date {
				continue
			}
			off := strings.TrimSpace(rec[offIdx])
			off = normalizeHHMM(off)
			if off == "" {
				continue
			}
			if !seenOff[off] {
				seenOff[off] = true
				rk := raceKey{Date: date, Region: region, RaceType: rtype, OffHHMM: off}
				out[raceKeyString(rk)] = rk
			}
		}
		return nil
	})

	return out
}

func indexOf(cols []string, name string) int {
	for i, c := range cols {
		if strings.EqualFold(strings.TrimSpace(c), name) {
			return i
		}
	}
	return -1
}

func normalizeHHMM(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.Contains(s, ":") {
		parts := strings.SplitN(s, ":", 3)
		if len(parts) >= 2 {
			h, _ := strconvAtoiSafe(parts[0])
			m, _ := strconvAtoiSafe(parts[1])
			return fmt.Sprintf("%02d:%02d", h, m)
		}
	}
	// Fallback: sometimes filenames carry HHMM; rows should be HH:MM though
	if len(s) == 4 {
		h, _ := strconvAtoiSafe(s[:2])
		m, _ := strconvAtoiSafe(s[2:])
		return fmt.Sprintf("%02d:%02d", h, m)
	}
	return s
}

func strconvAtoiSafe(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	return strconv.Atoi(s)
}

// ---------------- DB: what’s already loaded at race-level? ----------------

func mustOpenDB(conn, sp string) *sql.DB {
	db, err := sql.Open("postgres", conn)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	if _, err := db.Exec(`SET search_path TO ` + sp); err != nil {
		log.Fatal(err)
	}
	return db
}

// Map NH Flat into Jumps bucket so it lines up with Betfair "jumps"
func bucketTypeFromDB(raceType string) string {
	rt := strings.ToLower(strings.TrimSpace(raceType))
	switch {
	case strings.Contains(rt, "flat") && !strings.Contains(rt, "nh"):
		return "Flat"
	default:
		// hurdles/chases/nh flat → into "Hurdle/Chase"
		return "Hurdle/Chase"
	}
}

func mustFetchLoadedRaceTimes(ctx context.Context, db *sql.DB, since, until time.Time) map[string]raceKey {
	// Consider “loaded” if any runner for that race has BF prices/volumes.
	q := `
SELECT r.race_date::date                                                   AS d,
       UPPER(r.region)                                                     AS region,
       to_char(r.off_time, 'HH24:MI')                                      AS hhmm,
       CASE
         WHEN r.race_type ILIKE '%flat%' AND r.race_type NOT ILIKE '%nh%'  THEN 'Flat'
         ELSE 'Hurdle/Chase'
       END                                                                 AS typ
FROM races r
JOIN runners ru ON ru.race_id = r.race_id
WHERE r.race_date BETWEEN $1 AND $2
  AND r.off_time IS NOT NULL
  AND (ru.win_bsp IS NOT NULL OR ru.win_pre_vol IS NOT NULL OR ru.win_ip_vol IS NOT NULL
       OR ru.place_bsp IS NOT NULL OR ru.place_pre_vol IS NOT NULL OR ru.place_ip_vol IS NOT NULL)
GROUP BY 1,2,3,4;
`
	rows, err := db.QueryContext(ctx, q, since, until)
	if err != nil {
		log.Fatalf("fetch-loaded: %v", err)
	}
	defer rows.Close()

	out := make(map[string]raceKey, 2048)
	for rows.Next() {
		var d, region, hhmm, typ string
		if err := rows.Scan(&d, &region, &hhmm, &typ); err != nil {
			continue
		}
		typ = bucketTypeFromDB(typ)
		rk := raceKey{Date: d, Region: region, RaceType: typ, OffHHMM: hhmm}
		out[raceKeyString(rk)] = rk
	}
	_ = rows.Err()
	return out
}

// ---------------- Reporting helpers ----------------

func uniqueMissingDaysFromRaces(m map[string]raceKey) []string {
	seen := map[string]bool{}
	for _, v := range m {
		seen[v.Date] = true
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

func printRaceBucketSamples(missing map[string]raceKey, n int) {
	type bucket struct {
		Region, Typ string
		Items       []string // "YYYY-MM-DD HH:MM"
	}
	buckets := map[string]*bucket{}
	for _, rk := range missing {
		key := rk.Region + "|" + rk.RaceType
		b, ok := buckets[key]
		if !ok {
			b = &bucket{Region: rk.Region, Typ: rk.RaceType}
			buckets[key] = b
		}
		b.Items = append(b.Items, rk.Date+" "+rk.OffHHMM)
	}
	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	log.Println()
	log.Println("📊 Missing race sample by region/type:")
	for _, k := range keys {
		b := buckets[k]
		sort.Strings(b.Items)
		sample := b.Items
		if len(sample) > n {
			ns := make([]string, 0, 6)
			ns = append(ns, sample[:3]...)
			ns = append(ns, "...")
			ns = append(ns, sample[len(sample)-2:]...)
			sample = ns
		}
		log.Printf("   %s %s: %d races (%v)", b.Region, b.Typ, len(b.Items), sample)
	}
	log.Println()
}

func printRaceKeyDebug(expected, loaded, missing map[string]raceKey) {
	peek := func(title string, m map[string]raceKey, lim int) {
		i := 0
		log.Printf("— %s (count=%d) —", title, len(m))
		for _, v := range m {
			log.Printf("  %s | %s | %s | %s", v.Date, v.Region, v.RaceType, v.OffHHMM)
			i++
			if i >= lim {
				break
			}
		}
	}
	peek("expected races (first 5)", expected, 5)
	peek("loaded   races (first 5)", loaded, 5)
	peek("missing  races (first 5)", missing, 5)
}
