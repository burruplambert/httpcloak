# HTTPCloak Python

Browser fingerprint emulation HTTP client with HTTP/1.1, HTTP/2, and HTTP/3 support.

## Installation

```bash
pip install httpcloak
```

## Quick Start

### Synchronous Usage

```python
from httpcloak import Session

# Create a session with Chrome fingerprint
session = Session(preset="chrome-143")

# Make requests
response = session.get("https://www.cloudflare.com/cdn-cgi/trace")
print(response.status_code)
print(response.text)

# POST request
response = session.post("https://api.example.com/data", body={"key": "value"})

# Custom headers
response = session.get("https://example.com", headers={"X-Custom": "value"})

# With proxy
session = Session(preset="chrome-143", proxy="http://user:pass@host:port")
```

### Asynchronous Usage

```python
import asyncio
from httpcloak import Session

async def main():
    session = Session(preset="chrome-143")

    # Async GET
    response = await session.get_async("https://example.com")
    print(response.text)

    # Async POST
    response = await session.post_async("https://api.example.com/data", body={"key": "value"})

    # Multiple concurrent requests
    responses = await asyncio.gather(
        session.get_async("https://example.com/1"),
        session.get_async("https://example.com/2"),
        session.get_async("https://example.com/3"),
    )

asyncio.run(main())
```

### Cookie Management

```python
from httpcloak import Session

session = Session()

# Get all cookies
cookies = session.get_cookies()
print(cookies)

# Set a cookie
session.set_cookie("session_id", "abc123")

# Access cookies as property
print(session.cookies)
```

## Available Presets

```python
from httpcloak import available_presets

print(available_presets())
# ['chrome-143', 'chrome-143-windows', 'chrome-143-linux', 'chrome-143-macos',
#  'chrome-131', 'firefox-133', 'safari-18', ...]
```

## Response Object

```python
response = session.get("https://example.com")

response.status_code  # int: HTTP status code
response.headers      # dict: Response headers
response.body         # bytes: Raw response body
response.text         # str: Response body as text
response.final_url    # str: Final URL after redirects
response.protocol     # str: Protocol used (http/1.1, h2, h3)
```

## Error Handling

```python
from httpcloak import Session, HTTPCloakError

try:
    session = Session()
    response = session.get("https://example.com")
except HTTPCloakError as e:
    print(f"Request failed: {e}")
```

## Context Manager

```python
from httpcloak import Session

with Session(preset="chrome-143") as session:
    response = session.get("https://example.com")
    print(response.text)
# Session automatically closed
```

## Proxy Support

HTTPCloak supports HTTP, SOCKS5, and HTTP/3 (MASQUE) proxies with full fingerprint preservation.

### HTTP Proxy

```python
from httpcloak import Session

# Basic HTTP proxy
session = Session(preset="chrome-143", proxy="http://host:port")

# With authentication
session = Session(preset="chrome-143", proxy="http://user:pass@host:port")

# HTTPS proxy
session = Session(preset="chrome-143", proxy="https://user:pass@host:port")
```

### SOCKS5 Proxy

```python
from httpcloak import Session

# SOCKS5 proxy (with DNS resolution on proxy)
session = Session(preset="chrome-143", proxy="socks5h://host:port")

# With authentication
session = Session(preset="chrome-143", proxy="socks5h://user:pass@host:port")

response = session.get("https://www.cloudflare.com/cdn-cgi/trace")
print(response.protocol)  # h3 (HTTP/3 through SOCKS5!)
```

### HTTP/3 MASQUE Proxy

MASQUE (RFC 9484) enables HTTP/3 connections through compatible proxies:

```python
from httpcloak import Session

# MASQUE proxy (auto-detected for known providers like Bright Data)
session = Session(preset="chrome-143", proxy="https://user:pass@brd.superproxy.io:10001")

response = session.get("https://www.cloudflare.com/cdn-cgi/trace")
print(response.protocol)  # h3
```

## Advanced Features

### Encrypted Client Hello (ECH)

ECH encrypts the SNI (Server Name Indication) to prevent traffic analysis. Works with all Cloudflare domains:

```python
from httpcloak import Session

# Enable ECH for Cloudflare domains
session = Session(preset="chrome-143", ech_config_domain="cloudflare-ech.com")

response = session.get("https://www.cloudflare.com/cdn-cgi/trace")
print(response.text)
# Output includes: sni=encrypted, http=http/3
```

### Domain Fronting (Connect-To)

Connect to one server while requesting a different domain:

```python
from httpcloak import Session

# Connect to claude.ai's IP but request www.cloudflare.com
session = Session(
    preset="chrome-143",
    connect_to={"www.cloudflare.com": "claude.ai"}
)

response = session.get("https://www.cloudflare.com/cdn-cgi/trace")
```

### Combined: SOCKS5 + ECH

Get HTTP/3 with encrypted SNI through a SOCKS5 proxy:

```python
from httpcloak import Session

session = Session(
    preset="chrome-143",
    proxy="socks5h://user:pass@host:port",
    ech_config_domain="cloudflare-ech.com"
)

response = session.get("https://www.cloudflare.com/cdn-cgi/trace")
# Response shows: http=http/3, sni=encrypted
```

## Platform Support

- Linux (x64, arm64)
- macOS (x64, arm64)
- Windows (x64, arm64)

## License

MIT
