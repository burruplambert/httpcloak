# httpcloak

A Go HTTP client library with **browser-identical TLS/HTTP fingerprinting**. Makes HTTP requests indistinguishable from real browsers, bypassing bot detection systems that fingerprint TLS handshakes, HTTP/2 settings, and header patterns.

**Bindings available for:** Go (native) | Python | Node.js | C#

## Why This Library?

Modern bot detection doesn't just check headers or cookies - it analyzes the **cryptographic fingerprint** of your connection:

1. **TLS Fingerprint (JA3/JA4)**: Cipher suites, extensions, and elliptic curves in the TLS handshake
2. **HTTP/2 Fingerprint**: SETTINGS frame values, WINDOW_UPDATE, PRIORITY frames
3. **HTTP/3 Fingerprint**: QUIC transport parameters and settings
4. **Header Order**: The exact order and format of HTTP headers

Go's standard `net/http` has a recognizable fingerprint that bot detection systems (Cloudflare, Akamai, PerimeterX) identify instantly.

### Fingerprint Comparison

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    FINGERPRINT COMPARISON (from tls.peet.ws)                    │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  METRIC                    │ GO STDLIB         │ HTTPCLOAK         │ CHROME 143 │
│  ──────────────────────────┼───────────────────┼───────────────────┼─────────── │
│  Cipher Suites             │ 13                │ 16                │ 16         │
│  TLS Extensions            │ 12                │ 18                │ 18         │
│  GREASE Values             │ None              │ Yes (random)      │ Yes        │
│  Post-Quantum (X25519MLKEM)│ No                │ Yes               │ Yes        │
│  ECH Support               │ No                │ Yes               │ Yes        │
│                                                                                 │
│  HTTP/2 SETTINGS                                                                │
│  ──────────────────────────┼───────────────────┼───────────────────┼────────────│
│  HEADER_TABLE_SIZE         │ 4,096             │ 65,536            │ 65,536     │
│  ENABLE_PUSH               │ 1                 │ 0                 │ 0          │
│  INITIAL_WINDOW_SIZE       │ 65,535 (64KB)     │ 6,291,456 (6MB)   │ 6,291,456  │
│  MAX_HEADER_LIST_SIZE      │ 10,485,760        │ 262,144           │ 262,144    │
│                                                                                 │
│  FINGERPRINT HASHES                                                             │
│  ──────────────────────────┼───────────────────┼───────────────────┼────────────│
│  JA4                       │ t13d1312h2_...    │ t13d1516h2_8daaf6...    MATCH  │
│  Akamai HTTP/2             │ cbcbfae223...     │ 52d84b11737d...         MATCH  │
│  peetprint                 │ (different)       │ 1d4ffe9b0e34...         MATCH  │
│                                                                                 │
│  RESULT                    │ BLOCKED           │ PASSED            │ PASSED     │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Cloudflare CDN Trace

The `/cdn-cgi/trace` endpoint reveals connection details. Here's what httpcloak achieves:

```
fl=283f39
h=www.cloudflare.com
ip=2401:4900:8899:xxxx:xxxx:xxxx:xxxx:xxxx
ts=1767716387.683
visit_scheme=https
uag=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36
colo=CCU
sliver=none
http=http/3          <-- HTTP/3 (QUIC) connection
loc=IN
tls=TLSv1.3          <-- TLS 1.3
sni=plaintext
warp=off
gateway=off
rbi=off
kex=X25519MLKEM768   <-- Post-quantum key exchange (Chrome 143)
```

**Key fields:**
- `http=http/3` - Using HTTP/3 over QUIC (fastest, most modern protocol)
- `tls=TLSv1.3` - TLS 1.3 encryption
- `kex=X25519MLKEM768` - Post-quantum hybrid key exchange (only Chrome 131+ supports this)
- `uag` - User-Agent matching Chrome 143

The `kex=X25519MLKEM768` is critical - it's Chrome's post-quantum cryptography that Go's stdlib doesn't support. Bot detection systems check for this.

### HTTP/3 Support

HTTP/3 uses QUIC (UDP-based) instead of TCP, providing:
- **Faster connections**: 0-RTT resumption, no TCP handshake
- **Better performance**: No head-of-line blocking
- **Unique fingerprint**: QUIC transport parameters are also fingerprinted

