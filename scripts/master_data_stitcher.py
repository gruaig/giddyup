#!/usr/bin/env python3
"""
Master Data Stitcher - Racing Post + Betfair Integration
Produces monthly partitioned master data with exact matching logic
"""
import os
import sys
import csv
import json
import gzip
import hashlib
import re
import unicodedata
from datetime import datetime, timedelta
from collections import defaultdict, Counter
import glob
from pathlib import Path

# Configuration
RP_DATA_DIR = "/home/smonaghan/rpscrape/data/dates"
BF_DATA_DIR = "/home/smonaghan/rpscrape/data/betfair_stitched"
MASTER_DIR = "/home/smonaghan/rpscrape/master"
INDEX_FILE = "/home/smonaghan/rpscrape/betfair_index.csv"

# Schema definitions
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
    'date', 'off', 'event_name', 'best_jaccard', 'time_diff_min', 'candidate_count', 'reason'
]

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

def clean_dist_f(dist_f_str):
    """Clean distance in furlongs - convert to numeric"""
    if not dist_f_str:
        return ''
    
    try:
        # Remove 'f' suffix and any whitespace
        cleaned = str(dist_f_str).strip().lower().replace('f', '').strip()
        
        # Parse to float
        dist = float(cleaned)
        return str(dist)
    except:
        return ''

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

def match_races(rp_races, bf_races):
    """Match Racing Post races with Betfair races using scoring logic"""
    matches = []
    unmatched = []
    
    for bf_race in bf_races:
        bf_date = bf_race['date']
        bf_off = bf_race['off']
        bf_event_name = bf_race['event_name']
        bf_horses = set(normalize_text(h) for h in bf_race['horses'])
        
        # Find candidate RP races on same date within ±10 minutes
        candidates = []
        for rp_race in rp_races:
            if rp_race['date'] != bf_date:
                continue
            
            time_diff = time_diff_minutes(rp_race['off'], bf_off)
            if time_diff <= 10:  # Within 10 minutes
                candidates.append((rp_race, time_diff))
        
        if not candidates:
            unmatched.append({
                'date': bf_date,
                'off': bf_off,
                'event_name': bf_event_name,
                'best_jaccard': 0.0,
                'time_diff_min': '',
                'candidate_count': 0,
                'reason': 'no_rp_candidates_in_time_window'
            })
            continue
        
        # Score each candidate
        best_score = 0.0
        best_match = None
        best_jaccard = 0.0
        best_time_diff = float('inf')
        
        for rp_race, time_diff in candidates:
            rp_horses = set(normalize_text(h) for h in rp_race['horses'])
            
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
            # Determine reason for failure
            if best_jaccard < 0.60:
                reason = f"jaccard_below_threshold_{best_jaccard:.3f}"
            elif best_score < 1.5:
                reason = f"score_below_threshold_{best_score:.3f}"
            else:
                reason = "unknown"
            
            unmatched.append({
                'date': bf_date,
                'off': bf_off,
                'event_name': bf_event_name,
                'best_jaccard': round(best_jaccard, 4),
                'time_diff_min': (int(best_time_diff) if best_time_diff != float('inf') else ''),
                'candidate_count': len(candidates),
                'reason': reason
            })
    
    return matches, unmatched

def join_runners(rp_race, bf_race, match_info):
    """Join runners from matched races"""
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

