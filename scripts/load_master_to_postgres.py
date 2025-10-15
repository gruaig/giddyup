#!/usr/bin/env python3
"""
Master Data Loader - PostgreSQL Ingestion
Loads master CSVs (races + runners) into PostgreSQL database
Handles: schema init, partitions, dimension upserts, fact upserts, validation
"""
import os
import sys
import glob
import subprocess
import psycopg2
from psycopg2 import extras
from psycopg2.extensions import ISOLATION_LEVEL_AUTOCOMMIT
import csv
from datetime import datetime
from pathlib import Path
import time

# Configuration
MASTER_DIR = "/home/smonaghan/rpscrape/master"
DB_CONFIG = {
    'host': 'localhost',
    'port': 5432,
    'database': 'horse_db',
    'user': 'postgres',
    'password': 'password'
}

# Logging
def log(msg, level='INFO'):
    """Simple logging"""
    timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
    print(f"[{timestamp}] [{level}] {msg}")
    sys.stdout.flush()

def connect_db(dbname=None):
    """Connect to PostgreSQL"""
    config = DB_CONFIG.copy()
    if dbname:
        config['database'] = dbname
    
    try:
        conn = psycopg2.connect(**config)
        # Set search path to racing schema
        if dbname == 'horse_db':
            cur = conn.cursor()
            cur.execute("SET search_path TO racing, public;")
            conn.commit()
            cur.close()
        return conn
    except psycopg2.OperationalError as e:
        log(f"Database connection failed: {e}", 'ERROR')
        raise

def init_database():
    """Initialize database and schema if not exists"""
    log("Initializing database...")
    
    # Connect to postgres database to create horse_db
    conn = connect_db('postgres')
    conn.set_isolation_level(ISOLATION_LEVEL_AUTOCOMMIT)
    cur = conn.cursor()
    
    # Check if database exists
    cur.execute("SELECT 1 FROM pg_database WHERE datname = 'horse_db'")
    if not cur.fetchone():
        log("Creating database 'horse_db'...")
        cur.execute("CREATE DATABASE horse_db WITH ENCODING 'UTF8'")
    else:
        log("Database 'horse_db' already exists")
    
    cur.close()
    conn.close()
    
    # Now run init_clean.sql using docker exec (cleanest approach)
    init_sql_path = '/home/smonaghan/rpscrape/postgres/init_clean.sql'
    if os.path.exists(init_sql_path):
        log("Running init_clean.sql schema...")
        # Copy to container and execute
        result = subprocess.run(
            ['docker', 'cp', init_sql_path, 'horse_racing:/tmp/init_clean.sql'],
            capture_output=True
        )
        if result.returncode != 0:
            log(f"Failed to copy init script: {result.stderr.decode()}", 'ERROR')
            raise RuntimeError("Failed to copy init script to container")
        
        # Execute SQL
        result = subprocess.run(
            ['docker', 'exec', 'horse_racing', 'psql', '-U', 'postgres', '-d', 'horse_db', '-f', '/tmp/init_clean.sql'],
            capture_output=True,
            text=True
        )
        
        if result.returncode == 0:
            log("Schema initialized successfully")
        else:
            # Check if it's just "already exists" warnings
            if 'already exists' in result.stderr.lower():
                log("Schema already exists (OK)")
            else:
                log(f"init_clean.sql warnings: {result.stderr}", 'WARN')
    else:
        log(f"init_clean.sql not found at {init_sql_path}", 'ERROR')
        raise FileNotFoundError(f"Missing {init_sql_path}")

def create_partitions_for_years(years):
    """Create monthly partitions for given years"""
    conn = connect_db('horse_db')
    cur = conn.cursor()
    
    log(f"Creating partitions for years: {min(years)}-{max(years)}...")
    
    for year in years:
        try:
            cur.execute("SELECT create_partitions_for_year(%s)", (year,))
            conn.commit()
        except psycopg2.Error as e:
            if 'already exists' in str(e):
                log(f"Partitions for {year} already exist", 'DEBUG')
            else:
                log(f"Error creating partitions for {year}: {e}", 'WARN')
            conn.rollback()
    
    log(f"Partitions created for {len(years)} years")
    cur.close()
    conn.close()

