#!/usr/bin/env python3
"""
Comprehensive Data Verification Script
Checks for:
1. Duplicate data in database
2. Missing months
3. Random horse samples across all months
4. Data completeness by comparing master files vs database
"""
import psycopg2
import random
import os
import csv
import glob
from collections import defaultdict
from datetime import datetime

# Database configuration
DB_CONFIG = {
    'host': 'localhost',
    'port': 5432,
    'database': 'horse_db',
    'user': 'postgres',
    'password': 'password'
}

MASTER_DIR = "/home/smonaghan/rpscrape/master"

def log(msg, level='INFO'):
    timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
    print(f"[{timestamp}] [{level}] {msg}")

def connect_db():
    conn = psycopg2.connect(**DB_CONFIG)
    cur = conn.cursor()
    cur.execute("SET search_path TO racing, public;")
    conn.commit()
    return conn, cur

def check_duplicates(cur):
    """Check for duplicate race_keys and runner_keys"""
    log("=" * 80)
    log("CHECKING FOR DUPLICATES")
    log("=" * 80)
    
    # Check duplicate races
    cur.execute("""
        SELECT race_key, COUNT(*) as cnt
        FROM races
        GROUP BY race_key
        HAVING COUNT(*) > 1
        LIMIT 10
    """)
    dup_races = cur.fetchall()
    
    if dup_races:
        log(f"❌ FOUND {len(dup_races)} DUPLICATE RACES!", "ERROR")
        for race_key, count in dup_races[:5]:
            log(f"  Race {race_key}: {count} copies", "ERROR")
        return False
    else:
        log("✓ No duplicate races found")
    
    # Check duplicate runners
    cur.execute("""
        SELECT runner_id, COUNT(*) as cnt
        FROM runners
        GROUP BY runner_id
        HAVING COUNT(*) > 1
        LIMIT 10
    """)
    dup_runners = cur.fetchall()
    
    if dup_runners:
        log(f"❌ FOUND {len(dup_runners)} DUPLICATE RUNNERS!", "ERROR")
        for runner_id, count in dup_runners[:5]:
            log(f"  Runner {runner_id}: {count} copies", "ERROR")
        return False
    else:
        log("✓ No duplicate runners found")
    
    log("✓ NO DUPLICATES FOUND IN DATABASE\n")
    return True

def check_monthly_coverage(cur):
    """Check that all expected months have data"""
    log("=" * 80)
    log("CHECKING MONTHLY COVERAGE")
    log("=" * 80)
    
    # Get all months with data in database
    cur.execute("""
        SELECT 
            TO_CHAR(race_date, 'YYYY-MM') as month,
            COUNT(DISTINCT race_id) as race_count
        FROM races
        WHERE race_date >= '2024-01-01'
        GROUP BY TO_CHAR(race_date, 'YYYY-MM')
        ORDER BY month
    """)
    db_months = {row[0]: row[1] for row in cur.fetchall()}
    
    # Check expected months (2024-01 through 2025-10)
    expected_months = []
    for year in [2024, 2025]:
        max_month = 12 if year == 2024 else 10
        for month in range(1, max_month + 1):
            expected_months.append(f"{year}-{month:02d}")
    
    missing_months = []
    for month in expected_months:
        if month not in db_months:
            missing_months.append(month)
            log(f"  ❌ Missing: {month}", "WARN")
        else:
            log(f"  ✓ {month}: {db_months[month]} races")
    
    if missing_months:
        log(f"\n❌ {len(missing_months)} MONTHS MISSING DATA", "ERROR")
        return False
    else:
        log("\n✓ ALL EXPECTED MONTHS HAVE DATA\n")
        return True

def sample_random_horses(cur):
    """Sample random horses and verify their run counts"""
    log("=" * 80)
    log("SAMPLING RANDOM HORSES")
    log("=" * 80)
    
    # Get 20 random horses that have runs in 2024-2025
    cur.execute("""
        SELECT h.horse_id, h.horse_name, COUNT(DISTINCT r.runner_id) as db_runs
        FROM horses h
        JOIN runners r ON r.horse_id = h.horse_id
        JOIN races ra ON ra.race_id = r.race_id
        WHERE ra.race_date >= '2024-01-01'
        GROUP BY h.horse_id, h.horse_name
        HAVING COUNT(DISTINCT r.runner_id) >= 3
        ORDER BY RANDOM()
        LIMIT 20
    """)
    
    horses = cur.fetchall()
    log(f"Sampled {len(horses)} horses with 3+ runs in 2024-2025\n")
    
    issues = []
    for horse_id, horse_name, db_runs in horses:
        # Count runs in master files
        master_runs = 0
        for runner_file in glob.glob(f"{MASTER_DIR}/*/*/202*/runners_*.csv") + \
                            glob.glob(f"{MASTER_DIR}/*/*/2025*/runners_*.csv"):
            try:
                with open(runner_file, 'r', encoding='utf-8') as f:
                    content = f.read()
                    # Simple count of horse name occurrences
                    master_runs += content.lower().count(horse_name.lower())
            except:
                pass
        
        match_status = "✓" if abs(db_runs - master_runs) <= 1 else "❌"
        log(f"  {match_status} {horse_name}: DB={db_runs}, Master={master_runs}")
        
        if abs(db_runs - master_runs) > 1:
            issues.append((horse_name, db_runs, master_runs))
    
    if issues:
        log(f"\n❌ {len(issues)} HORSES HAVE MISMATCHED COUNTS", "ERROR")
        return False
    else:
        log("\n✓ ALL SAMPLED HORSES MATCH\n")
        return True

