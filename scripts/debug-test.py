#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Debug Test Script - Minimal test to diagnose script execution issues
"""

import sys
import os

sys.stdout.reconfigure(encoding='utf-8')
sys.stderr.reconfigure(encoding='utf-8')

print("=== Debug Test Started ===", flush=True)
print(f"Python version: {sys.version}", flush=True)
print(f"Working directory: {os.getcwd()}", flush=True)
print(f"Script path: {__file__}", flush=True)

# Test 1: Import test
print("\n[Test 1] Testing imports...", flush=True)
try:
    import json
    import urllib.request
    import urllib.error
    from pathlib import Path
    print("✓ All imports successful", flush=True)
except Exception as e:
    print(f"✗ Import failed: {e}", flush=True)
    sys.exit(1)

# Test 2: HTTP request test
print("\n[Test 2] Testing HTTP connection...", flush=True)
try:
    BASE_URL = "http://198.18.0.1:8765"
    response = urllib.request.urlopen(f'{BASE_URL}/health', timeout=5)
    data = json.loads(response.read().decode('utf-8'))
    print(f"✓ Health check successful: {data}", flush=True)
except urllib.error.URLError as e:
    print(f"✗ Connection failed: {e}", flush=True)
    print("  Make sure llm-proxy is running on localhost:8765", flush=True)
except Exception as e:
    print(f"✗ Unexpected error: {e}", flush=True)

# Test 3: API request test
print("\n[Test 3] Testing API request...", flush=True)
try:
    API_KEY = "sk-aNbDRYsSMcbdVUptFyy9yWpeN6agx"
    body = json.dumps({
        "model": "deepseek/deepseek-v3.2",
        "messages": [{"role": "user", "content": "Say 'test OK' and nothing else"}],
        "max_tokens": 10,
        "stream": False
    }).encode('utf-8')
    
    req = urllib.request.Request(
        f'{BASE_URL}/v1/chat/completions',
        data=body,
        headers={
            'Authorization': f'Bearer {API_KEY}',
            'Content-Type': 'application/json'
        }
    )
    
    response = urllib.request.urlopen(req, timeout=30)
    result = json.loads(response.read().decode('utf-8'))
    content = result['choices'][0]['message']['content']
    print(f"✓ API test successful: {content}", flush=True)
except Exception as e:
    print(f"✗ API test failed: {e}", flush=True)

# Test 4: Colors test (Windows compatibility)
print("\n[Test 4] Testing color output...", flush=True)
try:
    class Colors:
        CYAN = '\033[0;36m'
        GREEN = '\033[0;32m'
        YELLOW = '\033[1;33m'
        RED = '\033[0;31m'
        BLUE = '\033[0;34m'
        RESET = '\033[0m'
    
    print(f"{Colors.GREEN}✓ Green text{Colors.RESET}", flush=True)
    print(f"{Colors.RED}✗ Red text{Colors.RESET}", flush=True)
    print(f"{Colors.CYAN}Info text{Colors.RESET}", flush=True)
except Exception as e:
    print(f"✗ Color test failed: {e}", flush=True)

print("\n=== All Debug Tests Completed ===", flush=True)
print("If you see this message, Python execution is working correctly.", flush=True)
print("Try running the main test scripts again.", flush=True)
