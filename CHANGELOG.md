# Changelog

## 1.6.0-beta.13 (2026-02-15)

### New Features

- **`session.Fork(n)`** — Create N sessions sharing cookies and TLS session caches but with independent connections. Simulates multiple browser tabs from the same browser for parallel scraping. Available in Go, Python, Node.js, and C#.

- **`session.Warmup(url)`** — Simulate a real browser page load by fetching HTML and all subresources (CSS, JS, images, fonts) with realistic headers, priorities, and timing. Populates TLS session tickets, cookies, and cache headers before real work begins. Available in Go, Python, Node.js, and C#.

- **Speculative TLS** — Sends CONNECT + TLS ClientHello together on proxy connections, saving one round-trip (~25% faster proxy handshakes). Enabled by default; can be disabled with `disable_speculative_tls` for incompatible proxies.

- **`switch_protocol` on Refresh()** — Switch HTTP protocol version (h1/h2/h3) when calling `Refresh()`, persisting for future refreshes.

- **`-latest` preset aliases** — `chrome-latest`, `firefox-latest`, `safari-latest` aliases that automatically resolve to the newest preset version.

- **`available_presets()` returns dict** — Now returns a dict with protocol support info (`{name: {h1, h2, h3}}`) instead of a flat list.

- **Auto Content-Type for JSON POST** — Automatically sets `Content-Type: application/json` when body is a JSON object/dict.

- **C# CancellationToken support** — Native Go context cancellation for C# async methods.

- **C# Session finalizer** — Prevents Go session leaks when `Dispose()` is missed.

- **Parallel DNS + ECH resolution** — DNS and ECH config resolution parallelized in SOCKS5 proxy QUIC dial path.

### Bug Fixes

#### Transport Reliability
- Fix H2 head-of-line blocking: release `connsMu` during TCP+TLS dial so other requests aren't blocked
- Fix H2 cleanup killing long-running requests by adding in-flight request counter
- Fix H2 per-address dial timeout using `min(remaining_budget/remaining_addrs, 10s)`
- Fix H1 POST body never sent when preset header order omits `Content-Length`
- Fix H1 connection returned to pool before body is fully drained
- Fix H1 deadline cleared while response body still being read
- Fix H3 UDP fallback and narrow 0-RTT early data check
- Fix H3 GREASE ID/value and QPACK capacity drift in `Refresh()`/`recreateTransport()`
- Fix speculative TLS causing 30s body read delay on HTTP/2 connections
- Fix speculative TLS blocklist key mismatch in H1 and H2
- Fix `bufio.Reader` data loss in proxy CONNECT for H1 and H2
- Fix corrupted pool connections, swallowed flush errors, nil-proxy guards
- Fix case-sensitive `Connection` header, H2 cleanup race, dead MASQUE code
- Fix nil-return on UDP failure and stale H2 connection entry

#### Proxy & QUIC
- Fix `quic.Transport` goroutine leak in SOCKS5 H3 proxy path
- Auto-cleanup proxy QUIC resources when connection dies
- Replace `SOCKS5UDPConn` with `udpbara` for H3 proxy transport
- Fix proxy CONNECT deadline to respect context timeout in H1 and H2

#### Session & Config
- Fix `verify: false` not disabling TLS certificate validation
- Fix `connect_to` domain fronting connection pool key sharing
- Fix POST payload encoding: use `UnsafeRelaxedJsonEscaping` for all JSON serialization

#### Resource Leaks
- Fix resource leaks and race conditions across all HTTP transports (comprehensive audit)

### Internal
- 8 timeout bugs fixed where context cancellation/deadline was ignored across all transports
- `wg.Wait()` in goroutines now uses channel+select on `ctx.Done()`
- `time.Sleep()` in goroutines replaced with `select { case <-time.After(): case <-ctx.Done(): }`
- `http.ReadResponse()` on proxy connections now sets `conn.SetReadDeadline()`
- QUIC transport `Close()` wrapped in `closeWithTimeout()` in both `Refresh()` and `Close()` paths

---

## 1.5.10 (2025-12-18)

### Features at v1.5.10
- HTTP/1.1, HTTP/2, HTTP/3 with accurate TLS fingerprints (uTLS)
- ECH (Encrypted Client Hello) support
- 0-RTT session resumption with TLS session ticket persistence
- H3/H2 connection racing for automatic protocol fallback
- SOCKS5, HTTP, and MASQUE proxy support (including H3 over SOCKS5)
- Domain-scoped cookie jar with full Set-Cookie parsing
- Session persistence (save/load/marshal/unmarshal)
- Streaming downloads and uploads
- Fast-path zero-copy APIs for high-throughput transfers
- LocalProxy for transparent `HttpClient`/`fetch` integration
- Distributed TLS session cache backend (Redis, etc.)
- Runtime proxy switching
- Header order customization
- Domain fronting via `connect_to`
- Local address binding (IPv6 rotation)
- TLS key logging (Wireshark)
- `Refresh()` for browser page refresh simulation
- Async session cache support
- Chrome, Firefox, Safari, iOS, and Android presets
- Python, Node.js, C#, and Go bindings
