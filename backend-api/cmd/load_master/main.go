package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
)

/*
Assumptions:
- DB has schema `racing` and function racing.norm_text(text).
- Unique constraints exist:
  courses(course_name,region) via `courses_uniq`
  horses(horse_norm)          via `horses_uniq`
  trainers(trainer_norm)      via `trainers_uniq`
  jockeys(jockey_norm)        via `jockeys_uniq`
  owners(owner_norm)          via `owners_uniq`
  bloodlines(sire_norm,dam_norm,damsire_norm) via `bloodlines_uniq`
- races unique on (race_key, race_date)
- runners unique on (runner_key, race_date)
- Optional DB function: create_partitions_for_year(int)
*/

var (
	dsn        = flag.String("dsn", "host=localhost port=5432 dbname=horse_db user=postgres password=password sslmode=disable", "Postgres DSN")
	masterDir  = flag.String("master", "/home/smonaghan/hrmasterset/master", "Master directory root")
	regionF    = flag.String("region", "", "Filter region (gb|ire)")
	rtypeF     = flag.String("type", "", "Filter race type dir (flat|jumps|chase)")
	monthF     = flag.String("month", "", "Filter month (YYYY-MM)")
	limitN     = flag.Int("limit", 0, "Limit months processed")
	makeParts  = flag.Bool("create-partitions", true, "Call create_partitions_for_year")
	fixRan     = flag.Bool("fix-ran", false, "Update races.ran to computed starters when mismatched")
	verbose    = flag.Bool("v", true, "Verbose logging")
	timeoutMin = flag.Int("timeout", 60, "Overall timeout minutes")
)

type pair struct {
	Region     string
	RaceType   string
	YearMonth  string
	RacesCSV   string
	RunnersCSV string
}

func main() {
	flag.Parse()

	pairs, err := discoverPairs(*masterDir, *regionF, *rtypeF, *monthF)
	if err != nil {
		log.Fatal(err)
	}
	if len(pairs) == 0 {
		log.Println("No master file pairs found for given filters.")
		return
	}
	if *limitN > 0 && len(pairs) > *limitN {
		pairs = pairs[:*limitN]
	}

	log.Printf("Found %d month(s) to load.", len(pairs))

	years := uniqueYears(pairs)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutMin)*time.Minute)
	defer cancel()

	db, err := sql.Open("postgres", *dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, `SET search_path TO racing, public`); err != nil {
		log.Fatalf("set search_path: %v", err)
	}

	if *makeParts && len(years) > 0 {
		log.Printf("Creating partitions for %d year(s): %v", len(years), years)
		for _, y := range years {
			if _, err := db.ExecContext(ctx, `SELECT create_partitions_for_year($1)`, y); err != nil {
				// Not fatal; log and continue
				log.Printf("  warn: create_partitions_for_year(%d): %v", y, err)
			}
		}
	}

	totalRaces, totalRunners := 0, 0
	startAll := time.Now()

	for i, p := range pairs {
		log.Printf("[%d/%d] Loading %s/%s %s", i+1, len(pairs), strings.ToUpper(p.Region), p.RaceType, p.YearMonth)
		rUp, ruUp, err := loadMonth(ctx, db, p)
		if err != nil {
			log.Printf("  ❌ ERROR loading %s/%s %s: %v", p.Region, p.RaceType, p.YearMonth, err)
			continue
		}
		log.Printf("  ✓ Upserted: %d races, %d runners", rUp, ruUp)
		totalRaces += rUp
		totalRunners += ruUp
	}

	log.Printf("=== DONE in %s | races=%d runners=%d ===", time.Since(startAll).Round(time.Second), totalRaces, totalRunners)
}

/* -------------------- discovery -------------------- */

