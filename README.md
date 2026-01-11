# httpcloak

Browser-identical HTTP client for Go, Python, Node.js, and C#.

[![Go Reference](https://pkg.go.dev/badge/github.com/sardanioss/httpcloak.svg)](https://pkg.go.dev/github.com/sardanioss/httpcloak)
[![PyPI](https://img.shields.io/pypi/v/httpcloak)](https://pypi.org/project/httpcloak/)
[![npm](https://img.shields.io/npm/v/httpcloak)](https://www.npmjs.com/package/httpcloak)
[![NuGet](https://img.shields.io/nuget/v/HttpCloak)](https://www.nuget.org/packages/HttpCloak)

Modern bot detection doesn't just check headers—it fingerprints your TLS handshake, HTTP/2 frames, and QUIC parameters. Go's `net/http` has a recognizable fingerprint that gets blocked instantly.

**httpcloak fixes this.** Every connection looks exactly like Chrome, Firefox, or Safari.

---

## Install

```bash
go get github.com/sardanioss/httpcloak         # Go
pip install httpcloak                           # Python
npm install httpcloak                           # Node.js
dotnet add package HttpCloak                    # C#
```

---

## Quick Example

**Go**
```go
import "github.com/sardanioss/httpcloak/client"

c := client.NewClient("chrome-143")
resp, _ := c.Get(ctx, "https://example.com", nil)
```

**Python**
```python
import httpcloak

r = httpcloak.get("https://example.com")
```

**Node.js**
```javascript
import httpcloak from "httpcloak";

const r = await httpcloak.get("https://example.com");
```

**C#**
```csharp
using var session = new Session(Presets.Chrome143);
var r = session.Get("https://example.com");
```

> **See more:** [Go examples](examples/go-examples/) · [Python examples](examples/python-examples/) · [Node.js examples](examples/js-examples/) · [C# examples](examples/csharp-examples/)

---

## What You Get

| | Go stdlib | httpcloak |
|---|---|---|
| **TLS fingerprint** | Detected as Go | Identical to Chrome |
| **HTTP/2 fingerprint** | Detected | Identical to Chrome |
| **HTTP/3 support** | No | Yes, with Chrome fingerprint |
| **Post-quantum crypto** | No | Yes (X25519MLKEM768) |
| **Cloudflare bot score** | ~10 | ~99 |

---

## Key Features

### Perfect Browser Fingerprints
JA3, JA4, and Akamai fingerprints match real browsers exactly. Includes GREASE values, proper extension ordering, and accurate HTTP/2 SETTINGS frames.

### Session Resumption (0-RTT)
Save and restore TLS sessions. First request scores ~43, resumed sessions score **~99**.

```python
session.get("https://cloudflare.com/")  # Warm up
session.save("session.json")

# Later...
session = httpcloak.Session.load("session.json")
session.get("https://target.com/")  # 0-RTT, bot score 99
```

Session tickets from `cloudflare.com` work on **any** Cloudflare-protected site.

> **Examples:** [Go](examples/go-examples/session-resumption/main.go) · [Python](examples/python-examples/09_session_resumption.py) · [Node.js](examples/js-examples/11_session_resumption.js) · [C#](examples/csharp-examples/SessionResumption.cs)

### HTTP/3 (QUIC)
Full HTTP/3 with Chrome's QUIC fingerprint. Falls back to HTTP/2 automatically.

### HTTP/3 Through SOCKS5
QUIC over SOCKS5 using UDP ASSOCIATE. Most residential proxies support this.

```python
session = httpcloak.Session(
    preset="chrome-143",
    proxy="socks5://user:pass@proxy:1080"
)
r = session.get("https://cloudflare.com")
print(r.protocol)  # "h3"
```

### Automatic Cookies
Sessions persist cookies between requests automatically.

### Streaming
Stream large downloads/uploads without loading into memory.

---

## Browser Presets

| Preset | HTTP/2 | HTTP/3 | Post-Quantum |
|--------|:------:|:------:|:------------:|
| `chrome-143` | ✓ | ✓ | ✓ |
| `chrome-143-windows` | ✓ | ✓ | ✓ |
| `chrome-143-macos` | ✓ | ✓ | ✓ |
| `chrome-143-linux` | ✓ | ✓ | ✓ |
| `chrome-131` | ✓ | ✓ | ✓ |
| `firefox-133` | ✓ | | |
| `safari-18` | ✓ | | |
| `chrome-mobile-android` | ✓ | ✓ | ✓ |
| `chrome-mobile-ios` | ✓ | ✓ | ✓ |

---

## Proxy Support

```
http://user:pass@host:port
socks5://user:pass@host:port
```

SOCKS5 proxies with UDP support get HTTP/3 automatically.

---

## Response API

| | Go | Python | Node.js | C# |
|---|---|---|---|---|
| Status | `resp.StatusCode` | `r.status_code` | `r.statusCode` | `r.StatusCode` |
| Body | `resp.Text()` | `r.text` | `r.text` | `r.Text` |
| JSON | `resp.JSON(&v)` | `r.json()` | `r.json()` | `r.Json<T>()` |
| Protocol | `resp.Protocol` | `r.protocol` | `r.protocol` | `r.Protocol` |

---

## License

MIT