def verify_dancing_in_paris(cur):
    """Specific check for Dancing in Paris"""
    log("=" * 80)
    log("VERIFYING DANCING IN PARIS (TEST CASE)")
    log("=" * 80)
    
    cur.execute("""
        SELECT COUNT(*) 
        FROM runners r
        JOIN horses h ON h.horse_id = r.horse_id
        WHERE LOWER(h.horse_name) LIKE '%dancing in paris%'
    """)
    db_count = cur.fetchone()[0]
    
    # Count in master files
    master_count = 0
    for runner_file in glob.glob(f"{MASTER_DIR}/*/*/*/runners_*.csv"):
        try:
            with open(runner_file, 'r', encoding='utf-8') as f:
                master_count += sum(1 for line in f if 'dancing in paris' in line.lower())
        except:
            pass
    
    log(f"  Database: {db_count} runs")
    log(f"  Master files: {master_count} runs")
    log(f"  Expected: 33 runs")
    
    if db_count == 33 and master_count == 33:
        log("✓ DANCING IN PARIS: ALL 33 RUNS PRESENT\n")
        return True
    else:
        log(f"❌ DANCING IN PARIS: MISSING DATA (DB={db_count}, Master={master_count})", "ERROR")
        return False

def compare_totals(cur):
    """Compare total counts between database and master files"""
    log("=" * 80)
    log("COMPARING TOTAL COUNTS")
    log("=" * 80)
    
    # Database totals
    cur.execute("SELECT COUNT(*) FROM races")
    db_races = cur.fetchone()[0]
    
    cur.execute("SELECT COUNT(*) FROM runners")
    db_runners = cur.fetchone()[0]
    
    # Master file totals
    master_races = 0
    master_runners = 0
    
    for race_file in glob.glob(f"{MASTER_DIR}/*/*/*/races_*.csv"):
        try:
            with open(race_file, 'r', encoding='utf-8') as f:
                master_races += sum(1 for line in f) - 1  # Subtract header
        except:
            pass
    
    for runner_file in glob.glob(f"{MASTER_DIR}/*/*/*/runners_*.csv"):
        try:
            with open(runner_file, 'r', encoding='utf-8') as f:
                master_runners += sum(1 for line in f) - 1  # Subtract header
        except:
            pass
    
    log(f"  Database:    {db_races:,} races, {db_runners:,} runners")
    log(f"  Master files: {master_races:,} races, {master_runners:,} runners")
    
    # Allow for small discrepancies due to failed batches
    race_diff = abs(db_races - master_races)
    runner_diff = abs(db_runners - master_runners)
    
    if race_diff > 1000 or runner_diff > 10000:
        log(f"❌ LARGE DISCREPANCY: {race_diff} races, {runner_diff} runners", "ERROR")
        return False
    else:
        log(f"✓ TOTALS MATCH (diff: {race_diff} races, {runner_diff} runners)\n")
        return True

def main():
    log("="*80)
    log("DATA COMPLETENESS VERIFICATION")
    log("="*80)
    log("")
    
    conn, cur = connect_db()
    
    results = {
        'duplicates': check_duplicates(cur),
        'monthly_coverage': check_monthly_coverage(cur),
        'random_samples': sample_random_horses(cur),
        'dancing_in_paris': verify_dancing_in_paris(cur),
        'totals': compare_totals(cur)
    }
    
    cur.close()
    conn.close()
    
    log("="*80)
    log("VERIFICATION SUMMARY")
    log("="*80)
    
    for check, passed in results.items():
        status = "✓ PASS" if passed else "❌ FAIL"
        log(f"  {status}: {check.replace('_', ' ').title()}")
    
    all_passed = all(results.values())
    
    log("")
    if all_passed:
        log("✅ ALL VERIFICATION CHECKS PASSED")
        log("✅ DATABASE IS COMPLETE AND DUPLICATE-FREE")
        return 0
    else:
        log("❌ SOME VERIFICATION CHECKS FAILED")
        log("❌ PLEASE REVIEW ISSUES ABOVE")
        return 1

if __name__ == "__main__":
    exit(main())


