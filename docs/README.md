# HTTPCloak Documentation

This folder is the **master reference** for the HTTPCloak library. Before modifying any code, consult this README to identify which documentation files and source files you need to read.

---

## Documentation Files Overview

### [FEATURES.md](FEATURES.md)
**Purpose**: Complete feature inventory of the library.

**Contents**:
- HTTP protocol support (H1, H2, H3)
- TLS features (fingerprinting, ECH, 0-RTT, session resumption)
- Proxy types (HTTP, SOCKS5, MASQUE)
- Session features (cookies, persistence, caching)
- Network features (DNS, connection pooling, domain fronting)
- Hooks system
- Language bindings list

**Read this when**: You need to understand what the library can do, or check if a feature exists.

**Cross-references**:
- For implementation details of each feature → [ARCHITECTURE.md](ARCHITECTURE.md)
- For preset-specific features → [PRESETS.md](PRESETS.md)

---

### [ARCHITECTURE.md](ARCHITECTURE.md)
**Purpose**: Component relationships, data flow, and internal structure.

**Contents**:
- ASCII component diagram (Session → Transport → Pool hierarchy)
- Request lifecycle (step-by-step flow from user call to response)
- Connection pooling strategy (HostPool, health checks, LRU)
- Session resumption mechanism (PSK flow diagram)
- Protocol negotiation flow (auto-detection, racing)
- Key file paths with line numbers

**Read this when**: You need to understand how components interact, trace a request through the system, or find which file implements what.

**Cross-references**:
- For protocol-specific details → [PROTOCOLS.md](PROTOCOLS.md)
- For TLS handshake details → [TLS.md](TLS.md)
- For proxy connection flow → [PROXIES.md](PROXIES.md)

---

### [PRESETS.md](PRESETS.md)
**Purpose**: Browser fingerprint configurations and their exact values.

**Contents**:
- All 24 available presets with version numbers
- Per-preset configuration:
  - TLS ClientHelloID (TCP, PSK, QUIC variants)
  - User-Agent strings
  - HTTP/2 SETTINGS frame values
  - Header order arrays
  - Client hints (sec-ch-ua, etc.)
- Platform detection logic (Windows/Linux/macOS)
- Mobile presets (iOS, Android)
- Client hints behavior notes

**Read this when**: Adding a new browser preset, updating fingerprint values, or debugging fingerprint detection issues.

**Cross-references**:
- For HTTP/2 SETTINGS usage → [PROTOCOLS.md](PROTOCOLS.md)
- For TLS ClientHelloID usage → [TLS.md](TLS.md)
- For preset selection API → [API.md](API.md)

---

### [PROTOCOLS.md](PROTOCOLS.md)
**Purpose**: HTTP/1.1, HTTP/2, and HTTP/3 protocol-specific behaviors.

**Contents**:
- Protocol selection logic (auto mode, forced mode)
- HTTP/1.1: Connection handling, keep-alive
- HTTP/2:
  - SETTINGS frame values and order
  - WINDOW_UPDATE values
  - Stream priority (weight, exclusive, dependency)
  - Pseudo-header order (`:method`, `:authority`, `:scheme`, `:path`)
  - Header order
  - HPACK compression settings
- HTTP/3:
  - QUIC transport parameters
  - GREASE version generation
  - QPACK settings
  - 0-RTT behavior
- Decompression (gzip, br, zstd, deflate)

**Read this when**: Modifying protocol behavior, fixing fingerprint issues, or debugging HTTP/2/H3 specific problems.

**Cross-references**:
- For preset SETTINGS values → [PRESETS.md](PRESETS.md)
- For connection pooling → [ARCHITECTURE.md](ARCHITECTURE.md)
- For TLS in QUIC → [TLS.md](TLS.md)

---

### [PROXIES.md](PROXIES.md)
**Purpose**: Proxy types, configuration, and implementation details.

**Contents**:
- HTTP/HTTPS proxy (format, CONNECT tunneling, limitations)
- SOCKS5 proxy:
  - TCP CONNECT (for H1/H2)
  - UDP ASSOCIATE (for H3)
  - Authentication flow
- MASQUE proxy:
  - CONNECT-UDP for HTTP/3
  - Known providers list
  - Auto-detection logic
- Split proxy configuration (separate TCP/UDP proxies)
- Runtime proxy switching API
- Protocol-proxy compatibility matrix

