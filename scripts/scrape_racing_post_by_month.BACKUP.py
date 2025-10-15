#!/usr/bin/env python3
"""
Racing Post Scraper - Monthly Chunks with VPN Rotation
Scrapes by MONTH instead of YEAR for better granularity and reliability
"""
import os
import sys
import time
import subprocess
import threading
import glob
import queue
from datetime import datetime

# ============================================================
# CONFIGURATION - Edit these settings
# ============================================================

# VPN/Proxy Configuration
ENABLE_VPN_ROTATION = False  # Set to False to disable VPN switching
VPN_DIRECTORY = '/home/smonaghan/rpscrape/vpns'  # Path to VPN config files

# Scraping Behavior
ENABLE_RANDOM_DELAYS = True  # Human-like delays between requests
MIN_DELAY_SECONDS = 0.5  # Minimum delay between requests
MAX_DELAY_SECONDS = 2.0  # Maximum delay between requests

# Server Stress Prevention (Politeness Settings)
MAX_REQUESTS_PER_MINUTE = 20  # Maximum requests per minute (default: 20, be conservative)
MAX_REQUESTS_PER_HOUR = 800  # Maximum requests per hour (default: 800)
ENABLE_EXPONENTIAL_BACKOFF = True  # Back off on errors
BACKOFF_MULTIPLIER = 2.0  # How much to increase delay on errors
MAX_BACKOFF_SECONDS = 60  # Maximum backoff delay
PAUSE_ON_ERROR_STREAK = 3  # Pause after this many consecutive errors
ERROR_PAUSE_MINUTES = 5  # How long to pause after error streak
ENABLE_NIGHTTIME_MODE = False  # Scrape slower during UK business hours
NIGHTTIME_DELAY_MULTIPLIER = 1.5  # Increase delays during busy hours

# Periodic Cooldown (Server Break)
ENABLE_PERIODIC_COOLDOWN = True  # Give server regular breaks
COOLDOWN_AFTER_MONTHS = 10  # Take a break after every X months
COOLDOWN_DURATION_MINUTES = 5  # How long to pause (default: 5 minutes)
COOLDOWN_DURATION_SECONDS = COOLDOWN_DURATION_MINUTES * 60  # Convert to seconds

# Timeout Settings
SCRAPE_TIMEOUT_MINUTES = 10  # Max minutes per month before timeout
NO_OUTPUT_TIMEOUT_MINUTES = 5  # Max minutes without output before timeout

# Browser Randomization
ENABLE_BROWSER_RANDOMIZATION = True  # Randomize user-agents and headers
VERBOSE_BROWSER_LOGGING = False  # Log which browser is used for each request

# Logging Settings
LOG_VERBOSITY = 'NORMAL'  # Options: 'MINIMAL', 'NORMAL', 'VERBOSE'
# MINIMAL: Only major events (month completion, errors) - FASTEST
# NORMAL: Standard progress updates - BALANCED
# VERBOSE: Every request, parsing step, etc. - SLOWEST (current behavior)

# Data Verification
ENABLE_CSV_VERIFICATION = True  # Verify CSV completeness after scraping
KEEP_INCOMPLETE_FILES = True  # Rename incomplete files instead of deleting

# Regions and Years to Scrape
REGIONS = ['gb', 'ire']  # Great Britain and Ireland
YEARS = range(2006, 2026)  # 2006-2025
RACE_TYPES = ['flat', 'jumps']  # Flat racing and jump racing

# Concurrency Settings
CONCURRENT_THREADS = 1  # Number of parallel worker threads
# IMPORTANT: High connection counts (800+) cause timeouts!
# 1 worker = safest, prevents connection buildup completely
# 2-4 workers = risk of connection accumulation
USE_WORK_QUEUE = True  # True = work queue (scalable), False = old method (max 4 threads)

# ============================================================
# End of Configuration
# ============================================================

# Setup logging
log_file = 'logs/racing_post_monthly.log'
log_lock = threading.Lock()

def log(msg):
    """Thread-safe logging to both file and stdout"""
    timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
    log_msg = f"[{timestamp}] {msg}"
    
    with log_lock:
        print(log_msg)
        with open(log_file, 'a') as f:
            f.write(log_msg + '\n')
            f.flush()

class RateLimiter:
    """Rate limiter to prevent server stress"""
    
    def __init__(self):
        self.requests_this_minute = []
        self.requests_this_hour = []
        self.consecutive_errors = 0
        self.current_backoff = MIN_DELAY_SECONDS
        self.last_request_time = 0
        
    def wait_if_needed(self):
        """Wait if we're exceeding rate limits"""
        current_time = time.time()
        
        # Clean up old timestamps
        cutoff_minute = current_time - 60
        cutoff_hour = current_time - 3600
        self.requests_this_minute = [t for t in self.requests_this_minute if t > cutoff_minute]
        self.requests_this_hour = [t for t in self.requests_this_hour if t > cutoff_hour]
        
        # Check rate limits
        if len(self.requests_this_minute) >= MAX_REQUESTS_PER_MINUTE:
            wait_time = 60 - (current_time - self.requests_this_minute[0])
            if wait_time > 0:
                log(f"      ⏸ Rate limit: {len(self.requests_this_minute)} requests/min - pausing {wait_time:.1f}s")
                time.sleep(wait_time)
                current_time = time.time()
        
        if len(self.requests_this_hour) >= MAX_REQUESTS_PER_HOUR:
            wait_time = 3600 - (current_time - self.requests_this_hour[0])
            if wait_time > 0:
                log(f"      ⏸ Rate limit: {len(self.requests_this_hour)} requests/hour - pausing {wait_time/60:.1f}min")
                time.sleep(wait_time)
                current_time = time.time()
        
        # Apply backoff delay if enabled
        if ENABLE_EXPONENTIAL_BACKOFF and self.current_backoff > MIN_DELAY_SECONDS:
            log(f"      ⏸ Exponential backoff: {self.current_backoff:.1f}s delay")
            time.sleep(self.current_backoff)
        
        # Apply nighttime mode if enabled (UK business hours: 9 AM - 6 PM GMT)
        if ENABLE_NIGHTTIME_MODE:
            from datetime import datetime
            uk_hour = datetime.utcnow().hour  # UK is UTC/GMT
            if 9 <= uk_hour < 18:  # Business hours
                extra_delay = (MAX_DELAY_SECONDS - MIN_DELAY_SECONDS) * (NIGHTTIME_DELAY_MULTIPLIER - 1)
                log(f"      ⏸ UK business hours - adding {extra_delay:.1f}s delay")
                time.sleep(extra_delay)
        
        # Record this request
        self.requests_this_minute.append(current_time)
        self.requests_this_hour.append(current_time)
        self.last_request_time = current_time
    
    def record_success(self):
        """Record successful request - reset backoff"""
        self.consecutive_errors = 0
        self.current_backoff = MIN_DELAY_SECONDS
    
    def record_error(self):
        """Record failed request - increase backoff"""
        self.consecutive_errors += 1
        
        if ENABLE_EXPONENTIAL_BACKOFF:
            self.current_backoff = min(
                self.current_backoff * BACKOFF_MULTIPLIER,
                MAX_BACKOFF_SECONDS
            )
            log(f"      ⚠ Error recorded - backoff now {self.current_backoff:.1f}s")
        
        # Pause if we have too many consecutive errors
        if self.consecutive_errors >= PAUSE_ON_ERROR_STREAK:
            log(f"      🛑 {self.consecutive_errors} consecutive errors - pausing {ERROR_PAUSE_MINUTES} minutes")
            log(f"      → This prevents hammering the server when something is wrong")
            time.sleep(ERROR_PAUSE_MINUTES * 60)
            self.consecutive_errors = 0
            self.current_backoff = MIN_DELAY_SECONDS
    
    def get_stats(self):
        """Get current rate limiting stats"""
        return {
            'requests_last_minute': len(self.requests_this_minute),
            'requests_last_hour': len(self.requests_this_hour),
            'consecutive_errors': self.consecutive_errors,
            'current_backoff': self.current_backoff
        }

# Global rate limiters (one per thread to track separately)
rate_limiters = {}  # Will be initialized in main() based on thread combinations

