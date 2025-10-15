#!/usr/bin/env python3
"""
Load ONLY 2024-2025 data (Feb 2024 onwards)
Skips data already in database to avoid duplicates
"""
import os
import sys
import glob
import psycopg2
from psycopg2 import extras
import csv
from datetime import datetime

# Add parent directory to path
sys.path.insert(0, os.path.dirname(__file__))

MASTER_DIR = "/home/smonaghan/rpscrape/master"
DB_CONFIG = {
    'host': 'localhost',
    'port': 5432,
    'database': 'horse_db',
    'user': 'postgres',
    'password': 'password'
}

def log(msg):
    print(f"[{datetime.now().strftime('%Y-%m-%d %H:%M:%S')}] {msg}")
    sys.stdout.flush()

def connect_db():
    conn = psycopg2.connect(**DB_CONFIG)
    cur = conn.cursor()
    cur.execute("SET search_path TO racing, public;")
    conn.commit()
    return conn, cur

def get_loaded_months(cur):
    """Get months already in database"""
    cur.execute("""
        SELECT DISTINCT TO_CHAR(race_date, 'YYYY-MM') 
        FROM races 
        WHERE race_date >= '2024-01-01'
        ORDER BY 1
    """)
    return set(row[0] for row in cur.fetchall())

def upsert_dimension(cur, table, name_col, name_val):
    """Upsert dimension and return ID"""
    if not name_val:
        return None
    
    cur.execute(f"""
        INSERT INTO {table} ({name_col}) 
        VALUES (%s) 
        ON CONFLICT ({name_col}) DO UPDATE SET {name_col}={table}.{name_col}
        RETURNING {table.rstrip('s')}_id
    """, (name_val,))
    return cur.fetchone()[0]

