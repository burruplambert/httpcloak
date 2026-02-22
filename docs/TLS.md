# HTTPCloak TLS Features

This document covers TLS/crypto features including session resumption, ECH, certificate pinning, and fingerprinting.

## TLS Fingerprinting

HTTPCloak uses a custom fork of utls (`github.com/sardanioss/utls`) for TLS fingerprinting. Each browser preset includes a specific `ClientHelloID` that produces the correct JA3/JA4 fingerprint.

### ClientHelloID Types

```go
// fingerprint/presets.go:57-60
type Preset struct {
    ClientHelloID     tls.ClientHelloID // For TCP/TLS (HTTP/1.1, HTTP/2)
    PSKClientHelloID  tls.ClientHelloID // For TCP/TLS with PSK (session resumption)
    QUICClientHelloID tls.ClientHelloID // For QUIC/HTTP/3 (different TLS extensions)
    QUICPSKClientHelloID tls.ClientHelloID // For QUIC/HTTP/3 with PSK
}
```

### Chrome 145 Fingerprints (Default)

| Protocol | ClientHelloID |
|----------|---------------|
| TCP | `HelloChrome_145_Windows`, `HelloChrome_145_Linux`, `HelloChrome_145_macOS` |
| TCP + PSK | `HelloChrome_145_Windows_PSK`, `HelloChrome_145_Linux_PSK`, `HelloChrome_145_macOS_PSK` |
| QUIC | `HelloChrome_145_QUIC` |
| QUIC + PSK | `HelloChrome_145_QUIC_PSK` |

### Safari/iOS Fingerprints

| Protocol | ClientHelloID |
|----------|---------------|
| TCP (Safari 18) | `HelloSafari_18` |
| TCP (iOS 18) | `HelloIOS_18` |
| QUIC (all Safari/iOS) | `HelloIOS_18_QUIC` |

Note: iOS Chrome uses Safari/WebKit TLS (Apple requirement), not Chrome TLS.

### Extension Order Shuffling

Chrome shuffles TLS extension order once per session (not per connection). HTTPCloak mimics this:

```go
// pool/pool.go:121-122
// Shuffle seed for generating fresh specs per connection
shuffleSeed int64

// transport/http3_transport.go:104-105
// Shuffle seed for TLS and transport parameter ordering (consistent per session)
shuffleSeed int64
```

Usage:

```go
// pool/pool.go:148-150
spec, err := utls.UTLSIdToSpecWithSeed(preset.ClientHelloID, shuffleSeed)
```

---

## Session Resumption

TLS 1.3 session resumption allows faster subsequent connections using pre-shared keys (PSK).

### How It Works

1. **First Connection**: Full TLS handshake, server sends `NewSessionTicket`
2. **Subsequent Connections**: Client sends `pre_shared_key` extension, 0-RTT possible

### Session Cache

Each pool maintains a session cache:

```go
// pool/pool.go:113
sessionCache utls.ClientSessionCache

// pool/pool.go:167
sessionCache: utls.NewLRUClientSessionCache(32), // Cache up to 32 sessions per host
```

### PSK Spec Selection

Only use PSK spec when there's an actual cached session:

```go
// transport/http3_transport.go:163-170
func (t *HTTP3Transport) getSpecForHost(host string) *utls.ClientHelloSpec {
    // Only use PSK spec when there's a cached session for this host
    // This matches Chrome's behavior: no early_data on fresh connections
    if t.cachedClientHelloSpecPSK != nil && t.hasSessionForHost(host) {
        return t.cachedClientHelloSpecPSK
    }
    return t.cachedClientHelloSpec
}
```

### early_data Extension

The `early_data` extension is only sent on resumption (not fresh connections):

```go
// pool/pool.go:316
OmitEmptyPsk: true,  // Chrome doesn't send empty PSK on first connection
```

### Session Persistence

Sessions can be persisted to disk. The `PersistableSessionCache` uses LRU eviction with a maximum of 32 entries to prevent unbounded memory growth in long-running processes:

```go
// transport/tls_cache.go
const TLSSessionCacheMaxSize = 32  // Maximum cached sessions (LRU eviction)

// session/session.go:906-946
func (s *Session) exportTLSSessions() (map[string]transport.TLSSessionState, error) {
    allSessions := make(map[string]transport.TLSSessionState)

    // Export from HTTP/1.1 transport session cache
    if h1 := s.transport.GetHTTP1Transport(); h1 != nil {
        if cache, ok := h1.GetSessionCache().(*transport.PersistableSessionCache); ok {
            sessions, _ := cache.Export()
            for k, v := range sessions {
                allSessions["h1:"+k] = v
            }
        }
    }
    // ... similar for H2 and H3
}
```