def discover_master_files(region=None, race_type=None, year_month=None):
    """Discover master CSV files to load"""
    files = []
    
    # Pattern: master/{region}/{race_type}/{YYYY-MM}/races_*.csv
    pattern = f"{MASTER_DIR}"
    if region:
        pattern += f"/{region}"
    else:
        pattern += "/*"
    
    if race_type:
        pattern += f"/{race_type}"
    else:
        pattern += "/*"
    
    if year_month:
        pattern += f"/{year_month}"
    else:
        pattern += "/*"
    
    pattern += "/races_*.csv"
    
    race_files = sorted(glob.glob(pattern))
    
    for race_file in race_files:
        # Derive runner file path
        runner_file = race_file.replace('/races_', '/runners_')
        
        if os.path.exists(runner_file):
            # Extract metadata from path
            parts = race_file.replace(MASTER_DIR + '/', '').split('/')
            file_info = {
                'region': parts[0],
                'race_type': parts[1],
                'year_month': parts[2],
                'race_file': race_file,
                'runner_file': runner_file
            }
            files.append(file_info)
        else:
            log(f"Missing runner file for {race_file}", 'WARN')
    
    return files

def load_csv_to_staging(cur, csv_file, table_name):
    """Load CSV into staging table using COPY"""
    log(f"  Loading {os.path.basename(csv_file)} into {table_name}...")
    
    # Truncate staging table
    cur.execute(f"TRUNCATE {table_name}")
    
    # COPY CSV into staging
    with open(csv_file, 'r', encoding='utf-8') as f:
        # Skip header
        next(f)
        cur.copy_from(f, table_name, sep=',', null='')
    
    # Count rows
    cur.execute(f"SELECT COUNT(*) FROM {table_name}")
    count = cur.fetchone()[0]
    log(f"  Loaded {count:,} rows into {table_name}")
    
    return count

def upsert_dimensions(cur):
    """Upsert dimension tables from staging"""
    log("  Upserting dimensions...")
    
    # Courses
    cur.execute("""
        INSERT INTO courses (course_name, region)
        SELECT DISTINCT course, region FROM stage_races
        WHERE course IS NOT NULL AND course <> ''
        ON CONFLICT (region, course_norm) DO NOTHING
    """)
    courses_inserted = cur.rowcount
    
    # Horses
    cur.execute("""
        INSERT INTO horses (horse_name)
        SELECT DISTINCT horse FROM stage_runners
        WHERE horse IS NOT NULL AND horse <> ''
        ON CONFLICT (horse_norm) DO NOTHING
    """)
    horses_inserted = cur.rowcount
    
    # Trainers
    cur.execute("""
        INSERT INTO trainers (trainer_name)
        SELECT DISTINCT trainer FROM stage_runners
        WHERE trainer IS NOT NULL AND trainer <> ''
        ON CONFLICT (trainer_norm) DO NOTHING
    """)
    trainers_inserted = cur.rowcount
    
    # Jockeys
    cur.execute("""
        INSERT INTO jockeys (jockey_name)
        SELECT DISTINCT jockey FROM stage_runners
        WHERE jockey IS NOT NULL AND jockey <> ''
        ON CONFLICT (jockey_norm) DO NOTHING
    """)
    jockeys_inserted = cur.rowcount
    
    # Owners
    cur.execute("""
        INSERT INTO owners (owner_name)
        SELECT DISTINCT owner FROM stage_runners
        WHERE owner IS NOT NULL AND owner <> ''
        ON CONFLICT (owner_norm) DO NOTHING
    """)
    owners_inserted = cur.rowcount
    
    # Bloodlines
    cur.execute("""
        INSERT INTO bloodlines (sire, dam, damsire)
        SELECT DISTINCT sire, dam, damsire FROM stage_runners
        WHERE (sire IS NOT NULL AND sire <> '') 
           OR (dam IS NOT NULL AND dam <> '') 
           OR (damsire IS NOT NULL AND damsire <> '')
        ON CONFLICT (sire_norm, dam_norm, damsire_norm) DO NOTHING
    """)
    bloodlines_inserted = cur.rowcount
    
    log(f"  Dimensions: courses={courses_inserted}, horses={horses_inserted}, "
        f"trainers={trainers_inserted}, jockeys={jockeys_inserted}, "
        f"owners={owners_inserted}, bloodlines={bloodlines_inserted}")

