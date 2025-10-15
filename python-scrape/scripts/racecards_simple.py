#!/usr/bin/env python3
"""Simplified racecards script with debug output"""

import os
import requests
import sys
from datetime import datetime
from lxml import html
from orjson import dumps, OPT_NON_STR_KEYS

print("Starting racecards script...")

def get_race_urls(session, url):
    print(f"Fetching race URLs from: {url}")
    
    headers = {
        'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36'
    }
    
    r = session.get(url, headers=headers)
    print(f"Response status: {r.status_code}")
    
    if r.status_code != 200:
        print(f"Failed to fetch racecards: {r.status_code}")
        return []
    
    doc = html.fromstring(r.content)
    
    race_urls = []
    meetings = doc.xpath('//section[@data-accordion-row]')
    print(f"Found {len(meetings)} meetings")
    
    for meeting in meetings:
        try:
            course = meeting.xpath(".//span[contains(@class, 'RC-accordion__courseName')]")[0]
            course_name = course.text_content().strip()
            print(f"  Course: {course_name}")
            
            races = meeting.xpath(".//a[@class='RC-meetingItem__link js-navigate-url']")
            print(f"    Found {len(races)} races")
            
            for race in races:
                race_url = 'https://www.racingpost.com' + race.attrib['href']
                race_urls.append(race_url)
        except (IndexError, KeyError) as e:
            print(f"    Error parsing meeting: {e}")
            continue
    
    print(f"Total race URLs: {len(race_urls)}")
    return sorted(list(set(race_urls)))

def main():
    print("Python version:", sys.version)
    
    if len(sys.argv) != 2 or sys.argv[1].lower() not in {'today', 'tomorrow'}:
        return print('Usage: ./racecards_simple.py [today|tomorrow]')
    
    racecard_url = 'https://www.racingpost.com/racecards'
    date = datetime.today().strftime('%Y-%m-%d')
    
    if sys.argv[1].lower() == 'tomorrow':
        racecard_url += '/tomorrow'
    
    print(f"Fetching racecards for: {date}")
    
    session = requests.Session()
    race_urls = get_race_urls(session, racecard_url)
    
    # Just save the URLs for now
    output = {
        'date': date,
        'race_urls': race_urls,
        'count': len(race_urls)
    }
    
    if not os.path.exists('../racecards'):
        os.makedirs('../racecards')
        print("Created racecards directory")
    
    output_file = f'../racecards/{date}_simple.json'
    with open(output_file, 'w', encoding='utf-8') as f:
        f.write(dumps(output, option=OPT_NON_STR_KEYS).decode('utf-8'))
    
    print(f"\nSaved to: {output_file}")
    print(f"Found {len(race_urls)} races")

if __name__ == '__main__':
    main()

