#!/usr/bin/env python3
"""
Daily Data Updater - Keep Racing Post + Betfair Data Current
Analyzes missing data and provides interactive update options

Usage:
  python3 daily_updater.py           # Normal mode (downloads and scrapes)
  python3 daily_updater.py --dry-run # Dry run (shows what would be updated)
"""
import os
import sys
import csv
import requests
import subprocess
import time
import glob
import re
from datetime import datetime, date, timedelta
from pathlib import Path
from collections import defaultdict

# Check for dry-run mode
DRY_RUN = '--dry-run' in sys.argv or '-n' in sys.argv

# Configuration
RP_DATA_DIR = "/home/smonaghan/rpscrape/data/dates"
BF_DATA_DIR = "/home/smonaghan/rpscrape/data/betfair_daily"  # New directory for daily files
SCRIPTS_DIR = "/home/smonaghan/rpscrape/scripts"  
PYTHON_PATH = "/home/smonaghan/rpscrape/venv/bin/python"
LOG_FILE = "/home/smonaghan/rpscrape/logs/daily_updater.log"

def log(msg):
    """Simple logging"""
    timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
    log_msg = f"[{timestamp}] {msg}"
    print(log_msg)
    
    with open(LOG_FILE, 'a') as f:
        f.write(log_msg + '\n')
        f.flush()

def analyze_rp_coverage():
    """Analyze Racing Post data coverage in detail"""
    log("Analyzing Racing Post data coverage...")
    
    coverage = defaultdict(lambda: defaultdict(set))  # region -> race_type -> set of dates
    monthly_files = defaultdict(lambda: defaultdict(list))  # region -> race_type -> [files]
    
    for region in ['gb', 'ire']:
        for race_type in ['flat', 'jumps']:
            pattern = f"{RP_DATA_DIR}/{region}/{race_type}/*.csv"
            files = [f for f in glob.glob(pattern) if 'INCOMPLETE' not in f]
            
            monthly_files[region][race_type] = files
            
            for file_path in files:
                # Extract dates from filename: YYYY_MM_DD-YYYY_MM_DD.csv
                filename = os.path.basename(file_path)
                match = re.search(r'(\d{4})_(\d{2})_(\d{2})-(\d{4})_(\d{2})_(\d{2})\.csv', filename)
                if match:
                    start_year, start_month, start_day = match.groups()[:3]
                    end_year, end_month, end_day = match.groups()[3:6]
                    
                    try:
                        start_date = date(int(start_year), int(start_month), int(start_day))
                        end_date = date(int(end_year), int(end_month), int(end_day))
                        
                        # Add all days in range
                        current = start_date
                        while current <= end_date:
                            coverage[region][race_type].add(current)
                            current += timedelta(days=1)
                            
                    except:
                        continue
    
    # Report coverage
    total_days = 0
    latest_date = date.min
    earliest_date = date.max
    
    for region in ['gb', 'ire']:
        for race_type in ['flat', 'jumps']:
            dates = coverage[region][race_type]
            if dates:
                total_days += len(dates)
                latest_date = max(latest_date, max(dates))
                earliest_date = min(earliest_date, min(dates))
                
                log(f"  {region.upper()} {race_type.upper()}: {len(dates)} days covered, latest: {max(dates)}")
                log(f"    Files: {len(monthly_files[region][race_type])} monthly files")
    
    log(f"Total Racing Post coverage: {total_days} days")
    log(f"Date range: {earliest_date} to {latest_date}")
    
    return coverage, latest_date