### Distributed Session Cache

For distributed deployments (multiple HTTPCloak instances behind a load balancer), session tickets can be shared via an external cache like Redis or Memcached. This enables TLS session resumption across all instances.

#### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Distributed Cache (Redis)                     │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ httpcloak:sessions:chrome-145:h2:api.example.com:443    │    │
│  │ httpcloak:sessions:chrome-145:h1:httpbin.org:443        │    │
│  │ httpcloak:ech:chrome-145:cloudflare.com:443             │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
                              ▲
                              │ Get/Put
                              │
┌─────────────────────────────┼─────────────────────────────────┐
│  Instance A                 │          Instance B              │
│  ┌─────────┐               │          ┌─────────┐             │
│  │ Session │◄──────────────┼──────────│ Session │             │
│  │  Cache  │               │          │  Cache  │             │
│  │ (Local) │               │          │ (Local) │             │
│  └────┬────┘               │          └────┬────┘             │
│       │                    │               │                   │
│       ▼                    │               ▼                   │
│  ┌─────────┐               │          ┌─────────┐             │
│  │Transport│               │          │Transport│             │
│  └─────────┘               │          └─────────┘             │
└────────────────────────────┴──────────────────────────────────┘
```

#### Cache Key Format

Session tickets are stored with preset information to prevent mixing sessions from different browser fingerprints:

```
TLS Sessions: httpcloak:sessions:{preset}:{protocol}:{host}:{port}
ECH Configs:  httpcloak:ech:{preset}:{host}:{port}

Examples:
- httpcloak:sessions:chrome-145:h2:api.example.com:443
- httpcloak:sessions:firefox-133:h1:httpbin.org:443
- httpcloak:ech:chrome-145:cloudflare.com:443
```

#### Session State Format

```go
// transport/tls_cache.go
type TLSSessionState struct {
    Ticket    string    `json:"ticket"`     // Base64-encoded session ticket
    State     string    `json:"state"`      // Base64-encoded TLS session state
    CreatedAt time.Time `json:"created_at"` // For TTL enforcement
}
```

#### TTL Considerations

- TLS 1.3 session tickets are typically valid for 24 hours (server-defined)
- Recommended cache TTL: 23 hours (slightly less than ticket lifetime)
- Sessions older than TTL are ignored during Get operations

#### Error Handling

Cache operations use an error callback for monitoring:

```go
type ErrorCallback func(operation string, key string, err error)

// Operations: "get", "put", "delete", "get_ech", "put_ech"
```

Errors are propagated but don't fail requests - sessions fall back to full TLS handshake.

---

## 0-RTT Early Data

TLS 1.3 0-RTT allows sending data before the handshake completes on resumed connections.

### Requirements

1. Cached session ticket from previous connection
2. Same ECH config as original connection
3. PSK ClientHello spec

### Behavior

- Enabled automatically when session ticket exists
- Data is sent with the ClientHello
- Reduces latency by 1-RTT

---

## Encrypted Client Hello (ECH)

ECH encrypts the SNI (Server Name Indication) to prevent network observers from seeing the destination hostname.

### ECH Config Sources

1. **Automatic DNS Fetch**: ECH config from DNS HTTPS records
2. **Custom Config**: User-provided ECH config bytes
3. **Alternate Domain**: Fetch ECH from a different domain

### DNS HTTPS Records

```go
// dns/cache.go:305-336
func FetchECHConfigs(ctx context.Context, hostname string) ([]byte, error) {
    // Query DNS for HTTPS records (type 65)
    // Extract ech parameter from SVCB record
}
```

### Custom ECH Config

```go
transport.SetECHConfig(echConfigBytes)
```

### ECH from Alternate Domain

Useful for domain fronting with ECH:

```go
transport.SetECHConfigDomain("cloudflare-ech.com")
```

### ECH Caching

ECH configs are cached for session resumption:

```go
// transport/http3_transport.go:128-131
// ECH config cache - stores ECH configs per host for session resumption
// When resuming a session, we must use the same ECH config that was used
// to create the original session ticket, not a fresh one from DNS
echConfigCache   map[string][]byte
```

### Minimum TLS Version

ECH requires TLS 1.3:

```go
// pool/pool.go:301-305
// ECH requires TLS 1.3, so set MinVersion accordingly
minVersion := uint16(tls.VersionTLS12)
if len(echConfigList) > 0 {
    minVersion = tls.VersionTLS13
}
```

### Configurable ECH DNS Servers

By default, ECH queries use Google (8.8.8.8), Cloudflare (1.1.1.1), and Quad9 (9.9.9.9) DNS servers. This can be customized for environments with restricted network access or privacy requirements:

```go
// dns/cache.go
// Set custom DNS servers for ECH queries
dns.SetECHDNSServers([]string{"10.0.0.53:53", "192.168.1.1:53"})

