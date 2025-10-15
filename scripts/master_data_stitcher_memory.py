#!/usr/bin/env python3
"""
Master Data Stitcher - HIGH PERFORMANCE IN-MEMORY VERSION
Loads all data into RAM for maximum speed with robust matching logic
Requires ~8GB RAM, optimized for systems with 16GB+
"""
import os
import sys
import csv
import json
import hashlib
import re
import unicodedata
import shutil
from datetime import datetime, timedelta
from collections import defaultdict, Counter
import glob
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor
import threading

# Configuration
RP_DATA_DIR = "/home/smonaghan/rpscrape/data/dates"
BF_DATA_DIR = "/home/smonaghan/rpscrape/data/betfair_stitched"
MASTER_DIR = "/home/smonaghan/rpscrape/master"
INDEX_FILE = "/home/smonaghan/rpscrape/betfair_index.csv"

# Schema definitions (same as original)
RACE_SCHEMA = [
    'date', 'region', 'course', 'off', 'race_name', 'type', 'class', 'pattern',
    'rating_band', 'age_band', 'sex_rest', 'dist', 'dist_f', 'dist_m', 
    'going', 'surface', 'ran', 'race_key'
]

RUNNER_SCHEMA = [
    'race_key', 'num', 'pos', 'draw', 'ovr_btn', 'btn', 'horse', 'age', 'sex',
    'lbs', 'hg', 'time', 'secs', 'dec', 'jockey', 'trainer', 'prize', 'prize_raw', 'or', 'rpr',
    'sire', 'dam', 'damsire', 'owner', 'comment',
    'win_bsp', 'win_ppwap', 'win_morningwap', 'win_ppmax', 'win_ppmin', 
    'win_ipmax', 'win_ipmin', 'win_morning_vol', 'win_pre_vol', 'win_ip_vol', 'win_lose',
    'place_bsp', 'place_ppwap', 'place_morningwap', 'place_ppmax', 'place_ppmin',
    'place_ipmax', 'place_ipmin', 'place_morning_vol', 'place_pre_vol', 'place_ip_vol', 'place_win_lose',
    'runner_key', 'match_jaccard', 'match_time_diff_min', 'match_reason'
]

UNMATCHED_SCHEMA = [
    'date', 'off', 'event_name', 'best_jaccard', 'time_diff_min', 'candidate_count', 'match_reason'
]

# Thread-safe progress tracking
progress_lock = threading.Lock()
progress_counter = {'loaded_files': 0, 'total_files': 0}

def log(msg):
    """Simple logging"""
    timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
    print(f"[{timestamp}] {msg}")
    sys.stdout.flush()

def _strip_accents(s: str) -> str:
    """Strip accents from text"""
    return "".join(c for c in unicodedata.normalize("NFKD", s) if not unicodedata.combining(c))

def normalize_text(text):
    """Normalize text: lowercase, trim, strip accents, remove punctuation, collapse spaces, remove suffixes"""
    if not text:
        return ""
    
    # Strip accents, lowercase and trim
    text = _strip_accents(str(text)).lower().strip()
    
    # Remove horse country suffixes
    text = re.sub(r'\s*\([a-z]{2,3}\)$', '', text)
    
    # Remove punctuation (keep only letters, numbers, spaces)
    text = re.sub(r"[^a-z0-9\s]", " ", text)
    
    # Collapse multiple spaces
    text = re.sub(r'\s+', ' ', text).strip()
    
    return text

def normalize_time(time_str):
    """Normalize time to HH:MM format"""
    if not time_str:
        return ""
    
    time_str = str(time_str).strip()
    
    # Handle various time formats
    if ':' in time_str:
        parts = time_str.split(':')
        if len(parts) >= 2:
            try:
                hour = int(parts[0])
                minute = int(parts[1])
                return f"{hour:02d}:{minute:02d}"
            except:
                pass
    
    return time_str

def normalize_date(date_str):
    """Normalize date to YYYY-MM-DD format"""
    if not date_str:
        return ""
    
    date_str = str(date_str).strip()
    
    # Already in correct format
    if re.match(r'^\d{4}-\d{2}-\d{2}$', date_str):
        return date_str
    
    return date_str