**Read this when**: Adding proxy support, fixing proxy issues, or adding new MASQUE providers.

**Cross-references**:
- For proxy in request flow → [ARCHITECTURE.md](ARCHITECTURE.md)
- For proxy API → [API.md](API.md)
- For QUIC over proxy → [PROTOCOLS.md](PROTOCOLS.md)

---

### [TLS.md](TLS.md)
**Purpose**: TLS/crypto features and configuration.

**Contents**:
- TLS fingerprinting (ClientHelloID, extension shuffling)
- Session resumption:
  - Session cache (LRU, per-host)
  - PSK spec selection logic
  - early_data extension behavior
  - Session persistence
- 0-RTT early data
- Encrypted Client Hello (ECH):
  - DNS HTTPS record fetching
  - Custom ECH config
  - ECH from alternate domain
  - ECH caching for resumption
- Certificate pinning (SPKI hash, per-host, subdomain)
- InsecureSkipVerify propagation
- Domain fronting (ConnectTo)

**Read this when**: Modifying TLS behavior, fixing ECH issues, implementing certificate pinning, or debugging session resumption.

**Cross-references**:
- For TLS ClientHelloIDs per preset → [PRESETS.md](PRESETS.md)
- For TLS in connection flow → [ARCHITECTURE.md](ARCHITECTURE.md)
- For QUIC TLS specifics → [PROTOCOLS.md](PROTOCOLS.md)

---

### [API.md](API.md)
**Purpose**: Public API reference for Go and language bindings.

**Contents**:
- Go API:
  - Session creation and configuration
  - Request/Response structs
  - Transport methods
  - Session Manager
  - Hooks system
  - Certificate pinning API
- Python bindings (sync + async)
- Node.js bindings
- C library exports
- Error types and categories

**Read this when**: Adding new API methods, modifying public interfaces, or updating language bindings.

**Cross-references**:
- For internal implementation → [ARCHITECTURE.md](ARCHITECTURE.md)
- For configuration options → [PRESETS.md](PRESETS.md), [PROXIES.md](PROXIES.md), [TLS.md](TLS.md)

---

## Task-Based Guide

### "I want to add a new browser preset"
1. Read [PRESETS.md](PRESETS.md) - understand preset structure
2. Read [PROTOCOLS.md](PROTOCOLS.md) - HTTP/2 SETTINGS values needed
3. Read [TLS.md](TLS.md) - TLS ClientHelloID requirements
4. **Modify**: `fingerprint/presets.go`

### "I want to fix an HTTP/2 fingerprint issue"
1. Read [PROTOCOLS.md](PROTOCOLS.md) - SETTINGS, pseudo-headers, priority
2. Read [PRESETS.md](PRESETS.md) - current values for the preset
3. **Modify**: `pool/pool.go` (HTTP/2 transport setup), `fingerprint/presets.go` (values)

### "I want to add a new proxy type"
1. Read [PROXIES.md](PROXIES.md) - existing proxy implementations
2. Read [ARCHITECTURE.md](ARCHITECTURE.md) - where proxy fits in request flow
3. **Modify**: `proxy/` directory, `transport/transport.go` (proxy detection)

### "I want to fix session resumption"
1. Read [TLS.md](TLS.md) - PSK flow, session cache, early_data
2. Read [ARCHITECTURE.md](ARCHITECTURE.md) - session resumption diagram
3. **Modify**: `pool/pool.go` (TCP), `transport/http3_transport.go` (QUIC)

### "I want to modify ECH behavior"
1. Read [TLS.md](TLS.md) - ECH sources, caching
2. **Modify**: `dns/cache.go` (ECH fetching), `pool/pool.go` (ECH usage)

### "I want to add a new API method"
1. Read [API.md](API.md) - existing API patterns
2. Read [ARCHITECTURE.md](ARCHITECTURE.md) - component to modify
3. **Modify**: `session/session.go` or `transport/transport.go`

### "I want to update language bindings"
1. Read [API.md](API.md) - binding patterns
2. **Modify**: `bindings/python/`, `bindings/nodejs/`, `bindings/clib/`

### "I want to customize header order"
1. Read [FEATURES.md](FEATURES.md) - Header Order Customization
2. Read [API.md](API.md) - SetHeaderOrder/GetHeaderOrder methods
3. **Use**: `session.SetHeaderOrder([]string{...})` in Go, similar in other bindings

