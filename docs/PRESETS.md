# HTTPCloak Browser Presets

This document lists all available browser presets and their configurations.

## Available Presets (24 total)

```go
// From fingerprint.Available()
// Chrome Desktop (default)
chrome-145          // Default, auto-detects platform
chrome-145-windows
chrome-145-linux
chrome-145-macos
chrome-144
chrome-144-windows
chrome-144-linux
chrome-144-macos
chrome-143
chrome-143-windows
chrome-143-linux
chrome-143-macos
chrome-141
chrome-133

// Firefox
firefox-133

// Safari (macOS)
safari-18

// iOS (WebKit-based)
safari-18-ios
safari-17-ios
chrome-145-ios
chrome-144-ios
chrome-143-ios

// Android
chrome-145-android
chrome-144-android
chrome-143-android
```

## Default Preset

The default preset is **`chrome-145`** (defined in `client/options.go`).

When using `chrome-145`, the library auto-detects the platform (Windows, macOS, Linux) and uses the appropriate TLS fingerprint variant.

## Chrome 145 (Default)

### Platform Detection Logic

```go
// From fingerprint/presets.go:19-46
switch runtime.GOOS {
case "windows":
    Platform: "Windows"
    UserAgentOS: "(Windows NT 10.0; Win64; x64)"
case "darwin":
    Platform: "macOS"
    UserAgentOS: "(Macintosh; Intel Mac OS X 10_15_7)"
default: // linux
    Platform: "Linux"
    UserAgentOS: "(X11; Linux x86_64)"
}
```

### TLS ClientHelloID

| Platform | ClientHelloID | PSK ClientHelloID | QUIC ClientHelloID |
|----------|---------------|-------------------|--------------------|
| Windows | `HelloChrome_145_Windows` | `HelloChrome_145_Windows_PSK` | `HelloChrome_145_QUIC` |
| Linux | `HelloChrome_145_Linux` | `HelloChrome_145_Linux_PSK` | `HelloChrome_145_QUIC` |
| macOS | `HelloChrome_145_macOS` | `HelloChrome_145_macOS_PSK` | `HelloChrome_145_QUIC` |

### User-Agent

```
Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36
```

### HTTP Headers

```go
"sec-ch-ua":          `"Not(A:Brand";v="8", "Chromium";v="145", "Google Chrome";v="145"`
"sec-ch-ua-mobile":   "?0"
"sec-ch-ua-platform": `"Linux"` // or "Windows" or "macOS"
"Upgrade-Insecure-Requests": "1"
"Accept":             "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"
"Sec-Fetch-Site":     "none"
"Sec-Fetch-Mode":     "navigate"
"Sec-Fetch-User":     "?1"
"Sec-Fetch-Dest":     "document"
"Accept-Encoding":    "gzip, deflate, br, zstd"
"Accept-Language":    "en-US,en;q=0.9"
"Priority":           "u=0, i"
```

### Header Order (HTTP/2 and HTTP/3)

```go
// From fingerprint/presets.go:276-289
HeaderOrder: []HeaderPair{
    {"accept-language", "en-US,en;q=0.9"},
    {"sec-ch-ua", `"Google Chrome";v="143", ...`},
    {"accept", "text/html,..."},
    {"sec-fetch-site", "none"},
    {"sec-fetch-user", "?1"},
    {"accept-encoding", "gzip, deflate, br, zstd"},
    {"upgrade-insecure-requests", "1"},
    {"sec-ch-ua-platform", `"Linux"`},
    {"sec-ch-ua-mobile", "?0"},
    {"sec-fetch-dest", "document"},
    {"priority", "u=0, i"},
    {"sec-fetch-mode", "navigate"},
}
```

### HTTP/2 SETTINGS

```go
// From fingerprint/presets.go:290-300
HTTP2Settings: HTTP2Settings{
    HeaderTableSize:        65536,
    EnablePush:             false,
    MaxConcurrentStreams:   0,      // No limit from client
    InitialWindowSize:      6291456,
    MaxFrameSize:           16384,
    MaxHeaderListSize:      262144,
    ConnectionWindowUpdate: 15663105,
    StreamWeight:           220,    // Wire: 219 (code does -1)
    StreamExclusive:        true,
}
```

### HTTP/2 SETTINGS Order

```go
// From pool/pool.go:389-394
SettingsOrder: []http2.SettingID{
    http2.SettingHeaderTableSize,
    http2.SettingEnablePush,
    http2.SettingInitialWindowSize,
    http2.SettingMaxHeaderListSize,
}
```

### Pseudo-Header Order

```go
// From pool/pool.go:395
PseudoHeaderOrder: []string{":method", ":authority", ":scheme", ":path"}
```