func discoverPairs(root, region, rtype, month string) ([]pair, error) {
	// pattern: master/{region}/{type}/{YYYY-MM}/races_*.csv
	glob := filepath.Join(root, sel(region, "*"), sel(rtype, "*"), sel(month, "*"), "races_*.csv")
	matches, err := filepath.Glob(glob)
	if err != nil {
		return nil, err
	}
	var out []pair
	for _, races := range matches {
		runners := strings.Replace(races, "/races_", "/runners_", 1)
		if _, err := os.Stat(runners); err != nil {
			continue
		}
		parts := strings.Split(races, string(os.PathSeparator))
		// .../master/<region>/<type>/<YYYY-MM>/races_*.csv
		if len(parts) < 4 {
			continue
		}
		n := len(parts)
		out = append(out, pair{
			Region:     parts[n-4],
			RaceType:   parts[n-3],
			YearMonth:  parts[n-2],
			RacesCSV:   races,
			RunnersCSV: runners,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Region != b.Region {
			return a.Region < b.Region
		}
		if a.RaceType != b.RaceType {
			return a.RaceType < b.RaceType
		}
		return a.YearMonth < b.YearMonth
	})
	return out, nil
}

func sel(val, wildcard string) string {
	if strings.TrimSpace(val) == "" {
		return wildcard
	}
	return val
}

func uniqueYears(pairs []pair) []int {
	m := map[int]struct{}{}
	for _, p := range pairs {
		if len(p.YearMonth) >= 4 {
			if y, err := strconvAtoiSafe(p.YearMonth[:4]); err == nil {
				m[y] = struct{}{}
			}
		}
	}
	ys := make([]int, 0, len(m))
	for y := range m {
		ys = append(ys, y)
	}
	sort.Ints(ys)
	return ys
}

func strconvAtoiSafe(s string) (int, error) {
	var x int
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, errors.New("not int")
		}
		x = x*10 + int(ch-'0')
	}
	return x, nil
}

/* -------------------- month loader -------------------- */