def build_betfair_index():
    """Build betfair_index.csv for efficient processing"""
    log("Building Betfair index...")
    
    index_data = []
    
    for region in ['gb', 'ire']:
        for race_type in ['flat', 'jumps']:
            bf_dir = f"{BF_DATA_DIR}/{region}/{race_type}"
            if not os.path.exists(bf_dir):
                continue
            
            pattern = f"{bf_dir}/*.csv"
            files = glob.glob(pattern)
            
            for file_path in files:
                try:
                    # Extract date from filename
                    filename = os.path.basename(file_path)
                    # Format: {region}_{race_type}_YYYY-MM-DD_HHMM.csv
                    match = re.search(r'(\d{4}-\d{2}-\d{2})', filename)
                    if not match:
                        continue
                    
                    date = match.group(1)
                    
                    # Check file validity
                    row_count = 0
                    has_required_cols = False
                    
                    with open(file_path, 'r', encoding='utf-8') as f:
                        reader = csv.DictReader(f)
                        
                        # Check headers
                        required_fields = ['date', 'off', 'event_name', 'horse']
                        has_required_cols = all(field in reader.fieldnames for field in required_fields)
                        
                        if has_required_cols:
                            for row in reader:
                                if row.get('horse', '').strip():  # Non-empty horse
                                    row_count += 1
                    
                    # Skip if less than 3 runners (incomplete)
                    if row_count < 3:
                        continue
                    
                    index_data.append({
                        'region': region,
                        'race_type': race_type,
                        'date': date,
                        'file_path': file_path,
                        'row_count': row_count,
                        'has_required_cols': has_required_cols
                    })
                
                except Exception as e:
                    log(f"Error processing {file_path}: {e}")
                    continue
    
    # Write index
    with open(INDEX_FILE, 'w', newline='', encoding='utf-8') as f:
        writer = csv.DictWriter(f, fieldnames=['region', 'race_type', 'date', 'file_path', 'row_count', 'has_required_cols'])
        writer.writeheader()
        writer.writerows(index_data)
    
    log(f"Betfair index built: {len(index_data)} valid files")
    return index_data

def load_betfair_index():
    """Load existing Betfair index or build new one"""
    if os.path.exists(INDEX_FILE):
        log("Loading existing Betfair index...")
        index_data = []
        with open(INDEX_FILE, 'r', encoding='utf-8') as f:
            reader = csv.DictReader(f)
            for row in reader:
                row['row_count'] = int(row['row_count'])
                row['has_required_cols'] = row['has_required_cols'].lower() == 'true'
                index_data.append(row)
        log(f"Loaded {len(index_data)} entries from index")
        return index_data
    else:
        return build_betfair_index()

def load_rp_day(region, race_type, date):
    """Load Racing Post data for a specific day"""
    races_map = {}  # race_key -> race dict
    year, month = date.split("-")[:2]
    pattern = f"{RP_DATA_DIR}/{region}/{race_type}/{year}_{month}_*.csv"
    files = [f for f in glob.glob(pattern) if 'INCOMPLETE' not in f]
    if not files:
        return []

    rp_file = files[0]
    try:
        with open(rp_file, 'r', encoding='utf-8') as f:
            rdr = csv.DictReader(f)
            for row in rdr:
                if normalize_date(row.get('date','')) != date:
                    continue
                race_info = {
                    'date': date,
                    'region': row.get('region','').upper(),  # Uppercase region
                    'course': row.get('course',''),
                    'off': normalize_time(row.get('off','')),
                    'race_name': row.get('race_name',''),
                    'type': row.get('type',''),
                    'class': row.get('class',''),
                    'pattern': row.get('pattern',''),
                    'rating_band': row.get('rating_band',''),
                    'age_band': row.get('age_band',''),
                    'sex_rest': row.get('sex_rest',''),
                    'dist': row.get('dist',''),  # Keep original format (e.g., "1m2f")
                    'dist_f': clean_dist_f(row.get('dist_f','')),  # Convert to numeric
                    'dist_m': row.get('dist_m',''),
                    'going': row.get('going',''),
                    'surface': row.get('surface',''),
                }
                rkey = generate_race_key(
                    race_info['date'], race_info['region'], race_info['course'],
                    race_info['off'], race_info['race_name'], race_info['type']
                )
                if rkey not in races_map:
                    race_info['race_key'] = rkey
                    race_info['runners'] = []
                    race_info['horses'] = set()
                    races_map[rkey] = race_info

                runner = {k: row.get(k,'') for k in row.keys()}
                races_map[rkey]['runners'].append(runner)
                name_norm = normalize_text(runner.get('horse',''))
                if name_norm:
                    races_map[rkey]['horses'].add(name_norm)

        # finalize
        races = []
        for r in races_map.values():
            r['horses'] = list(r['horses'])
            r['ran'] = str(len(r['runners']))  # Actual runner count (trustworthy)
            races.append(r)
        return races

    except Exception as e:
        log(f"Error loading RP day {date} from {rp_file}: {e}")
        return []

