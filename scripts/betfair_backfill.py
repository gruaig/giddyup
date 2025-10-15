#!/usr/bin/env python3
"""
Betfair Backfill + Stitcher
Downloads missing Betfair data (WIN + PLACE markets) and stitches them together
Matches the existing betfair_stitched directory format
"""
import os
import sys
import csv
import requests
import time
from datetime import datetime, date, timedelta
from collections import defaultdict
import glob
import re

# Configuration
BF_RAW_DIR = "/home/smonaghan/rpscrape/data/betfair_raw"  # Downloaded WIN/PLACE files
BF_STITCHED_DIR = "/home/smonaghan/rpscrape/data/betfair_stitched"  # Final stitched output
LOG_FILE = "/home/smonaghan/rpscrape/logs/betfair_backfill.log"

def log(msg):
    """Simple logging"""
    timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
    log_msg = f"[{timestamp}] {msg}"
    print(log_msg)
    sys.stdout.flush()
    
    os.makedirs(os.path.dirname(LOG_FILE), exist_ok=True)
    with open(LOG_FILE, 'a') as f:
        f.write(log_msg + '\n')
        f.flush()

def download_betfair_files(start_date, end_date, dry_run=False):
    """Download Betfair WIN and PLACE files for date range"""
    log(f"Downloading Betfair data from {start_date} to {end_date}")
    
    if not dry_run:
        os.makedirs(BF_RAW_DIR, exist_ok=True)
    
    current_date = start_date
    downloaded = 0
    skipped = 0
    failed = 0
    
    while current_date <= end_date:
        formatted_date = current_date.strftime('%d%m%Y')
        day_files = []
        
        # Download 4 files per day: UK WIN/PLACE, IRE WIN/PLACE
        regions = [('uk', 'gb'), ('ire', 'ire')]
        markets = ['win', 'place']
        
        for bf_region, _ in regions:
            for market in markets:
                filename = f"dwbfprices{bf_region}{market}{formatted_date}.csv"
                file_path = f"{BF_RAW_DIR}/{filename}"
                url = f"https://promo.betfair.com/betfairsp/prices/{filename}"
                
                # Skip if already exists
                if os.path.exists(file_path) and os.path.getsize(file_path) > 100:
                    skipped += 1
                    continue
                
                if dry_run:
                    log(f"  → Would download: {filename}")
                    day_files.append(filename)
                    continue
                
                try:
                    response = requests.get(url, timeout=30)
                    
                    if response.status_code == 200:
                        content = response.text.strip()
                        if len(content) > 100 and 'event_id' in content:
                            with open(file_path, 'w') as f:
                                f.write(content)
                            downloaded += 1
                            day_files.append(filename)
                        else:
                            log(f"  ⚠ {filename} empty or invalid")
                            failed += 1
                    
                    elif response.status_code == 404:
                        # No data available for this date/region/market
                        pass
                    
                    elif response.status_code == 429:
                        log(f"  ⚠ Rate limited - waiting 60s")
                        time.sleep(60)
                        failed += 1
                    
                    else:
                        log(f"  ✗ HTTP {response.status_code} for {filename}")
                        failed += 1
                
                except Exception as e:
                    log(f"  ✗ Error downloading {filename}: {e}")
                    failed += 1
                
                # Small delay between requests
                time.sleep(2)
        
        if day_files:
            if dry_run:
                log(f"  {current_date}: {len(day_files)} files would be downloaded")
            else:
                log(f"  ✓ {current_date}: {len(day_files)} files downloaded")
        
        current_date += timedelta(days=1)
        
        # Progress report every 10 days
        days_done = (current_date - start_date).days
        if days_done % 10 == 0:
            log(f"  Progress: {days_done}/{(end_date - start_date).days + 1} days")
    
    log(f"Download summary: {downloaded} downloaded, {skipped} skipped, {failed} failed")
    return downloaded, skipped, failed

def parse_event_dt(event_dt):
    """Parse event_dt to extract date and time"""
    # Format: "10-10-2025 13:57"
    try:
        parts = event_dt.split()
        date_part = parts[0]  # "10-10-2025"
        time_part = parts[1] if len(parts) > 1 else "00:00"  # "13:57"
        
        # Parse date (DD-MM-YYYY)
        day, month, year = date_part.split('-')
        date_str = f"{year}-{month}-{day}"
        
        # Parse time (HH:MM)
        time_str = time_part[:5]  # Just HH:MM
        
        return date_str, time_str
    except:
        return None, None