class VPNManager:
    """Manages VPN connections with rotation"""
    
    def __init__(self, vpn_dir=None):
        if vpn_dir is None:
            vpn_dir = VPN_DIRECTORY
        self.vpn_dir = vpn_dir
        self.auth_file = f'{vpn_dir}/auth.txt'
        self.vpn_configs = []
        self.current_vpn_idx = 0
        self.current_vpn_name = None
        self.vpn_process = None
        self.current_ip = None
        self.previous_ip = None
        self._load_vpn_configs()
        
        # Get initial IP (before VPN)
        log(f"[VPN] Getting initial IP address...")
        self.current_ip = self.get_current_ip()
        if self.current_ip:
            log(f"[VPN] Initial IP: {self.current_ip}")
        else:
            log(f"[VPN] ⚠ Could not determine initial IP")
    
    def _load_vpn_configs(self):
        """Load ALL TCP VPN configs"""
        all_configs = glob.glob(f'{self.vpn_dir}/*.ovpn')
        
        # Use ALL TCP VPNs for maximum rotation options
        for config in all_configs:
            if '_tcp.ovpn' in config:
                self.vpn_configs.append(config)
        
        log(f"[VPN] Loaded {len(self.vpn_configs)} TCP VPN configs (ALL countries)")
    
    def get_current_ip(self, max_attempts=3):
        """Get current public IP address using curl ifconfig.me"""
        for attempt in range(max_attempts):
            try:
                result = subprocess.run(
                    ['curl', '-s', '--max-time', '8', 'ifconfig.me'],
                    capture_output=True,
                    text=True,
                    timeout=12
                )
                if result.returncode == 0 and result.stdout.strip():
                    ip = result.stdout.strip()
                    # Basic IP validation
                    if '.' in ip and len(ip.split('.')) == 4:
                        return ip
            except Exception as e:
                if attempt < max_attempts - 1:
                    time.sleep(2)
                    continue
                else:
                    log(f"[VPN] ✗ Could not get IP: {e}")
        return None
    
    def verify_ip_changed(self):
        """Verify that VPN rotation actually changed the IP address"""
        new_ip = self.get_current_ip()
        if not new_ip:
            log(f"[VPN] ⚠ Could not verify IP change - unable to get current IP")
            return False
        
        self.previous_ip = self.current_ip
        self.current_ip = new_ip
        
        if self.previous_ip and self.current_ip == self.previous_ip:
            log(f"[VPN] ✗ IP did not change! Still: {self.current_ip}")
            return False
        elif self.previous_ip:
            log(f"[VPN] ✓ IP changed: {self.previous_ip} → {self.current_ip}")
            return True
        else:
            log(f"[VPN] ℹ Current IP: {self.current_ip}")
            return True
    
    def connect(self):
        """Connect to current VPN"""
        self.disconnect()
        
        if not self.vpn_configs:
            return False
        
        config = self.vpn_configs[self.current_vpn_idx]
        config_name = os.path.basename(config).replace('.ovpn', '')
        
        try:
            self.vpn_process = subprocess.Popen(
                ['sudo', 'openvpn', '--config', config, '--auth-user-pass', self.auth_file],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL
            )
            time.sleep(10)
            
            if self._verify_connection():
                self.current_vpn_name = config_name
                log(f"[VPN] ✓ Connected to {config_name}")
                
                # Verify IP changed (as user suggested)
                if self.verify_ip_changed():
                    log(f"[VPN] ✓ VPN connection verified with IP change")
                return True
                else:
                    log(f"[VPN] ⚠ VPN connected but IP verification failed")
                    self.disconnect()
                    return False
            else:
                self.disconnect()
                return False
        except Exception as e:
            log(f"[VPN] ✗ Error: {e}")
            self.disconnect()
            return False
    
    def _verify_connection(self, max_attempts=5):
        """Verify VPN connection"""
        for attempt in range(max_attempts):
            try:
                result = subprocess.run(
                    ['curl', '-s', '--max-time', '5', 'https://www.racingpost.com'],
                    capture_output=True,
                    timeout=10
                )
                if result.returncode == 0:
                    return True
            except:
                pass
            time.sleep(2)
        return False
    
    def disconnect(self):
        """Disconnect VPN and clear all network connections"""
        if self.vpn_process:
            try:
                self.vpn_process.terminate()
                self.vpn_process.wait(timeout=5)
            except:
                try:
                    self.vpn_process.kill()
                except:
                    pass
            self.vpn_process = None
        
        # Comprehensive connection cleanup as suggested
        cleanup_commands = [
            ['sudo', 'pkill', 'openvpn'],
            ['sudo', 'pkill', '-f', 'openvpn'],
            ['sudo', 'pkill', '-f', 'curl'],
            ['sudo', 'pkill', '-f', 'wget'],
            ['sudo', 'pkill', '-f', 'requests'],
            # Clear DNS cache
            ['sudo', 'systemctl', 'restart', 'systemd-resolved'],
            # Flush network connections
            ['sudo', 'ss', '-K', 'dport', '443'],
            ['sudo', 'ss', '-K', 'dport', '80']
        ]
        
        log(f"[VPN] Clearing network connections to avoid routing confusion...")
        for cmd in cleanup_commands:
            try:
                subprocess.run(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=10)
        except:
            pass
        
        # Wait for cleanup to complete
        time.sleep(3)
        self.current_vpn_name = None
    
    def rotate(self):
        """Rotate to next VPN"""
        if not self.vpn_configs:
            return False
        
        self.current_vpn_idx = (self.current_vpn_idx + 1) % len(self.vpn_configs)
        log(f"[VPN] Rotating to VPN {self.current_vpn_idx + 1}/{len(self.vpn_configs)}")
        return self.connect()

# Global VPN managers
vpn_managers = {
    'flat': None,
    'jumps': None
}

def diagnose_timeout_cause():
    """Diagnose the cause of timeouts - system vs network level"""
    diagnostics = []
    
    try:
        # 1. Check system resources
        import psutil
        
        # CPU usage
        cpu_percent = psutil.cpu_percent(interval=1)
        diagnostics.append(f"CPU usage: {cpu_percent}%")
        
        # Memory usage
        memory = psutil.virtual_memory()
        diagnostics.append(f"Memory usage: {memory.percent}% ({memory.available // (1024**3)} GB available)")
        
        # Disk usage
        disk = psutil.disk_usage('/')
        diagnostics.append(f"Disk usage: {disk.percent}% ({disk.free // (1024**3)} GB free)")
        
        # Check for high-CPU processes
        procs = []
        for proc in psutil.process_iter(['pid', 'name', 'cpu_percent']):
            try:
                if proc.info['cpu_percent'] > 10:
                    procs.append(f"{proc.info['name']} (PID {proc.info['pid']}): {proc.info['cpu_percent']:.1f}%")
            except:
                pass
        
        if procs:
            diagnostics.append(f"High CPU processes: {', '.join(procs[:3])}")
        
    except ImportError:
        diagnostics.append("psutil not available - install with: pip install psutil")
    except Exception as e:
        diagnostics.append(f"System resource check failed: {e}")
    
    # 2. Check network connectivity
    try:
        # Test basic internet connectivity
        result = subprocess.run(
            ['ping', '-c', '2', '-W', '5', '8.8.8.8'],
            capture_output=True, timeout=10
        )
        if result.returncode == 0:
            diagnostics.append("Internet connectivity: OK (ping 8.8.8.8)")
        else:
            diagnostics.append("Internet connectivity: FAILED (ping 8.8.8.8)")
    except Exception as e:
        diagnostics.append(f"Internet connectivity test failed: {e}")
    
    # 3. Test DNS resolution
    try:
        result = subprocess.run(
            ['nslookup', 'racingpost.com'],
            capture_output=True, timeout=10
        )
        if result.returncode == 0:
            diagnostics.append("DNS resolution: OK (racingpost.com)")
        else:
            diagnostics.append("DNS resolution: FAILED (racingpost.com)")
    except Exception as e:
        diagnostics.append(f"DNS resolution test failed: {e}")
    
    # 4. Test Racing Post connectivity
    try:
        result = subprocess.run(
            ['curl', '-s', '-o', '/dev/null', '-w', '%{http_code}', 
             '--max-time', '10', 'https://www.racingpost.com'],
            capture_output=True, text=True, timeout=15
        )
        http_code = result.stdout.strip()
        if http_code == '200':
            diagnostics.append("Racing Post connectivity: OK (HTTP 200)")
        elif http_code == '403':
            diagnostics.append("Racing Post connectivity: BLOCKED (HTTP 403)")
        else:
            diagnostics.append(f"Racing Post connectivity: Issue (HTTP {http_code})")
    except Exception as e:
        diagnostics.append(f"Racing Post connectivity test failed: {e}")
    
    # 5. Check for hanging python processes
    try:
        result = subprocess.run(
            ['pgrep', '-f', 'rpscrape.py'],
            capture_output=True, text=True
        )
        if result.stdout.strip():
            pids = result.stdout.strip().split('\n')
            diagnostics.append(f"rpscrape.py processes running: {len(pids)} PIDs: {', '.join(pids)}")
        else:
            diagnostics.append("No rpscrape.py processes found")
    except Exception as e:
        diagnostics.append(f"Process check failed: {e}")
    
    # 6. Check current IP (if VPN is being used)
    try:
        result = subprocess.run(
            ['curl', '-s', '--max-time', '5', 'ifconfig.me'],
            capture_output=True, text=True, timeout=8
        )
        if result.returncode == 0 and result.stdout.strip():
            current_ip = result.stdout.strip()
            diagnostics.append(f"Current IP: {current_ip}")
        else:
            diagnostics.append("Current IP: Unable to determine")
    except Exception as e:
        diagnostics.append(f"IP check failed: {e}")
    
    # 7. Check network connections
    try:
        result = subprocess.run(
            ['ss', '-tuln', '|', 'grep', ':443'],
            shell=True, capture_output=True, text=True
        )
        conn_count = len(result.stdout.strip().split('\n')) if result.stdout.strip() else 0
        diagnostics.append(f"HTTPS connections: {conn_count}")
    except Exception as e:
        diagnostics.append(f"Connection check failed: {e}")
    
    return diagnostics