func loadMonth(ctx context.Context, db *sql.DB, p pair) (racesUp int, runnersUp int, err error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.ExecContext(ctx, `SET LOCAL synchronous_commit = off`)

	// 1) COPY CSVs into TEMP tables
	rTemp, rCols, rCount, err := copyCSVToTemp(ctx, tx, p.RacesCSV, "stage_races")
	if err != nil {
		return 0, 0, fmt.Errorf("copy races: %w", err)
	}
	ruTemp, ruCols, ruCount, err := copyCSVToTemp(ctx, tx, p.RunnersCSV, "stage_runners")
	if err != nil {
		return 0, 0, fmt.Errorf("copy runners: %w", err)
	}

	if *verbose {
		log.Printf("  staged: %s(%d)  %s(%d)", rTemp, rCount, ruTemp, ruCount)
	}

	// 2) Validate headers (required)
	requireCols(rCols,
		"date", "region", "course", "off", "race_name", "type", "race_key",
	)
	requireCols(ruCols,
		"race_key", "num", "pos", "horse", "jockey", "trainer", "runner_key",
	)

	// 3) Upsert dimensions (courses, horses, trainers, jockeys)
	if *verbose {
		log.Printf("  → upserting dimensions")
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(sqlUpsertCourses, rTemp)); err != nil {
		return 0, 0, fmt.Errorf("upsert courses: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(sqlUpsertHorses, ruTemp)); err != nil {
		return 0, 0, fmt.Errorf("upsert horses: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(sqlUpsertTrainers, ruTemp)); err != nil {
		return 0, 0, fmt.Errorf("upsert trainers: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(sqlUpsertJockeys, ruTemp)); err != nil {
		return 0, 0, fmt.Errorf("upsert jockeys: %w", err)
	}

	// 4) Upsert races
	if *verbose {
		log.Printf("  → upserting races")
	}
	if err := tx.QueryRowContext(ctx, fmt.Sprintf(sqlUpsertRaces, rTemp)).Scan(&racesUp); err != nil {
		return 0, 0, fmt.Errorf("upsert races: %w", err)
	}

	// 5) Upsert runners (join to races via race_key + date from races temp)
	if *verbose {
		log.Printf("  → upserting runners")
	}
	if err := tx.QueryRowContext(ctx, fmt.Sprintf(sqlUpsertRunners, ruTemp, rTemp, ruTemp)).Scan(&runnersUp); err != nil {
		return 0, 0, fmt.Errorf("upsert runners: %w", err)
	}

	// 6) Validation
	if *verbose {
		log.Printf("  → validation")
	}
	if err := runValidation(ctx, tx, *fixRan); err != nil {
		log.Printf("  warn: validation: %v", err)
	}

	// 7) Analyze hot tables (cheap)
	_, _ = tx.ExecContext(ctx, `ANALYZE races; ANALYZE runners;`)

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return racesUp, runnersUp, nil
}

/* -------------------- COPY helpers -------------------- */

func copyCSVToTemp(ctx context.Context, tx *sql.Tx, csvPath, base string) (tempTable string, headers []string, rows int, err error) {
	f, err := os.Open(csvPath)
	if err != nil {
		return "", nil, 0, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.ReuseRecord = true

	hdr, err := r.Read()
	if err != nil {
		return "", nil, 0, fmt.Errorf("read header: %w", err)
	}

	// Make a copy of headers since ReuseRecord=true will overwrite the slice
	headers = make([]string, len(hdr))
	copy(headers, hdr)

	// Create lowercase column names for consistency
	lowercaseHeaders := make([]string, len(headers))
	cols := make([]string, len(headers))
	for i, h := range headers {
		lowercaseHeaders[i] = strings.ToLower(strings.TrimSpace(h))
		cols[i] = pq.QuoteIdentifier(lowercaseHeaders[i])
	}

	tempTable = pq.QuoteIdentifier(base + "_" + randSuffix())
	create := fmt.Sprintf("CREATE TEMP TABLE %s (\n", tempTable)
	for i, c := range cols {
		if i == 0 {
			create += fmt.Sprintf("  %s text", c)
		} else {
			create += fmt.Sprintf(", %s text", c)
		}
	}
	create += "\n) ON COMMIT DROP;"

	if _, err := tx.ExecContext(ctx, create); err != nil {
		return "", nil, 0, fmt.Errorf("create temp: %w", err)
	}

	stmt, err := tx.Prepare(pq.CopyIn(tempTable[1:len(tempTable)-1], lowercaseHeaders...)) // remove quotes for CopyIn
	if err != nil {
		return "", nil, 0, fmt.Errorf("prepare copyin: %w", err)
	}
	defer stmt.Close()

	count := 0
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, 0, fmt.Errorf("read row: %w", err)
		}
		vals := make([]any, len(rec))
		for i := range rec {
			s := strings.TrimSpace(rec[i])
			if s == "" {
				vals[i] = nil
			} else {
				vals[i] = s
			}
		}
		if _, err := stmt.Exec(vals...); err != nil {
			return "", nil, 0, fmt.Errorf("copy row: %w", err)
		}
		count++
	}
	if _, err := stmt.Exec(); err != nil {
		return "", nil, 0, fmt.Errorf("finish copy: %w", err)
	}
	return tempTable, headers, count, nil
}

func randSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano()%1e9)
}

func requireCols(hdr []string, needed ...string) {
	m := map[string]bool{}
	for _, c := range hdr {
		normalized := strings.ToLower(strings.TrimSpace(c))
		m[normalized] = true
	}
	for _, n := range needed {
		if !m[strings.ToLower(n)] {
			log.Fatalf("CSV missing required column: %s (available: %v)", n, hdr)
		}
	}
}

/* -------------------- SQL templates (fmt.Sprintf with temp names) -------------------- */

const sqlUpsertCourses = `
INSERT INTO racing.courses (course_name, region)
SELECT DISTINCT t.course, t.region
FROM %s t
WHERE t.course IS NOT NULL AND t.course <> '' AND t.region IS NOT NULL
ON CONFLICT ON CONSTRAINT courses_uniq DO NOTHING;
`

const sqlUpsertHorses = `
INSERT INTO racing.horses (horse_name)
SELECT DISTINCT t.horse
FROM %s t
WHERE t.horse IS NOT NULL AND t.horse <> ''
ON CONFLICT ON CONSTRAINT horses_uniq DO NOTHING;
`

const sqlUpsertTrainers = `
INSERT INTO racing.trainers (trainer_name)
SELECT DISTINCT t.trainer
FROM %s t
WHERE t.trainer IS NOT NULL AND t.trainer <> ''
ON CONFLICT ON CONSTRAINT trainers_uniq DO NOTHING;
`