def load_bf_day(region, race_type, date, index_data):
    """Load Betfair data for a specific day"""
    races = []
    
    # Find files for this date from index
    bf_files = [entry for entry in index_data 
                if entry['region'] == region 
                and entry['race_type'] == race_type 
                and entry['date'] == date
                and entry['has_required_cols']]
    
    for file_info in bf_files:
        try:
            with open(file_info['file_path'], 'r', encoding='utf-8') as f:
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
                            'event_name': row.get('event_name', '')
                        }
                    
                    # Add runner
                    runners.append(row)
                
                if race_info and len(runners) >= 3:  # Minimum 3 runners
                    race_info['horses'] = [normalize_text(r['horse']) for r in runners]
                    race_info['runners'] = runners
                    races.append(race_info)
        
        except Exception as e:
            log(f"Error loading BF file {file_info['file_path']}: {e}")
    
    return races

def validate_month_data(month_races, month_runners):
    """Validate monthly data for quality checks"""
    errors = []
    warnings = []
    
    # Check 1: Race keys match between races and runners
    race_keys_in_races = set(r['race_key'] for r in month_races)
    race_keys_in_runners = set(r['race_key'] for r in month_runners)
    
    if race_keys_in_races != race_keys_in_runners:
        missing_in_runners = race_keys_in_races - race_keys_in_runners
        missing_in_races = race_keys_in_runners - race_keys_in_races
        if missing_in_runners:
            errors.append(f"{len(missing_in_runners)} races have no runners")
        if missing_in_races:
            errors.append(f"{len(missing_in_races)} race_keys in runners but not in races")
    
    # Check 2: Runner count matches 'ran' field
    runner_counts = defaultdict(int)
    for runner in month_runners:
        runner_counts[runner['race_key']] += 1
    
    ran_mismatches = 0
    for race in month_races:
        expected_ran = runner_counts.get(race['race_key'], 0)
        declared_ran = race.get('ran', '')
        if declared_ran and str(expected_ran) != str(declared_ran):
            ran_mismatches += 1
    
    if ran_mismatches > 0:
        warnings.append(f"{ran_mismatches} races have ran != runner_count")
    
    # Check 3: No duplicate runner_keys
    runner_keys = [r['runner_key'] for r in month_runners]
    unique_runner_keys = set(runner_keys)
    if len(runner_keys) != len(unique_runner_keys):
        duplicates = len(runner_keys) - len(unique_runner_keys)
        errors.append(f"{duplicates} duplicate runner_keys found")
    
    # Check 4: Count rows with no Betfair data (all prices blank)
    no_betfair_count = 0
    for runner in month_runners:
        has_betfair = any(runner.get(f, '') for f in ['win_bsp', 'win_ppwap', 'place_bsp', 'place_ppwap'])
        if not has_betfair:
            no_betfair_count += 1
    
    return {
        'total_races': len(month_races),
        'total_runners': len(month_runners),
        'unique_race_keys': len(race_keys_in_races),
        'unique_runner_keys': len(unique_runner_keys),
        'runners_with_betfair': len(month_runners) - no_betfair_count,
        'runners_without_betfair': no_betfair_count,
        'errors': errors,
        'warnings': warnings,
        'validated_at': datetime.now().isoformat()
    }

def ensure_monthly_dir(region, race_type, year_month):
    """Ensure monthly directory exists and return path"""
    monthly_dir = f"{MASTER_DIR}/{region}/{race_type}/{year_month}"
    os.makedirs(monthly_dir, exist_ok=True)
    return monthly_dir

def load_manifest(monthly_dir):
    """Load existing manifest or create empty one"""
    manifest_path = f"{monthly_dir}/manifest.json"
    
    if os.path.exists(manifest_path):
        with open(manifest_path, 'r') as f:
            return json.load(f)
    
    return {
        'schema_version': '1.0',
        'processed_sources': [],
        'processed_dates': [],  # Track completed dates for idempotency
        'race_count': 0,
        'runner_count': 0,
        'unmatched_count': 0,
        'validation': {},
        'last_updated': None
    }

def save_manifest(monthly_dir, manifest):
    """Save manifest to directory"""
    manifest_path = f"{monthly_dir}/manifest.json"
    manifest['last_updated'] = datetime.now().isoformat()
    
    with open(manifest_path, 'w') as f:
        json.dump(manifest, f, indent=2)

def append_to_csv(file_path, data, schema):
    """Append data to CSV file"""
    file_exists = os.path.exists(file_path)
    
    with open(file_path, 'a', newline='', encoding='utf-8') as f:
        writer = csv.DictWriter(f, fieldnames=schema)
        
        if not file_exists:
            writer.writeheader()
        
        for row in data:
            # Ensure all schema fields are present
            filtered_row = {field: row.get(field, '') for field in schema}
            writer.writerow(filtered_row)