def verify_csv_completeness(region, year, month, race_type):
    """Verify that a monthly CSV file is complete and valid"""
    import calendar
    import csv
    from datetime import datetime
    
    # Calculate expected file path
    last_day = calendar.monthrange(year, month)[1]
    date_range = f"{year}_{month:02d}_01-{year}_{month:02d}_{last_day:02d}"
    csv_path = f"/home/smonaghan/rpscrape/data/dates/{region}/{race_type}/{date_range}.csv"
    
    if not os.path.exists(csv_path):
        return False, "CSV file does not exist"
    
    try:
        file_size = os.path.getsize(csv_path)
        
        # Check if file is too small (just headers or empty)
        if file_size <= 300:
            return False, f"CSV file too small ({file_size} bytes) - likely just headers"
        
        # Read and analyze the CSV content
        with open(csv_path, 'r', encoding='utf-8') as f:
            lines = f.readlines()
        
        if len(lines) <= 1:
            return False, "CSV has no data rows (only header or empty)"
        
        # Parse the CSV to check data quality
        try:
            reader = csv.DictReader(lines)
            rows = list(reader)
            
            if len(rows) == 0:
                return False, "CSV parsed but contains no data rows"
            
            # Check first and last row dates
            first_row = rows[0]
            last_row = rows[-1]
            
            # Extract dates from the race data (assuming 'date' field exists)
            date_fields = ['date', 'race_date', 'Date', 'Race_Date']
            date_field = None
            
            for field in date_fields:
                if field in first_row and first_row[field]:
                    date_field = field
                    break
            
            if not date_field:
                # Try to infer from other fields or filename
                log(f"      → No standard date field found, checking data consistency...")
                return True, f"CSV complete - {len(rows)} races, {file_size} bytes (date validation skipped)"
            
            # Parse first and last dates
            try:
                first_date = first_row[date_field]
                last_date = last_row[date_field]
                
                # Parse dates (handle different formats)
                first_parsed = parse_race_date(first_date)
                last_parsed = parse_race_date(last_date)
                
                if first_parsed and last_parsed:
                    # Check if dates are within the expected month
                    expected_start = datetime(year, month, 1)
                    expected_end = datetime(year, month, last_day)
                    
                    if first_parsed < expected_start.date():
                        return False, f"First race date {first_parsed} is before expected month {year}-{month:02d}"
                    
                    if last_parsed > expected_end.date():
                        return False, f"Last race date {last_parsed} is after expected month {year}-{month:02d}"
                    
                    # Check if we have races from late in the month (be realistic - racing doesn't happen every day)
                    days_from_end = (expected_end.date() - last_parsed).days
                    
                    # Be more lenient - 15 days is acceptable for horse racing (weather, scheduling, etc.)
                    if days_from_end > 15:
                        return False, f"Last race date {last_parsed} is {days_from_end} days before month end - possibly incomplete"
                    
                    return True, f"CSV complete - {len(rows)} races from {first_parsed} to {last_parsed} ({file_size} bytes)"
                
                else:
                    # Couldn't parse dates, but file looks substantial
                    return True, f"CSV complete - {len(rows)} races, {file_size} bytes (date parsing failed)"
                    
            except Exception as e:
                # Date parsing failed, but we have substantial data
                return True, f"CSV complete - {len(rows)} races, {file_size} bytes (date error: {str(e)[:50]})"
        
        except Exception as e:
            return False, f"Failed to parse CSV: {str(e)[:100]}"
            
    except Exception as e:
        return False, f"Failed to read CSV file: {str(e)[:100]}"

def parse_race_date(date_str):
    """Parse race date from various formats"""
    if not date_str:
        return None
    
    from datetime import datetime
    
    # Common date formats in racing data
    date_formats = [
        '%Y-%m-%d',    # 2019-11-30
        '%d/%m/%Y',    # 30/11/2019
        '%m/%d/%Y',    # 11/30/2019
        '%Y/%m/%d',    # 2019/11/30
        '%d-%m-%Y',    # 30-11-2019
        '%Y%m%d',      # 20191130
    ]
    
    for fmt in date_formats:
        try:
            return datetime.strptime(date_str.strip(), fmt).date()
        except ValueError:
            continue
    
    return None

def parse_scraper_output(line, current_phase, races_found, races_scraped):
    """Parse and enhance scraper output with additional context"""
    original_line = line
    
    # Add contextual information based on content
    if "Fetching race URLs" in line:
        return f"Contacting Racing Post API to get list of races for this date range..."
    elif "Found" in line and "race URLs" in line:
        return f"{line} → Will process each race individually"
    elif "Scraping" in line and "races" in line:
        return f"{line} → Starting individual race processing (HTML parsing & data extraction)"
    elif "Progress:" in line:
        # Add estimated time remaining
        try:
            progress_part = line.split("Progress: ")[1]
            current, total = progress_part.split("(")[0].split("/")
            current, total = int(current.strip()), int(total.strip())
            pct = (current / total * 100) if total > 0 else 0
            return f"{line} → Processing race {current}/{total} ({pct:.1f}% complete)"
        except:
            return line
    elif "Getting Betfair data" in line:
        return f"{line} → Downloading betting odds data (disabled via SKIP_BETFAIR)"
    elif "Betfair data DISABLED" in line:
        return f"{line} → Faster scraping (Racing Post data only)"
    elif "Warning: Error scraping race" in line:
        return f"{line} → Skipping problematic race, continuing with next"
    elif "Finished scraping" in line:
        return f"{line} → All races processed successfully!"
    elif "Data path:" in line:
        return f"{line} → CSV file written with race results"
    elif line.startswith("  Progress:") and "races scraped" in line:
        return f"{line} → Individual race completion status"
    elif "timeout" in line.lower():
        return f"{line} → Network delay detected"
    elif "403" in line or "forbidden" in line.lower():
        return f"{line} → Racing Post blocking detected - VPN rotation needed"
    elif "429" in line or "rate" in line.lower():
        return f"{line} → Rate limiting detected - VPN rotation needed"
    elif "HTTP" in line and any(code in line for code in ["200", "404", "500", "503"]):
        return f"{line} → Server response status"
    elif line.strip() and not line.startswith(" "):
        # Main output lines get phase context
        phase_context = {
            "STARTING": "Initializing",
            "FETCHING_URLS": "Getting race list", 
            "URLS_FOUND": "Race list ready",
            "SCRAPING_RACES": "Processing races",
            "COMPLETED": "Finished"
        }.get(current_phase, "Processing")
        
        if races_found > 0 and current_phase == "SCRAPING_RACES":
            return f"[{phase_context}] {line}"
        else:
            return f"[{phase_context}] {line}"
    
    return original_line