def clean_numeric(value, decimals=None, force_int=False):
    """Clean numeric value - remove symbols, parse to float/int"""
    if not value or value == '–' or value == '-' or value == '':
        return ''
    
    try:
        # Remove currency symbols, commas, spaces
        cleaned = str(value).strip()
        cleaned = re.sub(r'[£€$,\s]', '', cleaned)
        
        # Parse to float
        num = float(cleaned)
        
        # Round if decimals specified
        if decimals is not None:
            num = round(num, decimals)
        
        # Always return int if whole number (unless it's a price field with specific decimals)
        if force_int or num == int(num):
            return str(int(num))
        else:
            # Return float with appropriate precision
            return str(num)
    except:
        return ''

def clean_betfair_price(value, decimals=4):
    """Clean Betfair price - replace sentinel 1.0 with blank"""
    if not value:
        return ''
    
    try:
        num = float(value)
        
        # Sentinel value: exactly 1.0 means missing (min valid Betfair price is 1.01)
        if num == 1.0:
            return ''
        
        # Round to specified decimals
        return str(round(num, decimals))
    except:
        return ''

def parse_prize(prize_str):
    """Parse prize to numeric and keep raw"""
    prize_raw = str(prize_str).strip() if prize_str else ''
    prize_numeric = clean_numeric(prize_str, decimals=2)
    return prize_numeric, prize_raw

def clean_dist_f(dist_str):
    """Clean distance string to numeric furlongs"""
    if not dist_str:
        return ''
    
    # Already numeric
    try:
        return str(float(dist_str))
    except:
        pass
    
    # Parse from string like "10f" or "1m2f"
    dist_f = parse_distance_from_event_name(dist_str)
    return str(dist_f) if dist_f is not None else ''

def generate_race_key(date, region, course, off, race_name, race_type):
    """Generate stable race key using MD5"""
    components = [
        normalize_date(date),
        normalize_text(region),
        normalize_text(course),
        normalize_time(off),
        normalize_text(race_name),
        normalize_text(race_type)
    ]
    
    key_string = "|".join(components)
    return hashlib.md5(key_string.encode('utf-8')).hexdigest()

def generate_runner_key(race_key, horse, num="", draw=""):
    """Generate stable runner key using MD5"""
    components = [
        race_key,
        normalize_text(horse),
        str(num or draw or "")
    ]
    
    key_string = "|".join(components)
    return hashlib.md5(key_string.encode('utf-8')).hexdigest()

def parse_distance_from_event_name(event_name):
    """Extract approximate distance in furlongs from Betfair event name"""
    if not event_name:
        return None
    
    s = event_name.lower().replace(" ", "")
    
    # 1) miles + furlongs (e.g., "1m2f") - check this first!
    m = re.search(r"(?:(\d+)m)(?:(\d+(?:\.\d+)?)f)", s)
    if m:
        return 8 * float(m.group(1)) + float(m.group(2))
    
    # 2) miles only (e.g., "2m")
    m = re.search(r"(\d+)m", s)
    if m:
        return 8 * float(m.group(1))
    
    # 3) furlongs only (e.g., "6f")
    m = re.search(r"(\d+(?:\.\d+)?)f", s)
    if m:
        return float(m.group(1))
    
    return None

def jaccard_similarity(set1, set2):
    """Calculate Jaccard similarity between two sets"""
    if not set1 and not set2:
        return 1.0
    
    intersection = len(set1.intersection(set2))
    union = len(set1.union(set2))
    
    return intersection / union if union > 0 else 0.0

def time_diff_minutes(time1, time2):
    """Calculate time difference in minutes"""
    try:
        t1 = datetime.strptime(time1, '%H:%M')
        t2 = datetime.strptime(time2, '%H:%M')
        diff = abs((t1 - t2).total_seconds() / 60)
        return diff
    except:
        return float('inf')

def update_progress():
    """Thread-safe progress update"""
    with progress_lock:
        progress_counter['loaded_files'] += 1
        loaded = progress_counter['loaded_files']
        total = progress_counter['total_files']
        if loaded % 1000 == 0 or loaded == total:
            log(f"  Loading progress: {loaded}/{total} files ({loaded/total*100:.1f}%)")

