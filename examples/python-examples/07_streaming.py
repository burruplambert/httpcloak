#!/usr/bin/env python3
"""
Streaming Downloads with httpcloak

This example demonstrates how to stream large files without loading
them entirely into memory.

What you'll learn:
- Using get_stream() for memory-efficient downloads
- Reading response data in chunks
- Progress tracking during downloads
- When to use streaming vs get_fast()

Use Cases:
- Downloading files larger than available memory
- Progress bars for large downloads
- Processing data as it arrives
- Piping data to another destination

Requirements:
    pip install httpcloak

Run:
    python 07_streaming.py
"""

import httpcloak
import time
import sys

print("=" * 70)
print("httpcloak - Streaming Downloads")
print("=" * 70)

# =============================================================================
# Understanding Streaming
# =============================================================================
print("\n[INFO] Understanding Streaming")
print("-" * 50)
print("""
Streaming allows you to process response data as it arrives,
without loading the entire response into memory.

Use streaming when:
- File is larger than available memory
- You want to show download progress
- Processing data incrementally (parsing, transforming)
- Writing to disk as data arrives
""")

session = httpcloak.Session(preset="chrome-143")

# =============================================================================
# Example 1: Basic Streaming
# =============================================================================
print("\n[1] Basic Streaming")
print("-" * 50)

# Start a streaming request
stream = session.get_stream("https://httpbin.org/bytes/102400")

print(f"Status Code: {stream.status_code}")
print(f"Protocol: {stream.protocol}")
print(f"Content-Length: {stream.content_length}")

# Read data in chunks
total_bytes = 0
chunk_count = 0
while True:
    chunk = stream.read(8192)  # Read up to 8KB at a time
    if not chunk:
        break
    total_bytes += len(chunk)
    chunk_count += 1

stream.close()
print(f"Read {total_bytes} bytes in {chunk_count} chunks")

# =============================================================================
# Example 2: Download with Progress
# =============================================================================
print("\n[2] Download with Progress")
print("-" * 50)

stream = session.get_stream("https://httpbin.org/bytes/51200")
content_length = stream.content_length
downloaded = 0

print(f"Downloading {content_length} bytes...")
start_time = time.perf_counter()

while True:
    chunk = stream.read(4096)
    if not chunk:
        break
    downloaded += len(chunk)

    # Calculate progress
    if content_length > 0:
        percent = (downloaded / content_length) * 100
        bar_width = 40
        filled = int(bar_width * downloaded / content_length)
        bar = "=" * filled + "-" * (bar_width - filled)
        sys.stdout.write(f"\r[{bar}] {percent:.1f}%")
        sys.stdout.flush()

elapsed = time.perf_counter() - start_time
stream.close()

print(f"\nCompleted: {downloaded} bytes in {elapsed*1000:.0f}ms")

# =============================================================================
# Example 3: Stream to File
# =============================================================================
print("\n[3] Stream to File")
print("-" * 50)

import tempfile
import os

stream = session.get_stream("https://httpbin.org/bytes/102400")

with tempfile.NamedTemporaryFile(delete=False) as f:
    temp_path = f.name
    bytes_written = 0

    while True:
        chunk = stream.read(16384)
        if not chunk:
            break
        f.write(chunk)
        bytes_written += len(chunk)

stream.close()

file_size = os.path.getsize(temp_path)
print(f"Streamed {bytes_written} bytes to file")
print(f"File size on disk: {file_size} bytes")
os.unlink(temp_path)

# =============================================================================
# Example 4: Iterator Pattern
# =============================================================================
print("\n[4] Iterator Pattern")
print("-" * 50)

stream = session.get_stream("https://httpbin.org/bytes/32768")

# Use the iterator for cleaner code
chunks = list(stream.iter_content(chunk_size=8192))
stream.close()

print(f"Received {len(chunks)} chunks")
print(f"Total bytes: {sum(len(c) for c in chunks)}")

# =============================================================================
# Example 5: Streaming with Different Protocols
# =============================================================================
print("\n[5] Streaming with Different Protocols")
print("-" * 50)

# HTTP/2 streaming
session_h2 = httpcloak.Session(preset="chrome-143", http_version="h2")
stream = session_h2.get_stream("https://cloudflare.com/cdn-cgi/trace")
data = b""
while True:
    chunk = stream.read(1024)
    if not chunk:
        break
    data += chunk
stream.close()
print(f"HTTP/2 stream: {len(data)} bytes, protocol: {stream.protocol}")
session_h2.close()

# HTTP/3 streaming
session_h3 = httpcloak.Session(preset="chrome-143", http_version="h3")
stream = session_h3.get_stream("https://cloudflare.com/cdn-cgi/trace")
data = b""
while True:
    chunk = stream.read(1024)
    if not chunk:
        break
    data += chunk
stream.close()
print(f"HTTP/3 stream: {len(data)} bytes, protocol: {stream.protocol}")
session_h3.close()

# =============================================================================
# Example 6: When to Use Streaming vs get_fast()
# =============================================================================
print("\n[6] Streaming vs get_fast() Comparison")
print("-" * 50)
print("""
STREAMING (get_stream):
- Memory efficient - only holds one chunk at a time
- Good for files larger than RAM
- Enables progress tracking
- Slower due to chunk-by-chunk processing

get_fast():
- Fastest download speed
- Loads entire response into memory
- Best for files that fit in memory
- ~10-50x faster than streaming for small/medium files

RECOMMENDATIONS:
- Files < 100MB: Use get_fast()
- Files > 100MB or unknown size: Use streaming
- Need progress bar: Use streaming
- Memory constrained: Use streaming
""")

# =============================================================================
# Cleanup
# =============================================================================
session.close()

print("\n" + "=" * 70)
print("SUMMARY")
print("=" * 70)
print("""
Streaming methods:
- get_stream(url) - Start streaming GET request
- stream.read(size) - Read up to 'size' bytes
- stream.iter_content(chunk_size) - Iterate over chunks
- stream.close() - Close the stream

Properties:
- stream.status_code - HTTP status
- stream.headers - Response headers
- stream.content_length - Total size (if known)
- stream.protocol - HTTP protocol used
""")