def process_month(region, race_type, year_month, index_data):
    """Process a single month of data"""
    log(f"Processing {region.upper()} {race_type.upper()} {year_month}")
    
    monthly_dir = ensure_monthly_dir(region, race_type, year_month)
    manifest = load_manifest(monthly_dir)
    
    # File paths
    races_file = f"{monthly_dir}/races_{region}_{race_type}_{year_month}.csv"
    runners_file = f"{monthly_dir}/runners_{region}_{race_type}_{year_month}.csv"
    unmatched_file = f"{monthly_dir}/unmatched_{region}_{race_type}_{year_month}.csv"
    
    # Get days in this month
    year, month = year_month.split('-')
    start_date = f"{year}-{month}-01"
    
    # Generate all days in month
    from calendar import monthrange
    days_in_month = monthrange(int(year), int(month))[1]
    
    # Track processed dates for idempotency
    processed_dates = set(manifest.get('processed_dates', []))
    
    month_races = []
    month_runners = []
    month_unmatched = []
    
    for day in range(1, days_in_month + 1):
        date = f"{year}-{month}-{day:02d}"
        
        # Skip if already processed
        if date in processed_dates:
            continue
        
        # Load RP and BF data for this day
        rp_races = load_rp_day(region, race_type, date)
        bf_races = load_bf_day(region, race_type, date, index_data)
        
        if not rp_races or not bf_races:
            continue
        
        # Match races
        matches, unmatched = match_races(rp_races, bf_races)
        
        # Process matches
        for match in matches:
            rp_race = match['rp_race']
            bf_race = match['bf_race']
            
            # Add race (avoid duplicates by checking race_key)
            race_exists = any(r['race_key'] == rp_race['race_key'] for r in month_races)
            if not race_exists:
                race_row = {field: rp_race.get(field, '') for field in RACE_SCHEMA}
                month_races.append(race_row)
            
            # Join runners
            joined_runners = join_runners(rp_race, bf_race, match)
            month_runners.extend(joined_runners)
        
        # Collect unmatched
        month_unmatched.extend(unmatched)
        
        # Mark this date as processed
        processed_dates.add(date)
    
    # Write monthly files
    if month_races:
        append_to_csv(races_file, month_races, RACE_SCHEMA)
    
    if month_runners:
        append_to_csv(runners_file, month_runners, RUNNER_SCHEMA)
    
    if month_unmatched:
        append_to_csv(unmatched_file, month_unmatched, UNMATCHED_SCHEMA)
    
    # Validation checks
    validation = validate_month_data(month_races, month_runners)
    
    # Update manifest
    manifest['race_count'] += len(month_races)
    manifest['runner_count'] += len(month_runners)
    manifest['unmatched_count'] += len(month_unmatched)
    manifest['processed_dates'] = sorted(processed_dates)
    manifest['validation'] = validation
    save_manifest(monthly_dir, manifest)
    
    log(f"  ✓ {region.upper()} {race_type.upper()} {year_month}: {len(month_races)} races, {len(month_runners)} runners, {len(month_unmatched)} unmatched")
    
    # Log validation issues if any
    if validation.get('errors'):
        for error in validation['errors']:
            log(f"    ⚠ Validation: {error}")

def main():
    """Main processing pipeline"""
    log("="*80)
    log("MASTER DATA STITCHER - Racing Post + Betfair Integration")
    log("="*80)
    
    # Build/load Betfair index
    index_data = load_betfair_index()
    
    # Get available months from index (Betfair coverage: 2007-2011)
    available_months = set()
    for entry in index_data:
        year_month = entry['date'][:7]  # YYYY-MM
        available_months.add((entry['region'], entry['race_type'], year_month))
    
    available_months = sorted(available_months)
    log(f"Found {len(available_months)} region/race_type/month combinations to process")
    
    # Process each month
    processed = 0
    for region, race_type, year_month in available_months:
        try:
            process_month(region, race_type, year_month, index_data)
            processed += 1
        except Exception as e:
            log(f"Error processing {region} {race_type} {year_month}: {e}")
            continue
    
    log("="*80)
    log(f"MASTER DATA STITCHING COMPLETE - {processed} months processed")
    log(f"Output directory: {MASTER_DIR}")
    log("="*80)

if __name__ == "__main__":
    main()