def upsert_races(cur):
    """Upsert races from staging"""
    log("  Upserting races...")
    
    cur.execute("""
        WITH casted AS (
          SELECT
            race_key,
            NULLIF(date, '')::date AS race_date,
            region,
            (SELECT course_id FROM courses c
              WHERE c.region = stage_races.region
                AND norm_text(c.course_name) = norm_text(stage_races.course)
              LIMIT 1) AS course_id,
            NULLIF(off, '')::time AS off_time,
            race_name,
            NULLIF(type, '') AS race_type,
            NULLIF(class, '') AS class,
            NULLIF(pattern, '') AS pattern,
            NULLIF(rating_band, '') AS rating_band,
            NULLIF(age_band, '') AS age_band,
            NULLIF(sex_rest, '') AS sex_rest,
            NULLIF(dist, '') AS dist_raw,
            NULLIF(replace(dist_f, 'f', ''), '')::double precision AS dist_f,
            NULLIF(dist_m, '')::int AS dist_m,
            NULLIF(going, '') AS going,
            NULLIF(surface, '') AS surface,
            NULLIF(ran, '')::int AS ran
          FROM stage_races
          WHERE race_key IS NOT NULL AND race_key <> ''
        )
        INSERT INTO races (
            race_key, race_date, region, course_id, off_time, race_name, race_type,
            class, pattern, rating_band, age_band, sex_rest,
            dist_raw, dist_f, dist_m, going, surface, ran
        )
        SELECT * FROM casted
        ON CONFLICT (race_key, race_date) DO UPDATE
        SET going = COALESCE(EXCLUDED.going, races.going),
            ran   = COALESCE(EXCLUDED.ran, races.ran),
            dist_f = COALESCE(EXCLUDED.dist_f, races.dist_f)
    """)
    
    races_upserted = cur.rowcount
    log(f"  Upserted {races_upserted:,} races")
    return races_upserted