const sqlUpsertJockeys = `
INSERT INTO racing.jockeys (jockey_name)
SELECT DISTINCT t.jockey
FROM %s t
WHERE t.jockey IS NOT NULL AND t.jockey <> ''
ON CONFLICT ON CONSTRAINT jockeys_uniq DO NOTHING;
`

const sqlUpsertOwners = `
INSERT INTO racing.owners (owner_name)
SELECT DISTINCT t.owner
FROM %s t
WHERE t.owner IS NOT NULL AND t.owner <> ''
ON CONFLICT ON CONSTRAINT owners_uniq DO NOTHING;
`

const sqlUpsertBloodlines = `
INSERT INTO racing.bloodlines (sire, dam, damsire)
SELECT DISTINCT t.sire, t.dam, t.damsire
FROM %s t
WHERE (t.sire IS NOT NULL AND t.sire <> '')
   OR (t.dam IS NOT NULL AND t.dam <> '')
   OR (t.damsire IS NOT NULL AND t.damsire <> '')
ON CONFLICT ON CONSTRAINT bloodlines_uniq DO NOTHING;
`

const sqlUpsertRaces = `
WITH c AS (
  SELECT course_id, course_name, region FROM racing.courses
),
ins AS (
  INSERT INTO racing.races (
    race_key, race_date, region, course_id, off_time, race_name, race_type,
    class, pattern, rating_band, age_band, sex_rest,
    dist_raw, dist_f, dist_m, going, surface, ran
  )
  SELECT DISTINCT
    t.race_key,
    t.date::date,
    t.region,
    c.course_id,
    CASE WHEN t.off ~ '^[0-9]{1,2}:[0-9]{2}$' THEN t.off::time ELSE NULL END,
    t.race_name,
    t.type,
    NULLIF(t.class,''),
    NULLIF(t.pattern,''),
    NULLIF(t.rating_band,''),
    NULLIF(t.age_band,''),
    NULLIF(t.sex_rest,''),
    NULLIF(t.dist,''),
    NULLIF(REPLACE(t.dist_f,'f',''),'')::double precision,
    NULLIF(t.dist_m,'')::int,
    NULLIF(t.going,''),
    NULLIF(t.surface,''),
    NULLIF(t.ran,'')::int
  FROM %s t
  JOIN c ON racing.norm_text(c.course_name) = racing.norm_text(t.course)
       AND c.region = t.region
  ON CONFLICT (race_key, race_date) DO UPDATE SET
    region    = EXCLUDED.region,
    course_id = EXCLUDED.course_id,
    off_time  = EXCLUDED.off_time,
    race_name = EXCLUDED.race_name,
    race_type = EXCLUDED.race_type,
    class     = EXCLUDED.class,
    pattern   = EXCLUDED.pattern,
    rating_band = EXCLUDED.rating_band,
    age_band  = EXCLUDED.age_band,
    sex_rest  = EXCLUDED.sex_rest,
    dist_raw  = EXCLUDED.dist_raw,
    dist_f    = COALESCE(EXCLUDED.dist_f, races.dist_f),
    dist_m    = COALESCE(EXCLUDED.dist_m, races.dist_m),
    going     = COALESCE(EXCLUDED.going, races.going),
    surface   = EXCLUDED.surface,
    ran       = COALESCE(EXCLUDED.ran, races.ran)
  RETURNING 1
)
SELECT count(*) FROM ins;
`