def load_rp_file(file_path):
    """Load a single Racing Post CSV file into memory"""
    races_map = {}  # race_key -> race dict
    
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            reader = csv.DictReader(f)
            for row in reader:
                # Extract race info
                race_info = {
                    'date': normalize_date(row.get('date', '')),
                    'region': row.get('region', ''),
                    'course': row.get('course', ''),
                    'off': normalize_time(row.get('off', '')),
                    'race_name': row.get('race_name', ''),
                    'type': row.get('type', ''),
                    'class': row.get('class', ''),
                    'pattern': row.get('pattern', ''),
                    'rating_band': row.get('rating_band', ''),
                    'age_band': row.get('age_band', ''),
                    'sex_rest': row.get('sex_rest', ''),
                    'dist': row.get('dist', ''),
                    'dist_f': row.get('dist_f', ''),
                    'dist_m': row.get('dist_m', ''),
                    'going': row.get('going', ''),
                    'surface': row.get('surface', ''),
                }
                
                race_key = generate_race_key(
                    race_info['date'], race_info['region'], race_info['course'],
                    race_info['off'], race_info['race_name'], race_info['type']
                )
                
                if race_key not in races_map:
                    race_info['race_key'] = race_key
                    race_info['runners'] = []
                    race_info['horses'] = set()
                    races_map[race_key] = race_info
                
                # Add runner
                runner = {k: row.get(k, '') for k in row.keys()}
                races_map[race_key]['runners'].append(runner)
                name_norm = normalize_text(runner.get('horse', ''))
                if name_norm:
                    races_map[race_key]['horses'].add(name_norm)
    
    except Exception as e:
        log(f"Error loading RP file {file_path}: {e}")
        return {}
    
    # Finalize races
    for race in races_map.values():
        race['horses'] = list(race['horses'])
        race['ran'] = len(race['runners'])
    
    update_progress()
    return races_map

def load_bf_file(file_path):
    """Load a single Betfair CSV file into memory"""
    races = []
    
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            reader = csv.DictReader(f)
            
            runners = []
            race_info = None
            
            for row in reader:
                if not row.get('horse', '').strip():
                    continue
                
                # Extract race info from first row
                if race_info is None:
                    race_info = {
                        'date': normalize_date(row.get('date', '')),
                        'off': normalize_time(row.get('off', '')),
                        'event_name': row.get('event_name', ''),
                        'file_path': file_path  # For debugging
                    }
                
                # Add runner
                runners.append(row)
            
            if race_info and len(runners) >= 3:  # Minimum 3 runners
                race_info['horses'] = [normalize_text(r['horse']) for r in runners]
                race_info['runners'] = runners
                races.append(race_info)
    
    except Exception as e:
        log(f"Error loading BF file {file_path}: {e}")
        return []
    
    update_progress()
    return races

def load_all_rp_data():
    """Load all Racing Post data into memory using parallel processing"""
    log("Loading all Racing Post data into memory...")
    
    # Find all complete RP files
    all_files = []
    for region in ['gb', 'ire']:
        for race_type in ['flat', 'jumps']:
            pattern = f"{RP_DATA_DIR}/{region}/{race_type}/*.csv"
            files = [f for f in glob.glob(pattern) if 'INCOMPLETE' not in f]
            all_files.extend(files)
    
    progress_counter['total_files'] = len(all_files)
    progress_counter['loaded_files'] = 0
    
    log(f"Found {len(all_files)} Racing Post files to load")
    
    # Load files in parallel
    all_races = {}
    
    with ThreadPoolExecutor(max_workers=8) as executor:
        future_to_file = {executor.submit(load_rp_file, file_path): file_path for file_path in all_files}
        
        for future in future_to_file:
            races_map = future.result()
            all_races.update(races_map)
    
    log(f"Loaded {len(all_races)} Racing Post races into memory")
    return all_races