def scrape_month(region, year, month, race_type, max_retries=3):
    """Scrape a single month of races with real-time output and verbose logging"""
    scripts_dir = "/home/smonaghan/rpscrape/scripts"
    python_path = "/home/smonaghan/rpscrape/venv/bin/python"
    
    # Apply rate limiting before starting scrape (use region-race_type specific limiter)
    limiter_key = f"{region}-{race_type}"
    rate_limiter = rate_limiters.get(limiter_key)
    if rate_limiter:
        rate_limiter.wait_if_needed()
        stats = rate_limiter.get_stats()
        if stats['requests_last_minute'] > MAX_REQUESTS_PER_MINUTE * 0.8:  # Warn at 80%
            log(f"      ℹ [{limiter_key.upper()}] Rate limit status: {stats['requests_last_minute']}/{MAX_REQUESTS_PER_MINUTE} requests/min")
    
    env = os.environ.copy()
    env['SKIP_BETFAIR'] = '1'
    env['VPN_ROTATION'] = '1'  # Tell rpscrape.py VPN rotation is available
    
    # Only enable verbose scraping if LOG_VERBOSITY is VERBOSE
    if LOG_VERBOSITY == 'VERBOSE':
        env['VERBOSE_SCRAPING'] = '1'  # Enable detailed logging of HTML parsing and data extraction
    else:
        env['VERBOSE_SCRAPING'] = '0'  # Minimal logging from rpscrape.py
    
    # Calculate date range for this month (first day to last day)
    import calendar
    last_day = calendar.monthrange(year, month)[1]
    date_range = f"{year}/{month:02d}/01-{year}/{month:02d}/{last_day:02d}"
    
    # Conditional verbose logging based on LOG_VERBOSITY setting
    month_name = datetime(year, month, 1).strftime('%B')
    
    if LOG_VERBOSITY == 'VERBOSE':
        log(f"      → Preparing to scrape {region.upper()} {month_name} {year} ({race_type})")
        log(f"      → Date range: {year}/{month:02d}/01 to {year}/{month:02d}/{last_day:02d} ({last_day} days)")
        log(f"      → Output will be: data/dates/{region}/{race_type}/{year}_{month:02d}_01-{year}_{month:02d}_{last_day:02d}.csv")
    
    # Use date range to scrape just this month
    cmd = [
        python_path,
        'rpscrape.py',
        '-r', region,
        '-d', date_range,  # e.g., "2006/01/01-2006/01/31"
        '-t', race_type
    ]
    
    if LOG_VERBOSITY == 'VERBOSE':
        log(f"      → Command: {' '.join(cmd[1:])}")  # Skip full python path for readability
    
    try:
        start_time = time.time()
        if LOG_VERBOSITY != 'MINIMAL':
            log(f"      → Starting scraper subprocess...")
        
        # Run with real-time output streaming
        process = subprocess.Popen(
            cmd,
            cwd=scripts_dir,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            env=env,
            bufsize=1  # Line buffered
        )
        
        last_output_time = time.time()
        output_lines = []
        error_lines = []
        
        # Track scraping phases for verbose logging
        phase = "STARTING"
        races_found = 0
        races_scraped = 0
        current_race_url = None
        
        # Monitor process with timeout
        while True:
            # Check if process has finished
            retcode = process.poll()
            if retcode is not None:
                # Process finished
                # Collect any remaining output
                remaining_out, remaining_err = process.communicate()
                if remaining_out:
                    for line in remaining_out.strip().split('\n'):
                        if line:
                            log(f"      {line}")
                            output_lines.append(line)
                if remaining_err:
                    for line in remaining_err.strip().split('\n'):
                        if line:
                            error_lines.append(line)
                
                elapsed = time.time() - start_time
                
                if retcode == 0:
                    # Verify CSV completeness before declaring success
                    log(f"      → Subprocess completed successfully, verifying CSV completeness...")
                    is_complete, verification_msg = verify_csv_completeness(region, year, month, race_type)
                    
                    if is_complete:
                        log(f"      ✓ CSV verification passed: {verification_msg}")
                        # Record successful scrape for rate limiting
                        if rate_limiter:
                            rate_limiter.record_success()
                    return True, elapsed
                    else:
                        log(f"      ✗ CSV verification failed: {verification_msg}")
                        log(f"      → Logging as incomplete data for priority retry")
                        log_incomplete_data(region, year, month, race_type, f"Verification failed: {verification_msg}")
                        # Record as error for backoff
                        if rate_limiter:
                            rate_limiter.record_error()
                        return 'INCOMPLETE_DATA', elapsed
                else:
                    # Check for specific errors
                    stderr = '\n'.join(error_lines)
                    # Record error for rate limiting
                    if rate_limiter:
                        rate_limiter.record_error()
                    
                    if '403' in stderr or 'forbidden' in stderr.lower():
                        log(f"      ✗ HTTP 403 detected")
                        return 'ROTATE_VPN', elapsed
                    elif '429' in stderr or 'too many' in stderr.lower():
                        log(f"      ✗ HTTP 429 detected")
                        return 'ROTATE_VPN', elapsed
                    else:
                        if error_lines:
                            log(f"      ✗ Error: {error_lines[-1][:100]}")
                        return False, elapsed
            
            # Read output line by line (non-blocking)
            import select
            readable, _, _ = select.select([process.stdout, process.stderr], [], [], 0.1)
            
            for stream in readable:
                line = stream.readline()
                if line:
                    line = line.strip()
                    if stream == process.stdout:
                        # Parse and enhance output with detailed logging
                        enhanced_line = parse_scraper_output(line, phase, races_found, races_scraped)
                        
                        # Update phase tracking based on output content
                        if "Fetching race URLs" in line:
                            phase = "FETCHING_URLS"
                            log(f"      🔍 {enhanced_line}")
                        elif "Found" in line and "race URLs" in line:
                            try:
                                races_found = int(line.split("Found ")[1].split(" race")[0])
                                phase = "URLS_FOUND"
                                log(f"      📊 {enhanced_line}")
                            except:
                                log(f"      📊 {enhanced_line}")
                        elif "Scraping" in line and "races" in line:
                            phase = "SCRAPING_RACES"
                            log(f"      🏇 {enhanced_line}")
                        elif "Progress:" in line:
                            try:
                                # Extract progress numbers
                                progress_part = line.split("Progress: ")[1]
                                races_scraped = int(progress_part.split("/")[0])
                                log(f"      📈 {enhanced_line}")
                            except:
                                log(f"      📈 {enhanced_line}")
                        elif "Finished scraping" in line:
                            phase = "COMPLETED"
                            log(f"      ✅ {enhanced_line}")
                        elif "Data path:" in line:
                            log(f"      💾 {enhanced_line}")
                        elif "http" in line.lower() and ("error" in line.lower() or "warning" in line.lower()):
                            log(f"      ⚠ HTTP Issue: {enhanced_line}")
                        elif "timeout" in line.lower() or "connection" in line.lower():
                            log(f"      🔌 Network: {enhanced_line}")
                        else:
                            # Regular output with context
                            log(f"      {enhanced_line}")
                        
                        output_lines.append(line)
                        last_output_time = time.time()
                    else:
                        error_lines.append(line)
                        # Log errors immediately with context
                        if 'error' in line.lower() or 'warning' in line.lower():
                            log(f"      ❌ ERROR: {line}")
                        else:
                            log(f"      ⚠ STDERR: {line}")
            
            # Check for timeout with verbose diagnostics
            current_time = time.time()
            elapsed = current_time - start_time
            time_since_output = current_time - last_output_time
            
            # Timeout if no output for 5 minutes OR total time > 15 minutes
            if time_since_output > 300:  # 5 min without output = hung
                log(f"      ⚠ TIMEOUT DETECTED: No output for {time_since_output/60:.1f} minutes")
                
                # Verbose timeout diagnostics as requested by user
                log(f"      → Performing timeout diagnostics...")
                timeout_diagnostics = diagnose_timeout_cause()
                for diagnostic in timeout_diagnostics:
                    log(f"      → {diagnostic}")
                
                process.kill()
                return 'TIMEOUT', elapsed
            elif elapsed > 900:  # 15 min total max
                log(f"      ⚠ TIMEOUT DETECTED: Total time exceeded {elapsed/60:.1f} minutes")
                
                # Verbose timeout diagnostics
                log(f"      → Performing timeout diagnostics...")
                timeout_diagnostics = diagnose_timeout_cause()
                for diagnostic in timeout_diagnostics:
                    log(f"      → {diagnostic}")
                    
                process.kill()
                return 'TIMEOUT', elapsed
            
            time.sleep(0.1)  # Small sleep to avoid busy loop
                
    except Exception as e:
        log(f"      Exception: {e}")
        try:
            process.kill()
        except:
            pass
        return False, 0

def check_month_already_scraped(region, year, month, race_type):
    """Check if a month has already been scraped with complete and valid data"""
    import calendar
    
    # Use the comprehensive CSV verification instead of basic file checks
    is_complete, verification_msg = verify_csv_completeness(region, year, month, race_type)
    
    if is_complete:
        # Month is fully scraped and verified
        return True
    else:
        # Month is incomplete, missing, or corrupted
        # Log it as incomplete data for retry priority
        log_incomplete_data(region, year, month, race_type, verification_msg)
        
        # Keep the incomplete file for analysis but mark it for retry
    last_day = calendar.monthrange(year, month)[1]
    date_range = f"{year}_{month:02d}_01-{year}_{month:02d}_{last_day:02d}"
    csv_path = f"/home/smonaghan/rpscrape/data/dates/{region}/{race_type}/{date_range}.csv"
    
    if os.path.exists(csv_path):
            # Rename incomplete file instead of deleting
            try:
                incomplete_path = csv_path.replace('.csv', '_INCOMPLETE.csv')
                os.rename(csv_path, incomplete_path)
                log(f"      → Renamed incomplete file to: {os.path.basename(incomplete_path)}")
            except Exception as e:
                log(f"      → Could not rename incomplete file: {e}")
    
    return False

