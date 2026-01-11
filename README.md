<p align="center">
<img src="httpcloak.png" alt="httpcloak" width="600">
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/sardanioss/httpcloak"><img src="https://pkg.go.dev/badge/github.com/sardanioss/httpcloak.svg" alt="Go Reference"></a>
  <a href="https://pypi.org/project/httpcloak/"><img src="https://img.shields.io/pypi/v/httpcloak" alt="PyPI"></a>
  <a href="https://www.npmjs.com/package/httpcloak"><img src="https://img.shields.io/npm/v/httpcloak" alt="npm"></a>
  <a href="https://www.nuget.org/packages/HttpCloak"><img src="https://img.shields.io/nuget/v/HttpCloak" alt="NuGet"></a>
</p>

<p align="center">
<i>Every Byte of your Request Indistinguishable from Chrome.</i>
</p>

<br>

---

## The Problem

Bot detection doesn't just check your User-Agent anymore.

It fingerprints your **TLS handshake**. Your **HTTP/2 frames**. Your **QUIC parameters**. The order of your headers. Whether you have a session ticket. Whether your SNI is encrypted.

One mismatch = blocked.

## The Solution

```python
import httpcloak

r = httpcloak.get("https://target.com", preset="chrome-143")
```

That's it. Full browser fingerprint. Every layer.

---

## What Gets Emulated

<table>
<tr>
<td width="33%" valign="top">

### 🔐 TLS Layer

- JA3 / JA4 fingerprints
- GREASE randomization
- Post-quantum X25519MLKEM768
- ECH (Encrypted Client Hello)
- Session tickets & 0-RTT

</td>
<td width="33%" valign="top">

### 🚀 Transport Layer

- HTTP/2 SETTINGS frames
- WINDOW_UPDATE values
- Stream priorities (HPACK)
- QUIC transport parameters
- HTTP/3 GREASE frames

</td>
<td width="33%" valign="top">

### 🧠 Header Layer

- Sec-Fetch-* coherence
- Client Hints (Sec-Ch-UA)
- Accept / Accept-Language
- Header ordering
- Cookie persistence

</td>
</tr>
</table>

---

## Proof

```
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│   WITHOUT SESSION TICKET          WITH SESSION TICKET                   │
│                                                                         │
│   Bot Score: 43                   Bot Score: 99                         │
│   ██████░░░░░░░░░░░░░░            ██████████████████████████████████    │
│   ↑ New TLS handshake             ↑ 0-RTT resumption                    │
│   ↑ Looks like a bot              ↑ Looks like returning Chrome         │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

```
┌─────────────────────────────────┐
│  ECH (Encrypted Client Hello)   │
├─────────────────────────────────┤
│  WITHOUT:  sni=plaintext        │
│  WITH:     sni=encrypted   +    │
└─────────────────────────────────┘
```

```
┌─────────────────────────────────┐
│  HTTP/3 Fingerprint Match       │
├─────────────────────────────────┤
│  Protocol:        h3       +    │
│  QUIC Version:    1        +    │
│  Transport Params:         +    │
│  GREASE Frames:            +    │
└─────────────────────────────────┘
```

---

## vs curl_cffi

```
┌────────────────────────────────┬────────────────────────────────┐
│        BOTH LIBRARIES          │       HTTPCLOAK ONLY           │
├────────────────────────────────┼────────────────────────────────┤
│                                │                                │
│  + TLS fingerprint (JA3/JA4)   │  + HTTP/3 fingerprinting       │
│  + HTTP/2 fingerprint          │  + ECH (encrypted SNI)         │
│  + Post-quantum TLS            │  + Session persistence         │
│  + Bot score: 99               │  + 0-RTT resumption            │
│                                │  + MASQUE proxy                │
│                                │  + Domain fronting             │
│                                │  + Certificate pinning         │
│                                │  + Go, Python, Node.js, C#     │
│                                │                                │
└────────────────────────────────┴────────────────────────────────┘
```

---

## Install

```bash
pip install httpcloak        # Python
npm install httpcloak        # Node.js
go get github.com/sardanioss/httpcloak   # Go
dotnet add package HttpCloak # C#
```

---

## Quick Start

### Python

```python
import httpcloak

# One-liner
r = httpcloak.get("https://example.com", preset="chrome-143")
print(r.text, r.protocol)

# With session (for 0-RTT)
with httpcloak.Session(preset="chrome-143") as session:
    session.get("https://cloudflare.com/")  # Warm up
    session.save("session.json")

# Later
session = httpcloak.Session.load("session.json")
r = session.get("https://target.com/")  # Bot score: 99
```

### Go

```go
c := client.NewClient("chrome-143")
defer c.Close()

resp, _ := c.Get(context.Background(), "https://example.com", nil)
text, _ := resp.Text()
fmt.Println(text, resp.Protocol)
```

### Node.js

```javascript
import httpcloak from "httpcloak";