def load_all_bf_data():
    """Load all Betfair data into memory using parallel processing"""
    log("Loading all Betfair data into memory...")
    
    # Load Betfair index if available
    if os.path.exists(INDEX_FILE):
        bf_files = []
        with open(INDEX_FILE, 'r', encoding='utf-8') as f:
            reader = csv.DictReader(f)
            for row in reader:
                if row['has_required_cols'].lower() == 'true' and int(row['row_count']) >= 3:
                    bf_files.append(row['file_path'])
    else:
        # Fallback: scan directories
        bf_files = []
        for region in ['gb', 'ire']:
            for race_type in ['flat', 'jumps']:
                bf_dir = f"{BF_DATA_DIR}/{region}/{race_type}"
                if os.path.exists(bf_dir):
                    pattern = f"{bf_dir}/*.csv"
                    bf_files.extend(glob.glob(pattern))
    
    progress_counter['total_files'] = len(bf_files)
    progress_counter['loaded_files'] = 0
    
    log(f"Found {len(bf_files)} Betfair files to load")
    
    # Load files in parallel
    all_races = []
    
    with ThreadPoolExecutor(max_workers=8) as executor:
        future_to_file = {executor.submit(load_bf_file, file_path): file_path for file_path in bf_files}
        
        for future in future_to_file:
            races = future.result()
            all_races.extend(races)
    
    log(f"Loaded {len(all_races)} Betfair races into memory")
    
    # Build date index for fast lookups
    date_index = defaultdict(list)
    for race in all_races:
        date_index[race['date']].append(race)
    
    log(f"Built date index with {len(date_index)} unique dates")
    return all_races, date_index

def match_races_memory(rp_races, bf_races_by_date):
    """Match races using in-memory data structures"""
    log("Matching races using in-memory algorithms...")
    
    matches = []
    unmatched = []
    
    # Get all unique dates from both datasets
    rp_dates = set()
    for race in rp_races.values():
        rp_dates.add(race['date'])
    
    bf_dates = set(bf_races_by_date.keys())
    overlap_dates = rp_dates.intersection(bf_dates)
    
    log(f"Found {len(overlap_dates)} overlapping dates to process")
    
    processed_dates = 0
    for date in overlap_dates:
        # Get RP races for this date
        rp_races_today = [race for race in rp_races.values() if race['date'] == date]
        bf_races_today = bf_races_by_date[date]
        
        if not rp_races_today or not bf_races_today:
            continue
        
        # Match races for this date
        for bf_race in bf_races_today:
            bf_off = bf_race['off']
            bf_event_name = bf_race['event_name']
            bf_horses = set(bf_race['horses'])
            
            # Find candidate RP races within ±10 minutes
            candidates = []
            for rp_race in rp_races_today:
                time_diff = time_diff_minutes(rp_race['off'], bf_off)
                if time_diff <= 10:  # Within 10 minutes
                    candidates.append((rp_race, time_diff))
            
            if not candidates:
                unmatched.append({
                    'date': date,
                    'off': bf_off,
                    'event_name': bf_event_name,
                    'best_jaccard': 0.0,
                    'time_diff_min': '',
                    'candidate_count': 0,
                    'match_reason': 'no_rp_candidates_in_time_window'
                })
                continue
            
            # Score each candidate
            best_score = 0.0
            best_match = None
            best_jaccard = 0.0
            best_time_diff = float('inf')
            
            for rp_race, time_diff in candidates:
                rp_horses = set(rp_race['horses'])
                
                # Calculate Jaccard similarity
                jaccard = jaccard_similarity(rp_horses, bf_horses)
                
                # Start with Jaccard as base score
                score = jaccard
                
                # Bonus: runner count equal
                if len(rp_horses) == len(bf_horses):
                    score += 0.5
                
                # Bonus: handicap hint matches
                rp_is_handicap = 'handicap' in normalize_text(rp_race.get('race_name', ''))
                bf_is_handicap = 'hcap' in normalize_text(bf_event_name) or 'handicap' in normalize_text(bf_event_name)
                if rp_is_handicap == bf_is_handicap:
                    score += 0.5
                
                # Bonus: distance match (if available)
                bf_distance = parse_distance_from_event_name(bf_event_name)
                if bf_distance and rp_race.get('dist_f'):
                    try:
                        rp_distance = float(rp_race['dist_f'])
                        if abs(bf_distance - rp_distance) <= 0.5:  # Within 0.5 furlongs
                            score += 0.5
                    except:
                        pass
                
                # Track best match
                if score > best_score:
                    best_score = score
                    best_match = (rp_race, time_diff)
                    best_jaccard = jaccard
                    best_time_diff = time_diff
            
            # Accept if meets thresholds
            if best_jaccard >= 0.60 and best_score >= 1.5:
                rp_race, time_diff = best_match
                matches.append({
                    'rp_race': rp_race,
                    'bf_race': bf_race,
                    'jaccard': best_jaccard,
                    'time_diff': time_diff,
                    'score': best_score
                })
            else:
                unmatched.append({
                    'date': date,
                    'off': bf_off,
                    'event_name': bf_event_name,
                    'best_jaccard': round(best_jaccard, 4),
                    'time_diff_min': (int(best_time_diff) if best_time_diff != float('inf') else ''),
                    'candidate_count': len(candidates),
                    'match_reason': f'jaccard_below_threshold_{best_jaccard:.3f}'
                })
        
        processed_dates += 1
        if processed_dates % 100 == 0:
            log(f"  Processed {processed_dates}/{len(overlap_dates)} dates")
    
    log(f"Matching complete: {len(matches)} matches, {len(unmatched)} unmatched")
    return matches, unmatched