### "I want to modify connection pooling"
1. Read [ARCHITECTURE.md](ARCHITECTURE.md) - pooling strategy
2. Read [PROTOCOLS.md](PROTOCOLS.md) - protocol-specific connection handling
3. **Modify**: `pool/pool.go`

### "I want to fix a redirect/cookie issue"
1. Read [FEATURES.md](FEATURES.md) - redirect/cookie features
2. Read [ARCHITECTURE.md](ARCHITECTURE.md) - request lifecycle
3. **Modify**: `session/session.go`

### "I want to bind to a local IP address (IPv6 rotation)"
1. Read [FEATURES.md](FEATURES.md) - Local Address Binding section
2. Read [API.md](API.md) - WithLocalAddress option
3. **Use**: `httpcloak.WithLocalAddress("2001:db8::1")` in Go, similar in other bindings
4. **Note**: IP family filtering is automatic (IPv6 local → IPv6 targets only)

### "I want to debug TLS traffic with Wireshark"
1. Read [FEATURES.md](FEATURES.md) - TLS Key Logging section
2. Read [API.md](API.md) - WithKeyLogFile option
3. **Use**: `httpcloak.WithKeyLogFile("/tmp/keys.log")` or `SSLKEYLOGFILE` env var
4. **Wireshark**: Edit → Preferences → Protocols → TLS → (Pre)-Master-Secret log filename

---

## Source File → Documentation Mapping

| Source File | Primary Doc | Secondary Docs |
|-------------|-------------|----------------|
| `httpcloak.go` | [API.md](API.md) | [ARCHITECTURE.md](ARCHITECTURE.md) |
| `fingerprint/presets.go` | [PRESETS.md](PRESETS.md) | [PROTOCOLS.md](PROTOCOLS.md), [TLS.md](TLS.md) |
| `session/session.go` | [API.md](API.md) | [ARCHITECTURE.md](ARCHITECTURE.md), [FEATURES.md](FEATURES.md) |
| `session/manager.go` | [API.md](API.md) | [ARCHITECTURE.md](ARCHITECTURE.md) |
| `session/state.go` | [TLS.md](TLS.md) | [API.md](API.md) |
| `transport/transport.go` | [ARCHITECTURE.md](ARCHITECTURE.md) | [PROTOCOLS.md](PROTOCOLS.md), [PROXIES.md](PROXIES.md) |
| `transport/http1_transport.go` | [PROTOCOLS.md](PROTOCOLS.md) | [ARCHITECTURE.md](ARCHITECTURE.md) |
| `transport/http2_transport.go` | [PROTOCOLS.md](PROTOCOLS.md) | [ARCHITECTURE.md](ARCHITECTURE.md) |
| `transport/http3_transport.go` | [PROTOCOLS.md](PROTOCOLS.md) | [TLS.md](TLS.md), [ARCHITECTURE.md](ARCHITECTURE.md) |
| `transport/tls_cache.go` | [TLS.md](TLS.md) | [ARCHITECTURE.md](ARCHITECTURE.md) |
| `transport/stream.go` | [API.md](API.md) | [FEATURES.md](FEATURES.md) |
| `transport/errors.go` | [API.md](API.md) | - |
| `transport/keylog.go` | [TLS.md](TLS.md) | [FEATURES.md](FEATURES.md) |
| `pool/pool.go` | [ARCHITECTURE.md](ARCHITECTURE.md) | [PROTOCOLS.md](PROTOCOLS.md), [TLS.md](TLS.md) |
| `dns/cache.go` | [TLS.md](TLS.md) | [ARCHITECTURE.md](ARCHITECTURE.md) |
| `proxy/socks5_tcp.go` | [PROXIES.md](PROXIES.md) | [ARCHITECTURE.md](ARCHITECTURE.md) |
| `proxy/socks5_udp.go` | [PROXIES.md](PROXIES.md) | [PROTOCOLS.md](PROTOCOLS.md) |
| `proxy/masque_providers.go` | [PROXIES.md](PROXIES.md) | - |
| `client/client.go` | [API.md](API.md) | [FEATURES.md](FEATURES.md) |
| `client/hooks.go` | [API.md](API.md) | [FEATURES.md](FEATURES.md) |
| `client/certpin.go` | [TLS.md](TLS.md) | [API.md](API.md) |
| `client/options.go` | [API.md](API.md) | [PRESETS.md](PRESETS.md) |
| `protocol/types.go` | [API.md](API.md) | - |
| `bindings/clib/httpcloak.go` | [API.md](API.md) | - |
| `bindings/python/httpcloak/client.py` | [API.md](API.md) | - |
| `bindings/nodejs/lib/index.js` | [API.md](API.md) | - |
| `local_proxy.go` | [API.md](API.md) | [FEATURES.md](FEATURES.md) |