def get_latest_bf_date():
    """Find the latest date in Betfair data"""
    log("Scanning for latest Betfair data...")
    
    latest_date = None
    
    # Check both old stitched data and any new daily data
    bf_dirs = [
        "/home/smonaghan/rpscrape/data/betfair_stitched",
        BF_DATA_DIR
    ]
    
    for bf_dir in bf_dirs:
        if not os.path.exists(bf_dir):
            continue
            
        # Find all CSV files
        pattern = f"{bf_dir}/**/*.csv"
        files = glob.glob(pattern, recursive=True)
        
        for file_path in files:
            filename = os.path.basename(file_path)
            
            # Try different patterns
            # Pattern 1: dwbfpricesukwin11102025.csv (DDMMYYYY)
            match = re.search(r'dwbfprices\w+(?:win|place)(\d{2})(\d{2})(\d{4})\.csv', filename)
            if match:
                day, month, year = match.groups()
                try:
                    file_date = date(int(year), int(month), int(day))
                    if latest_date is None or file_date > latest_date:
                        latest_date = file_date
                except:
                    continue
            
            # Pattern 2: gb_flat_2011-10-26_1525.csv (YYYY-MM-DD)
            match = re.search(r'(\d{4})-(\d{2})-(\d{2})', filename)
            if match:
                year, month, day = match.groups()
                try:
                    file_date = date(int(year), int(month), int(day))
                    if latest_date is None or file_date > latest_date:
                        latest_date = file_date
                except:
                    continue
    
    if latest_date:
        log(f"Latest Betfair data: {latest_date}")
    else:
        log("No Betfair data found")
    
    return latest_date

def download_betfair_day(target_date, dry_run=False):
    """Download Betfair data for a specific date"""
    success = True
    downloaded_count = 0
    skipped_count = 0
    
    if not dry_run:
        # Create daily betfair directory
        os.makedirs(BF_DATA_DIR, exist_ok=True)
    
    # Format date as DDMMYYYY for Betfair URLs
    formatted_date = target_date.strftime('%d%m%Y')
    
    # Download for both UK and IRE, both WIN and PLACE markets
    regions = [('uk', 'gb'), ('ire', 'ire')]  # (betfair_code, our_code)
    markets = ['win', 'place']
    
    for bf_region, our_region in regions:
        for market in markets:
            url = f"https://promo.betfair.com/betfairsp/prices/dwbfprices{bf_region}{market}{formatted_date}.csv"
            output_file = f"{BF_DATA_DIR}/dwbfprices{bf_region}{market}{formatted_date}.csv"
            
            # Skip if already exists
            if os.path.exists(output_file):
                skipped_count += 1
                if dry_run:
                    log(f"    ⊙ {os.path.basename(output_file)} already exists (SKIP)")
                continue
            
            if dry_run:
                log(f"    → Would download {our_region.upper()} {market.upper()}: {url}")
                downloaded_count += 1
                continue
            
            log(f"  Downloading {our_region.upper()} {market.upper()} for {target_date}")
            
            try:
                response = requests.get(url, timeout=30)
                
                if response.status_code == 200:
                    # Check if file has content (not empty or error page)
                    content = response.text.strip()
                    if len(content) > 100 and 'EVENT_ID' in content:  # Basic validation
                        with open(output_file, 'w') as f:
                            f.write(content)
                        log(f"    ✓ {os.path.basename(output_file)} downloaded ({len(content)} bytes)")
                        downloaded_count += 1
                    else:
                        log(f"    ⚠ {os.path.basename(output_file)} empty or invalid")
                
                elif response.status_code == 404:
                    log(f"    ⊙ {os.path.basename(output_file)} not available (404)")
                
                elif response.status_code == 429:
                    log(f"    ⚠ Rate limited - waiting 30s")
                    time.sleep(30)
                    success = False  # Will retry
                    
                else:
                    log(f"    ✗ HTTP {response.status_code} for {os.path.basename(output_file)}")
                    success = False
                    
            except Exception as e:
                log(f"    ✗ Error downloading {os.path.basename(output_file)}: {e}")
                success = False
            
            # Small delay between requests
            if not dry_run:
                time.sleep(1)
    
    if dry_run and (downloaded_count > 0 or skipped_count > 0):
        log(f"    Summary: {downloaded_count} would download, {skipped_count} already exist")
    
    return success