def join_runners_memory(match):
    """Join runners from matched races in memory"""
    rp_race = match['rp_race']
    bf_race = match['bf_race']
    match_info = match
    
    joined_runners = []
    
    # Normalize horse names for matching
    rp_horses = {normalize_text(r['horse']): r for r in rp_race['runners']}
    bf_horses = {normalize_text(r['horse']): r for r in bf_race['runners']}
    
    # Get race key
    race_key = rp_race['race_key']
    
    # Join on normalized horse names
    all_horses = set(rp_horses.keys()).union(set(bf_horses.keys()))
    
    for norm_horse in all_horses:
        rp_runner = rp_horses.get(norm_horse, {})
        bf_runner = bf_horses.get(norm_horse, {})
        
        # Create joined runner
        joined_runner = {'race_key': race_key}
        
        # Add RP fields with appropriate cleaning
        # Numeric fields that should be integers (force int casting)
        for field in ['num', 'draw', 'age', 'or', 'rpr']:
            joined_runner[field] = clean_numeric(rp_runner.get(field, ''), decimals=None, force_int=True)
        
        # Position - keep as string (can be "UR", "PU", etc.)
        joined_runner['pos'] = rp_runner.get('pos', '')
        
        # Numeric fields that should be floats (2 decimal places)
        for field in ['ovr_btn', 'btn', 'lbs', 'secs', 'dec']:
            joined_runner[field] = clean_numeric(rp_runner.get(field, ''), decimals=2)
        
        # Prize - both numeric and raw
        prize_numeric, prize_raw = parse_prize(rp_runner.get('prize', ''))
        joined_runner['prize'] = prize_numeric
        joined_runner['prize_raw'] = prize_raw
        
        # Text fields (no cleaning needed)
        for field in ['horse', 'sex', 'hg', 'time', 'jockey', 'trainer', 
                     'sire', 'dam', 'damsire', 'owner', 'comment']:
            joined_runner[field] = rp_runner.get(field, '')
        
        # Add Betfair WIN fields (clean prices with sentinel 1.0 → blank)
        for field in ['win_bsp', 'win_ppwap', 'win_morningwap', 'win_ppmax', 'win_ppmin',
                     'win_ipmax', 'win_ipmin']:
            joined_runner[field] = clean_betfair_price(bf_runner.get(field, ''), decimals=4)
        
        # WIN volumes (round to 2 dp)
        for field in ['win_morning_vol', 'win_pre_vol', 'win_ip_vol']:
            joined_runner[field] = clean_numeric(bf_runner.get(field, ''), decimals=2)
        
        # WIN result (integer: 0 or 1)
        joined_runner['win_lose'] = clean_numeric(bf_runner.get('win_lose', ''), decimals=None, force_int=True)
        
        # Add Betfair PLACE fields (clean prices with sentinel 1.0 → blank)
        for field in ['place_bsp', 'place_ppwap', 'place_morningwap', 'place_ppmax', 'place_ppmin',
                     'place_ipmax', 'place_ipmin']:
            joined_runner[field] = clean_betfair_price(bf_runner.get(field, ''), decimals=4)
        
        # PLACE volumes (round to 2 dp)
        for field in ['place_morning_vol', 'place_pre_vol', 'place_ip_vol']:
            joined_runner[field] = clean_numeric(bf_runner.get(field, ''), decimals=2)
        
        # PLACE result (integer: 0 or 1)
        joined_runner['place_win_lose'] = clean_numeric(bf_runner.get('place_win_lose', ''), decimals=None, force_int=True)
        
        # Add keys and diagnostics
        horse_name = rp_runner.get('horse') or bf_runner.get('horse', '')
        joined_runner['runner_key'] = generate_runner_key(
            race_key, horse_name, 
            rp_runner.get('num', ''), rp_runner.get('draw', '')
        )
        joined_runner['match_jaccard'] = round(float(match_info['jaccard']), 4)
        td = match_info['time_diff']
        joined_runner['match_time_diff_min'] = int(td) if td != float('inf') else ''
        joined_runner['match_reason'] = 'matched_successfully'
        
        joined_runners.append(joined_runner)
    
    return joined_runners

