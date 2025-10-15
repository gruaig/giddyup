#!/usr/bin/env python3
"""
Simple Monthly Scraper - Back to Working Version
2 parallel threads, month by month, no complex features
"""
import os
import sys
import time
import subprocess
import threading
from datetime import datetime
import calendar

# Simple configuration
REGIONS = ['gb', 'ire']
YEARS = range(2006, 2026)
RACE_TYPES = ['flat', 'jumps']

def log(msg):
    """Simple logging"""
    timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
    log_msg = f"[{timestamp}] {msg}"
    print(log_msg)
    with open('logs/monthly_simple.log', 'a') as f:
        f.write(log_msg + '\n')
        f.flush()

def scrape_month(region, year, month, race_type):
    """Scrape a single month"""
    python_path = "/home/smonaghan/rpscrape/venv/bin/python"
    scripts_dir = "/home/smonaghan/rpscrape/scripts"
    
    # Calculate date range
    last_day = calendar.monthrange(year, month)[1]
    date_range = f"{year}/{month:02d}/01-{year}/{month:02d}/{last_day:02d}"
    
    env = os.environ.copy()
    env['SKIP_BETFAIR'] = '1'
    
    cmd = [python_path, 'rpscrape.py', '-r', region, '-d', date_range, '-t', race_type]
    
    try:
        result = subprocess.run(
            cmd,
            cwd=scripts_dir,
            env=env,
            capture_output=True,
            text=True,
            timeout=600  # 10 minute timeout per month
        )
        return result.returncode == 0
    except subprocess.TimeoutExpired:
        log(f"  ⚠ Timeout scraping {region.upper()} {year}-{month:02d}")
        return False
    except Exception as e:
        log(f"  ✗ Error: {e}")
        return False

def check_month_exists(region, year, month, race_type):
    """Simple check if month CSV exists"""
    last_day = calendar.monthrange(year, month)[1]
    date_range = f"{year}_{month:02d}_01-{year}_{month:02d}_{last_day:02d}"
    csv_path = f"/home/smonaghan/rpscrape/data/dates/{region}/{race_type}/{date_range}.csv"
    
    if os.path.exists(csv_path):
        size = os.path.getsize(csv_path)
        return size > 300  # Has more than just headers
    return False

def worker_thread(race_type, regions, years):
    """Worker thread for one race type"""
    log(f"[{race_type.upper()}] Started")
    
    completed = 0
    skipped = 0
    failed = 0
    
    for region in regions:
        for year in years:
            for month in range(1, 13):
                # Check if already done
                if check_month_exists(region, year, month, race_type):
                    skipped += 1
                    if skipped % 10 == 0:
                        log(f"[{race_type.upper()}] Skipped {skipped} already complete")
                    continue
                
                month_name = datetime(year, month, 1).strftime('%B')
                log(f"[{race_type.upper()}] Scraping {region.upper()} {year} {month_name}...")
                
                success = scrape_month(region, year, month, race_type)
                
                if success:
                    completed += 1
                    log(f"[{race_type.upper()}] ✓ {region.upper()} {year}-{month:02d} ({completed} done)")
                else:
                    failed += 1
                    log(f"[{race_type.upper()}] ✗ {region.upper()} {year}-{month:02d} failed")
                    time.sleep(30)  # Cooldown on failure
                
                # Progress report every 10
                if completed % 10 == 0 and completed > 0:
                    log(f"[{race_type.upper()}] Progress: {completed} completed, {failed} failed")
    
    log(f"[{race_type.upper()}] Finished - {completed} completed, {skipped} skipped, {failed} failed")

def main():
    log("="*60)
    log("SIMPLE MONTHLY SCRAPER")
    log("="*60)
    log(f"Started: {datetime.now()}")
    log(f"Regions: {REGIONS}")
    log(f"Years: {min(YEARS)}-{max(YEARS)}")
    log(f"Threads: 2 (flat + jumps parallel)")
    log("")
    
    # Create threads - one for flat, one for jumps
    threads = []
    
    for race_type in RACE_TYPES:
        thread = threading.Thread(
            target=worker_thread,
            args=(race_type, REGIONS, YEARS),
            name=f"{race_type.upper()}_Thread"
        )
        threads.append(thread)
        thread.start()
        log(f"✓ {race_type.upper()} thread started")
    
    log("")
    
    # Wait for completion
    try:
        for thread in threads:
            thread.join()
            log(f"✓ {thread.name} finished")
    except KeyboardInterrupt:
        log("\n⚠ Interrupted by user")
    
    log("")
    log("="*60)
    log("COMPLETE!")
    log(f"Finished: {datetime.now()}")
    log("="*60)

if __name__ == "__main__":
    main()