const session = new httpcloak.Session({ preset: "chrome-143" });
const r = await session.get("https://example.com");
console.log(r.text, r.protocol);
session.close();
```

### C#

```csharp
using var session = new Session(Presets.Chrome143);
var r = session.Get("https://example.com");
Console.WriteLine($"{r.Text} {r.Protocol}");
```

---

## Features

### 🔐 ECH (Encrypted Client Hello)

Hides which domain you're connecting to from network observers.

```python
session = httpcloak.Session(
    preset="chrome-143",
    ech_from="cloudflare.com"  # Fetches ECH config from DNS
)
```

Cloudflare trace shows `sni=encrypted` instead of `sni=plaintext`.

### ⚡ Session Resumption (0-RTT)

TLS session tickets make you look like a returning visitor.

```python
# Warm up on any Cloudflare site
session.get("https://cloudflare.com/")
session.save("session.json")

# Use on your target
session = httpcloak.Session.load("session.json")
r = session.get("https://target.com/")  # Bot score: 99
```

Cross-domain warming works because Cloudflare sites share TLS infrastructure.

### 🌐 HTTP/3 Through Proxies

Two methods for QUIC through proxies:

| Method | How it works |
|--------|--------------|
| **SOCKS5 UDP ASSOCIATE** | Proxy relays UDP packets. Most residential proxies support this. |
| **MASQUE (CONNECT-UDP)** | RFC 9298. Tunnels UDP over HTTP/3. Premium providers only. |

```python
# SOCKS5 with UDP
session = httpcloak.Session(proxy="socks5://user:pass@proxy:1080")

# MASQUE
session = httpcloak.Session(proxy="masque://proxy:443")
```

Known MASQUE providers (auto-detected): Bright Data, Oxylabs, Smartproxy, SOAX.

### 🎭 Domain Fronting

Connect to a different host than what appears in TLS SNI.

```go
client := httpcloak.NewClient("chrome-143",
    httpcloak.WithConnectTo("public-cdn.com", "actual-backend.internal"),
)
```

### 📌 Certificate Pinning

```go
client.PinCertificate("sha256/AAAA...",
    httpcloak.PinOptions{IncludeSubdomains: true})
```

### 🪝 Request Hooks

```go
client.OnPreRequest(func(req *http.Request) error {
    req.Header.Set("X-Custom", "value")
    return nil
})

client.OnPostResponse(func(resp *httpcloak.Response) {
    log.Printf("Got %d from %s", resp.StatusCode, resp.FinalURL)
})
```

### ⏱️ Request Timing

```go
fmt.Printf("DNS: %dms, TCP: %dms, TLS: %dms, Total: %dms\n",
    resp.Timing.DNSLookup,
    resp.Timing.TCPConnect,
    resp.Timing.TLSHandshake,
    resp.Timing.Total)
```

### 🔄 Protocol Selection

```python
session = httpcloak.Session(preset="chrome-143", http_version="h3")  # Force HTTP/3
session = httpcloak.Session(preset="chrome-143", http_version="h2")  # Force HTTP/2
session = httpcloak.Session(preset="chrome-143", http_version="h1")  # Force HTTP/1.1
```

Auto mode tries HTTP/3 first, falls back gracefully.

### 📤 Streaming & Uploads

```python
# Stream large downloads
with session.get(url, stream=True) as r:
    for chunk in r.iter_content(chunk_size=8192):
        file.write(chunk)

# Multipart upload
r = session.post(url, files={
    "file": ("filename.jpg", file_bytes, "image/jpeg")
})
```

---

## Browser Presets

| Preset | Platform | PQ | H3 |
|--------|----------|:--:|:--:|
| `chrome-143` | Auto | ✅ | ✅ |
| `chrome-143-windows` | Windows | ✅ | ✅ |
| `chrome-143-macos` | macOS | ✅ | ✅ |
| `chrome-143-linux` | Linux | ✅ | ✅ |
| `firefox-133` | Auto | ❌ | ❌ |
| `chrome-mobile-android` | Android | ✅ | ✅ |
| `chrome-mobile-ios` | iOS | ✅ | ✅ |

**PQ** = Post-Quantum (X25519MLKEM768) · **H3** = HTTP/3

---

## Testing Tools

| Tool | Tests |
|------|-------|
| [tls.peet.ws](https://tls.peet.ws/api/all) | JA3, JA4, HTTP/2 Akamai |
| [quic.browserleaks.com](https://quic.browserleaks.com/) | HTTP/3 QUIC fingerprint |
| [cf.erisa.uk](https://cf.erisa.uk/) | Cloudflare bot score |
| [cloudflare.com/cdn-cgi/trace](https://www.cloudflare.com/cdn-cgi/trace) | ECH status, TLS version |

---

## Dependencies

Custom forks for browser-accurate fingerprinting:

- [sardanioss/utls](https://github.com/sardanioss/utls) — TLS fingerprinting
- [sardanioss/quic-go](https://github.com/sardanioss/quic-go) — HTTP/3 fingerprinting
- [sardanioss/net](https://github.com/sardanioss/net) — HTTP/2 frame fingerprinting

---

<p align="center">
MIT License
</p>
