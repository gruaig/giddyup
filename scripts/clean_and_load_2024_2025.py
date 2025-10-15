#!/usr/bin/env python3
"""
Clean 2024-2025 master data and load to PostgreSQL
Handles data quality issues: € symbols, em-dashes, etc.
"""
import psycopg2
import csv
import glob
import os
import re
from datetime import datetime

DB_CONFIG = {
    'host': 'localhost',
    'port': 5432,
    'database': 'horse_db',
    'user': 'postgres',
    'password': 'password'
}

MASTER_DIR = "/tmp/master_2024_2025_clean"

def log(msg):
    print(f"[{datetime.now().strftime('%Y-%m-%d %H:%M:%S')}] {msg}")

def clean_numeric(val):
    """Clean numeric fields: remove €,£,$,commas,em-dashes"""
    if not val or val.strip() == '':
        return ''
    # Remove currency symbols, commas, em-dashes
    val = re.sub(r'[€£$,–—]', '', val)
    # Trim
    val = val.strip()
    # If just a dash or empty, return empty
    if val in ['-', '–', '—', '']:
        return ''
    return val

def clean_integer(val):
    """Clean integer fields"""
    cleaned = clean_numeric(val)
    if not cleaned:
        return ''
    # Remove decimal points for integers
    cleaned = cleaned.split('.')[0]
    return cleaned

def load_month(conn, cur, region, race_type, month_dir):
    """Load a single month with data cleaning"""
    race_file = f"{month_dir}/races_{region}_{race_type}_{os.path.basename(month_dir)}.csv"
    runner_file = f"{month_dir}/runners_{region}_{race_type}_{os.path.basename(month_dir)}.csv"
    
    if not os.path.exists(race_file) or not os.path.exists(runner_file):
        return 0, 0
    
    races_loaded = 0
    runners_loaded = 0
    
    # Load races (races rarely have data issues)
    cur.execute("TRUNCATE stage_races, stage_runners")
    
    with open(race_file, 'r', encoding='utf-8') as f:
        next(f)  # Skip header
        cur.copy_from(f, 'stage_races', sep=',', null='')
    
    # Load runners with cleaning
    runner_data = []
    with open(runner_file, 'r', encoding='utf-8') as f:
        reader = csv.DictReader(f)
        for row in reader:
            # Clean numeric fields
            row['num'] = clean_integer(row.get('num', ''))
            row['draw'] = clean_integer(row.get('draw', ''))
            row['age'] = clean_integer(row.get('age', ''))
            row['lbs'] = clean_integer(row.get('lbs', ''))
            row['or'] = clean_integer(row.get('or', ''))
            row['rpr'] = clean_integer(row.get('rpr', ''))
            row['win_lose'] = clean_integer(row.get('win_lose', ''))
            row['place_win_lose'] = clean_integer(row.get('place_win_lose', ''))
            row['match_time_diff_min'] = clean_integer(row.get('match_time_diff_min', ''))
            
            row['ovr_btn'] = clean_numeric(row.get('ovr_btn', ''))
            row['btn'] = clean_numeric(row.get('btn', ''))
            row['secs'] = clean_numeric(row.get('secs', ''))
            row['dec'] = clean_numeric(row.get('dec', ''))
            row['prize'] = clean_numeric(row.get('prize', ''))
            row['win_bsp'] = clean_numeric(row.get('win_bsp', ''))
            row['win_ppwap'] = clean_numeric(row.get('win_ppwap', ''))
            row['win_morningwap'] = clean_numeric(row.get('win_morningwap', ''))
            row['win_ppmax'] = clean_numeric(row.get('win_ppmax', ''))
            row['win_ppmin'] = clean_numeric(row.get('win_ppmin', ''))
            row['win_ipmax'] = clean_numeric(row.get('win_ipmax', ''))
            row['win_ipmin'] = clean_numeric(row.get('win_ipmin', ''))
            row['win_morning_vol'] = clean_numeric(row.get('win_morning_vol', ''))
            row['win_pre_vol'] = clean_numeric(row.get('win_pre_vol', ''))
            row['win_ip_vol'] = clean_numeric(row.get('win_ip_vol', ''))
            row['place_bsp'] = clean_numeric(row.get('place_bsp', ''))
            row['place_ppwap'] = clean_numeric(row.get('place_ppwap', ''))
            row['place_morningwap'] = clean_numeric(row.get('place_morningwap', ''))
            row['place_ppmax'] = clean_numeric(row.get('place_ppmax', ''))
            row['place_ppmin'] = clean_numeric(row.get('place_ppmin', ''))
            row['place_ipmax'] = clean_numeric(row.get('place_ipmax', ''))
            row['place_ipmin'] = clean_numeric(row.get('place_ipmin', ''))
            row['place_morning_vol'] = clean_numeric(row.get('place_morning_vol', ''))
            row['place_pre_vol'] = clean_numeric(row.get('place_pre_vol', ''))
            row['place_ip_vol'] = clean_numeric(row.get('place_ip_vol', ''))
            row['match_jaccard'] = clean_numeric(row.get('match_jaccard', ''))
            
            runner_data.append(row)
    
    # Write cleaned data to temp CSV
    temp_runner_file = '/tmp/cleaned_runners.csv'
    with open(temp_runner_file, 'w', newline='', encoding='utf-8') as f:
        if runner_data:
            writer = csv.DictWriter(f, fieldnames=runner_data[0].keys())
            writer.writeheader()
            writer.writerows(runner_data)
    
    # Copy cleaned data
    with open(temp_runner_file, 'r', encoding='utf-8') as f:
        next(f)  # Skip header
        cur.copy_from(f, 'stage_runners', sep=',', null='')
    
    # Now call the existing V2 upsert logic
    cur.execute("""
        SELECT racing.load_batch_v2()
    """)
    
    result = cur.fetchone()
    if result:
        races_loaded = result[0] if len(result) > 0 else 0
        runners_loaded = result[1] if len(result) > 1 else 0
    
    conn.commit()
    os.remove(temp_runner_file)
    
    return races_loaded, runners_loaded

def main():
    log("="*80)
    log("LOADING CLEANED 2024-2025 DATA")
    log("="*80)
    
    conn = psycopg2.connect(**DB_CONFIG)
    cur = conn.cursor()
    cur.execute("SET search_path TO racing, public")
    conn.commit()
    
    total_races = 0
    total_runners = 0
    
    # Find all month directories
    for region in ['gb', 'ire']:
        for race_type in ['flat', 'jumps']:
            pattern = f"{MASTER_DIR}/{region}/{race_type}/202*"
            month_dirs = glob.glob(pattern)
            
            for month_dir in sorted(month_dirs):
                month_name = os.path.basename(month_dir)
                log(f"Loading {region}/{race_type} {month_name}...")
                
                races, runners = load_month(conn, cur, region, race_type, month_dir)
                total_races += races
                total_runners += runners
                
                log(f"  ✓ {races} races, {runners} runners")
    
    cur.close()
    conn.close()
    
    log("")
    log("="*80)
    log("COMPLETE")
    log("="*80)
    log(f"Total loaded: {total_races:,} races, {total_runners:,} runners")

if __name__ == "__main__":
    main()