def upsert_runners(cur):
    """Upsert runners from staging"""
    log("  Upserting runners...")
    
    cur.execute("""
        WITH maps AS (
          SELECT race_id, race_key, race_date FROM races
        ),
        casted AS (
          SELECT
            r.runner_key,
            m.race_id,
            m.race_date,
            (SELECT horse_id   FROM horses   h WHERE h.horse_norm   = norm_text(r.horse)   LIMIT 1) AS horse_id,
            (SELECT trainer_id FROM trainers t WHERE t.trainer_norm = norm_text(r.trainer) LIMIT 1) AS trainer_id,
            (SELECT jockey_id  FROM jockeys  j WHERE j.jockey_norm  = norm_text(r.jockey)  LIMIT 1) AS jockey_id,
            (SELECT owner_id   FROM owners   o WHERE o.owner_norm   = norm_text(r.owner)   LIMIT 1) AS owner_id,
            (SELECT blood_id   FROM bloodlines b
              WHERE b.sire_norm = norm_text(r.sire)
                AND b.dam_norm = norm_text(r.dam)
                AND b.damsire_norm = norm_text(r.damsire)
              LIMIT 1) AS blood_id,
            NULLIF(r.num, '')::int AS num,
            NULLIF(r.pos, '') AS pos_raw,
            NULLIF(r.draw, '')::int AS draw,
            NULLIF(r.ovr_btn, '')::double precision AS ovr_btn,
            NULLIF(r.btn, '')::double precision AS btn,
            NULLIF(r.age, '')::int AS age,
            NULLIF(r.sex, '') AS sex,
            NULLIF(r.lbs, '')::int AS lbs,
            NULLIF(r.hg, '') AS hg,
            NULLIF(r.time, '') AS time_raw,
            NULLIF(r.secs, '')::double precision AS secs,
            NULLIF(NULLIF(r.dec, '1'), '')::double precision AS dec,
            NULLIF(r.prize, '')::double precision AS prize,
            NULLIF(r.prize_raw, '') AS prize_raw,
            NULLIF(r."or", '')::int AS "or",
            NULLIF(r.rpr, '')::int AS rpr,
            NULLIF(r.comment, '') AS comment,
            NULLIF(NULLIF(r.win_bsp, '1'), '')::double precision AS win_bsp,
            NULLIF(NULLIF(r.win_ppwap, '1'), '')::double precision AS win_ppwap,
            NULLIF(NULLIF(r.win_morningwap, '1'), '')::double precision AS win_morningwap,
            NULLIF(NULLIF(r.win_ppmax, '1'), '')::double precision AS win_ppmax,
            NULLIF(NULLIF(r.win_ppmin, '1'), '')::double precision AS win_ppmin,
            NULLIF(NULLIF(r.win_ipmax, '1'), '')::double precision AS win_ipmax,
            NULLIF(NULLIF(r.win_ipmin, '1'), '')::double precision AS win_ipmin,
            NULLIF(r.win_morning_vol, '')::double precision AS win_morning_vol,
            NULLIF(r.win_pre_vol, '')::double precision AS win_pre_vol,
            NULLIF(r.win_ip_vol, '')::double precision AS win_ip_vol,
            NULLIF(r.win_lose, '')::int AS win_lose,
            NULLIF(NULLIF(r.place_bsp, '1'), '')::double precision AS place_bsp,
            NULLIF(NULLIF(r.place_ppwap, '1'), '')::double precision AS place_ppwap,
            NULLIF(NULLIF(r.place_morningwap, '1'), '')::double precision AS place_morningwap,
            NULLIF(NULLIF(r.place_ppmax, '1'), '')::double precision AS place_ppmax,
            NULLIF(NULLIF(r.place_ppmin, '1'), '')::double precision AS place_ppmin,
            NULLIF(NULLIF(r.place_ipmax, '1'), '')::double precision AS place_ipmax,
            NULLIF(NULLIF(r.place_ipmin, '1'), '')::double precision AS place_ipmin,
            NULLIF(r.place_morning_vol, '')::double precision AS place_morning_vol,
            NULLIF(r.place_pre_vol, '')::double precision AS place_pre_vol,
            NULLIF(r.place_ip_vol, '')::double precision AS place_ip_vol,
            NULLIF(r.place_win_lose, '')::int AS place_win_lose,
            NULLIF(r.match_jaccard, '')::double precision AS match_jaccard,
            NULLIF(r.match_time_diff_min, '')::int AS match_time_diff_min,
            NULLIF(r.match_reason, '') AS match_reason
          FROM stage_runners r
          JOIN maps m ON m.race_key = r.race_key
          WHERE r.runner_key IS NOT NULL AND r.runner_key <> ''
        )
        INSERT INTO runners (
            runner_key, race_id, race_date,
            horse_id, trainer_id, jockey_id, owner_id, blood_id,
            num, pos_raw, draw, ovr_btn, btn, age, sex, lbs, hg,
            time_raw, secs, dec, prize, prize_raw, "or", rpr, comment,
            win_bsp, win_ppwap, win_morningwap, win_ppmax, win_ppmin,
            win_ipmax, win_ipmin, win_morning_vol, win_pre_vol, win_ip_vol, win_lose,
            place_bsp, place_ppwap, place_morningwap, place_ppmax, place_ppmin,
            place_ipmax, place_ipmin, place_morning_vol, place_pre_vol, place_ip_vol, place_win_lose,
            match_jaccard, match_time_diff_min, match_reason
        )
        SELECT * FROM casted
        ON CONFLICT (runner_key, race_date) DO UPDATE
        SET pos_raw = COALESCE(EXCLUDED.pos_raw, runners.pos_raw),
            secs    = COALESCE(EXCLUDED.secs, runners.secs),
            dec     = COALESCE(EXCLUDED.dec, runners.dec),
            prize   = COALESCE(EXCLUDED.prize, runners.prize),
            "or"    = COALESCE(EXCLUDED."or", runners."or"),
            rpr     = COALESCE(EXCLUDED.rpr, runners.rpr),
            win_bsp = COALESCE(EXCLUDED.win_bsp, runners.win_bsp),
            win_ppwap = COALESCE(EXCLUDED.win_ppwap, runners.win_ppwap),
            place_bsp = COALESCE(EXCLUDED.place_bsp, runners.place_bsp)
    """)
    
    runners_upserted = cur.rowcount
    log(f"  Upserted {runners_upserted:,} runners")
    return runners_upserted

