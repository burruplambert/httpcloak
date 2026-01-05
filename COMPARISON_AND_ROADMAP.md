# httpcloak vs bogdanfinn/tls-client Comparison & Roadmap

## Current State Comparison

### What bogdanfinn Has

| Feature | bogdanfinn | httpcloak (current) | sardanioss/net+http |
|---------|------------|---------------------|---------------------|
| HTTP/2 Settings Order | ✅ fhttp | ❌ Frame interception | ✅ Native |
| HTTP/2 Pseudo-header Order | ✅ fhttp | ❌ HPACK re-encoding | ✅ Native |
| HTTP/2 Connection Flow | ✅ fhttp | ❌ Hardcoded | ✅ Configurable |
| HTTP/2 Priority in HEADERS | ✅ fhttp | ❌ Manual injection | ✅ Native |
| HTTP/2 PRIORITY frames | ✅ fhttp | ❌ | ✅ Native |
| HTTP/2 Header Order | ✅ fhttp | ❌ | ✅ Native |
| HTTP/2 Stream Priority Tree | ❌ | ❌ | ✅ Native |
| HTTP/2 HPACK Indexing Control | ❌ | ❌ | ✅ Native |
| Per-request Header Order | ✅ HeaderOrderKey magic | ❌ | ✅ Magic keys supported |
| Custom User-Agent | ✅ | ❌ | ✅ Transport-level |
| HTTP/1.1 Header Order | ✅ fhttp (forked net/http) | ❌ | ✅ sardanioss/http |
| HTTP/1.1 Keep-Alive Patterns | ❌ | ❌ | ✅ sardanioss/http |
| HTTP/1.1 Connection Reuse | ❌ | ❌ | ✅ sardanioss/http |
| TLS ClientHelloSpec | ✅ SpecFactory pattern | ✅ Uses utls presets | ✅ Same |
| Browser Profiles | ✅ 50+ profiles | ~7 profiles | N/A |
| HTTP/3 Support | ✅ quic-go-utls fork | ✅ quic-go | Need fingerprinting |
| CFFI Bindings | ✅ Pre-compiled | ❌ IPC daemon | ❌ |
| Session Resumption | ✅ PSK support | ✅ Basic | ✅ |

---

## What's Completed in sardanioss Libraries

### sardanioss/net (HTTP/2 Fingerprinting)
- ✅ Custom Settings with ordered SETTINGS frame
- ✅ Custom ConnectionFlow (WINDOW_UPDATE)
- ✅ Custom PseudoHeaderOrder (`:method`, `:authority`, `:scheme`, `:path`)
- ✅ Custom HeaderOrder for regular headers
- ✅ HeaderPriority in HEADERS frame
- ✅ PRIORITY frames after connection preface
- ✅ Per-request magic keys (`Header-Order:`, `PHeader-Order:`)
- ✅ Custom default UserAgent
- ✅ Stream Priority Tree (Chrome/Firefox modes)
- ✅ HPACK Indexing Behavior control (Chrome-like, Never, Always, Custom)

### sardanioss/http (HTTP/1.1 Fingerprinting)
- ✅ Header ordering via magic keys (from fhttp)
- ✅ Keep-Alive pattern fingerprinting (Chrome/Firefox modes)
- ✅ Connection reuse behavior control
- ✅ Custom timeout and max requests configuration
- ✅ Force HTTP/1.0 option
- ✅ Connection prewarming option

---

## What's Still Missing (Must Have)

### 1. HTTP/3 (QUIC) Fingerprinting
QUIC has its own fingerprint vectors:
- Transport parameters order
- Initial packet size
- Connection ID length
- QUIC version negotiation

**Solution:** Fork quic-go like bogdanfinn did (quic-go-utls)