def load_month(conn, cur, region, race_type, year, month):
    """Load a single month's data"""
    month_dir = f"{MASTER_DIR}/{region}/{race_type}/{year}-{month}"
    
    if not os.path.exists(month_dir):
        log(f"  No data for {region}/{race_type} {year}-{month}")
        return 0, 0
    
    race_file = f"{month_dir}/races_{region}_{race_type}_{year}-{month}.csv"
    runner_file = f"{month_dir}/runners_{region}_{race_type}_{year}-{month}.csv"
    
    if not os.path.exists(race_file) or not os.path.exists(runner_file):
        log(f"  Missing files for {region}/{race_type} {year}-{month}")
        return 0, 0
    
    log(f"  Loading {region}/{race_type} {year}-{month}")
    
    # Load races
    races_loaded = 0
    runners_loaded = 0
    
    with open(race_file, 'r', encoding='utf-8') as f:
        reader = csv.DictReader(f)
        for row in reader:
            try:
                # Upsert dimensions
                course_id = upsert_dimension(cur, 'courses', 'course_name', row.get('course'))
                
                # Insert race
                cur.execute("""
                    INSERT INTO races (
                        race_key, race_date, region, course_id, off_time, race_name,
                        race_type, class, pattern, rating_band, age_band, sex_rest,
                        dist_raw, dist_f, dist_m, going, surface, ran
                    ) VALUES (
                        %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s
                    )
                    ON CONFLICT (race_key, race_date) DO NOTHING
                """, (
                    row['race_key'], row['date'], row['region'], course_id, row['off'],
                    row['race_name'], row['type'], row['class'], row['pattern'],
                    row['rating_band'], row['age_band'], row['sex_rest'],
                    row['dist'], row['dist_f'] or None, row['dist_m'] or None,
                    row['going'], row['surface'], row['ran'] or None
                ))
                if cur.rowcount > 0:
                    races_loaded += 1
            except Exception as e:
                log(f"    Error loading race: {e}")
                conn.rollback()
                continue
    
    conn.commit()
    
    # Load runners
    with open(runner_file, 'r', encoding='utf-8') as f:
        reader = csv.DictReader(f)
        for row in reader:
            try:
                # Get race_id
                cur.execute("SELECT race_id FROM races WHERE race_key = %s", (row['race_key'],))
                result = cur.fetchone()
                if not result:
                    continue
                race_id = result[0]
                
                # Upsert dimensions
                horse_id = upsert_dimension(cur, 'horses', 'horse_name', row.get('horse'))
                jockey_id = upsert_dimension(cur, 'jockeys', 'jockey_name', row.get('jockey'))
                trainer_id = upsert_dimension(cur, 'trainers', 'trainer_name', row.get('trainer'))
                
                # Parse numeric fields safely
                def safe_float(val):
                    if not val or val == '':
                        return None
                    try:
                        # Remove currency symbols
                        val = val.replace('€', '').replace('£', '').replace(',', '')
                        return float(val)
                    except:
                        return None
                
                def safe_int(val):
                    if not val or val == '':
                        return None
                    try:
                        return int(val)
                    except:
                        return None
                
                # Insert runner
                cur.execute("""
                    INSERT INTO runners (
                        race_id, horse_id, jockey_id, trainer_id,
                        num, pos_num, pos_raw, draw, ovr_btn, btn,
                        age, sex, lbs, hg, time, secs, dec,
                        prize, prize_raw, "or", rpr,
                        sire, dam, damsire, owner, comment,
                        win_bsp, win_ppwap, win_morningwap, win_ppmax, win_ppmin,
                        win_ipmax, win_ipmin, win_morning_vol, win_pre_vol, win_ip_vol, win_lose,
                        place_bsp, place_ppwap, place_morningwap, place_ppmax, place_ppmin,
                        place_ipmax, place_ipmin, place_morning_vol, place_pre_vol, place_ip_vol, place_win_lose,
                        win_flag
                    ) VALUES (
                        %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s,
                        %s, %s, %s, %s, %s, %s, %s, %s, %s,
                        %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s,
                        %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s,
                        %s
                    )
                    ON CONFLICT DO NOTHING
                """, (
                    race_id, horse_id, jockey_id, trainer_id,
                    safe_int(row.get('num')), safe_int(row.get('pos')), row.get('pos'),
                    safe_int(row.get('draw')), safe_float(row.get('ovr_btn')), safe_float(row.get('btn')),
                    safe_int(row.get('age')), row.get('sex'), safe_int(row.get('lbs')),
                    row.get('hg'), row.get('time'), safe_float(row.get('secs')), safe_float(row.get('dec')),
                    safe_float(row.get('prize')), row.get('prize_raw'), safe_int(row.get('or')), safe_int(row.get('rpr')),
                    row.get('sire'), row.get('dam'), row.get('damsire'), row.get('owner'), row.get('comment'),
                    safe_float(row.get('win_bsp')), safe_float(row.get('win_ppwap')), safe_float(row.get('win_morningwap')),
                    safe_float(row.get('win_ppmax')), safe_float(row.get('win_ppmin')),
                    safe_float(row.get('win_ipmax')), safe_float(row.get('win_ipmin')),
                    safe_float(row.get('win_morning_vol')), safe_float(row.get('win_pre_vol')),
                    safe_float(row.get('win_ip_vol')), safe_int(row.get('win_lose')),
                    safe_float(row.get('place_bsp')), safe_float(row.get('place_ppwap')), safe_float(row.get('place_morningwap')),
                    safe_float(row.get('place_ppmax')), safe_float(row.get('place_ppmin')),
                    safe_float(row.get('place_ipmax')), safe_float(row.get('place_ipmin')),
                    safe_float(row.get('place_morning_vol')), safe_float(row.get('place_pre_vol')),
                    safe_float(row.get('place_ip_vol')), safe_int(row.get('place_win_lose')),
                    safe_int(row.get('pos')) == 1
                ))
                if cur.rowcount > 0:
                    runners_loaded += 1
            except Exception as e:
                log(f"    Error loading runner: {e}")
                conn.rollback()
                continue
    
    conn.commit()
    log(f"    ✓ Loaded {races_loaded} races, {runners_loaded} runners")
    return races_loaded, runners_loaded

def main():
    log("="*80)
    log("LOADING NEW 2024-2025 DATA ONLY")
    log("="*80)
    
    conn, cur = connect_db()
    
    # Get months already loaded
    loaded_months = get_loaded_months(cur)
    log(f"Already loaded months: {sorted(loaded_months)}")
    log("")
    
    total_races = 0
    total_runners = 0
    
    # Load missing months
    for year in [2024, 2025]:
        max_month = 12 if year == 2024 else 10
        for month in range(2, max_month + 1):  # Start from Feb (Jan already loaded)
            month_str = f"{year}-{month:02d}"
            
            if month_str in loaded_months:
                log(f"Skipping {month_str} (already loaded)")
                continue
            
            log(f"Processing {month_str}:")
            for region in ['gb', 'ire']:
                for race_type in ['flat', 'jumps']:
                    races, runners = load_month(conn, cur, region, race_type, str(year), f"{month:02d}")
                    total_races += races
                    total_runners += runners
    
    cur.close()
    conn.close()
    
    log("")
    log("="*80)
    log("COMPLETE")
    log("="*80)
    log(f"Total loaded: {total_races} races, {total_runners} runners")

if __name__ == "__main__":
    main()

