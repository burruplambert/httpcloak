# httpcloak Improvements Roadmap

This document outlines all identified issues and planned improvements for the httpcloak library, based on comparison with bogdanfinn/tls-client and analysis of why Akamai and other bot detection systems may block httpcloak requests.

---

## Critical Issues (Causing Detection)

### 1. Using Pre-built uTLS ClientHello Specs

**Problem:** httpcloak uses pre-defined `tls.HelloChrome_133` from `github.com/refraction-networking/utls` instead of custom-built `ClientHelloSpec`.

**Current Code (fingerprint/presets.go:119):**
```go
ClientHelloID: tls.HelloChrome_133,  // Relies on library's pre-built spec
```

**Why It Fails:**
- The upstream utls library's specs may not exactly match real browser traffic
- Extension order, GREASE placement, and cipher suite order may be slightly off
- No control over exact TLS fingerprint details

**Solution:**
- Fork utls (like bogdanfinn did)
- Create custom `ClientHelloSpec` with exact extension order matching real browsers
- Define specs using `SpecFactory` pattern for full control

**Reference (tls-client approach):**
```go
SpecFactory: func() (tls.ClientHelloSpec, error) {
    return tls.ClientHelloSpec{
        CipherSuites: []uint16{
            tls.GREASE_PLACEHOLDER,
            tls.TLS_AES_128_GCM_SHA256,
            // ... exact order
        },
        Extensions: []tls.TLSExtension{
            &tls.UtlsGREASEExtension{},
            &tls.SessionTicketExtension{},
            // ... exact order matching real Chrome
        },
    }, nil
}
```

---

### 2. HTTP/2 SETTINGS Frame Sends Wrong Settings

**Problem:** httpcloak sends `MAX_CONCURRENT_STREAMS` in SETTINGS frame, but Chrome doesn't send this setting.

**Current Code (transport/http2_custom.go:185-187):**
```go
// MAX_CONCURRENT_STREAMS (3) - Chrome sends 0 (no limit)
binary.Write(&payload, binary.BigEndian, uint16(settingMaxConcurrentStreams))
binary.Write(&payload, binary.BigEndian, settings.MaxConcurrentStreams)
```

**Why It Fails:**
- Real Chrome 133 sends only: `HEADER_TABLE_SIZE`, `ENABLE_PUSH`, `INITIAL_WINDOW_SIZE`, `MAX_HEADER_LIST_SIZE`
- Sending extra settings is a fingerprint
- Akamai checks exact SETTINGS frame content

**Solution:**
- Remove `MAX_CONCURRENT_STREAMS` from SETTINGS frame
- Only send the 4 settings Chrome actually sends
- Make settings list configurable per preset

**Correct Chrome 133 SETTINGS:**
```
HEADER_TABLE_SIZE: 65536
ENABLE_PUSH: 0
INITIAL_WINDOW_SIZE: 6291456
MAX_HEADER_LIST_SIZE: 262144
```

---

### 3. HTTP/2 SETTINGS Order Not Guaranteed

**Problem:** While httpcloak hardcodes order, the preset struct doesn't enforce it, and future changes could break it.

**Current Code:** Settings written in hardcoded order in `buildCustomSettingsFrame()`

**Solution:**
- Add `SettingsOrder []uint16` field to `HTTP2Settings` struct
- Write settings in the order specified by the preset
- Match tls-client's `settingsOrder` pattern

**New Struct:**
```go
type HTTP2Settings struct {
    HeaderTableSize        uint32
    EnablePush             bool
    MaxConcurrentStreams   uint32
    InitialWindowSize      uint32
    MaxFrameSize           uint32
    MaxHeaderListSize      uint32
    ConnectionWindowUpdate uint32
    StreamWeight           uint16
    StreamExclusive        bool
    SettingsOrder          []uint16  // NEW: Explicit order
}
```

---

### 4. HPACK State Corruption from Frame Interception

**Problem:** httpcloak intercepts HTTP/2 frames and re-encodes headers using a fresh HPACK decoder each time, breaking compression state.

**Current Code (transport/http2_custom.go:250):**
```go
decoder := hpack.NewDecoder(65536, nil)  // New decoder per frame!
headers, err := decoder.DecodeFull(headerBlock)
```

**Why It Fails:**
- HPACK uses stateful compression with a dynamic table
- Encoder and decoder must share synchronized state
- Creating new decoder per frame corrupts the table
- May cause subtle protocol violations that Akamai detects

**Solution Options:**
1. **Option A:** Fork golang.org/x/net/http2 to support custom settings natively (like bogdanfinn/fhttp)
2. **Option B:** Maintain persistent HPACK encoder/decoder state per connection
3. **Option C:** Don't intercept HEADERS frames, only SETTINGS and WINDOW_UPDATE

---

### 5. Missing PSK (Pre-Shared Key) Extension Support

**Problem:** httpcloak doesn't properly support TLS session resumption with PSK.

**Why It Matters:**
- Chrome uses PSK for faster reconnections
- Missing PSK extension in ClientHello is a fingerprint
- tls-client has separate `Chrome_133_PSK` profile with `&tls.UtlsPreSharedKeyExtension{}`

**Solution:**
- Add PSK profiles for each browser
- Include `UtlsPreSharedKeyExtension` in appropriate specs
- Ensure session cache properly stores PSK data

---

## High Priority Issues

### 6. Limited Browser Profiles

**Problem:** httpcloak has only 7 profiles vs tls-client's 50+

