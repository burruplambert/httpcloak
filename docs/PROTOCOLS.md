# HTTPCloak Protocol Specifics

This document covers HTTP/1.1, HTTP/2, and HTTP/3 protocol-specific behaviors.

## Protocol Selection

### Auto Mode (Default)

When `Protocol` is set to `Auto`, the library:

1. Checks the learned protocol cache for the host
2. If unknown, races HTTP/3 and HTTP/2 connections in parallel
3. Uses whichever protocol connects first
4. Caches the result for future requests

This eliminates the 5-second HTTP/3 timeout delay when QUIC is blocked by firewalls or VPNs.

```go
// transport/transport.go:667-722
func (t *Transport) doAuto(ctx context.Context, req *Request) (*Response, error) {
    // Check if we already know the best protocol for this host
    if known {
        switch knownProtocol {
        case ProtocolHTTP3:
            return t.doHTTP3(ctx, req)
        case ProtocolHTTP2:
            resp, err := t.doHTTP2(ctx, req)
            // ...
        }
    }
    // Race HTTP/3 and HTTP/2 in parallel if H3 is supported
    if t.preset.SupportHTTP3 {
        resp, protocol, err := t.raceH3H2(ctx, req)
        // ...
    }
}
```

### Force Protocol

#### Session API

```go
// Force HTTP/1.1
session.SetProtocol(transport.ProtocolHTTP1)

// Force HTTP/2
session.SetProtocol(transport.ProtocolHTTP2)

// Force HTTP/3
session.SetProtocol(transport.ProtocolHTTP3)
```

#### Client Package Options

```go
import "github.com/sardanioss/httpcloak/client"

// Force HTTP/1.1 only - useful for servers that don't support H2
c := client.NewClient("chrome-145", client.WithForceHTTP1())

// Force HTTP/2 only - no H3 or H1 fallback
c := client.NewClient("chrome-145", client.WithForceHTTP2())

// Force HTTP/3 only - requires SOCKS5 or MASQUE proxy if using proxy
c := client.NewClient("chrome-145", client.WithForceHTTP3())

// Disable HTTP/3 but allow HTTP/2 with HTTP/1.1 fallback
c := client.NewClient("chrome-145", client.WithDisableHTTP3())
```

#### httpcloak Package Options

```go
// Force HTTP/1.1
session := httpcloak.NewSession("chrome-145", httpcloak.WithForceHTTP1())

// Force HTTP/2
session := httpcloak.NewSession("chrome-145", httpcloak.WithForceHTTP2())
```

---

## HTTP/1.1

### Connection Handling

- Keep-alive connections by default
- Connection reuse within session
- Automatic connection cleanup on idle timeout (90 seconds)

### Supported Features

- Chunked transfer encoding
- Gzip, deflate, brotli, zstd decompression
- Basic and proxy authentication
- CONNECT tunneling through proxy

### Implementation

```go
// transport/http1_transport.go
type HTTP1Transport struct {
    transport     *http.Transport
    preset        *fingerprint.Preset
    dnsCache      *dns.Cache
    proxyURL      string
    config        *TransportConfig
}
```

---

## HTTP/2

### SETTINGS Frame

HTTP/2 SETTINGS are sent in the connection preface. The values are critical for fingerprinting.

#### Chrome 143 SETTINGS

| Setting | ID | Chrome 143 Value | Wire Encoding |
|---------|-----|------------------|---------------|
| HEADER_TABLE_SIZE | 0x1 | 65536 | `00 01 00 01 00 00` |
| ENABLE_PUSH | 0x2 | 0 (false) | `00 02 00 00 00 00` |
| INITIAL_WINDOW_SIZE | 0x4 | 6291456 | `00 04 00 60 00 00` |
| MAX_HEADER_LIST_SIZE | 0x6 | 262144 | `00 06 00 04 00 00` |

#### SETTINGS Order

Chrome sends SETTINGS in this specific order:

```go
// pool/pool.go:389-394
SettingsOrder: []http2.SettingID{
    http2.SettingHeaderTableSize,    // 0x1
    http2.SettingEnablePush,         // 0x2
    http2.SettingInitialWindowSize,  // 0x4
    http2.SettingMaxHeaderListSize,  // 0x6
}
```

#### Firefox 133 SETTINGS

| Setting | ID | Firefox 133 Value |
|---------|-----|-------------------|
| HEADER_TABLE_SIZE | 0x1 | 65536 |
| ENABLE_PUSH | 0x2 | 1 (true) |
| INITIAL_WINDOW_SIZE | 0x4 | 131072 |
| MAX_HEADER_LIST_SIZE | 0x6 | 0 (omitted) |

#### Safari 18 SETTINGS

Safari/iOS uses a different SETTINGS order and includes `NO_RFC7540_PRIORITIES`:

| Setting | ID | Safari 18 Value |
|---------|-----|-----------------|
| ENABLE_PUSH | 0x2 | 0 (false) |
| INITIAL_WINDOW_SIZE | 0x4 | 2097152 |
| MAX_CONCURRENT_STREAMS | 0x3 | 100 |
| NO_RFC7540_PRIORITIES | 0x9 | 1 |