### 2. More Browser Profiles
Currently only ~7 profiles, need:
- Chrome 103-143 (all versions)
- Firefox 102-135
- Safari 15-18 (macOS + iOS)
- Opera, Edge, Brave
- Mobile browsers (Chrome Mobile, Safari iOS)
- Android OkHttp profiles
- Custom app profiles (Zalando, Nike, etc.)

---

## What We Can Add (Differentiators)

### 1. Automatic Header Coherence
**Problem:** Setting wrong Sec-Fetch-* or Client Hints is a fingerprint.

**Solution:** Auto-generate coherent headers based on:
- Request context (navigate vs fetch vs XHR)
- Browser profile (Chrome vs Firefox have different headers)
- Referrer policy

```go
// Instead of manual headers
client.SetFetchMode(httpcloak.FetchModeNavigate) // Auto-sets Sec-Fetch-*
client.SetClientHints(true) // Auto-generates matching Sec-Ch-Ua-*
```

### 2. Dynamic Fingerprint Generation
**Problem:** Static fingerprints get detected over time.

**Solution:**
- Capture real browser traffic and extract fingerprints
- Generate fingerprints from browser version + OS combination
- Add slight organic variations (like real browsers)

```go
fp := httpcloak.GenerateFingerprint(httpcloak.Chrome, "143", httpcloak.Windows11)
```

### 3. Protocol Intelligence
**Problem:** Choosing wrong protocol reveals bot behavior.

**Solution:** (httpcloak already has some of this)
- Remember which protocol each host supports
- Smart fallback (H3 → H2 → H1)
- Match real browser protocol selection behavior

### 4. TLS Session Ticket Fingerprinting
**Problem:** Session resumption behavior differs between bots and browsers.

**Solution:**
- Proper PSK extension handling
- Session ticket reuse patterns matching Chrome
- Early data (0-RTT) support with correct fingerprint

### 5. QUIC Transport Parameter Fingerprinting
HTTP/3 specific fingerprints:
```go
type QUICFingerprint struct {
    InitialMaxData              uint64
    InitialMaxStreamDataBidiLocal  uint64
    InitialMaxStreamDataBidiRemote uint64
    InitialMaxStreamDataUni     uint64
    InitialMaxStreamsBidi       uint64
    InitialMaxStreamsUni        uint64
    MaxIdleTimeout              time.Duration
    MaxUDPPayloadSize           uint64
    ActiveConnectionIDLimit     uint64
    // ... parameter order matters!
}
```

### 6. Navigator Fingerprint Alignment
**Problem:** TLS/HTTP fingerprint passes but JS fingerprint fails.

**Solution:** Provide aligned values for:
- User-Agent string that matches TLS fingerprint
- Client Hints that match browser profile
- Accept-Language patterns

### 7. Connection Behavior Fingerprinting
- TCP window scaling values
- Keep-alive timing patterns
- Connection reuse behavior
- Retry patterns

### 8. Request Timing Fingerprinting
- Time between requests
- Parallel request patterns
- Resource loading order

### 9. CFFI Bindings (Performance)
Replace IPC with CFFI for Python/Node.js:
- 100-1000x faster than JSON over stdin/stdout
- Pre-compiled binaries for all platforms
- Better error handling

---

## Implementation Priority

### Phase 1: Core Fingerprinting (COMPLETED)
- [x] HTTP/2 Settings order
- [x] HTTP/2 Pseudo-header order
- [x] HTTP/2 Connection flow
- [x] HTTP/2 Priority frames
- [x] HTTP/2 Header priority
- [x] HTTP/2 Header order
- [x] Per-request magic keys
- [x] Custom UserAgent support
- [x] HTTP/2 Stream Priority Tree
- [x] HTTP/2 HPACK Indexing Behavior

### Phase 2: Protocol Coverage (IN PROGRESS)
- [x] HTTP/1.1 header order (sardanioss/http fork)
- [x] HTTP/1.1 Keep-Alive patterns
- [x] HTTP/1.1 Connection reuse behavior
- [ ] HTTP/3 QUIC fingerprinting
- [ ] TLS session resumption fingerprint
- [ ] More browser profiles (50+)