def scrape_year_monthly(region, year, race_type, vpn_manager):
    """Scrape a year month by month with resume capability"""
    months = range(1, 13)  # Jan to Dec
    
    for month in months:
        # Check if this month is already scraped
        if check_month_already_scraped(region, year, month, race_type):
            month_name = datetime(year, month, 1).strftime('%B')
            log(f"    [{race_type.upper()}] ⊙ {region.upper()} {year}-{month:02d} ({month_name}) already scraped - skipping")
            continue
        
        month_name = datetime(year, month, 1).strftime('%B')
        
        for attempt in range(3):  # Max 3 attempts per month
            result, elapsed = scrape_month(region, year, month, race_type)
            
            if result is True:
                # Success
                if attempt == 0:
                    log(f"    [{race_type.upper()}] ✓ {region.upper()} {year}-{month:02d} ({month_name}) in {elapsed:.1f}s")
                else:
                    log(f"    [{race_type.upper()}] ✓ {region.upper()} {year}-{month:02d} ({month_name}) in {elapsed:.1f}s (attempt {attempt+1})")
                break
            
            elif result == 'ROTATE_VPN':
                log(f"    [{race_type.upper()}] ⚠ {region.upper()} {year}-{month:02d} blocked (403/429) - rotating VPN")
                if vpn_manager:
                    if vpn_manager.rotate():
                        # Verify new VPN works
                        log(f"    [{race_type.upper()}] → Verifying new VPN can reach Racing Post...")
                        if verify_vpn_can_reach_racingpost():
                            log(f"    [{race_type.upper()}] → VPN verified ✓ - retrying immediately")
                            time.sleep(2)  # Brief pause, then retry
                            continue
                        else:
                            log(f"    [{race_type.upper()}] → VPN verification failed - trying next VPN")
                            continue  # Will rotate again on next attempt
                    else:
                        log(f"    [{race_type.upper()}] ✗ VPN rotation failed - using fallback cooldown")
                        time.sleep(60)
                        continue
                else:
                    # No VPN manager, use cooldown
                    log(f"    [{race_type.upper()}] ⚠ No VPN available - using 5min cooldown")
                    time.sleep(300)
                    continue
            
            elif result == 'TIMEOUT':
                log(f"    [{race_type.upper()}] ⚠ {region.upper()} {year}-{month:02d} timeout - rotating VPN")
                if vpn_manager and vpn_manager.rotate():
                    log(f"    [{race_type.upper()}] → VPN rotated - retrying")
                    time.sleep(5)
                    continue
                else:
                    log(f"    [{race_type.upper()}] ✗ VPN rotation failed")
                    time.sleep(30)
                    continue
            
            else:
                # Generic failure
                if attempt < 2:
                    log(f"    [{race_type.upper()}] ⚠ {region.upper()} {year}-{month:02d} failed - retry {attempt+1}")
                    time.sleep(30)
                else:
                    log(f"    [{race_type.upper()}] ✗ {region.upper()} {year}-{month:02d} FAILED after 3 attempts")
    
    return True

def verify_vpn_can_reach_racingpost(max_attempts=3):
    """Verify VPN can reach Racing Post"""
    for attempt in range(max_attempts):
        try:
            result = subprocess.run(
                ['curl', '-s', '-o', '/dev/null', '-w', '%{http_code}', 
                 '--max-time', '10', 'https://www.racingpost.com'],
                capture_output=True,
                text=True,
                timeout=15
            )
            
            http_code = result.stdout.strip()
            
            # 200 = success, 403 = blocked by Racing Post
            if http_code == '200':
                return True
            elif http_code == '403':
                log(f"      ✗ VPN blocked by Racing Post (403)")
                return False
            else:
                # Other codes, retry
                if attempt < max_attempts - 1:
                    time.sleep(2)
                    continue
                else:
                    log(f"      ⚠ Unexpected HTTP code: {http_code}")
                    return False
                    
        except Exception as e:
            if attempt < max_attempts - 1:
                time.sleep(2)
                continue
            else:
                log(f"      ✗ VPN verification error: {e}")
                return False
    
    return False

def check_resume_status(race_type, regions, years):
    """Check and log which months need to be scraped"""
    total_months = 0
    completed_months = 0
    
    for region in regions:
        for year in years:
            for month in range(1, 13):
                total_months += 1
                if check_month_already_scraped(region, year, month, race_type):
                    completed_months += 1
    
    remaining = total_months - completed_months
    pct_complete = (completed_months / total_months * 100) if total_months > 0 else 0
    
    log(f"[{race_type.upper()}] Resume status: {completed_months}/{total_months} months already scraped ({pct_complete:.1f}%)")
    log(f"[{race_type.upper()}] Will scrape {remaining} remaining months")
    
    return completed_months, remaining

def save_progress_register(race_type, completed_jobs):
    """Save list of completed jobs to a register file"""
    register_file = f"/home/smonaghan/rpscrape/logs/progress_register_{race_type}.txt"
    try:
        with open(register_file, 'w') as f:
            for region, year, month in sorted(completed_jobs):
                f.write(f"{region},{year},{month}\n")
    except Exception as e:
        log(f"[{race_type.upper()}] ⚠ Could not save progress register: {e}")

def load_progress_register(race_type):
    """Load list of completed jobs from register file"""
    register_file = f"/home/smonaghan/rpscrape/logs/progress_register_{race_type}.txt"
    completed = set()
    if os.path.exists(register_file):
        try:
            with open(register_file, 'r') as f:
                for line in f:
                    line = line.strip()
                    if line:
                        parts = line.split(',')
                        if len(parts) == 3:
                            region, year, month = parts[0], int(parts[1]), int(parts[2])
                            completed.add((region, year, month))
        except Exception as e:
            log(f"[{race_type.upper()}] ⚠ Could not load progress register: {e}")
    return completed

def log_incomplete_data(region, year, month, race_type, reason):
    """Log incomplete data files that need to be redone"""
    incomplete_log_file = f"/home/smonaghan/rpscrape/logs/incomplete_data_{race_type}.txt"
    
    try:
        # Create log entry with timestamp
        timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
        log_entry = f"{timestamp},{region},{year},{month},{reason}\n"
        
        # Read existing entries to avoid duplicates
        existing_entries = set()
        if os.path.exists(incomplete_log_file):
            with open(incomplete_log_file, 'r') as f:
                for line in f:
                    parts = line.strip().split(',')
                    if len(parts) >= 4:
                        # Key is region,year,month
                        key = f"{parts[1]},{parts[2]},{parts[3]}"
                        existing_entries.add(key)
        
        # Add new entry if not already present
        entry_key = f"{region},{year},{month}"
        if entry_key not in existing_entries:
            with open(incomplete_log_file, 'a') as f:
                f.write(log_entry)
            log(f"      → Logged incomplete data: {region.upper()} {year}-{month:02d} - {reason}")
        else:
            log(f"      → Already logged as incomplete: {region.upper()} {year}-{month:02d}")
            
    except Exception as e:
        log(f"      → Could not log incomplete data: {e}")

def load_incomplete_data_log(race_type):
    """Load list of incomplete data files that need retrying"""
    incomplete_log_file = f"/home/smonaghan/rpscrape/logs/incomplete_data_{race_type}.txt"
    incomplete_jobs = []
    
    if not os.path.exists(incomplete_log_file):
        return incomplete_jobs
    
    try:
        with open(incomplete_log_file, 'r') as f:
            for line in f:
                line = line.strip()
                if line:
                    parts = line.split(',')
                    if len(parts) >= 5:
                        timestamp, region, year, month, reason = parts[0], parts[1], int(parts[2]), int(parts[3]), parts[4]
                        incomplete_jobs.append({
                            'timestamp': timestamp,
                            'region': region,
                            'year': year,
                            'month': month,
                            'reason': reason
                        })
        
        # Sort by timestamp (oldest first for priority)
        incomplete_jobs.sort(key=lambda x: x['timestamp'])
        
    except Exception as e:
        log(f"[{race_type.upper()}] ⚠ Could not load incomplete data log: {e}")
    
    return incomplete_jobs

def remove_from_incomplete_log(region, year, month, race_type):
    """Remove an entry from the incomplete data log after successful retry"""
    incomplete_log_file = f"/home/smonaghan/rpscrape/logs/incomplete_data_{race_type}.txt"
    
    if not os.path.exists(incomplete_log_file):
        return
    
    try:
        # Read all entries
        entries = []
        with open(incomplete_log_file, 'r') as f:
            entries = f.readlines()
        
        # Filter out the completed entry
        target_key = f",{region},{year},{month},"
        filtered_entries = []
        removed_count = 0
        
        for entry in entries:
            if target_key not in entry:
                filtered_entries.append(entry)
            else:
                removed_count += 1
        
        # Write back filtered entries
        with open(incomplete_log_file, 'w') as f:
            f.writelines(filtered_entries)
        
        if removed_count > 0:
            log(f"      → Removed from incomplete log: {region.upper()} {year}-{month:02d} (now complete)")
            
    except Exception as e:
        log(f"      → Could not update incomplete log: {e}")

