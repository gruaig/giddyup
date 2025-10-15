#!/usr/bin/env python3
"""
Autonomous monitoring script - checks progress every 30 min and restarts if stuck
"""
import os
import sys
import time
import subprocess
import glob
from datetime import datetime

CHECK_INTERVAL_MINUTES = 30
PROGRESS_THRESHOLD = 2  # Must complete at least 2 months per check period

def log(msg):
    """Log with timestamp"""
    timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
    print(f"[{timestamp}] {msg}")
    with open('logs/auto_monitor.log', 'a') as f:
        f.write(f"[{timestamp}] {msg}\n")
        f.flush()

def count_completed_months():
    """Count total CSV files"""
    try:
        result = subprocess.run(
            ['find', 'data/dates', '-name', '*.csv', '-type', 'f'],
            capture_output=True, text=True, timeout=30
        )
        return len(result.stdout.strip().split('\n')) if result.stdout.strip() else 0
    except:
        return 0

def get_data_size():
    """Get total data size in MB"""
    try:
        result = subprocess.run(
            ['du', '-sm', 'data/dates'],
            capture_output=True, text=True, timeout=30
        )
        size_mb = int(result.stdout.split()[0]) if result.stdout.strip() else 0
        return size_mb
    except:
        return 0

def is_scraper_running():
    """Check if scraper is running"""
    try:
        result = subprocess.run(
            ['pgrep', '-f', 'scrape_racing_post_by_month.py'],
            capture_output=True, text=True, timeout=10
        )
        return bool(result.stdout.strip())
    except:
        return False

def get_connection_count():
    """Get HTTPS connection count"""
    try:
        result = subprocess.run(
            ['ss', '-tn', 'state', 'established'],
            capture_output=True, text=True, timeout=10
        )
        lines = result.stdout.strip().split('\n')
        count = len([l for l in lines if ':443' in l])
        return count
    except:
        return 0

def kill_scraper():
    """Kill scraper and all rpscrape subprocesses"""
    log("🛑 Stopping scraper...")
    try:
        subprocess.run(['pkill', '-f', 'scrape_racing_post_by_month.py'], timeout=10)
        time.sleep(2)
        subprocess.run(['pkill', '-f', 'rpscrape.py'], timeout=10)
        time.sleep(3)
        log("✓ Scraper stopped")
    except Exception as e:
        log(f"⚠ Error stopping scraper: {e}")

def start_scraper():
    """Start the scraper"""
    log("🚀 Starting scraper...")
    try:
        subprocess.Popen(
            ['python3', 'scrape_racing_post_by_month.py'],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            cwd='/home/smonaghan/rpscrape'
        )
        time.sleep(5)
        if is_scraper_running():
            log("✓ Scraper started successfully")
            return True
        else:
            log("✗ Scraper failed to start")
            return False
    except Exception as e:
        log(f"✗ Error starting scraper: {e}")
        return False

def main():
    """Main monitoring loop"""
    log("="*80)
    log("AUTONOMOUS SCRAPER MONITOR - Starting at 00:36")
    log("="*80)
    log(f"Check interval: {CHECK_INTERVAL_MINUTES} minutes")
    log(f"Progress threshold: {PROGRESS_THRESHOLD} months per check")
    log("")
    
    # Initial state
    last_count = count_completed_months()
    last_size = get_data_size()
    check_number = 0
    restart_count = 0
    no_progress_count = 0
    
    log(f"Initial state: {last_count} months, {last_size} MB")
    
    # Start scraper if not running
    if not is_scraper_running():
        log("Scraper not running - starting it...")
        start_scraper()
    else:
        log("Scraper already running")
    
    log(f"Will check progress every {CHECK_INTERVAL_MINUTES} minutes...")
    log("")
    
    while True:
        # Wait for check interval
        time.sleep(CHECK_INTERVAL_MINUTES * 60)
        
        check_number += 1
        log("="*80)
        log(f"CHECK #{check_number} - {datetime.now().strftime('%H:%M:%S')}")
        log("="*80)
        
        # Get current state
        current_count = count_completed_months()
        current_size = get_data_size()
        connections = get_connection_count()
        running = is_scraper_running()
        
        progress_made = current_count - last_count
        size_growth = current_size - last_size
        
        log(f"Files: {current_count} ({progress_made:+d} since last check)")
        log(f"Size: {current_size} MB ({size_growth:+d} MB)")
        log(f"Connections: {connections}")
        log(f"Scraper running: {running}")
        log(f"Progress: {current_count}/960 ({current_count/960*100:.1f}%)")
        
        # Check for problems
        problems = []
        
        if not running:
            problems.append("Scraper not running")
        
        if progress_made < PROGRESS_THRESHOLD:
            problems.append(f"Low progress ({progress_made} months in {CHECK_INTERVAL_MINUTES} min)")
            no_progress_count += 1
        else:
            no_progress_count = 0  # Reset if good progress
        
        if connections > 200:
            problems.append(f"High connections ({connections})")
        
        # Take action if problems
        if problems:
            log(f"⚠️ PROBLEMS DETECTED: {', '.join(problems)}")
            
            if no_progress_count >= 2:  # Two checks with no progress
                log("🔧 No progress for 2 checks - restarting scraper...")
                kill_scraper()
                time.sleep(5)
                start_scraper()
                restart_count += 1
                no_progress_count = 0
                log(f"✓ Restarted (restart #{restart_count})")
            elif not running:
                log("🔧 Scraper died - restarting...")
                start_scraper()
                restart_count += 1
                log(f"✓ Restarted (restart #{restart_count})")
            elif connections > 500:
                log("🔧 Connection leak detected - restarting...")
                kill_scraper()
                time.sleep(5)
                start_scraper()
                restart_count += 1
                log(f"✓ Restarted (restart #{restart_count})")
            else:
                log("→ Monitoring - will restart if no progress next check")
        else:
            log("✅ All good - scraper making progress")
            rate = progress_made / (CHECK_INTERVAL_MINUTES / 60)
            remaining = 960 - current_count
            eta_hours = remaining / rate if rate > 0 else 0
            log(f"→ Current rate: {rate:.1f} months/hour")
            if eta_hours > 0:
                log(f"→ ETA: {eta_hours:.1f} hours")
        
        # Update for next iteration
        last_count = current_count
        last_size = current_size
        
        log("")
        
        # Check if complete
        if current_count >= 960:
            log("🎉 ALL SCRAPING COMPLETE!")
            break
        
        # Safety: Stop if too many restarts
        if restart_count > 10:
            log("⚠️ Too many restarts (10+) - stopping monitor")
            log("→ Please investigate manually")
            break

if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        log("\n⚠️ Monitor stopped by user")
    except Exception as e:
        log(f"\n✗ Monitor error: {e}")