def stitch_daily_files(target_date, dry_run=False):
    """Stitch WIN and PLACE markets for a single day"""
    formatted_date = target_date.strftime('%d%m%Y')
    
    # File paths for this day
    files_to_stitch = {
        'gb': {
            'win': f"{BF_RAW_DIR}/dwbfpricesukwin{formatted_date}.csv",
            'place': f"{BF_RAW_DIR}/dwbfpricesukplace{formatted_date}.csv"
        },
        'ire': {
            'win': f"{BF_RAW_DIR}/dwbfpricesirewin{formatted_date}.csv",
            'place': f"{BF_RAW_DIR}/dwbfpricesireplace{formatted_date}.csv"
        }
    }
    
    stitched_count = 0
    
    for region, files in files_to_stitch.items():
        win_file = files['win']
        place_file = files['place']
        
        # Skip if files don't exist
        if not os.path.exists(win_file) or not os.path.exists(place_file):
            continue
        
        # Load WIN market data
        win_data = {}  # (date, off, horse) -> row
        try:
            with open(win_file, 'r', encoding='utf-8') as f:
                reader = csv.DictReader(f)
                for row in reader:
                    date_str, time_str = parse_event_dt(row.get('event_dt', ''))
                    if not date_str or not time_str:
                        continue
                    
                    horse = row.get('selection_name', '').strip().lower()
                    if not horse:
                        continue
                    
                    key = (date_str, time_str, horse)
                    win_data[key] = {
                        'date': date_str,
                        'off': time_str,
                        'event_name': row.get('event_name', ''),
                        'horse': row.get('selection_name', ''),
                        'win_bsp': row.get('bsp', ''),
                        'win_ppwap': row.get('ppwap', ''),
                        'win_morningwap': row.get('morningwap', ''),
                        'win_ppmax': row.get('ppmax', ''),
                        'win_ppmin': row.get('ppmin', ''),
                        'win_ipmax': row.get('ipmax', ''),
                        'win_ipmin': row.get('ipmin', ''),
                        'win_morning_vol': row.get('morningtradedvol', ''),
                        'win_pre_vol': row.get('pptradedvol', ''),
                        'win_ip_vol': row.get('iptradedvol', ''),
                        'win_lose': row.get('win_lose', '')
                    }
        except Exception as e:
            log(f"  ✗ Error reading {win_file}: {e}")
            continue
        
        # Load PLACE market data
        place_data = {}  # (date, off, horse) -> row
        try:
            with open(place_file, 'r', encoding='utf-8') as f:
                reader = csv.DictReader(f)
                for row in reader:
                    date_str, time_str = parse_event_dt(row.get('event_dt', ''))
                    if not date_str or not time_str:
                        continue
                    
                    horse = row.get('selection_name', '').strip().lower()
                    if not horse:
                        continue
                    
                    key = (date_str, time_str, horse)
                    place_data[key] = {
                        'place_bsp': row.get('bsp', ''),
                        'place_ppwap': row.get('ppwap', ''),
                        'place_morningwap': row.get('morningwap', ''),
                        'place_ppmax': row.get('ppmax', ''),
                        'place_ppmin': row.get('ppmin', ''),
                        'place_ipmax': row.get('ipmax', ''),
                        'place_ipmin': row.get('ipmin', ''),
                        'place_morning_vol': row.get('morningtradedvol', ''),
                        'place_pre_vol': row.get('pptradedvol', ''),
                        'place_ip_vol': row.get('iptradedvol', ''),
                        'place_win_lose': row.get('win_lose', '')
                    }
        except Exception as e:
            log(f"  ✗ Error reading {place_file}: {e}")
            continue
        
        # Stitch together by grouping into races
        races = defaultdict(list)  # (date, off, event_name) -> [stitched rows]
        
        # Get all horses from both markets
        all_keys = set(win_data.keys()).union(set(place_data.keys()))
        
        for key in all_keys:
            date_str, time_str, horse_lower = key
            
            win_row = win_data.get(key, {})
            place_row = place_data.get(key, {})
            
            # Create stitched row
            stitched = {
                'date': win_row.get('date', date_str),
                'off': win_row.get('off', time_str),
                'event_name': win_row.get('event_name', ''),
                'horse': win_row.get('horse', place_row.get('horse', '')),
            }
            
            # Add WIN market columns
            for col in ['win_bsp', 'win_ppwap', 'win_morningwap', 'win_ppmax', 'win_ppmin',
                       'win_ipmax', 'win_ipmin', 'win_morning_vol', 'win_pre_vol', 'win_ip_vol', 'win_lose']:
                stitched[col] = win_row.get(col, '')
            
            # Add PLACE market columns
            for col in ['place_bsp', 'place_ppwap', 'place_morningwap', 'place_ppmax', 'place_ppmin',
                       'place_ipmax', 'place_ipmin', 'place_morning_vol', 'place_pre_vol', 'place_ip_vol', 'place_win_lose']:
                stitched[col] = place_row.get(col, '')
            
            # Group by race (date + off + event_name)
            race_key = (date_str, time_str, stitched['event_name'])
            races[race_key].append(stitched)
        
        # Write stitched files (one per race)
        for race_key, runners in races.items():
            date_str, time_str, event_name = race_key
            
            if not runners or len(runners) < 3:  # Skip races with < 3 runners
                continue
            
            # Determine race type from event_name (flat vs jumps)
            event_lower = event_name.lower()
            if any(word in event_lower for word in ['chase', 'hurdle', 'nh flat', 'nhf']):
                race_type = 'jumps'
            else:
                race_type = 'flat'
            
            # Create output filename: {region}_{race_type}_YYYY-MM-DD_HHMM.csv
            time_formatted = time_str.replace(':', '')
            output_filename = f"{region}_{race_type}_{date_str}_{time_formatted}.csv"
            output_dir = f"{BF_STITCHED_DIR}/{region}/{race_type}"
            output_path = f"{output_dir}/{output_filename}"
            
            # Skip if already exists
            if os.path.exists(output_path):
                continue
            
            if dry_run:
                log(f"    → Would create: {output_filename} ({len(runners)} runners)")
                stitched_count += 1
                continue
            
            # Create directory and write stitched file
            os.makedirs(output_dir, exist_ok=True)
            
            header = [
                'date', 'off', 'event_name', 'horse',
                'win_bsp', 'win_ppwap', 'win_morningwap', 'win_ppmax', 'win_ppmin',
                'win_ipmax', 'win_ipmin', 'win_morning_vol', 'win_pre_vol', 'win_ip_vol', 'win_lose',
                'place_bsp', 'place_ppwap', 'place_morningwap', 'place_ppmax', 'place_ppmin',
                'place_ipmax', 'place_ipmin', 'place_morning_vol', 'place_pre_vol', 'place_ip_vol', 'place_win_lose'
            ]
            
            with open(output_path, 'w', newline='', encoding='utf-8') as f:
                writer = csv.DictWriter(f, fieldnames=header)
                writer.writeheader()
                for runner in runners:
                    # Ensure all fields present
                    row = {field: runner.get(field, '') for field in header}
                    writer.writerow(row)
            
            stitched_count += 1
    
    return stitched_count