// Get current ECH DNS servers
servers := dns.GetECHDNSServers()

// Reset to defaults (pass empty slice)
dns.SetECHDNSServers(nil)
```

These functions are thread-safe and can be called at runtime.

---

## Certificate Pinning

Certificate pinning prevents MITM attacks by verifying the server's certificate against known pins.

### Pin Types

```go
// client/certpin.go:16-24
type PinType int

const (
    PinTypeSHA256 PinType = iota  // SHA256 of SPKI (standard)
    PinTypeCertificate            // Entire certificate
)
```

### Adding Pins

```go
// SHA256 hash of SPKI (HPKP format)
pinner := client.NewCertPinner()
pinner.AddPin("sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")

// For specific host
pinner.AddPin("sha256/...", client.ForHost("example.com"))

// Include subdomains
pinner.AddPin("sha256/...", client.ForHost("example.com"), client.IncludeSubdomains())

// From PEM file
pinner.AddPinFromCertFile("/path/to/cert.pem")

// From PEM data
pinner.AddPinFromPEM(pemBytes)
```

### SPKI Hash Calculation

```go
// client/certpin.go:186-190
func CalculateSPKIHash(cert *x509.Certificate) string {
    spkiHash := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
    return base64.StdEncoding.EncodeToString(spkiHash[:])
}
```

### Verification

```go
// client/certpin.go:111-142
func (p *CertPinner) Verify(host string, certs []*x509.Certificate) error {
    // Find applicable pins for this host
    // Check each certificate in chain against pins
    // Return error if no match found
}
```

### Pin Error

```go
// client/certpin.go:227-231
type CertPinError struct {
    Host           string
    ExpectedHashes []string
    ActualHashes   []string
}
```

---

## InsecureSkipVerify

For testing, TLS certificate verification can be disabled:

```go
transport.SetInsecureSkipVerify(true)
```

This propagates to all sub-transports (HTTP/1.1, HTTP/2, and HTTP/3):

```go
// transport/transport.go:329-340
func (t *Transport) SetInsecureSkipVerify(skip bool) {
    t.insecureSkipVerify = skip
    t.h1Transport.SetInsecureSkipVerify(skip)
    if t.h2Transport != nil {
        t.h2Transport.SetInsecureSkipVerify(skip)
    }
    if t.h3Transport != nil {
        t.h3Transport.SetInsecureSkipVerify(skip)
    }
}
```

---

## TLS-Only Mode

TLS-only mode allows you to use HTTPCloak's TLS fingerprinting without applying preset HTTP headers. This is useful when you want full control over HTTP headers while still maintaining a browser-like TLS fingerprint.

### Use Cases

1. **Custom HTTP Stacks**: When integrating with systems that manage their own HTTP headers
2. **Minimal Headers**: When you want only your specified headers sent (no Sec-Ch-Ua, Accept, etc.)
3. **Testing**: When testing TLS fingerprinting without HTTP header interference
4. **Specific Requirements**: When your target expects specific headers that differ from browser presets

### What TLS-Only Mode Does

| Feature | Normal Mode | TLS-Only Mode |
|---------|------------|---------------|
| TLS Fingerprint (JA3/JA4) | Applied | Applied |
| Peetprint | Applied | Applied |
| Akamai Fingerprint | Applied | Applied |
| HTTP/2 Settings | Applied | Applied |
| Preset HTTP Headers | Applied | **Skipped** |
| User-Agent | Preset value | Your value only |
| Sec-Ch-Ua headers | Auto-added | **Not added** |
| Accept headers | Auto-added | **Not added** |

### Usage

```go
// Go
session := httpcloak.NewSession(httpcloak.SessionConfig{
    Preset:  "chrome-145",
    TLSOnly: true,
})
```

```python
# Python
session = Session(preset="chrome-145", tls_only=True)
```

```javascript
// Node.js
const session = new Session({ preset: "chrome-145", tlsOnly: true });
```

```csharp
// C#
var session = new Session(preset: "chrome-145", tlsOnly: true);
```

### Implementation

The `TLSOnly` flag is checked in the transport layer before applying preset headers:

```go
// transport/http3_transport.go
tlsOnly := t.config != nil && t.config.TLSOnly
if !tlsOnly {
    // Apply preset headers in specified order
    for _, hp := range t.preset.HeaderOrder {
        if req.Header.Get(hp.Key) == "" {
            req.Header.Set(hp.Key, hp.Value)
        }
    }
}
```

---

## TLS Key Logging (SSLKEYLOGFILE)

HTTPCloak supports writing TLS session keys to a file in NSS Key Log Format for traffic decryption with Wireshark.

### Use Cases

1. **Protocol Analysis**: Debug TLS handshakes and HTTP traffic
2. **Testing**: Verify request/response content through proxies
3. **Security Research**: Analyze encrypted traffic patterns
4. **Development**: Debug fingerprint detection issues

### Supported Protocols

| Protocol | Support |
|----------|---------|
| HTTP/1.1 (TLS 1.3) | ✓ |
| HTTP/2 (TLS 1.3) | ✓ |
| HTTP/3 (QUIC) | ✓ |

### Usage

#### Environment Variable (Global)

```bash
export SSLKEYLOGFILE=/tmp/keys.log
# All sessions will write keys to this file
```

#### Per-Session Option

```go
// Go
session := httpcloak.NewSession("chrome-145",
    httpcloak.WithKeyLogFile("/tmp/keys.log"),
)
```

```python
# Python
session = Session(preset="chrome-145", key_log_file="/tmp/keys.log")
```

```javascript
// Node.js
const session = new Session({ preset: "chrome-145", keyLogFile: "/tmp/keys.log" });
```

```csharp
// C#
var session = new Session(preset: "chrome-145", keyLogFile: "/tmp/keys.log");
```

### Wireshark Configuration

1. Open Wireshark
2. Go to: **Edit → Preferences → Protocols → TLS**
3. Set **(Pre)-Master-Secret log filename** to your key log file path
4. Capture traffic - Wireshark will automatically decrypt TLS

### File Format

The key log file uses NSS Key Log Format:

```
CLIENT_HANDSHAKE_TRAFFIC_SECRET <client_random> <secret>
SERVER_HANDSHAKE_TRAFFIC_SECRET <client_random> <secret>
CLIENT_TRAFFIC_SECRET_0 <client_random> <secret>
SERVER_TRAFFIC_SECRET_0 <client_random> <secret>
```

### Implementation

```go
// transport/keylog.go - Global key log writer
func GetKeyLogWriter() io.Writer {
    // Returns writer from SSLKEYLOGFILE env or nil
}