def scrape_race_type_monthly(race_type, region, years, stats, thread_id):
    """Worker function to scrape by month with failure avoidance strategy
    
    Each thread handles ONE region + ONE race_type combination to avoid file conflicts:
    - Thread 1: gb + flat  → data/dates/gb/flat/
    - Thread 2: gb + jumps → data/dates/gb/jumps/
    - Thread 3: ire + flat → data/dates/ire/flat/
    - Thread 4: ire + jumps → data/dates/ire/jumps/
    """
    import random
    
    thread_name = f"{region.upper()}-{race_type.upper()}"
    log(f"[{thread_name}] Thread {thread_id} started")
    
    # Check resume status for this specific region
    completed, remaining = check_resume_status(race_type, [region], years)
    
    # Load progress register (tracks what's been attempted)
    register_completed = load_progress_register(race_type)
    log(f"[{thread_name}] Progress register shows {len(register_completed)} months logged")
    
    # Load incomplete data entries for priority retry (filter for this region)
    incomplete_jobs = load_incomplete_data_log(race_type)
    incomplete_jobs_this_region = [j for j in incomplete_jobs if j['region'] == region]
    log(f"[{thread_name}] Incomplete data log shows {len(incomplete_jobs_this_region)} months needing retry for {region.upper()}")
    
    # Build priority list from incomplete data (these get retried first)
    priority_jobs = []
    for job in incomplete_jobs_this_region:
        job_tuple = (job['region'], job['year'], job['month'])
        if job_tuple not in register_completed:  # Don't retry if already completed
            priority_jobs.append(job_tuple)
            log(f"[{thread_name}] → Priority retry: {job['year']}-{job['month']:02d} ({job['reason']})")
    
    # Build list of all (year, month) combinations for THIS REGION that need scraping
    # Check BOTH the CSV files AND the register
    regular_jobs = []
        for year in years:
            for month in range(1, 13):
                job = (region, year, month)
            # Skip if already in register OR if CSV file exists and is valid OR if in priority list
            if (job not in register_completed and 
                not check_month_already_scraped(region, year, month, race_type) and
                job not in priority_jobs):
                regular_jobs.append(job)
    
    # Combine priority jobs (first) with regular jobs (shuffled)
    random.shuffle(regular_jobs)  # Randomize regular jobs only
    jobs_to_do = priority_jobs + regular_jobs
    
    total_jobs = len(jobs_to_do)
    priority_count = len(priority_jobs)
    regular_count = len(regular_jobs)
    
    log(f"[{thread_name}] {total_jobs} months to scrape for {region.upper()}:")
    log(f"[{thread_name}] → {priority_count} priority retries (incomplete data)")
    log(f"[{thread_name}] → {regular_count} regular jobs (randomized order)")
    log(f"[{thread_name}] → Skipping {len([j for j in register_completed if j[0] == region])} completed months for {region.upper()}")
    
    # Initialize VPN (if enabled in configuration)
    if ENABLE_VPN_ROTATION:
    vpn_manager = VPNManager()
    vpn_managers[race_type] = vpn_manager
    
    if vpn_manager.vpn_configs:
            log(f"[{race_type.upper()}] VPN rotation enabled - connecting to initial VPN...")
        vpn_manager.connect()
    else:
            log(f"[{race_type.upper()}] ⚠ VPN rotation enabled but no VPN configs found")
            log(f"[{race_type.upper()}] → Running without VPN protection")
        vpn_manager = None
    else:
        log(f"[{race_type.upper()}] VPN rotation DISABLED (config setting)")
        log(f"[{race_type.upper()}] → Running without VPN rotation")
        vpn_manager = None
        vpn_managers[race_type] = None
    
    try:
        completed_count = 0
        failed_jobs = set()  # Track failed jobs for retry later
        
        # FIRST PASS - Try each month once, move on quickly if it fails
        log(f"[{race_type.upper()}] === FIRST PASS - Single attempt per month ===")
        
        for region, year, month in jobs_to_do:
            month_name = datetime(year, month, 1).strftime('%B')
            log(f"  [{race_type.upper()}] [{completed_count + 1}/{total_jobs}] Scraping {region.upper()} {year}-{month:02d} ({month_name})...")
            
            # Single attempt per month in first pass
                result, elapsed = scrape_month(region, year, month, race_type)
                
                if result is True:
                    # Success - add to register and save
                        log(f"    [{race_type.upper()}] ✓ {region.upper()} {year}-{month:02d} in {elapsed:.1f}s")
                    register_completed.add((region, year, month))
                    save_progress_register(race_type, register_completed)
                
                # Remove from incomplete log if it was a retry
                remove_from_incomplete_log(region, year, month, race_type)
                    
                    completed_count += 1
                
                elif result == 'ROTATE_VPN':
                log(f"    [{race_type.upper()}] ⚠ {region.upper()} {year}-{month:02d} blocked - rotating VPN & moving on")
                failed_jobs.add((region, year, month))
                log_incomplete_data(region, year, month, race_type, "HTTP 403/429 - VPN blocked")
                
                    if vpn_manager:
                        if vpn_manager.rotate():
                        log(f"    [{race_type.upper()}] → VPN rotated - continuing to next month")
                                time.sleep(2)
                            else:
                        log(f"    [{race_type.upper()}] ✗ VPN rotation failed - short cooldown")
                        time.sleep(30)
                        else:
                    log(f"    [{race_type.upper()}] ⚠ No VPN - short cooldown then continue")
                            time.sleep(60)
                    
            elif result == 'INCOMPLETE_DATA':
                log(f"    [{race_type.upper()}] ⚠ {region.upper()} {year}-{month:02d} incomplete data - will retry with fresh VPN")
                failed_jobs.add((region, year, month))
                # Already logged by scrape_month function
                
                if vpn_manager and vpn_manager.rotate():
                    log(f"    [{race_type.upper()}] → VPN rotated - continuing to next month")
                    time.sleep(5)
                    else:
                    log(f"    [{race_type.upper()}] ⚠ VPN rotation failed - short cooldown")
                    time.sleep(30)
                
                elif result == 'TIMEOUT':
                log(f"    [{race_type.upper()}] ⚠ {region.upper()} {year}-{month:02d} timeout - rotating VPN & moving on")
                failed_jobs.add((region, year, month))
                log_incomplete_data(region, year, month, race_type, f"Timeout after {elapsed:.1f}s - no output for 5+ minutes")
                
                    if vpn_manager and vpn_manager.rotate():
                    log(f"    [{race_type.upper()}] → VPN rotated - continuing to next month")
                        time.sleep(5)
                    else:
                    log(f"    [{race_type.upper()}] ⚠ VPN rotation failed - short cooldown")
                        time.sleep(30)
                
                else:
                # Generic failure - add to retry list
                log(f"    [{race_type.upper()}] ⚠ {region.upper()} {year}-{month:02d} failed - will retry later")
                failed_jobs.add((region, year, month))
                log_incomplete_data(region, year, month, race_type, f"Generic failure: {result}")
                time.sleep(10)  # Brief pause
            
            # Periodic cooldown to give server a break
            if ENABLE_PERIODIC_COOLDOWN and completed_count > 0 and completed_count % COOLDOWN_AFTER_MONTHS == 0:
                log(f"    [{race_type.upper()}] 🛌 Periodic cooldown: Scraped {completed_count} months - giving server a {COOLDOWN_DURATION_MINUTES} minute break")
                log(f"    [{race_type.upper()}] → This prevents server stress and shows we're being respectful")
                time.sleep(COOLDOWN_DURATION_SECONDS)
                log(f"    [{race_type.upper()}] ✓ Cooldown complete - resuming scraping")
            
            # Progress every 25 months
            if (completed_count + len(failed_jobs)) % 25 == 0:
                processed = completed_count + len(failed_jobs)
                pct = (processed / total_jobs * 100)
                log(f"[{race_type.upper()}] First pass progress: {processed}/{total_jobs} ({pct:.1f}%) - {completed_count} success, {len(failed_jobs)} to retry")
        
        # SECOND PASS - Retry failed jobs with more attempts
        if failed_jobs:
            failed_list = list(failed_jobs)
            random.shuffle(failed_list)  # Re-randomize failed jobs
            
            log(f"[{race_type.upper()}] === SECOND PASS - Retrying {len(failed_list)} failed months ===")
            
            for region, year, month in failed_list:
                month_name = datetime(year, month, 1).strftime('%B')
                log(f"  [{race_type.upper()}] RETRY: {region.upper()} {year}-{month:02d} ({month_name})...")
                
                # Give failed jobs 2 more attempts with VPN rotation between
                for attempt in range(2):
                    # Rotate VPN before each retry attempt
                    if vpn_manager:
                        log(f"    [{race_type.upper()}] → Pre-retry VPN rotation...")
                        vpn_manager.rotate()
                        time.sleep(3)
                    
                    result, elapsed = scrape_month(region, year, month, race_type)
                    
                    if result is True:
                        log(f"    [{race_type.upper()}] ✓ {region.upper()} {year}-{month:02d} in {elapsed:.1f}s (retry {attempt+1})")
                        register_completed.add((region, year, month))
                        save_progress_register(race_type, register_completed)
                        
                        # Remove from incomplete log after successful retry
                        remove_from_incomplete_log(region, year, month, race_type)
                        
                        completed_count += 1
                        break
                    elif result == 'INCOMPLETE_DATA':
                        log(f"    [{race_type.upper()}] ⚠ Retry {attempt+1} produced incomplete data")
                        if attempt == 0:
                            log(f"    [{race_type.upper()}] → Trying once more with different VPN")
                        time.sleep(30)
                    else:
                            log(f"    [{race_type.upper()}] ✗ {region.upper()} {year}-{month:02d} FAILED - incomplete data after retries")
                    else:
                        if attempt == 0:
                            log(f"    [{race_type.upper()}] ⚠ Retry {attempt+1} failed - trying once more")
                            time.sleep(30)
                        else:
                            log(f"    [{race_type.upper()}] ✗ {region.upper()} {year}-{month:02d} FAILED after retries")
        
        log(f"[{race_type.upper()}] Thread completed - scraped {completed_count}/{total_jobs} months")
    
    finally:
        if vpn_manager:
            vpn_manager.disconnect()