## Chrome 133

### TLS ClientHelloID
- Regular: `HelloChrome_133`
- PSK: `HelloChrome_133_PSK`

### User-Agent
```
Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36
```

### sec-ch-ua
```
"Google Chrome";v="133", "Chromium";v="133", "Not_A Brand";v="24"
```

## Firefox 133

### TLS ClientHelloID
- `HelloFirefox_120`

### User-Agent
```
Mozilla/5.0 (X11; Linux x86_64; rv:133.0) Gecko/20100101 Firefox/133.0
```

### Headers
```go
"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"
"Accept-Language": "en-US,en;q=0.5"
"Accept-Encoding": "gzip, deflate, br"
"Sec-Fetch-Dest":  "document"
"Sec-Fetch-Mode":  "navigate"
"Sec-Fetch-Site":  "none"
"Sec-Fetch-User":  "?1"
```

### HTTP/2 SETTINGS
```go
HTTP2Settings: HTTP2Settings{
    HeaderTableSize:        65536,
    EnablePush:             true,
    MaxConcurrentStreams:   0,
    InitialWindowSize:      131072,
    MaxFrameSize:           16384,
    MaxHeaderListSize:      0,
    ConnectionWindowUpdate: 12517377,
    StreamWeight:           42,
    StreamExclusive:        false,
}
```

## Safari 18

### TLS ClientHelloID
- TCP: `HelloSafari_18`
- QUIC: `HelloIOS_18_QUIC` (Safari uses same QUIC as iOS)

### User-Agent
```
Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15
```

### Headers
```go
"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
"Accept-Language": "en-US,en;q=0.9"
"Accept-Encoding": "gzip, deflate, br"
"Sec-Fetch-Dest":  "document"
"Sec-Fetch-Mode":  "navigate"
"Sec-Fetch-Site":  "none"
"Sec-Fetch-User":  "?1"
```

### Pseudo-Header Order
Safari uses a different order than Chrome: `:method`, `:scheme`, `:path`, `:authority` (m,s,p,a)

### HTTP/2 SETTINGS
```go
HTTP2Settings: HTTP2Settings{
    HeaderTableSize:        4096,
    EnablePush:             false,
    MaxConcurrentStreams:   100,
    InitialWindowSize:      2097152,
    MaxFrameSize:           16384,
    MaxHeaderListSize:      0,
    ConnectionWindowUpdate: 10485760,
    StreamWeight:           255,
    StreamExclusive:        false,
    NoRFC7540Priorities:    true,  // Safari sends NO_RFC7540_PRIORITIES=1
}
```

### HTTP/3 Support
`SupportHTTP3: true` - Safari 18 supports HTTP/3 with WebKit QUIC fingerprint.

## Mobile Presets

### iOS Chrome 145