const sqlUpsertRunners = `
WITH rmap AS (
  -- Map race_key to race_id + race_date using THIS month's races CSV
  SELECT DISTINCT rc.race_key, ra.race_id, ra.race_date
  FROM %s rr
  JOIN %s rc USING (race_key)
  JOIN racing.races ra
    ON ra.race_key = rc.race_key
   AND ra.race_date = rc.date::date
),
h AS (SELECT horse_id, horse_name, horse_norm FROM racing.horses),
j AS (SELECT jockey_id, jockey_name, jockey_norm FROM racing.jockeys),
t AS (SELECT trainer_id, trainer_name, trainer_norm FROM racing.trainers),
ins AS (
  INSERT INTO racing.runners (
    runner_key, race_id, race_date,
    num, pos_raw, draw,
    horse_id, age, jockey_id, trainer_id, lbs,
    "or", rpr, comment,
    win_bsp, win_ppwap, place_bsp, place_ppwap
  )
  SELECT DISTINCT
    t0.runner_key,
    rm.race_id,
    rm.race_date,
    NULLIF(regexp_replace(t0.num, '[^0-9]', '', 'g'),'')::int,
    NULLIF(t0.pos,''),
    NULLIF(regexp_replace(t0.draw, '[^0-9]', '', 'g'),'')::int,
    h.horse_id,
    NULLIF(regexp_replace(t0.age, '[^0-9]', '', 'g'),'')::int,
    j.jockey_id,
    t.trainer_id,
    NULLIF(regexp_replace(t0.lbs, '[^0-9]', '', 'g'),'')::int,
    NULLIF(regexp_replace(t0."or", '[^0-9]', '', 'g'),'')::int,
    NULLIF(regexp_replace(t0.rpr, '[^0-9]', '', 'g'),'')::int,
    NULLIF(t0.comment,''),
    CASE WHEN t0.win_bsp ~ '^[0-9]+\.?[0-9]*$' AND t0.win_bsp::double precision > 1.0 THEN t0.win_bsp::double precision ELSE NULL END,
    CASE WHEN t0.win_ppwap ~ '^[0-9]+\.?[0-9]*$' AND t0.win_ppwap::double precision > 1.0 THEN t0.win_ppwap::double precision ELSE NULL END,
    CASE WHEN t0.place_bsp ~ '^[0-9]+\.?[0-9]*$' AND t0.place_bsp::double precision > 1.0 THEN t0.place_bsp::double precision ELSE NULL END,
    CASE WHEN t0.place_ppwap ~ '^[0-9]+\.?[0-9]*$' AND t0.place_ppwap::double precision > 1.0 THEN t0.place_ppwap::double precision ELSE NULL END
  FROM %s t0
  JOIN rmap rm ON rm.race_key = t0.race_key
  LEFT JOIN h ON racing.norm_text(h.horse_name) = racing.norm_text(t0.horse)
  LEFT JOIN j ON racing.norm_text(j.jockey_name) = racing.norm_text(t0.jockey)
  LEFT JOIN t ON racing.norm_text(t.trainer_name) = racing.norm_text(t0.trainer)
  ON CONFLICT (runner_key, race_date) DO UPDATE SET
    num         = EXCLUDED.num,
    pos_raw     = EXCLUDED.pos_raw,
    draw        = EXCLUDED.draw,
    horse_id    = COALESCE(EXCLUDED.horse_id, runners.horse_id),
    age         = EXCLUDED.age,
    jockey_id   = COALESCE(EXCLUDED.jockey_id, runners.jockey_id),
    trainer_id  = COALESCE(EXCLUDED.trainer_id, runners.trainer_id),
    lbs         = EXCLUDED.lbs,
    "or"        = COALESCE(EXCLUDED."or", runners."or"),
    rpr         = COALESCE(EXCLUDED.rpr, runners.rpr),
    comment     = COALESCE(EXCLUDED.comment, runners.comment),
    win_bsp     = COALESCE(EXCLUDED.win_bsp, runners.win_bsp),
    win_ppwap   = COALESCE(EXCLUDED.win_ppwap, runners.win_ppwap),
    place_bsp   = COALESCE(EXCLUDED.place_bsp, runners.place_bsp),
    place_ppwap = COALESCE(EXCLUDED.place_ppwap, runners.place_ppwap)
  RETURNING 1
)
SELECT count(*) FROM ins;
`

/* -------------------- validation -------------------- */