Note: Safari sends SETTINGS in order: 2, 4, 3, 9 (not 1, 2, 4, 6 like Chrome).

### WINDOW_UPDATE

After SETTINGS, Chrome sends WINDOW_UPDATE for the connection window:

```go
// pool/pool.go:382
ConnectionFlow: 15663105  // Chrome 143 connection window update
```

### Stream Priority (HEADERS frame)

Chrome sends priority information in the HEADERS frame:

```go
// pool/pool.go:396-400
HeaderPriority: &http2.PriorityParam{
    Weight:    uint8(settings.StreamWeight - 1),  // Wire format is weight-1
    Exclusive: settings.StreamExclusive,          // true for Chrome
    StreamDep: 0,                                 // Stream dependency
}
```

| Browser | Weight (wire) | Exclusive | StreamDep |
|---------|---------------|-----------|-----------|
| Chrome 143 | 219 | true | 0 |
| Firefox 133 | 41 | false | 0 |
| Safari 18 | 254 | false | 0 |

### Pseudo-Header Order

HTTP/2 pseudo-headers (`:method`, `:path`, etc.) order matters for fingerprinting.

```go
// pool/pool.go - dynamically selected based on preset
PseudoHeaderOrder: func() []string {
    if settings.NoRFC7540Priorities {
        return []string{":method", ":scheme", ":path", ":authority"} // Safari order (m,s,p,a)
    }
    return []string{":method", ":authority", ":scheme", ":path"} // Chrome order (m,a,s,p)
}()
```

| Browser | Pseudo-Header Order | `NoRFC7540Priorities` |
|---------|---------------------|-----------------------|
| Chrome | `:method`, `:authority`, `:scheme`, `:path` (m,a,s,p) | false |
| Firefox | `:method`, `:path`, `:authority`, `:scheme` | false |
| Safari/iOS | `:method`, `:scheme`, `:path`, `:authority` (m,s,p,a) | true |

### Header Order

HTTP/2 header order is preserved and matters for fingerprinting. Chrome 143 sends headers in this specific order (verified via tls.peet.ws):

```go
// pool/pool.go and transport/http2_transport.go (Chrome 143)
HeaderOrder: []string{
    "cache-control",                    // appears on reload/session resumption
    "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform",
    "upgrade-insecure-requests", "user-agent",
    "content-type", "content-length",   // for POST requests
    "accept", "origin",                 // origin for CORS
    "sec-fetch-site", "sec-fetch-mode", "sec-fetch-user", "sec-fetch-dest",
    "referer",
    "accept-encoding", "accept-language",
    "cookie", "priority",
}
```

Both the connection pool (`pool/pool.go`) and HTTP/2 transport (`transport/http2_transport.go`) use identical header ordering to ensure consistent fingerprints regardless of which code path handles the request.

#### Runtime Header Order Customization

Header order can be customized at runtime using `SetHeaderOrder()`:

```go
// Get current header order (from preset)
order := session.GetHeaderOrder()

// Set custom header order
session.SetHeaderOrder([]string{
    "accept-language", "sec-ch-ua", "accept",
    "sec-fetch-site", "sec-fetch-mode", "user-agent",
})

// Reset to preset's default order
session.SetHeaderOrder(nil)
```

This is useful when:
- You need to match a specific target's expected header order
- You're reverse-engineering a server that checks header order
- You need to test different fingerprints without changing presets

Available in all language bindings: Python (`set_header_order`), Node.js (`setHeaderOrder`), C# (`SetHeaderOrder`).

### HPACK Compression

HTTP/2 uses HPACK for header compression with configurable table size:

```go
// pool/pool.go:378-379
MaxDecoderHeaderTableSize: settings.HeaderTableSize,  // 65536 for Chrome
MaxEncoderHeaderTableSize: settings.HeaderTableSize,
```

---

## HTTP/3 (QUIC)

### QUIC Transport Parameters

Chrome sends specific QUIC transport parameters:

```go
// transport/http3_transport.go:48-71
func buildChromeTransportParams() map[uint64][]byte {
    params := make(map[uint64][]byte)

    // version_information (0x11) - RFC 9368
    // Format: chosen_version + available_versions
    // Chrome sends: QUICv1 (chosen) + [GREASE, QUICv1] (available)

    // google_version (0x4752) - Google's custom parameter
    // 4-byte QUICv1 version
}
```

#### Transport Parameter IDs

| Parameter | ID | Description |
|-----------|-----|-------------|
| version_information | 0x11 | RFC 9368 version negotiation |
| google_version | 0x4752 (18258) | Google's custom version param |

### GREASE Versions

Chrome includes GREASE (Generate Random Extensions And Sustain Extensibility) versions:

```go
// transport/http3_transport.go:74-81
// GREASE versions are of form 0x?a?a?a?a where ? is random nibble
func generateGREASEVersion() uint32 {
    nibble := byte(rand.Intn(16))
    return uint32(nibble)<<28 | 0x0a000000 |
           uint32(nibble)<<20 | 0x000a0000 |
           uint32(nibble)<<12 | 0x00000a00 |
           uint32(nibble)<<4  | 0x0000000a
}
```

### HTTP/3 SETTINGS

HTTP/3 uses QPACK for header compression. Chrome and Safari send different settings:

```go
// transport/http3_transport.go
const (
    settingQPACKMaxTableCapacity = 0x1
    settingMaxFieldSectionSize   = 0x6
    settingQPACKBlockedStreams   = 0x7
    settingH3Datagram            = 0x33
)
```

#### Chrome H3 SETTINGS
| Setting | ID | Value |
|---------|-----|-------|
| QPACK_MAX_TABLE_CAPACITY | 0x1 | 0 |
| MAX_FIELD_SECTION_SIZE | 0x6 | 262144 |
| QPACK_BLOCKED_STREAMS | 0x7 | 100 |
| H3_DATAGRAM | 0x33 | 1 |
| GREASE | random | random non-zero |

#### Safari/iOS H3 SETTINGS
| Setting | ID | Value |
|---------|-----|-------|
| QPACK_MAX_TABLE_CAPACITY | 0x1 | 16383 |
| QPACK_BLOCKED_STREAMS | 0x7 | 100 |
| GREASE | random | random non-zero |

Note: Safari does NOT send `MAX_FIELD_SECTION_SIZE` or `H3_DATAGRAM`.

### QUIC ClientHelloID

Each preset has specific QUIC TLS fingerprints:

```go
// Chrome 145
QUICClientHelloID:    tls.HelloChrome_145_QUIC,     // For HTTP/3
QUICPSKClientHelloID: tls.HelloChrome_145_QUIC_PSK, // For session resumption

// Safari/iOS
QUICClientHelloID:    tls.HelloIOS_18_QUIC,         // WebKit QUIC fingerprint
```

| Browser | QUIC ClientHelloID | QUIC PSK ClientHelloID |
|---------|--------------------|-----------------------|
| Chrome 145 | `HelloChrome_145_QUIC` | `HelloChrome_145_QUIC_PSK` |
| Chrome 144 | `HelloChrome_144_QUIC` | `HelloChrome_144_QUIC_PSK` |
| Chrome 143 | `HelloChrome_143_QUIC` | `HelloChrome_143_QUIC_PSK` |
| Safari 18 | `HelloIOS_18_QUIC` | - |
| iOS Safari 18 | `HelloIOS_18_QUIC` | - |
| iOS Chrome 143/144/145 | `HelloIOS_18_QUIC` | - |
| Android Chrome | `HelloChrome_*_QUIC` | `HelloChrome_*_QUIC_PSK` |

### 0-RTT (Early Data)

HTTP/3 supports 0-RTT for reduced latency on resumed connections:

- Session tickets stored in TLS session cache
- `early_data` extension only sent when resuming (not on fresh connections)
- ECH config must match the one used when creating the ticket

### Connection Reuse

HTTP/3 connections are reused via the underlying `http3.Transport`:

```go
// transport/http3_transport.go:84-85
// http3.Transport handles connection pooling internally
type HTTP3Transport struct {
    transport    *http3.Transport
    sessionCache tls.ClientSessionCache
    // ...
}
```

---

## Decompression

All protocols support automatic decompression:

```go
// transport/transport.go:1298-1330
func decompress(data []byte, encoding string) ([]byte, error) {
    switch strings.ToLower(encoding) {
    case "gzip":
        reader, _ := gzip.NewReader(bytes.NewReader(data))
        return io.ReadAll(reader)
    case "br":
        reader := brotli.NewReader(bytes.NewReader(data))
        return io.ReadAll(reader)
    case "zstd":
        decoder, _ := zstd.NewReader(bytes.NewReader(data))
        return io.ReadAll(decoder)
    case "deflate":
        reader := flate.NewReader(bytes.NewReader(data))
        return io.ReadAll(reader)
    }
}
```

| Encoding | Library |
|----------|---------|
| gzip | `compress/gzip` |
| br (brotli) | `github.com/andybalholm/brotli` |
| zstd | `github.com/klauspost/compress/zstd` |
| deflate | `compress/flate` |

---

## File References

| Topic | File | Lines |
|-------|------|-------|
| Protocol Selection | `transport/transport.go` | 572-663 |
| Protocol Racing | `transport/transport.go` | 733-820 |
| HTTP/2 Transport | `pool/pool.go` | 368-400 |
| HTTP/2 SETTINGS | `pool/pool.go` | 383-394 |
| HTTP/3 Transport | `transport/http3_transport.go` | 83-135 |
| QUIC Params | `transport/http3_transport.go` | 33-71 |
| Header Order Customization | `client/client.go` | SetHeaderOrder, GetHeaderOrder |
| Decompression | `transport/transport.go` | 1298-1330 |