- TLS: `HelloIOS_18` (iOS Chrome uses Safari's TLS due to WebKit requirement)
- QUIC: `HelloIOS_18_QUIC`
- User-Agent: `Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/145.0.6917.0 Mobile/15E148 Safari/604.1`
- **No Client Hints**: WebKit doesn't support sec-ch-ua headers
- Pseudo-Header Order: `:method`, `:scheme`, `:path`, `:authority` (m,s,p,a)
- HTTP/2 Settings: Same as Safari (with `NoRFC7540Priorities: true`)
- `SupportHTTP3: true`

### iOS Chrome 144

- TLS: `HelloIOS_18` (iOS Chrome uses Safari's TLS due to WebKit requirement)
- QUIC: `HelloIOS_18_QUIC`
- User-Agent: `Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/144.0.6917.0 Mobile/15E148 Safari/604.1`
- **No Client Hints**: WebKit doesn't support sec-ch-ua headers
- Pseudo-Header Order: `:method`, `:scheme`, `:path`, `:authority` (m,s,p,a)
- HTTP/2 Settings: Same as Safari (with `NoRFC7540Priorities: true`)
- `SupportHTTP3: true`

### iOS Chrome 143

- TLS: `HelloIOS_18` (iOS Chrome uses Safari's TLS due to WebKit requirement)
- QUIC: `HelloIOS_18_QUIC`
- User-Agent: `Mozilla/5.0 (iPhone; CPU iPhone OS 17_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/143.0.6917.0 Mobile/15E148 Safari/604.1`
- **No Client Hints**: WebKit doesn't support sec-ch-ua headers
- Pseudo-Header Order: `:method`, `:scheme`, `:path`, `:authority` (m,s,p,a)
- HTTP/2 Settings: Same as Safari (with `NoRFC7540Priorities: true`)
- `SupportHTTP3: true`

### iOS Safari 18

- TLS: `HelloIOS_18`
- QUIC: `HelloIOS_18_QUIC`
- User-Agent: `Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1`
- No Client Hints (Safari doesn't send them)
- Pseudo-Header Order: `:method`, `:scheme`, `:path`, `:authority` (m,s,p,a)
- HTTP/2 Settings: Same as Safari (with `NoRFC7540Priorities: true`)
- `SupportHTTP3: true`

### iOS Safari 17

- TLS: `HelloIOS_14`
- User-Agent: `Mozilla/5.0 (iPhone; CPU iPhone OS 17_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.7 Mobile/15E148 Safari/604.1`
- No Client Hints (Safari doesn't send them)
- HTTP/2 Settings: Same as Safari (with `NoRFC7540Priorities: true`)
- `SupportHTTP3: false` (no QUIC TLS spec for iOS 17)

### Android Chrome 145

- TLS: `HelloChrome_145_Linux` (Android Chrome uses Chrome's TLS)
- PSK: `HelloChrome_145_Linux_PSK`
- QUIC: `HelloChrome_145_QUIC`
- User-Agent: `Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Mobile Safari/537.36`
- `sec-ch-ua`: `"Not(A:Brand";v="8", "Chromium";v="145", "Google Chrome";v="145"`
- `sec-ch-ua-mobile`: `?1`
- `sec-ch-ua-platform`: `"Android"`
- Pseudo-Header Order: `:method`, `:authority`, `:scheme`, `:path` (m,a,s,p - same as Chrome desktop)
- HTTP/2 Settings: Same as Chrome desktop
- `SupportHTTP3: true`

### Android Chrome 144

- TLS: `HelloChrome_144_Linux` (Android Chrome uses Chrome's TLS)
- PSK: `HelloChrome_144_Linux_PSK`
- QUIC: `HelloChrome_144_QUIC`
- User-Agent: `Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Mobile Safari/537.36`
- `sec-ch-ua`: `"Not(A:Brand";v="8", "Chromium";v="144", "Google Chrome";v="144"`
- `sec-ch-ua-mobile`: `?1`
- `sec-ch-ua-platform`: `"Android"`
- Pseudo-Header Order: `:method`, `:authority`, `:scheme`, `:path` (m,a,s,p - same as Chrome desktop)
- HTTP/2 Settings: Same as Chrome desktop
- `SupportHTTP3: true`

### Android Chrome 143

- TLS: `HelloChrome_143_Linux` (Android Chrome uses Chrome's TLS)
- PSK: `HelloChrome_143_Linux_PSK`
- QUIC: `HelloChrome_143_QUIC`
- User-Agent: `Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Mobile Safari/537.36`
- `sec-ch-ua`: `"Google Chrome";v="143", "Chromium";v="143", "Not A(Brand";v="24"`
- `sec-ch-ua-mobile`: `?1`
- `sec-ch-ua-platform`: `"Android"`
- Pseudo-Header Order: `:method`, `:authority`, `:scheme`, `:path` (m,a,s,p - same as Chrome desktop)
- HTTP/2 Settings: Same as Chrome desktop
- `SupportHTTP3: true`

## Client Hints Note

Chrome sends only **low-entropy** Client Hints by default:
- `sec-ch-ua`
- `sec-ch-ua-mobile`
- `sec-ch-ua-platform`

**High-entropy** hints are only sent after server requests them via `Accept-CH` header:
- `sec-ch-ua-arch`
- `sec-ch-ua-bitness`
- `sec-ch-ua-full-version-list`
- `sec-ch-ua-model`
- `sec-ch-ua-platform-version`

Sending high-entropy hints without `Accept-CH` is a bot fingerprint.

## File References

All presets are defined in `fingerprint/presets.go`. Key sections:

| Component | Description |
|-----------|-------------|
| `HTTP2Settings` struct | HTTP/2 SETTINGS frame configuration including `NoRFC7540Priorities` |
| Chrome 133/141 | Legacy Chrome presets (H2 only) |
| Chrome 143/144/145 | Current Chrome presets with platform variants and H3 support |
| Firefox 133 | Firefox preset (H2 only) |
| Safari 18 | macOS Safari with WebKit TLS and H3 support |
| iOS Safari 17/18 | Mobile Safari presets |
| iOS Chrome 143/144/145 | Chrome on iOS (WebKit TLS, no Client Hints) |
| Android Chrome 143/144/145 | Chrome on Android (Chrome TLS, full fingerprint) |
| `presets` map | Maps preset names to factory functions |
| `Get()` function | Returns preset by name, defaults to Chrome 145 |