// transport/http1_transport.go, http2_transport.go, http3_transport.go
// Per-session override via TransportConfig.KeyLogWriter
if config != nil && config.KeyLogWriter != nil {
    keyLogWriter = config.KeyLogWriter
} else {
    keyLogWriter = GetKeyLogWriter()
}
```

---

## Domain Fronting

Domain fronting uses different hosts for DNS resolution and HTTP Host header:

```go
// transport/transport.go:479-498
func (t *Transport) SetConnectTo(requestHost, connectHost string) {
    // requestHost: Host header and SNI
    // connectHost: Actual IP to connect to
}
```

This is useful for bypassing censorship where the SNI is inspected.

---

## TLS Session State

Session state includes:

```go
// transport/tls_cache.go
type TLSSessionState struct {
    SessionTicket   []byte    `json:"session_ticket"`
    MasterSecret    []byte    `json:"master_secret"`
    CipherSuite     uint16    `json:"cipher_suite"`
    ServerName      string    `json:"server_name"`
    OCSPResponse    []byte    `json:"ocsp_response,omitempty"`
    SCTList         []byte    `json:"sct_list,omitempty"`
    CreatedAt       time.Time `json:"created_at"`
}
```

---

## File References

| Topic | File | Lines |
|-------|------|-------|
| TLS Fingerprint Presets | `fingerprint/presets.go` | 54-66 |
| Session Cache | `pool/pool.go` | 111-123 |
| PSK Spec Selection | `pool/pool.go` | 323-338 |
| PSK Spec (H3) | `transport/http3_transport.go` | 163-170 |
| ECH Fetch | `dns/cache.go` | 305-385 |
| ECH DNS Servers | `dns/cache.go` | 48-80 |
| ECH Cache | `transport/http3_transport.go` | 128-131 |
| TLS Config | `pool/pool.go` | 309-318 |
| Certificate Pinning | `client/certpin.go` | 1-252 |
| TLS Session State | `transport/tls_cache.go` | 1-100 |
| PersistableSessionCache LRU | `transport/tls_cache.go` | 15-80 |
| InsecureSkipVerify | `transport/transport.go` | 329-340 |
| TLS-Only Mode | `transport/http3_transport.go` | 900-924 |
| TLS Key Logging | `transport/keylog.go` | 1-120 |