httpcloak supports HTTP/3 with proper Chrome fingerprinting, automatically falling back to HTTP/2 or HTTP/1.1 when needed.

---

## Installation

### Go
```bash
go get github.com/sardanioss/httpcloak
```

### Python
```bash
pip install httpcloak
```

### Node.js
```bash
npm install httpcloak
```

### C# / .NET
```bash
dotnet add package HttpCloak
```

---

## Usage

### Go

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/sardanioss/httpcloak/client"
)

func main() {
    // Create client with Chrome 143 fingerprint
    c := client.NewClient("chrome-143")
    defer c.Close()

    // Simple GET request
    resp, err := c.Get(context.Background(), "https://www.cloudflare.com/cdn-cgi/trace", nil)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Status: %d\n", resp.StatusCode)
    fmt.Printf("Protocol: %s\n", resp.Protocol) // "h2" or "h3"
    fmt.Println(resp.Text())
}
```

#### POST with JSON

```go
c := client.NewClient("chrome-143")
defer c.Close()

body := []byte(`{"username": "test", "password": "secret"}`)
resp, err := c.Do(context.Background(), &client.Request{
    Method:  "POST",
    URL:     "https://api.example.com/login",
    Body:    body,
    Headers: map[string]string{
        "Content-Type": "application/json",
    },
})
```

#### Session with Cookies

```go
// Sessions persist cookies between requests
session := client.NewSession("chrome-143")
defer session.Close()

// Login - cookies are saved automatically
session.Post(ctx, "https://example.com/login",
    []byte(`{"user":"test"}`),
    map[string]string{"Content-Type": "application/json"})

// Subsequent requests include cookies
resp, _ := session.Get(ctx, "https://example.com/dashboard", nil)
```

#### With Proxy

```go
c := client.NewClient("chrome-143",
    client.WithProxy("http://user:pass@proxy.example.com:8080"),
    client.WithTimeout(30*time.Second),
)
defer c.Close()
```

#### With Retry

```go
c := client.NewClient("chrome-143",
    client.WithRetry(3), // Retry up to 3 times on 429, 500, 502, 503, 504
)
```

#### Force Protocol

```go
// Force HTTP/2 (skip HTTP/3 attempt)
c := client.NewClient("chrome-143", client.WithForceHTTP2())

// Force HTTP/1.1
c := client.NewClient("chrome-143", client.WithForceHTTP1())
```

#### Redirect Control

```go
// Disable redirects
c := client.NewClient("chrome-143", client.WithoutRedirects())

// Custom redirect limit
c := client.NewClient("chrome-143", client.WithRedirects(true, 5))
```

---

### Python

httpcloak for Python provides a **requests-compatible API** - drop-in replacement with browser fingerprinting.

```python
import httpcloak

# Simple GET request
r = httpcloak.get("https://www.cloudflare.com/cdn-cgi/trace")
print(r.status_code)
print(r.text)
print(r.protocol)  # "h2" or "h3"
```

#### POST with JSON

```python
r = httpcloak.post("https://api.example.com/login", json={
    "username": "test",
    "password": "secret"
})
print(r.json())
```

#### Session with Cookies

```python
# Sessions persist cookies and connections
with httpcloak.Session(preset="chrome-143") as session:
    # Login
    session.post("https://example.com/login", json={"user": "test"})

    # Subsequent requests include cookies
    r = session.get("https://example.com/dashboard")
    print(r.json())
```

#### File Upload (Multipart)

```python
# Upload a file
with open("image.png", "rb") as f:
    r = httpcloak.post("https://api.example.com/upload", files={
        "file": f
    })

# Upload with custom filename and content type
r = httpcloak.post("https://api.example.com/upload", files={
    "file": ("photo.jpg", image_bytes, "image/jpeg")
})

# Upload with form data
r = httpcloak.post("https://api.example.com/upload",
    data={"description": "My photo"},
    files={"file": open("photo.jpg", "rb")}
)
```

#### Configure Defaults

```python
# Configure global defaults
httpcloak.configure(
    preset="chrome-143",
    proxy="http://user:pass@proxy:8080",
    timeout=30,
    verify=True,           # SSL verification
    allow_redirects=True,
    retry=3,               # Retry failed requests
)

