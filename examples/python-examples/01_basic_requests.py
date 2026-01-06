#!/usr/bin/env python3
"""
Basic HTTP Requests with httpcloak

This example demonstrates:
- Simple GET and POST requests
- Using module-level functions
- Accessing response data
"""

import httpcloak

# Simple GET request
print("=" * 60)
print("Example 1: Simple GET Request")
print("-" * 60)

r = httpcloak.get("https://httpbin.org/get")
print(f"Status: {r.status_code}")
print(f"Protocol: {r.protocol}")
print(f"OK: {r.ok}")

# GET with query parameters
print("\n" + "=" * 60)
print("Example 2: GET with Query Parameters")
print("-" * 60)

r = httpcloak.get("https://httpbin.org/get", params={"search": "httpcloak", "page": 1})
print(f"Status: {r.status_code}")
print(f"Final URL: {r.url}")

# POST with JSON body
print("\n" + "=" * 60)
print("Example 3: POST with JSON Body")
print("-" * 60)

r = httpcloak.post("https://httpbin.org/post", json={"name": "httpcloak", "version": "1.0"})
print(f"Status: {r.status_code}")
data = r.json()
print(f"Echoed JSON: {data.get('json')}")

# POST with form data
print("\n" + "=" * 60)
print("Example 4: POST with Form Data")
print("-" * 60)

r = httpcloak.post("https://httpbin.org/post", data={"username": "user", "password": "pass"})
print(f"Status: {r.status_code}")
data = r.json()
print(f"Echoed Form: {data.get('form')}")

# Custom headers
print("\n" + "=" * 60)
print("Example 5: Custom Headers")
print("-" * 60)

r = httpcloak.get("https://httpbin.org/headers", headers={
    "X-Custom-Header": "my-value",
    "X-Request-ID": "12345"
})
print(f"Status: {r.status_code}")
data = r.json()
print(f"Custom header received: {data['headers'].get('X-Custom-Header')}")

# Response helpers
print("\n" + "=" * 60)
print("Example 6: Response Helpers")
print("-" * 60)

r = httpcloak.get("https://httpbin.org/json")
print(f"Status Code: {r.status_code}")
print(f"Is OK (status < 400): {r.ok}")
print(f"Content type: {r.headers.get('content-type')}")
print(f"Body length: {len(r.content)} bytes")

# Parse JSON
data = r.json()
print(f"JSON parsed: {type(data)}")

print("\n" + "=" * 60)
print("All basic examples completed!")
print("=" * 60)