### Phase 3: Intelligence
- [ ] Automatic header coherence
- [ ] Dynamic fingerprint generation
- [ ] Protocol intelligence improvements

### Phase 4: Distribution
- [ ] CFFI bindings
- [ ] Python SDK
- [ ] Node.js SDK

### Phase 5: Advanced
- [ ] Request timing fingerprinting
- [ ] Connection behavior fingerprinting
- [ ] Real browser traffic analysis tools

---

## Files That Need Work

### sardanioss/net (HTTP/2) - DONE
- `http2/transport.go` - ✅ Done
- `http2/hpack/encode.go` - ✅ Done (HPACK indexing)
- `internal/httpcommon/request.go` - ✅ Done
- ✅ Per-request magic key support
- ✅ Stream priority tree
- ✅ HPACK indexing control

### sardanioss/http (HTTP/1.1) - DONE
- `transport.go` - ✅ Done
- ✅ Keep-alive patterns
- ✅ Connection reuse behavior
- ✅ Header ordering (from fhttp)

### New Fork Needed: sardanioss/quic (HTTP/3)
- Fork `quic-go`
- Add transport parameter fingerprinting
- Add QUIC-specific settings

### httpcloak Main Library
- [ ] Remove frame interception code
- [ ] Use sardanioss/net and sardanioss/http instead
- [ ] Add profile system
- [ ] Add auto-coherence features

---

## Chrome 133 Target Fingerprint

```
TLS:
  JA4: t13d1516h2_8daaf6152771_d8a2da3f94cd

HTTP/2:
  Settings: 1:65536;2:0;4:6291456;6:262144
  Window Update: 15663105
  Pseudo-header Order: m,a,s,p
  Header Priority: weight=256, exclusive=1, depends_on=0
  Stream Priority: exclusive on 0, weight 256
  HPACK: Chrome-like indexing
  Akamai Hash: 52d84b11737d980aef856699f885ca86

HTTP/3 (if supported):
  QUIC Version: 1
  Transport Params: [specific order and values TBD]
```

---

## Usage Example (sardanioss libraries)

```go
import (
    http "github.com/sardanioss/http"
    "github.com/sardanioss/net/http2"
    "github.com/sardanioss/net/http2/hpack"
)

// HTTP/1.1 Transport with fingerprinting
h1Transport := &http.Transport{
    KeepAliveMode:       http.KeepAliveModeChrome,
    ConnectionReuseMode: http.ConnectionReuseModeChrome,
}

// HTTP/2 Transport with Chrome 133 fingerprint
h2Transport := &http2.Transport{
    Settings: map[http2.SettingID]uint32{
        http2.SettingHeaderTableSize:   65536,
        http2.SettingEnablePush:        0,
        http2.SettingInitialWindowSize: 6291456,
        http2.SettingMaxHeaderListSize: 262144,
    },
    SettingsOrder: []http2.SettingID{
        http2.SettingHeaderTableSize,
        http2.SettingEnablePush,
        http2.SettingInitialWindowSize,
        http2.SettingMaxHeaderListSize,
    },
    ConnectionFlow: 15663105,
    PseudoHeaderOrder: []string{":method", ":authority", ":scheme", ":path"},
    HeaderPriority: &http2.PriorityParam{
        Weight:    255,
        Exclusive: true,
        StreamDep: 0,
    },
    StreamPriorityMode:  http2.StreamPriorityChrome,
    HPACKIndexingPolicy: hpack.IndexingChrome,
}

// Per-request header ordering (compatible with bogdanfinn/fhttp)
req.Header[http.HeaderOrderKey] = []string{"accept", "user-agent", "accept-language"}
req.Header[http.PHeaderOrderKey] = []string{":method", ":authority", ":scheme", ":path"}
```