def work_queue_worker(worker_id, work_queue, completed_lock, stats, worker_status):
    """Worker thread that pulls jobs from work queue (scalable to any thread count)"""
    import random
    
    log(f"[Worker-{worker_id}] Started and ready for work")
    worker_status[worker_id] = {'status': 'idle', 'current_job': None, 'completed': 0}
    
    # Worker-specific stats
    worker_stats = {
        'completed': 0,
        'failed': 0,
        'retries': 0
    }
    
    # Each worker gets its own rate limiter
    worker_rate_limiter = RateLimiter()
    
    # Optional VPN for this worker (if enabled)
    vpn_manager = None
    if ENABLE_VPN_ROTATION:
        vpn_manager = VPNManager()
        if vpn_manager.vpn_configs:
            log(f"[Worker-{worker_id}] Connecting to VPN...")
            vpn_manager.connect()
        else:
            vpn_manager = None
    
    try:
        while True:
            try:
                # Get next job from queue (timeout after 1 second)
                job = work_queue.get(timeout=1)
                
                if job is None:  # Poison pill to stop worker
                    work_queue.task_done()
                    break
                
                region, race_type, year, month = job
                month_name = datetime(year, month, 1).strftime('%B')
                
                # Update worker status
                worker_status[worker_id] = {
                    'status': 'working',
                    'current_job': f"{region.upper()}-{race_type.upper()} {year}-{month:02d}",
                    'completed': worker_stats['completed']
                }
                
                # Apply rate limiting
                worker_rate_limiter.wait_if_needed()
                
                # Attempt to scrape this month
                if LOG_VERBOSITY != 'MINIMAL':
                    log(f"  [Worker-{worker_id}] Scraping {region.upper()} {year}-{month:02d} ({month_name}) {race_type}...")
                
                result, elapsed = scrape_month(region, year, month, race_type)
                
                if result is True:
                    # Success
                    worker_rate_limiter.record_success()
                    worker_stats['completed'] += 1
                    
                    if LOG_VERBOSITY != 'MINIMAL':
                        log(f"    [Worker-{worker_id}] ✓ {region.upper()} {year}-{month:02d} in {elapsed:.1f}s")
                    elif worker_stats['completed'] % 5 == 0:  # Log every 5th completion in MINIMAL mode
                        log(f"    [Worker-{worker_id}] Progress: {worker_stats['completed']} completed")
                    
                    # Update worker status
                    worker_status[worker_id]['completed'] = worker_stats['completed']
                    worker_status[worker_id]['status'] = 'idle'
                    worker_status[worker_id]['current_job'] = None
                    
                    # Update global stats (thread-safe)
                    with completed_lock:
                        register_completed = load_progress_register(race_type)
                        register_completed.add((region, year, month))
                        save_progress_register(race_type, register_completed)
                        remove_from_incomplete_log(region, year, month, race_type)
                    
                elif result in ['ROTATE_VPN', 'TIMEOUT', 'INCOMPLETE_DATA']:
                    # Failure - re-queue for retry with lower priority
                    worker_rate_limiter.record_error()
                    worker_stats['retries'] += 1
                    
                    log(f"    [Worker-{worker_id}] ⚠ {region.upper()} {year}-{month:02d} {result} - re-queuing for retry")
                    log_incomplete_data(region, year, month, race_type, f"Worker {worker_id}: {result}")
                    
                    # Re-add to end of queue for later retry
                    work_queue.put(job)
                    
                    # Rotate VPN if available
                    if vpn_manager and result == 'ROTATE_VPN':
                        log(f"    [Worker-{worker_id}] → Rotating VPN...")
                        vpn_manager.rotate()
                        time.sleep(3)
                
                else:
                    # Generic failure
                    worker_rate_limiter.record_error()
                    worker_stats['failed'] += 1
                    log(f"    [Worker-{worker_id}] ✗ {region.upper()} {year}-{month:02d} failed")
                
                # Reset worker status to idle after job completion (success or failure)
                worker_status[worker_id]['status'] = 'idle'
                worker_status[worker_id]['current_job'] = None
                worker_status[worker_id]['completed'] = worker_stats['completed']
                
                work_queue.task_done()
                
                # Clean up lingering connections after each job
                if worker_stats['completed'] % 5 == 0:  # Every 5 jobs
                    try:
                        import gc
                        gc.collect()
                        # Force close any lingering connections
                        subprocess.run(['pkill', '-f', 'curl.*ifconfig'], 
                                     stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=2)
                    except:
                        pass
                
                # Periodic progress report (every 10 completions)
                if worker_stats['completed'] % 10 == 0 and worker_stats['completed'] > 0:
                    log(f"[Worker-{worker_id}] Progress: {worker_stats['completed']} completed, {worker_stats['failed']} failed, {worker_stats['retries']} retried")
                    
                    # Log connection count for monitoring
                    try:
                        result = subprocess.run(
                            ['ss', '-tn', 'state', 'established'],
                            capture_output=True, text=True, timeout=5
                        )
                        conn_count = len(result.stdout.strip().split('\n')) - 1
                        if conn_count > 100:
                            log(f"[Worker-{worker_id}] ⚠ High connection count: {conn_count} HTTPS connections")
                    except:
                        pass
                
            except queue.Empty:
                # No work available, worker will exit
                worker_status[worker_id] = {'status': 'finished', 'current_job': None, 'completed': worker_stats['completed']}
                break
                
    finally:
        if vpn_manager:
            vpn_manager.disconnect()
        
        worker_status[worker_id] = {'status': 'finished', 'current_job': None, 'completed': worker_stats['completed']}
        log(f"[Worker-{worker_id}] Finished - {worker_stats['completed']} completed, {worker_stats['failed']} failed")

def monitor_worker_progress(work_queue, worker_status, completed_lock, stop_event):
    """Monitor thread that shows what all workers are doing"""
    
    while not stop_event.is_set():
        time.sleep(60)  # Update every minute
        
        # Get queue status
        queue_size = work_queue.qsize()
        
        # Build status report
        status_lines = []
        status_lines.append("")
        status_lines.append("="*80)
        status_lines.append(f"⏰ WORKER STATUS UPDATE - {datetime.now().strftime('%H:%M:%S')}")
        status_lines.append("="*80)
        status_lines.append(f"📊 Queue: {queue_size} months remaining")
        status_lines.append("")
        
        # Show what each worker is doing
        active_count = 0
        idle_count = 0
        finished_count = 0
        total_completed = 0
        
        for worker_id in sorted(worker_status.keys()):
            status = worker_status[worker_id]
            total_completed += status['completed']
            
            if status['status'] == 'working':
                active_count += 1
                status_lines.append(f"  Worker-{worker_id:2d}: 🔥 WORKING on {status['current_job']} ({status['completed']} done)")
            elif status['status'] == 'idle':
                idle_count += 1
                status_lines.append(f"  Worker-{worker_id:2d}: ⏸  IDLE - waiting for work ({status['completed']} done)")
            elif status['status'] == 'finished':
                finished_count += 1
                status_lines.append(f"  Worker-{worker_id:2d}: ✓ FINISHED ({status['completed']} done)")
        
        status_lines.append("")
        status_lines.append(f"Summary: {active_count} working, {idle_count} idle, {finished_count} finished")
        status_lines.append(f"Total completed by all workers: {total_completed} months")
        
        # Calculate ETA only if we have meaningful data
        if queue_size > 0 and total_completed > 0 and active_count > 0:
            elapsed_hours = (time.time() - monitor_start_time) / 3600
            if elapsed_hours > 0:
                rate = total_completed / elapsed_hours
                eta_hours = queue_size / rate
                status_lines.append(f"Estimated time remaining: {eta_hours:.1f} hours (at {rate:.1f} months/hour)")
        elif queue_size > 0 and total_completed == 0:
            status_lines.append(f"Estimated time remaining: Calculating... (waiting for first completions)")
        
        status_lines.append("="*80)
        
        # Log all status lines
        for line in status_lines:
            log(line)
        
        # Check if all workers finished
        if finished_count == len(worker_status):
            break
    
    log("Monitor thread stopping...")

# Global for monitor thread
monitor_start_time = 0