# All subsequent requests use these defaults
r = httpcloak.get("https://example.com")
```

#### Session Options

```python
session = httpcloak.Session(
    preset="chrome-143",
    proxy="http://proxy:8080",
    timeout=30,
    http_version="auto",      # "auto", "h1", "h2", "h3"
    verify=True,              # SSL certificate verification
    allow_redirects=True,
    max_redirects=10,
    retry=3,
    retry_on_status=[429, 500, 502, 503, 504],
)
```

#### Basic Authentication

```python
# Per-request auth
r = httpcloak.get("https://api.example.com/data", auth=("user", "pass"))

# Global auth
httpcloak.configure(auth=("user", "pass"))
```

---

### Node.js

```javascript
// CommonJS
const httpcloak = require("httpcloak");

// ESM (ES Modules)
import httpcloak from "httpcloak";
// or named imports:
import { Session, get, post } from "httpcloak";

// Simple GET request
const r = await httpcloak.get("https://www.cloudflare.com/cdn-cgi/trace");
console.log(r.statusCode);
console.log(r.text);
console.log(r.protocol); // "h2" or "h3"
```

#### POST with JSON

```javascript
const r = await httpcloak.post("https://api.example.com/login", {
  json: { username: "test", password: "secret" }
});
console.log(r.json());
```

#### Session with Cookies

```javascript
const session = new httpcloak.Session({ preset: "chrome-143" });

// Login
await session.post("https://example.com/login", {
  json: { user: "test" }
});

// Subsequent requests include cookies
const r = await session.get("https://example.com/dashboard");
console.log(r.json());

session.close();
```

#### Synchronous Requests

```javascript
const session = new httpcloak.Session({ preset: "chrome-143" });

// Sync methods available
const r = session.getSync("https://example.com");
console.log(r.statusCode);

session.close();
```

#### File Upload (Multipart)

```javascript
const session = new httpcloak.Session({ preset: "chrome-143" });

// Upload a buffer
const r = session.postSync("https://api.example.com/upload", {
  files: {
    file: Buffer.from(fileData)
  }
});

// Upload with filename and content type
const r = session.postSync("https://api.example.com/upload", {
  files: {
    file: {
      filename: "photo.jpg",
      content: imageBuffer,
      contentType: "image/jpeg"
    }
  }
});

// Upload with form data
const r = session.postSync("https://api.example.com/upload", {
  data: { description: "My photo" },
  files: { file: imageBuffer }
});
```

#### Configure Defaults

```javascript
httpcloak.configure({
  preset: "chrome-143",
  proxy: "http://user:pass@proxy:8080",
  timeout: 30,
  verify: true,
  allowRedirects: true,
  retry: 3,
});

const r = await httpcloak.get("https://example.com");
```

#### Session Options

```javascript
const session = new httpcloak.Session({
  preset: "chrome-143",
  proxy: "http://proxy:8080",
  timeout: 30,
  httpVersion: "auto",     // "auto", "h1", "h2", "h3"
  verify: true,
  allowRedirects: true,
  maxRedirects: 10,
  retry: 3,
  retryOnStatus: [429, 500, 502, 503, 504],
});
```

---

### C# / .NET

```csharp
using HttpCloak;

// Simple GET request
using var session = new Session(Presets.Chrome143);
var r = session.Get("https://www.cloudflare.com/cdn-cgi/trace");
Console.WriteLine(r.StatusCode);
Console.WriteLine(r.Text);
Console.WriteLine(r.Protocol); // "h2" or "h3"
```

#### POST with JSON

```csharp
using var session = new Session(Presets.Chrome143);
var r = session.Post("https://api.example.com/login",
    body: """{"username": "test", "password": "secret"}""",
    headers: new Dictionary<string, string> {
        ["Content-Type"] = "application/json"
    });
Console.WriteLine(r.Json<LoginResponse>());
```

#### Session with Cookies

```csharp
// Sessions persist cookies between requests
using var session = new Session(Presets.Chrome143);

// Login - cookies are saved automatically
session.Post("https://example.com/login",
    body: """{"user": "test"}""",
    headers: new Dictionary<string, string> {
        ["Content-Type"] = "application/json"
    });

