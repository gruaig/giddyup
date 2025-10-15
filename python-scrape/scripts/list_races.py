#!/usr/bin/env python3
"""Simple script to list today's races"""

import requests
from lxml import html
from datetime import datetime

def get_todays_races():
    """Fetch and display today's race meetings"""
    
    # Get today's date in the format Racing Post uses
    today = datetime.now().strftime('%Y-%m-%d')
    url = f'https://www.racingpost.com/racecards/{today}'
    
    print(f"\nFetching races for {today}...")
    print(f"URL: {url}\n")
    
    try:
        headers = {
            'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36'
        }
        response = requests.get(url, headers=headers, timeout=10)
        response.raise_for_status()
        
        tree = html.fromstring(response.content)
        
        # Try to find race meetings/courses
        # The structure may vary, so let's try a few selectors
        print(f"Status Code: {response.status_code}")
        print(f"Page Title: {tree.xpath('//title/text()')}")
        print(f"\nPage loaded successfully ({len(response.content)} bytes)")
        
        # Look for course names or race meetings
        courses = tree.xpath('//a[contains(@href, "/racecards/")]/@href')
        print(f"\nFound {len(courses)} racecard links")
        
        # Get unique courses
        unique_courses = list(set(courses))[:10]  # First 10 unique
        for course in unique_courses:
            print(f"  - {course}")
        
        return True
        
    except Exception as e:
        print(f"Error: {e}")
        import traceback
        traceback.print_exc()
        return False

if __name__ == "__main__":
    get_todays_races()