func runValidation(ctx context.Context, tx *sql.Tx, fixRan bool) error {
	// 1) Race count sanity
	var rc, rru int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM racing.races`).Scan(&rc); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(DISTINCT race_id) FROM racing.runners`).Scan(&rru); err != nil {
		return err
	}
	if rc != rru {
		log.Printf("  warn: race count mismatch: races=%d distinct(race_id in runners)=%d", rc, rru)
	}

	// 2) Compute starters per race_id (exclude obvious non-runners)
	//    NOTE: keep the list tight to avoid false positives; adjust as needed.
	const startersCTE = `
WITH starters AS (
  SELECT
    race_id,
    COUNT(*) FILTER (
      WHERE pos_raw IS DISTINCT FROM '' AND pos_raw IS NOT NULL
        AND NOT (pos_raw ILIKE 'NR' OR pos_raw ILIKE 'N/R' OR pos_raw ILIKE 'WD' OR pos_raw ILIKE 'W/D'
                 OR pos_raw ILIKE 'RES' OR pos_raw ILIKE 'RESERVE')
    ) AS n_starters
  FROM racing.runners
  GROUP BY race_id
)
SELECT ra.race_id, ra.race_key, ra.race_date, ra.ran, s.n_starters
FROM racing.races ra
LEFT JOIN starters s USING (race_id)
WHERE COALESCE(ra.ran, -1) <> COALESCE(s.n_starters, -1);
`

	rows, err := tx.QueryContext(ctx, startersCTE)
	if err != nil {
		return fmt.Errorf("compute starters: %w", err)
	}
	defer rows.Close()

	type mis struct {
		raceID    int64
		raceKey   string
		raceDate  time.Time
		ran       sql.NullInt64
		nStarters sql.NullInt64
	}
	var mismatches []mis
	for rows.Next() {
		var m mis
		if err := rows.Scan(&m.raceID, &m.raceKey, &m.raceDate, &m.ran, &m.nStarters); err != nil {
			return err
		}
		mismatches = append(mismatches, m)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if len(mismatches) > 0 {
		log.Printf("  warn: %d races have RAN != starters", len(mismatches))
		for i := 0; i < len(mismatches) && i < 5; i++ {
			m := mismatches[i]
			log.Printf("    sample race_id=%d date=%s ran=%v starters=%v",
				m.raceID, m.raceDate.Format("2006-01-02"),
				nullI64(m.ran), nullI64(m.nStarters))
		}

		if fixRan {
			// Update ran to computed starters (only where > 0)
			const upd = `
WITH starters AS (
  SELECT
    race_id,
    COUNT(*) FILTER (
      WHERE pos_raw IS DISTINCT FROM '' AND pos_raw IS NOT NULL
        AND NOT (pos_raw ILIKE 'NR' OR pos_raw ILIKE 'N/R' OR pos_raw ILIKE 'WD' OR pos_raw ILIKE 'W/D'
                 OR pos_raw ILIKE 'RES' OR pos_raw ILIKE 'RESERVE')
    ) AS n_starters
  FROM racing.runners
  GROUP BY race_id
)
UPDATE racing.races ra
SET ran = s.n_starters
FROM starters s
WHERE ra.race_id = s.race_id
  AND s.n_starters IS NOT NULL
  AND s.n_starters > 0
  AND ra.ran IS DISTINCT FROM s.n_starters;
`
			res, err := tx.ExecContext(ctx, upd)
			if err != nil {
				log.Printf("  warn: failed to fix ran values: %v", err)
			} else {
				if n, _ := res.RowsAffected(); n > 0 {
					log.Printf("  ✓ fixed %d races.ran to computed starters", n)
				}
			}
		}
	}

	// 3) Sentinel prices still present?
	var sent int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM racing.runners WHERE win_bsp = 1`).Scan(&sent); err != nil {
		return err
	}
	if sent > 0 {
		log.Printf("  warn: %d rows still have sentinel price 1.0 (should be NULL)", sent)
	}

	// 4) Duplicate keys (should be 0)
	var dupR, dupRu int
	_ = tx.QueryRowContext(ctx, `SELECT COUNT(*) - COUNT(DISTINCT race_key) FROM racing.races`).Scan(&dupR)
	_ = tx.QueryRowContext(ctx, `SELECT COUNT(*) - COUNT(DISTINCT runner_key) FROM racing.runners`).Scan(&dupRu)
	if dupR > 0 {
		log.Printf("  error: %d duplicate race_key in races", dupR)
	}
	if dupRu > 0 {
		log.Printf("  error: %d duplicate runner_key in runners", dupRu)
	}
	return nil
}

func nullI64(v sql.NullInt64) any {
	if v.Valid {
		return v.Int64
	}
	return nil
}
