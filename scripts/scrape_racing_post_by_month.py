#!/usr/bin/env python3
"""
Racing Post Scraper - Monthly Chunks (SIMPLE WORKING VERSION)
2 parallel threads: flat + jumps
Month by month scraping
"""
import os
import sys
import time
import subprocess
import threading
from datetime import datetime
import calendar

# Configuration
REGIONS = ['gb', 'ire']
YEARS = range(2006, 2026)

def log(msg):
    """Thread-safe logging"""
    timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
    log_msg = f"[{timestamp}] {msg}"
    print(log_msg)
    sys.stdout.flush()

def scrape_month(region, year, month, race_type):
    """Scrape a single month"""
    scripts_dir = "/home/smonaghan/rpscrape/scripts"
    python_path = "/home/smonaghan/rpscrape/venv/bin/python"
    
    # Calculate date range
    last_day = calendar.monthrange(year, month)[1]
    date_range = f"{year}/{month:02d}/01-{year}/{month:02d}/{last_day:02d}"
    
    env = os.environ.copy()
    env['SKIP_BETFAIR'] = '1'
    
    cmd = [python_path, 'rpscrape.py', '-r', region, '-d', date_range, '-t', race_type]
    
    try:
        start_time = time.time()
        result = subprocess.run(
            cmd,
            cwd=scripts_dir,
            env=env,
            capture_output=True,
            text=True,
            timeout=600  # 10 minute timeout
        )
        elapsed = time.time() - start_time
        
        if result.returncode == 0:
            return True, elapsed
        else:
            return False, elapsed
            
    except subprocess.TimeoutExpired:
        return 'TIMEOUT', 600
    except Exception as e:
        log(f"    ✗ Exception: {str(e)[:100]}")
        return False, 0

def check_month_exists(region, year, month, race_type):
    """Check if month already scraped"""
    last_day = calendar.monthrange(year, month)[1]
    date_range = f"{year}_{month:02d}_01-{year}_{month:02d}_{last_day:02d}"
    csv_path = f"/home/smonaghan/rpscrape/data/dates/{region}/{race_type}/{date_range}.csv"
    
    if os.path.exists(csv_path):
        size = os.path.getsize(csv_path)
        if size > 300:  # More than just header
            return True
    return False

def scrape_race_type(race_type, regions, years):
    """Worker thread for one race type"""
    import random
    
    log(f"[{race_type.upper()}] Thread started")
    
    # Build list of months to scrape
    jobs = []
    for region in regions:
        for year in years:
            for month in range(1, 13):
                if not check_month_exists(region, year, month, race_type):
                    jobs.append((region, year, month))
    
    # Randomize to avoid patterns
    random.shuffle(jobs)
    
    total = len(jobs)
    completed = 0
    failed = 0
    
    log(f"[{race_type.upper()}] {total} months to scrape")
    
    for region, year, month in jobs:
        month_name = datetime(year, month, 1).strftime('%B')
        
        log(f"  [{race_type.upper()}] [{completed + 1}/{total}] Scraping {region.upper()} {year} {month_name}...")
        
        result, elapsed = scrape_month(region, year, month, race_type)
        
        if result is True:
            completed += 1
            log(f"    [{race_type.upper()}] ✓ {region.upper()} {year}-{month:02d} in {elapsed:.1f}s")
        elif result == 'TIMEOUT':
            failed += 1
            log(f"    [{race_type.upper()}] ⚠ {region.upper()} {year}-{month:02d} timeout - skipping")
            time.sleep(30)  # Cooldown on timeout
        else:
            failed += 1
            log(f"    [{race_type.upper()}] ✗ {region.upper()} {year}-{month:02d} failed")
            time.sleep(10)
        
        # Progress every 10 months
        if completed % 10 == 0 and completed > 0:
            pct = (completed / total * 100) if total > 0 else 0
            log(f"[{race_type.upper()}] Progress: {completed}/{total} ({pct:.1f}%)")
    
    log(f"[{race_type.upper()}] Finished - {completed} completed, {failed} failed")

def main():
    log("="*60)
    log("RACING POST MONTHLY SCRAPER - SIMPLE VERSION")
    log("="*60)
    log(f"Started: {datetime.now()}")
    log(f"Regions: {', '.join(REGIONS)}")
    log(f"Years: {min(YEARS)}-{max(YEARS)}")
    log(f"Threads: 2 (flat + jumps in parallel)")
    log("")
    
    # Create 2 threads
    threads = []
    
    for race_type in ['flat', 'jumps']:
        thread = threading.Thread(
            target=scrape_race_type,
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