def validate_data(cur):
    """Run data quality checks"""
    log("  Running validation checks...")
    
    errors = []
    warnings = []
    
    # Check 1: Race count consistency
    cur.execute("SELECT COUNT(*) FROM races")
    race_count = cur.fetchone()[0]
    
    cur.execute("SELECT COUNT(DISTINCT race_id) FROM runners")
    race_count_from_runners = cur.fetchone()[0]
    
    if race_count != race_count_from_runners:
        warnings.append(f"Race count mismatch: {race_count} races but {race_count_from_runners} distinct race_ids in runners")
    
    # Check 2: Ran field consistency
    cur.execute("""
        SELECT COUNT(*)
        FROM races ra
        JOIN (
            SELECT race_id, COUNT(*) as runner_count
            FROM runners
            GROUP BY race_id
        ) ru ON ru.race_id = ra.race_id
        WHERE ra.ran <> ru.runner_count
    """)
    ran_mismatches = cur.fetchone()[0]
    if ran_mismatches > 0:
        warnings.append(f"{ran_mismatches} races have 'ran' field mismatch with actual runner count")
    
    # Check 3: Sentinel prices
    cur.execute("SELECT COUNT(*) FROM runners WHERE win_bsp = 1 OR dec = 1")
    sentinel_count = cur.fetchone()[0]
    if sentinel_count > 0:
        warnings.append(f"{sentinel_count} rows have sentinel 1.0 prices (should be NULL)")
    
    # Check 4: Unique keys
    cur.execute("SELECT COUNT(*) - COUNT(DISTINCT race_key) FROM races")
    dup_races = cur.fetchone()[0]
    if dup_races > 0:
        errors.append(f"{dup_races} duplicate race_keys found")
    
    cur.execute("SELECT COUNT(*) - COUNT(DISTINCT runner_key) FROM runners")
    dup_runners = cur.fetchone()[0]
    if dup_runners > 0:
        errors.append(f"{dup_runners} duplicate runner_keys found")
    
    # Report
    if errors:
        for err in errors:
            log(f"  ERROR: {err}", 'ERROR')
    
    if warnings:
        for warn in warnings:
            log(f"  WARN: {warn}", 'WARN')
    
    if not errors and not warnings:
        log(f"  ✓ All validation checks passed ({race_count:,} races, {race_count_from_runners:,} distinct races)")
    
    return len(errors) == 0

