#!/usr/bin/env python3
"""
Configuration and Browser Presets

This example demonstrates:
- Using configure() for global defaults
- Different browser presets
- Forcing HTTP versions
"""

import httpcloak

# Configure global defaults
print("=" * 60)
print("Example 1: Configure Global Defaults")
print("-" * 60)

httpcloak.configure(
    preset="chrome-143-linux",
    headers={"Accept-Language": "en-US,en;q=0.9"},
    timeout=30,
)

r = httpcloak.get("https://www.cloudflare.com/cdn-cgi/trace")
print(f"Protocol: {r.protocol}")
print(f"First few lines of trace:")
for line in r.text.split("\n")[:5]:
    print(f"  {line}")

# Different browser presets
print("\n" + "=" * 60)
print("Example 2: Different Browser Presets")
print("-" * 60)

presets = [
    "chrome-143",
    "chrome-143-windows",
    "chrome-143-linux",
    "chrome-131",
    "firefox-133",
    "safari-18",
]

for preset in presets:
    session = httpcloak.Session(preset=preset)
    r = session.get("https://www.cloudflare.com/cdn-cgi/trace")

    # Parse trace to get HTTP version
    trace = dict(line.split("=", 1) for line in r.text.strip().split("\n") if "=" in line)
    print(f"{preset:25} | Protocol: {r.protocol:5} | http={trace.get('http', 'N/A')}")
    session.close()

# Force HTTP versions
print("\n" + "=" * 60)
print("Example 3: Force HTTP Versions")
print("-" * 60)

http_versions = ["auto", "h1", "h2", "h3"]

for version in http_versions:
    session = httpcloak.Session(preset="chrome-143", http_version=version)
    try:
        r = session.get("https://www.cloudflare.com/cdn-cgi/trace")
        trace = dict(line.split("=", 1) for line in r.text.strip().split("\n") if "=" in line)
        print(f"http_version={version:5} | Actual Protocol: {r.protocol:5} | http={trace.get('http', 'N/A')}")
    except Exception as e:
        print(f"http_version={version:5} | Error: {e}")
    finally:
        session.close()

# List available presets
print("\n" + "=" * 60)
print("Example 4: List Available Presets")
print("-" * 60)

presets = httpcloak.available_presets()
print("Available presets:")
for preset in presets:
    print(f"  - {preset}")

print(f"\nhttpcloak version: {httpcloak.version()}")

print("\n" + "=" * 60)
print("Configuration examples completed!")
print("=" * 60)