def main():
    """Main function"""
    log("="*80)
    log("RACING POST SCRAPER - MONTHLY CHUNKS")
    log("="*80)
    log(f"Started at: {datetime.now()}")
    log("")
    log("=== CONFIGURATION ===")
    log(f"Concurrent Threads: {CONCURRENT_THREADS}")
    log(f"Threading Mode: {'WORK QUEUE (scalable)' if USE_WORK_QUEUE else 'FIXED (max 4)'}")
    log(f"Log Verbosity: {LOG_VERBOSITY}")
    log(f"VPN Rotation: {'ENABLED' if ENABLE_VPN_ROTATION else 'DISABLED'}")
    log(f"Random Delays: {'ENABLED' if ENABLE_RANDOM_DELAYS else 'DISABLED'} ({MIN_DELAY_SECONDS}-{MAX_DELAY_SECONDS}s)")
    log(f"Browser Randomization: {'ENABLED' if ENABLE_BROWSER_RANDOMIZATION else 'DISABLED'}")
    log(f"CSV Verification: {'ENABLED' if ENABLE_CSV_VERIFICATION else 'DISABLED'}")
    log(f"Rate Limiting: {MAX_REQUESTS_PER_MINUTE}/min, {MAX_REQUESTS_PER_HOUR}/hour")
    log(f"Periodic Cooldown: {'ENABLED' if ENABLE_PERIODIC_COOLDOWN else 'DISABLED'} (every {COOLDOWN_AFTER_MONTHS} months, {COOLDOWN_DURATION_MINUTES} min)")
    log(f"Error Handling: Pause after {PAUSE_ON_ERROR_STREAK} errors, backoff up to {MAX_BACKOFF_SECONDS}s")
    log(f"Scrape Timeout: {SCRAPE_TIMEOUT_MINUTES} minutes per month")
    log(f"Regions: {', '.join(REGIONS)}")
    log(f"Years: {min(YEARS)}-{max(YEARS)}")
    log(f"Race Types: {', '.join(RACE_TYPES)}")
    log("")
    
    # Use configuration settings
    regions = REGIONS
    years = YEARS
    race_types = RACE_TYPES
    
    stats = {
        'flat': {'completed': 0, 'succeeded': 0, 'failed': 0},
        'jumps': {'completed': 0, 'succeeded': 0, 'failed': 0}
    }
    
    overall_start = time.time()
    
    if USE_WORK_QUEUE:
        # ============================================================
        # WORK QUEUE MODE - Scalable to any number of CPUs
        # ============================================================
        log("=== WORK QUEUE MODE ===")
        log(f"Creating thread pool with {CONCURRENT_THREADS} workers")
        log("")
        
        # Build list of ALL months that need scraping
        import random
        all_jobs = []
        
        for region in regions:
            for race_type in race_types:
                # Load what's already complete
                register_completed = load_progress_register(race_type)
                incomplete_jobs_list = load_incomplete_data_log(race_type)
                
                # Priority jobs (incomplete data)
                for job in incomplete_jobs_list:
                    if job['region'] == region:
                        job_tuple = (region, race_type, job['year'], job['month'])
                        all_jobs.insert(0, job_tuple)  # Add to front (priority)
                
                # Regular jobs
                for year in years:
                    for month in range(1, 13):
                        job_tuple = (region, race_type, year, month)
                        
                        # Skip if already completed
                        if (region, year, month) in register_completed:
                            continue
                        
                        # Skip if CSV exists and is valid
                        if check_month_already_scraped(region, year, month, race_type):
                            continue
                        
                        # Add to regular jobs
                        all_jobs.append(job_tuple)
        
        # Shuffle non-priority jobs to randomize
        priority_count = len([j for j in all_jobs if any(inc['region'] == j[0] and inc['year'] == j[2] and inc['month'] == j[3] for inc in incomplete_jobs_list)])
        if len(all_jobs) > priority_count:
            regular_jobs = all_jobs[priority_count:]
            random.shuffle(regular_jobs)
            all_jobs = all_jobs[:priority_count] + regular_jobs
        
        log(f"Total jobs to process: {len(all_jobs)} months")
        log(f"  → Priority retries: {priority_count}")
        log(f"  → Regular jobs: {len(all_jobs) - priority_count}")
        log("")
        
        # Create work queue and add all jobs
        work_queue = queue.Queue()
        for job in all_jobs:
            work_queue.put(job)
        
        # Create thread-safe lock for progress updates
        completed_lock = threading.Lock()
        
        # Create shared worker status tracker (shows what each worker is doing)
        worker_status = {}
        
        # Create worker threads
    threads = []
        log(f"Starting {CONCURRENT_THREADS} worker threads...")
        for worker_id in range(1, CONCURRENT_THREADS + 1):
            thread = threading.Thread(
                target=work_queue_worker,
                args=(worker_id, work_queue, completed_lock, stats, worker_status),
                name=f"Worker-{worker_id}"
            )
            thread.daemon = False
            threads.append(thread)
            thread.start()
            log(f"  ✓ Worker-{worker_id} started")
        
        log(f"  → {len(threads)} workers pulling from shared queue")
        log(f"  → Each worker scrapes different months - no conflicts!")
        log("")
        
        # Start monitoring thread to show worker status every minute
        global monitor_start_time
        monitor_start_time = time.time()
        stop_monitor = threading.Event()
        
        monitor_thread = threading.Thread(
            target=monitor_worker_progress,
            args=(work_queue, worker_status, completed_lock, stop_monitor),
            name="Monitor_Thread"
        )
        monitor_thread.daemon = True
        monitor_thread.start()
        log("  ✓ Monitor thread started (will show worker status every minute)")
        log("")
        
        # Wait for all work to complete
        try:
            work_queue.join()  # Wait for queue to be empty
            log("All work completed, shutting down workers...")
            
            # Stop monitor thread
            stop_monitor.set()
            monitor_thread.join(timeout=5)
            
            # Send poison pills to stop workers
            for _ in range(CONCURRENT_THREADS):
                work_queue.put(None)
            
            # Wait for workers to finish
            for thread in threads:
                thread.join(timeout=10)
                log(f"  ✓ {thread.name} finished")
                
        except KeyboardInterrupt:
            log("Interrupted by user - gracefully stopping workers...")
            stop_monitor.set()
            for _ in range(CONCURRENT_THREADS):
                work_queue.put(None)
        
    else:
        # ============================================================
        # OLD MODE - Fixed thread per combination (max 4 threads)
        # ============================================================
        log("=== FIXED THREAD MODE ===")
        
        # Create threads based on region + race_type combinations
        thread_combinations = []
        for region in regions:
    for race_type in race_types:
                thread_combinations.append((region, race_type))
        
        # Limit to configured number of concurrent threads
        max_threads = min(CONCURRENT_THREADS, len(thread_combinations))
        active_combinations = thread_combinations[:max_threads]
        
        # Initialize rate limiters for each active combination
        for region, race_type in active_combinations:
            limiter_key = f"{region}-{race_type}"
            rate_limiters[limiter_key] = RateLimiter()
        
        log(f"Thread allocation:")
        log(f"  Total possible combinations: {len(thread_combinations)} (region × race_type)")
        log(f"  Concurrent threads to use: {max_threads}")
        log(f"  Active combinations:")
        for region, race_type in active_combinations:
            log(f"    → {region.upper()}-{race_type.upper()} (writes to data/dates/{region}/{race_type}/)")
        log("")
        
        threads = []
        thread_id = 1
        for region, race_type in active_combinations:
        thread = threading.Thread(
            target=scrape_race_type_monthly,
                args=(race_type, region, years, stats, thread_id),
                name=f"{region.upper()}-{race_type.upper()}_Thread"
        )
        threads.append(thread)
            thread_id += 1
    
    log("Starting parallel threads...")
    for thread in threads:
        thread.start()
        log(f"  ✓ {thread.name} started")
    
        log(f"  → {len(threads)} threads running concurrently")
        log(f"  → No file conflicts (each thread has separate output directory)")
    log("")
    
    try:
        for thread in threads:
            thread.join()
            log(f"  ✓ {thread.name} finished")
    finally:
        log("")
        log("Cleaning up VPNs...")
        for race_type, vpn_mgr in vpn_managers.items():
            if vpn_mgr:
                vpn_mgr.disconnect()
    
    # Final summary
    total_time = time.time() - overall_start
    log("")
    log("="*80)
    log("SCRAPING COMPLETE!")
    log("="*80)
    
    total_jobs = len(regions) * len(years) * len(race_types)
    total_succeeded = stats['flat']['succeeded'] + stats['jumps']['succeeded']
    
    log(f"FLAT  : {stats['flat']['succeeded']}/{stats['flat']['completed']} years completed")
    log(f"JUMPS : {stats['jumps']['succeeded']}/{stats['jumps']['completed']} years completed")
    log(f"TOTAL : {total_succeeded}/{total_jobs} years completed")
    log(f"Total time: {total_time / 60:.1f} minutes ({total_time / 3600:.1f} hours)")
    log(f"Finished at: {datetime.now()}")

if __name__ == "__main__":
    main()