def load_month(file_info):
    """Load a single month's data"""
    region = file_info['region']
    race_type = file_info['race_type']
    year_month = file_info['year_month']
    
    log(f"Loading {region.upper()}/{race_type} {year_month}...")
    
    conn = connect_db('horse_db')
    cur = conn.cursor()
    
    try:
        # Load CSVs to staging
        races_loaded = load_csv_to_staging(cur, file_info['race_file'], 'stage_races')
        runners_loaded = load_csv_to_staging(cur, file_info['runner_file'], 'stage_runners')
        
        # Upsert dimensions
        upsert_dimensions(cur)
        
        # Upsert facts
        races_upserted = upsert_races(cur)
        runners_upserted = upsert_runners(cur)
        
        # Commit
        conn.commit()
        
        # Validate
        valid = validate_data(cur)
        
        # Analyze
        cur.execute("ANALYZE races")
        cur.execute("ANALYZE runners")
        
        log(f"✓ Completed {region.upper()}/{race_type} {year_month}: "
            f"{races_upserted:,} races, {runners_upserted:,} runners")
        
        cur.close()
        conn.close()
        
        return {
            'region': region,
            'race_type': race_type,
            'year_month': year_month,
            'races': races_upserted,
            'runners': runners_upserted,
            'valid': valid
        }
        
    except Exception as e:
        log(f"Error loading {region}/{race_type} {year_month}: {e}", 'ERROR')
        conn.rollback()
        cur.close()
        conn.close()
        raise

def main():
    """Main loader entry point"""
    import argparse
    
    parser = argparse.ArgumentParser(description='Load master CSVs into PostgreSQL')
    parser.add_argument('--init', action='store_true', help='Initialize database and schema')
    parser.add_argument('--region', choices=['gb', 'ire'], help='Filter by region')
    parser.add_argument('--type', choices=['flat', 'jumps'], help='Filter by race type')
    parser.add_argument('--month', help='Filter by year-month (YYYY-MM)')
    parser.add_argument('--limit', type=int, help='Limit number of months to load')
    
    args = parser.parse_args()
    
    log("="*80)
    log("MASTER DATA LOADER - PostgreSQL Ingestion")
    log("="*80)
    
    start_time = time.time()
    
    # Initialize database if requested
    if args.init:
        init_database()
    
    # Discover files
    log("Discovering master files...")
    files = discover_master_files(args.region, args.type, args.month)
    
    if not files:
        log("No files found matching criteria", 'WARN')
        return
    
    log(f"Found {len(files)} month(s) to load")
    
    # Apply limit
    if args.limit:
        files = files[:args.limit]
        log(f"Limited to {len(files)} month(s)")
    
    # Extract years for partitioning
    years = set()
    for f in files:
        year = int(f['year_month'].split('-')[0])
        years.add(year)
    
    # Create partitions
    create_partitions_for_years(sorted(years))
    
    # Load each month
    results = []
    errors = 0
    
    for idx, file_info in enumerate(files, 1):
        log(f"[{idx}/{len(files)}] Processing {file_info['region']}/{file_info['race_type']}/{file_info['year_month']}...")
        
        try:
            result = load_month(file_info)
            results.append(result)
        except Exception as e:
            log(f"Failed to load month: {e}", 'ERROR')
            errors += 1
    
    # Summary
    elapsed = time.time() - start_time
    total_races = sum(r['races'] for r in results)
    total_runners = sum(r['runners'] for r in results)
    
    log("="*80)
    log("LOADING COMPLETE")
    log("="*80)
    log(f"Loaded: {len(results)}/{len(files)} months")
    log(f"Errors: {errors}")
    log(f"Total races: {total_races:,}")
    log(f"Total runners: {total_runners:,}")
    log(f"Elapsed: {elapsed:.1f}s ({elapsed/60:.1f} minutes)")
    log("="*80)

if __name__ == '__main__':
    main()