def scrape_rp_day(target_date, dry_run=False):
    """Scrape Racing Post data for a specific date"""
    log(f"  Scraping Racing Post data for {target_date}")
    
    # Format date for rpscrape.py (YYYY/MM/DD)
    date_str = target_date.strftime('%Y/%m/%d')
    date_range = f"{date_str}-{date_str}"  # Single day range
    
    success = True
    scrape_count = 0
    
    for region in ['gb', 'ire']:
        for race_type in ['flat', 'jumps']:
            # Expected output file (daily format)
            year_month_day = target_date.strftime('%Y_%m_%d')
            output_file = f"{RP_DATA_DIR}/{region}/{race_type}/{year_month_day}-{year_month_day}.csv"
            
            # Check if already exists
            if os.path.exists(output_file):
                file_size = os.path.getsize(output_file)
                if file_size > 300:  # More than just header
                    if dry_run:
                        log(f"    ⊙ {region.upper()} {race_type.upper()} already exists ({file_size} bytes)")
                    continue
            
            if dry_run:
                log(f"    → Would scrape {region.upper()} {race_type.upper()} for {target_date}")
                log(f"      Command: rpscrape.py -r {region} -d {date_range} -t {race_type}")
                log(f"      Output: {output_file}")
                scrape_count += 1
                continue
            
            try:
                env = os.environ.copy()
                env['SKIP_BETFAIR'] = '1'
                
                cmd = [
                    PYTHON_PATH,
                    'rpscrape.py',
                    '-r', region,
                    '-d', date_range,
                    '-t', race_type
                ]
                
                result = subprocess.run(
                    cmd,
                    cwd=SCRIPTS_DIR,
                    env=env,
                    capture_output=True,
                    text=True,
                    timeout=300  # 5 minute timeout per region/type
                )
                
                if result.returncode == 0:
                    log(f"    ✓ {region.upper()} {race_type.upper()} completed")
                    scrape_count += 1
                else:
                    log(f"    ✗ {region.upper()} {race_type.upper()} failed (exit code {result.returncode})")
                    if result.stderr:
                        log(f"      Error: {result.stderr[:200]}")
                    success = False
                    
            except subprocess.TimeoutExpired:
                log(f"    ⚠ {region.upper()} {race_type.upper()} timeout")
                success = False
                
            except Exception as e:
                log(f"    ✗ {region.upper()} {race_type.upper()} exception: {e}")
                success = False
            
            # Brief pause between requests
            if not dry_run:
                time.sleep(2)
    
    if dry_run and scrape_count > 0:
        log(f"    Summary: {scrape_count} region/type combinations would be scraped")
    
    return success

def update_daily(start_date, end_date, dry_run=False):
    """Update data from start_date to end_date (inclusive)"""
    current_date = start_date
    days_processed = 0
    days_skipped = 0
    
    while current_date <= end_date:
        log(f"Updating data for {current_date} ({current_date.strftime('%A')})")
        
        # Download Betfair data
        bf_success = download_betfair_day(current_date, dry_run=dry_run)
        
        # Scrape Racing Post data
        rp_success = scrape_rp_day(current_date, dry_run=dry_run)
        
        if bf_success and rp_success:
            log(f"  ✓ {current_date} completed successfully")
            days_processed += 1
        else:
            log(f"  ⚠ {current_date} had some failures")
            days_skipped += 1
        
        current_date += timedelta(days=1)
        
        # Brief pause between days (skip in dry run)
        if not dry_run:
            time.sleep(5)
    
    return days_processed, days_skipped