**Current Profiles:**
- chrome-131, chrome-133, chrome-141, chrome-143, chrome-143-windows
- firefox-133
- safari-18

**Missing Profiles:**
- Older Chrome versions (103-130)
- Older Firefox versions (102-132)
- Safari iOS versions
- Opera
- Android OkHttp profiles
- Custom app profiles (Zalando, Nike, etc.)

**Solution:**
- Add more browser profiles
- Support mobile browser fingerprints
- Allow custom profile creation

---

### 7. No Random TLS Extension Order Option

**Problem:** tls-client has `WithRandomTLSExtensionOrder()` for additional fingerprint variation.

**Why It Matters:**
- Static extension order can be fingerprinted across requests
- Real browsers may have slight variations
- Randomization (within valid bounds) helps avoid detection

**Solution:**
- Add `RandomExtensionOrder` option to client config
- Implement extension shuffling (keeping GREASE and padding fixed)

---

### 8. No Custom Dialer Support

**Problem:** httpcloak doesn't allow custom dialers for advanced networking scenarios.

**Use Cases:**
- Custom DNS resolvers
- DNS-over-HTTPS
- Custom routing
- Network debugging

**Solution:**
- Add `WithDialer(net.Dialer)` option
- Add `WithProxyDialerFactory()` for custom proxy implementations

---

### 9. No IPV4/IPV6 Selection

**Problem:** Cannot force IPv4-only or IPv6-only connections.

**Solution:**
- Add `WithDisableIPV4()` and `WithDisableIPV6()` options
- Useful for specific network configurations

---

## Medium Priority Issues

### 10. IPC Instead of CFFI for Multi-Language Support

**Problem:** httpcloak uses JSON-over-stdin/stdout IPC which is slower than CFFI.

**Performance Impact:**
- IPC: ~1-10ms overhead per request
- CFFI: ~1-10μs overhead per request (100-1000x faster)

**Solution:**
- Implement CFFI bindings using `//export` and CGO
- Build shared libraries for each platform
- Create Python/Node.js SDK wrappers

---

### 11. No Bandwidth Tracking

**Problem:** tls-client has built-in bandwidth tracking, httpcloak doesn't.

**Solution:**
- Add optional bandwidth tracker
- Track bytes sent/received per request and session

---

### 12. No Connect Headers for Proxy

**Problem:** Cannot set custom headers for CONNECT request to proxy.

**Solution:**
- Add `WithConnectHeaders(http.Header)` option
- Useful for proxy authentication variations

---

### 13. No Local Address Binding

**Problem:** Cannot bind to specific local IP address.

**Use Cases:**
- Multi-IP servers
- Rotating source IPs
- Network interface selection

**Solution:**
- Add `WithLocalAddr(net.TCPAddr)` option

---

### 14. No Server Name Overwrite

**Problem:** Cannot override SNI (Server Name Indication) in TLS handshake.

**Solution:**
- Add `WithServerNameOverwrite(string)` option
- Useful for domain fronting and specific testing scenarios

---

### 15. No Key Log Writer for Debugging

**Problem:** Cannot export TLS master secrets for Wireshark decryption.

**Solution:**
- Add `WithKeyLogWriter(io.Writer)` option in TransportOptions
- Useful for debugging TLS issues

---

### 16. No Panic Recovery Option

**Problem:** Panics in request handling crash the application.

**Solution:**
- Add `WithCatchPanics()` option
- Recover from panics and return error instead

---

## Low Priority Issues

### 17. No Charles Proxy Helper

**Problem:** No convenience function for Charles proxy debugging.

**Solution:**
- Add `WithCharlesProxy(host, port string)` helper

---

### 18. No Custom Redirect Function

**Problem:** Cannot implement custom redirect logic.

**Solution:**
- Add `WithCustomRedirectFunc()` option

---

## Implementation Priority Order

### Phase 1: Critical Fixes (Detection Issues)
1. [ ] Fork utls and create custom ClientHelloSpec
2. [ ] Fix HTTP/2 SETTINGS frame (remove MAX_CONCURRENT_STREAMS)
3. [ ] Add explicit settings order to presets
4. [ ] Fix HPACK state management or use fhttp fork

### Phase 2: Profile Expansion
5. [ ] Add PSK variants for all profiles
6. [ ] Add more browser profiles (Chrome 103-130, Firefox 102-132)
7. [ ] Add mobile browser profiles

### Phase 3: Feature Parity
8. [ ] Add random TLS extension order option
9. [ ] Add custom dialer support
10. [ ] Add IPV4/IPV6 selection
11. [ ] Implement CFFI bindings

### Phase 4: Polish
12. [ ] Add bandwidth tracking
13. [ ] Add connect headers option
14. [ ] Add local address binding
15. [ ] Add remaining convenience options

---

## Testing Strategy

After each fix, verify against:
1. https://tls.peet.ws/api/all - Compare JA3, JA4, Akamai hash
2. Actual Akamai-protected sites
3. Cloudflare-protected sites
4. PerimeterX-protected sites

Compare fingerprints with:
- Real Chrome browser
- tls-client library
- curl with correct flags

---

## References

- [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) - Reference implementation
- [bogdanfinn/utls](https://github.com/bogdanfinn/utls) - Forked uTLS with custom specs
- [bogdanfinn/fhttp](https://github.com/bogdanfinn/fhttp) - Forked http with HTTP/2 fingerprint support
- [refraction-networking/utls](https://github.com/refraction-networking/utls) - Original uTLS
- [tls.peet.ws](https://tls.peet.ws) - TLS fingerprint testing