---

## Documentation → Source File Mapping

### [FEATURES.md](FEATURES.md) covers:
- All source files (feature inventory)

### [ARCHITECTURE.md](ARCHITECTURE.md) covers:
- `httpcloak.go` - Public API entry
- `session/session.go` - Session management
- `session/manager.go` - Multi-session manager
- `transport/transport.go` - Unified transport
- `pool/pool.go` - Connection pooling
- `dns/cache.go` - DNS caching

### [PRESETS.md](PRESETS.md) covers:
- `fingerprint/presets.go` - All preset definitions
- `client/options.go` - Default preset selection

### [PROTOCOLS.md](PROTOCOLS.md) covers:
- `transport/transport.go:572-722` - Protocol selection
- `transport/http1_transport.go` - HTTP/1.1 specifics
- `transport/http2_transport.go` - HTTP/2 specifics
- `transport/http3_transport.go` - HTTP/3 specifics
- `pool/pool.go:368-400` - HTTP/2 SETTINGS

### [PROXIES.md](PROXIES.md) covers:
- `proxy/socks5_tcp.go` - SOCKS5 TCP CONNECT
- `proxy/socks5_udp.go` - SOCKS5 UDP ASSOCIATE
- `proxy/masque_providers.go` - MASQUE detection
- `transport/transport.go:474-516` - Proxy type detection
- `session/session.go` - Runtime proxy switching

### [TLS.md](TLS.md) covers:
- `pool/pool.go:112-168` - TLS session cache setup
- `pool/pool.go:309-360` - TLS handshake
- `transport/http3_transport.go` - QUIC TLS
- `transport/tls_cache.go` - Session persistence (LRU cache with max 32 entries)
- `dns/cache.go:337-368` - ECH fetching
- `dns/cache.go:307-335` - Configurable ECH DNS servers
- `client/certpin.go` - Certificate pinning

### [API.md](API.md) covers:
- `httpcloak.go` - Main API
- `session/session.go` - Session API
- `transport/transport.go` - Transport API
- `client/client.go` - Header order API (SetHeaderOrder, GetHeaderOrder)
- `client/hooks.go` - Hooks API
- `client/certpin.go` - Cert pinning API
- `dns/cache.go` - DNS configuration API (SetECHDNSServers, GetECHDNSServers)
- `bindings/` - Language bindings

---

## Current Default Values

| Setting | Value | Source |
|---------|-------|--------|
| Default Preset | `chrome-145` | `client/options.go` |
| Default Timeout | 30 seconds | `session/session.go:69` |
| Max Redirects | 10 | `session/session.go:264` |
| Retry Status Codes | 429, 500, 502, 503, 504 | `session/session.go:173` |
| DNS Cache TTL | 5 minutes | `dns/cache.go:47` |
| DNS Min TTL | 30 seconds | `dns/cache.go:48` |
| Connection Max Age | 5 minutes | `pool/pool.go:170` |
| Connection Max Idle | 90 seconds | `pool/pool.go:169` |
| Session Cache Size | 32 per host | `pool/pool.go:167` |
| TLS Session Cache Size | 32 (LRU) | `transport/tls_cache.go:15` |
| Manager Max Sessions | 100 | `session/manager.go:29` |
| Manager Session Timeout | 30 minutes | `session/manager.go:30` |

---

## Other Documentation Files

These files exist in the docs folder but are not part of the core reference:

| File | Purpose |
|------|---------|
| `chrome-behaviour.md` | Research notes on Chrome's actual behavior |
| `tcp_ip_plan.md` | Planning document for TCP/IP features |
| `MASQUE_FINGERPRINT_LIMITATIONS.md` | Known limitations of MASQUE fingerprinting |
| `assets/` | Images and diagrams |

---

*Last Updated: February 22, 2026 (Chrome 145 default, 24 presets, Safari/iOS H3 support, Local Address Binding, TLS Key Logging)*