// Subsequent requests include cookies
var r = session.Get("https://example.com/dashboard");
```

#### With Proxy

```csharp
using var session = new Session(
    preset: Presets.Chrome143,
    proxy: "http://user:pass@proxy.example.com:8080",
    timeout: 30
);
```

#### Session Options

```csharp
using var session = new Session(
    preset: Presets.Chrome143,
    proxy: "http://proxy:8080",
    timeout: 30,
    httpVersion: "auto",      // "auto", "h1", "h2", "h3"
    verify: true,
    allowRedirects: true,
    maxRedirects: 10,
    retry: 3
);
```

---

## Available Presets

### Desktop Browsers

| Preset | Browser | Post-Quantum | HTTP/2 | HTTP/3 |
|--------|---------|--------------|--------|--------|
| `chrome-143` | Chrome 143 (default) | X25519MLKEM768 | Yes | Yes |
| `chrome-143-windows` | Chrome 143 (Windows) | X25519MLKEM768 | Yes | Yes |
| `chrome-143-linux` | Chrome 143 (Linux) | X25519MLKEM768 | Yes | Yes |
| `chrome-143-macos` | Chrome 143 (macOS) | X25519MLKEM768 | Yes | Yes |
| `chrome-131` | Chrome 131 | X25519MLKEM768 | Yes | Yes |
| `firefox-133` | Firefox 133 | X25519 | Yes | No |
| `safari-18` | Safari 18 | X25519 | Yes | No |

### Mobile Browsers

| Preset | Browser | Post-Quantum | HTTP/2 | HTTP/3 |
|--------|---------|--------------|--------|--------|
| `chrome-mobile-ios` | Chrome on iOS | X25519MLKEM768 | Yes | Yes |
| `chrome-mobile-android` | Chrome on Android | X25519MLKEM768 | Yes | Yes |
| `safari-mobile-ios` | Safari on iOS | X25519 | Yes | No |

**Recommended:** Use `chrome-143` for desktop, `chrome-mobile-android` or `chrome-mobile-ios` for mobile emulation.

---

## Features

### Browser Fingerprinting
- **TLS Fingerprinting**: JA3/JA4 hashes match real Chrome
- **HTTP/2 Fingerprinting**: SETTINGS, WINDOW_UPDATE, PRIORITY frames
- **HTTP/3 Fingerprinting**: QUIC transport parameters
- **Header Order**: Browser-accurate header ordering
- **Client Hints**: Sec-Ch-Ua-* headers matching the preset

### Protocol Support
- **HTTP/3**: QUIC with Chrome fingerprinting
- **HTTP/2**: Multiplexed connections with proper framing
- **HTTP/1.1**: Keep-alive connection pooling
- **Auto Fallback**: H3 -> H2 -> H1 with protocol learning

### HTTP Features
- **Connection Pooling**: Efficient connection reuse
- **Session Management**: Cookie persistence
- **Automatic Decompression**: gzip, brotli, zstd
- **Redirect Following**: Configurable with history
- **Retry with Backoff**: Exponential backoff with jitter
- **Proxy Support**: HTTP, HTTPS, SOCKS5 (including HTTP/3 over SOCKS5 UDP)

---

## Proxy Support

All languages support HTTP and SOCKS5 proxies:

```
http://host:port
http://user:pass@host:port
socks5://host:port
socks5://user:pass@host:port
```

### HTTP/3 over SOCKS5

httpcloak supports **HTTP/3 (QUIC) over SOCKS5 proxies** using UDP ASSOCIATE (RFC 1928). This is a unique feature - most libraries only support HTTP/1.1 and HTTP/2 through proxies.

**How it works:**
- SOCKS5 UDP ASSOCIATE establishes a UDP relay through the proxy
- QUIC packets are tunneled through the relay
- Happy Eyeballs racing tries IPv6 first, falls back to IPv4

**Requirements:**
- SOCKS5 proxy must support UDP ASSOCIATE command
- Most residential/ISP proxies support this

#### Go Example

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/sardanioss/httpcloak/client"
)

func main() {
    // Create client with SOCKS5 proxy - HTTP/3 works automatically
    c := client.NewClient("chrome-143",
        client.WithProxy("socks5://user:pass@proxy.example.com:1080"),
    )
    defer c.Close()

    resp, err := c.Get(context.Background(), "https://www.cloudflare.com/cdn-cgi/trace", nil)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Protocol: %s\n", resp.Protocol) // "h3" if proxy supports UDP
    fmt.Println(resp.Text())
}
```

#### Python Example