def get_latest_stitched_date():
    """Find the latest date in existing stitched Betfair data"""
    log("Scanning existing Betfair stitched data...")
    
    latest_date = None
    file_count = 0
    
    pattern = f"{BF_STITCHED_DIR}/**/*.csv"
    files = glob.glob(pattern, recursive=True)
    file_count = len(files)
    
    for file_path in files:
        filename = os.path.basename(file_path)
        # Pattern: {region}_{race_type}_YYYY-MM-DD_HHMM.csv
        match = re.search(r'(\d{4})-(\d{2})-(\d{2})', filename)
        if match:
            year, month, day = match.groups()
            try:
                file_date = date(int(year), int(month), int(day))
                if latest_date is None or file_date > latest_date:
                    latest_date = file_date
            except:
                continue
    
    log(f"  Existing files: {file_count}")
    if latest_date:
        log(f"  Latest date: {latest_date}")
    else:
        log(f"  No data found")
    
    return latest_date, file_count

def main():
    """Main backfill process"""
    dry_run = '--dry-run' in sys.argv or '-n' in sys.argv
    
    log("="*80)
    if dry_run:
        log("BETFAIR BACKFILL - DRY RUN MODE")
    else:
        log("BETFAIR BACKFILL - LIVE MODE")
    log("="*80)
    log(f"Started: {datetime.now()}")
    log("")
    
    # Find latest existing data
    latest_date, existing_count = get_latest_stitched_date()
    log("")
    
    # Calculate gap
    yesterday = date.today() - timedelta(days=1)
    
    if latest_date is None:
        log("⚠ No existing Betfair data found!")
        log("Please specify start date manually or check data directory")
        return
    
    start_date = latest_date + timedelta(days=1)
    
    if start_date > yesterday:
        log("="*80)
        log("✅ BETFAIR DATA IS UP TO DATE!")
        log("="*80)
        log(f"Latest data: {latest_date}")
        log(f"Yesterday:   {yesterday}")
        return
    
    # Gap analysis
    gap_days = (yesterday - start_date).days + 1
    
    log("="*80)
    log("GAP DETECTED")
    log("="*80)
    log(f"Latest data:  {latest_date}")
    log(f"Yesterday:    {yesterday}")
    log(f"Gap:          {start_date} to {yesterday}")
    log(f"Missing days: {gap_days}")
    log("")
    
    log("FILES TO DOWNLOAD:")
    log(f"  WIN files:   {gap_days * 2} (UK + IRE)")
    log(f"  PLACE files: {gap_days * 2} (UK + IRE)")
    log(f"  Total:       {gap_days * 4} raw CSV files")
    log("")
    
    log("STITCHED OUTPUT:")
    log(f"  Format:      {{region}}_{{race_type}}_YYYY-MM-DD_HHMM.csv")
    log(f"  Destination: {BF_STITCHED_DIR}/{{region}}/{{flat|jumps}}/")
    log(f"  Estimated:   ~{gap_days * 30} race files (assuming ~30 races/day)")
    log("")
    
    if not dry_run:
        # Ask for confirmation
        response = input("Proceed with download and stitching? (yes/no): ")
        if response.lower() not in ['yes', 'y']:
            log("Cancelled by user")
            return
        log("")
    
    # Phase 1: Download raw files
    log("="*80)
    log("PHASE 1: DOWNLOADING RAW FILES")
    log("="*80)
    downloaded, skipped, failed = download_betfair_files(start_date, yesterday, dry_run=dry_run)
    log("")
    
    # Phase 2: Stitch files day by day
    log("="*80)
    log("PHASE 2: STITCHING WIN + PLACE MARKETS")
    log("="*80)
    
    current_date = start_date
    total_stitched = 0
    
    while current_date <= yesterday:
        stitched = stitch_daily_files(current_date, dry_run=dry_run)
        total_stitched += stitched
        
        if stitched > 0:
            if dry_run:
                log(f"  {current_date}: Would create {stitched} stitched race files")
            else:
                log(f"  ✓ {current_date}: Created {stitched} stitched race files")
        
        current_date += timedelta(days=1)
        
        # Progress every 10 days
        days_done = (current_date - start_date).days
        if days_done % 10 == 0:
            log(f"  Progress: {days_done}/{gap_days} days stitched")
    
    log("")
    log("="*80)
    if dry_run:
        log("DRY RUN COMPLETE - NO FILES MODIFIED")
    else:
        log("BETFAIR BACKFILL COMPLETE")
    log("="*80)
    log(f"Downloaded:    {downloaded} files")
    log(f"Skipped:       {skipped} existing files")
    log(f"Failed:        {failed} files")
    log(f"Stitched:      {total_stitched} race files")
    log(f"Output dir:    {BF_STITCHED_DIR}")
    log(f"Finished:      {datetime.now()}")
    log("="*80)
    
    if dry_run:
        log("")
        log("To run actual backfill, run without --dry-run:")
        log("  python3 betfair_backfill.py")
        log("")

if __name__ == "__main__":
    main()
