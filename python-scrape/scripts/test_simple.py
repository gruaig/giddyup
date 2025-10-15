#!/usr/bin/env python3
"""Simple test to verify imports and basic functionality"""

import sys
print("Python executable:", sys.executable)
print("Python version:", sys.version)

# Test imports
print("\nTesting imports...")
try:
    import requests
    print("✓ requests")
except ImportError as e:
    print("✗ requests:", e)

try:
    from lxml import html, etree
    print("✓ lxml")
except ImportError as e:
    print("✗ lxml:", e)

try:
    import orjson
    print("✓ orjson")
except ImportError as e:
    print("✗ orjson:", e)

try:
    import aiohttp
    print("✓ aiohttp")
except ImportError as e:
    print("✗ aiohttp:", e)

try:
    from utils.cleaning import normalize_name
    print("✓ utils.cleaning")
except ImportError as e:
    print("✗ utils.cleaning:", e)

# Simple test request
print("\nTesting basic web request...")
try:
    response = requests.get("https://www.racingpost.com", timeout=5)
    print(f"✓ Racing Post accessible (status: {response.status_code})")
except Exception as e:
    print(f"✗ Request failed: {e}")

print("\nAll basic checks complete!")