```python
import httpcloak

# SOCKS5 proxy with HTTP/3 support
session = httpcloak.Session(
    preset="chrome-143",
    proxy="socks5://user:pass@proxy.example.com:1080"
)

r = session.get("https://www.cloudflare.com/cdn-cgi/trace")
print(f"Protocol: {r.protocol}")  # "h3" if proxy supports UDP
print(r.text)
```

#### Node.js Example

```javascript
import { Session } from "httpcloak";

const session = new Session({
    preset: "chrome-143",
    proxy: "socks5://user:pass@proxy.example.com:1080"
});

const r = await session.get("https://www.cloudflare.com/cdn-cgi/trace");
console.log(`Protocol: ${r.protocol}`);  // "h3" if proxy supports UDP
console.log(r.text);

session.close();
```

**Note:** If the SOCKS5 proxy doesn't support UDP ASSOCIATE, httpcloak automatically falls back to HTTP/2 over TCP.

---

## Response Object

### Go
```go
resp.StatusCode    // int
resp.Headers       // map[string]string
resp.Body          // []byte
resp.Text()        // string
resp.FinalURL      // string (after redirects)
resp.Protocol      // "h1", "h2", or "h3"
```

### Python
```python
r.status_code      # int
r.headers          # dict
r.content          # bytes
r.text             # str
r.json()           # parsed JSON
r.url              # final URL after redirects
r.protocol         # "h1", "h2", or "h3"
r.ok               # True if status < 400
r.raise_for_status()  # raises on 4xx/5xx
```

### Node.js
```javascript
r.statusCode       // number
r.headers          // object
r.content          // Buffer
r.text             // string
r.json()           // parsed JSON
r.url              // final URL after redirects
r.protocol         // "h1", "h2", or "h3"
r.ok               // true if status < 400
r.raiseForStatus() // throws on 4xx/5xx
```

### C#
```csharp
r.StatusCode       // int
r.Headers          // Dictionary<string, string>
r.Content          // byte[]
r.Text             // string
r.Json<T>()        // deserialized JSON
r.Url              // final URL after redirects
r.Protocol         // "h1", "h2", or "h3"
r.Ok               // true if status < 400
r.RaiseForStatus() // throws on 4xx/5xx
```

---

## Examples

See the `examples/` directory for complete, runnable examples:

```bash
# Go examples
go run examples/go-examples/basic/main.go        # Basic requests, headers, POST
go run examples/go-examples/session/main.go      # Sessions, cookies, login flow
go run examples/go-examples/cloudflare/main.go   # Cloudflare trace, fingerprinting

# Python examples
python examples/python-examples/01_basic_requests.py        # GET, POST, headers
python examples/python-examples/02_configure_and_presets.py # Presets, HTTP versions
python examples/python-examples/03_sessions_and_cookies.py  # Sessions, cookies
python examples/python-examples/04_auth_and_proxy.py        # Auth, proxy, errors
python examples/python-examples/05_async_requests.py        # Async/concurrent

# Node.js examples (CommonJS)
node examples/js-examples/01_basic_requests.js        # GET, POST, headers
node examples/js-examples/02_configure_and_presets.js # Presets, HTTP versions
node examples/js-examples/03_sessions_and_cookies.js  # Sessions, cookies
node examples/js-examples/04_auth_and_proxy.js        # Auth, proxy, errors
node examples/js-examples/05_sync_requests.js         # Synchronous requests
node examples/js-examples/06_async_concurrent.js      # Async/concurrent requests

# Node.js ESM example
node examples/js-examples/07_esm_example.mjs          # ES Modules syntax

# C# examples
dotnet run --project examples/csharp-examples         # All examples
```

---

## License

MIT

---

## Dependencies

Uses custom forks for browser-accurate fingerprinting:

| Library | Fork | Purpose |
|---------|------|---------|
| uTLS | sardanioss/utls | Chrome 143 TLS presets |
| quic-go | sardanioss/quic-go | HTTP/3 with Chrome fingerprinting |
| net | sardanioss/net | HTTP/2 frame fingerprinting |

## Credits

- [uTLS](https://github.com/refraction-networking/utls) - TLS fingerprint spoofing
- [quic-go](https://github.com/quic-go/quic-go) - HTTP/3 implementation
- [tls.peet.ws](https://tls.peet.ws) - Fingerprint analysis