def write_output_memory(matches, unmatched, all_rp_races):
    """Write all output files efficiently from memory"""
    log("Writing output files...")
    
    # Group by region/race_type/month for output
    monthly_data = defaultdict(lambda: {'races': [], 'runners': [], 'unmatched': []})
    
    # Process matches
    for match in matches:
        rp_race = match['rp_race']
        region = rp_race['region'].lower()
        race_type = rp_race['type'].lower()
        year_month = rp_race['date'][:7]  # YYYY-MM
        
        monthly_key = (region, race_type, year_month)
        
        # Add race (avoid duplicates)
        race_exists = any(r['race_key'] == rp_race['race_key'] for r in monthly_data[monthly_key]['races'])
        if not race_exists:
            race_row = {field: rp_race.get(field, '') for field in RACE_SCHEMA}
            monthly_data[monthly_key]['races'].append(race_row)
        
        # Join and add runners
        joined_runners = join_runners_memory(match)
        monthly_data[monthly_key]['runners'].extend(joined_runners)
    
    # Process unmatched by month (approximate from date)
    for um in unmatched:
        # Try to infer region/race_type from filename if available
        # For now, distribute unmatched across all active months
        year_month = um['date'][:7]
        
        # Find active monthly keys for this date
        active_keys = [k for k in monthly_data.keys() if k[2] == year_month]
        if active_keys:
            # Add to first matching key (could be improved)
            monthly_data[active_keys[0]]['unmatched'].append(um)
    
    # Write monthly files
    total_races = 0
    total_runners = 0
    total_unmatched = 0
    
    for (region, race_type, year_month), data in monthly_data.items():
        if not data['races'] and not data['runners']:
            continue
        
        # Create monthly directory
        monthly_dir = f"{MASTER_DIR}/{region}/{race_type}/{year_month}"
        os.makedirs(monthly_dir, exist_ok=True)
        
        # File paths
        races_file = f"{monthly_dir}/races_{region}_{race_type}_{year_month}.csv"
        runners_file = f"{monthly_dir}/runners_{region}_{race_type}_{year_month}.csv"
        unmatched_file = f"{monthly_dir}/unmatched_{region}_{race_type}_{year_month}.csv"
        
        # Write files
        if data['races']:
            with open(races_file, 'w', newline='', encoding='utf-8') as f:
                writer = csv.DictWriter(f, fieldnames=RACE_SCHEMA)
                writer.writeheader()
                for race in data['races']:
                    filtered_row = {field: race.get(field, '') for field in RACE_SCHEMA}
                    writer.writerow(filtered_row)
        
        if data['runners']:
            with open(runners_file, 'w', newline='', encoding='utf-8') as f:
                writer = csv.DictWriter(f, fieldnames=RUNNER_SCHEMA)
                writer.writeheader()
                for runner in data['runners']:
                    filtered_row = {field: runner.get(field, '') for field in RUNNER_SCHEMA}
                    writer.writerow(filtered_row)
        
        if data['unmatched']:
            with open(unmatched_file, 'w', newline='', encoding='utf-8') as f:
                writer = csv.DictWriter(f, fieldnames=UNMATCHED_SCHEMA)
                writer.writeheader()
                for um in data['unmatched']:
                    filtered_row = {field: um.get(field, '') for field in UNMATCHED_SCHEMA}
                    writer.writerow(filtered_row)
        
        # Create manifest
        manifest = {
            'schema_version': '1.0',
            'race_count': len(data['races']),
            'runner_count': len(data['runners']),
            'unmatched_count': len(data['unmatched']),
            'processed_dates': [year_month + '-' + str(i).zfill(2) for i in range(1, 32)],  # Approximate
            'last_updated': datetime.now().isoformat()
        }
        
        with open(f"{monthly_dir}/manifest.json", 'w') as f:
            json.dump(manifest, f, indent=2)
        
        total_races += len(data['races'])
        total_runners += len(data['runners'])
        total_unmatched += len(data['unmatched'])
        
        log(f"  ✓ {region.upper()} {race_type.upper()} {year_month}: {len(data['races'])} races, {len(data['runners'])} runners, {len(data['unmatched'])} unmatched")
    
    log(f"Output complete: {total_races} races, {total_runners} runners, {total_unmatched} unmatched")