def main():
    """Main daily update process"""
    log("="*80)
    if DRY_RUN:
        log("DAILY DATA UPDATER - DRY RUN MODE (NO ACTUAL DOWNLOADS/SCRAPES)")
    else:
        log("DAILY DATA UPDATER - LIVE MODE")
    log("="*80)
    log(f"Started: {datetime.now()}")
    log("")
    
    # Create directories
    if not DRY_RUN:
        os.makedirs(BF_DATA_DIR, exist_ok=True)
        os.makedirs(os.path.dirname(LOG_FILE), exist_ok=True)
    
    # Analyze Racing Post coverage in detail
    rp_coverage, latest_rp = analyze_rp_coverage()
    log("")
    
    # Find latest Betfair data
    latest_bf = get_latest_bf_date()
    log("")
    
    # Calculate update range
    yesterday = date.today() - timedelta(days=1)
    today = date.today()
    
    log("="*80)
    log("GAP ANALYSIS")
    log("="*80)
    
    # Determine start date (latest of either dataset + 1 day)
    if latest_rp and latest_bf:
        start_date = max(latest_rp, latest_bf) + timedelta(days=1)
        log(f"Latest Racing Post: {latest_rp}")
        log(f"Latest Betfair:     {latest_bf}")
        log(f"Latest overall:     {max(latest_rp, latest_bf)}")
    elif latest_rp:
        start_date = latest_rp + timedelta(days=1)
        log(f"Latest Racing Post: {latest_rp}")
        log(f"Latest Betfair:     None (will start fresh)")
    elif latest_bf:
        start_date = latest_bf + timedelta(days=1)
        log(f"Latest Racing Post: None (will start fresh)")
        log(f"Latest Betfair:     {latest_bf}")
    else:
        # No existing data - start from a week ago
        start_date = yesterday - timedelta(days=7)
        log(f"No existing data found - starting from {start_date}")
    
    log(f"Today:              {today}")
    log(f"Yesterday:          {yesterday}")
    log("")
    
    # Don't go beyond yesterday
    if start_date > yesterday:
        log("="*80)
        log("✅ ALL DATA IS UP TO DATE!")
        log("="*80)
        log(f"Latest data: {max(latest_rp or date.min, latest_bf or date.min)}")
        log(f"Yesterday:   {yesterday}")
        log(f"Gap:         0 days")
        return
    
    # Calculate missing days
    days_to_update = (yesterday - start_date).days + 1
    
    log("="*80)
    log("MISSING DATA DETECTED")
    log("="*80)
    log(f"Missing date range: {start_date} to {yesterday}")
    log(f"Total missing days: {days_to_update}")
    log("")
    
    # Show what would be downloaded
    log("BETFAIR DATA TO DOWNLOAD:")
    log(f"  4 files per day × {days_to_update} days = {days_to_update * 4} CSV files")
    log(f"  Regions: UK (GB), IRE")
    log(f"  Markets: WIN, PLACE")
    log(f"  Format:  dwbfprices{{region}}{{market}}DDMMYYYY.csv")
    log(f"  Example: dwbfpricesukwin{yesterday.strftime('%d%m%Y')}.csv")
    log("")
    
    log("RACING POST DATA TO SCRAPE:")
    log(f"  4 region/type combos per day × {days_to_update} days = {days_to_update * 4} scrape jobs")
    log(f"  Regions: GB, IRE")
    log(f"  Types:   FLAT, JUMPS")
    log(f"  Format:  YYYY_MM_DD-YYYY_MM_DD.csv (daily files)")
    log(f"  Example: {yesterday.strftime('%Y_%m_%d')}-{yesterday.strftime('%Y_%m_%d')}.csv")
    log("")
    
    if DRY_RUN:
        log("="*80)
        log("DRY RUN - Simulating update process...")
        log("="*80)
        log("")
    else:
        log("="*80)
        log("STARTING LIVE UPDATE...")
        log("="*80)
        log("")
    
    # Update data
    days_processed, days_failed = update_daily(start_date, yesterday, dry_run=DRY_RUN)
    
    log("")
    log("="*80)
    if DRY_RUN:
        log("DRY RUN COMPLETE - NO DATA WAS MODIFIED")
    else:
        log("DAILY UPDATE COMPLETE")
    log("="*80)
    log(f"Date range:  {start_date} to {yesterday}")
    log(f"Days processed: {days_processed}")
    log(f"Days failed:    {days_failed}")
    log(f"Finished: {datetime.now()}")
    log("="*80)
    
    if DRY_RUN:
        log("")
        log("To run actual update, run without --dry-run flag:")
        log("  python3 daily_updater.py")
        log("")

if __name__ == "__main__":
    main()