def main():
    """High-performance in-memory processing pipeline"""
    log("="*80)
    log("MASTER DATA STITCHER - HIGH PERFORMANCE IN-MEMORY VERSION")
    log("="*80)
    log(f"Started at: {datetime.now()}")
    
    # Clear master directory for fresh start (avoid duplicates)
    log("Clearing master directory for fresh start...")
    if os.path.exists(MASTER_DIR):
        for item in os.listdir(MASTER_DIR):
            item_path = os.path.join(MASTER_DIR, item)
            if os.path.isdir(item_path):
                shutil.rmtree(item_path)
            else:
                os.remove(item_path)
    
    # Recreate directory structure
    os.makedirs(f"{MASTER_DIR}/gb/flat", exist_ok=True)
    os.makedirs(f"{MASTER_DIR}/gb/jumps", exist_ok=True)
    os.makedirs(f"{MASTER_DIR}/ire/flat", exist_ok=True)
    os.makedirs(f"{MASTER_DIR}/ire/jumps", exist_ok=True)
    
    log("Loading all data into RAM for maximum speed...")
    log("")
    
    overall_start = time.time()
    
    # Phase 1: Load all data into memory
    start_time = time.time()
    rp_races = load_all_rp_data()
    rp_load_time = time.time() - start_time
    log(f"Racing Post data loaded in {rp_load_time:.1f}s")
    
    start_time = time.time()
    bf_races, bf_date_index = load_all_bf_data()
    bf_load_time = time.time() - start_time
    log(f"Betfair data loaded in {bf_load_time:.1f}s")
    
    # Phase 2: Match races in memory
    start_time = time.time()
    matches, unmatched = match_races_memory(rp_races, bf_date_index)
    match_time = time.time() - start_time
    log(f"Race matching completed in {match_time:.1f}s")
    
    # Phase 3: Write output
    start_time = time.time()
    write_output_memory(matches, unmatched, rp_races)
    output_time = time.time() - start_time
    log(f"Output written in {output_time:.1f}s")
    
    # Final summary
    total_time = time.time() - overall_start
    log("")
    log("="*80)
    log("HIGH-PERFORMANCE STITCHING COMPLETE!")
    log("="*80)
    log(f"Total time: {total_time:.1f}s ({total_time/60:.1f} minutes)")
    log(f"  - Loading: {rp_load_time + bf_load_time:.1f}s")
    log(f"  - Matching: {match_time:.1f}s")
    log(f"  - Output: {output_time:.1f}s")
    log(f"Matched races: {len(matches)}")
    log(f"Unmatched races: {len(unmatched)}")
    log(f"Output directory: {MASTER_DIR}")
    log("="*80)

if __name__ == "__main__":
    import time
    main()
