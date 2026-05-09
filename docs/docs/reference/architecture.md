---
title: Architecture
sidebar_position: 4
---

# Architecture

The high-level component map of the library. Use this when you're trying to figure out which package owns a behaviour, or when you want to know how a request flows from `Session.Do` to a TLS ClientHello on the wire.

---

## The layering

```
                    +---------------------------------+
   user code  --->  |  httpcloak (root package)       |
                    |  - Client / Session             |
                    |  - WithX option constructors    |
                    |  - Multipart helpers            |
                    +-----------------+---------------+
                                      |
                                      v
                    +---------------------------------+
                    |  session/                       |
                    |  - cookie jar (own + sardanioss)|
                    |  - per-host transport routing   |
                    |  - Refresh / Fork / Warmup      |
                    |  - state save / load            |
                    +-----------------+---------------+
                                      |
              +-----------------------+-----------------------+
              |                       |                       |
              v                       v                       v
      +---------------+       +---------------+       +---------------+
      | HTTP1Transport|       | HTTP2Transport|       | HTTP3Transport|
      | (raw TCP+TLS) |       | (uTLS + h2cc) |       | (quic-go h3)  |
      +---------------+       +---------------+       +---------------+
              |                       |                       |
              v                       v                       v
      +---------------+       +---------------+       +---------------+
      | proxy (HTTP   |       | proxy (HTTP   |       | proxy (UDP    |
      | CONNECT,      |       | CONNECT,      |       | SOCKS5,       |
      | SOCKS5)       |       | SOCKS5)       |       | MASQUE)       |
      +---------------+       +---------------+       +---------------+
              \                       |                      /
               \                      v                     /
                \               +-----+-----+              /
                 +------------> |  network  | <-----------+
                                +-----------+

               +-------------------------------+
   built once, +  fingerprint registry         |
   shared:     |  - Go presets (presets.go)    |
               |  - JSON presets (embedded/)   |
               |  - JA3 / Akamai / TCP fp data |
               +---------------+---------------+
                               |
                               v
               +-------------------------------+
   shared:     |  dns/                         |
               |  - SNI resolution             |
               |  - HTTPS RR / ECH lookup      |
               +-------------------------------+
```

Three transports, one session layer, one fingerprint registry, one DNS layer. The session decides which transport handles a given request based on what the host advertises (Alt-Svc for H3) and what was forced via options.

---

## Packages

### `httpcloak` (root)

The public Go API. `Client`, `Session`, all `WithX` option constructors, multipart helpers. This package is intentionally thin: everything interesting lives in subpackages. Most options just set fields on a `protocol.SessionConfig` that gets handed to `session.NewSession`.

Files: `httpcloak.go`, `local_proxy.go`.

### `session/`

Stateful session. Owns the cookie jar, the per-host transport map, and the lifecycle of `Refresh` / `Fork` / `Warmup` / state persistence.

Notable behaviours:

- One transport per host, lazily created. The first request to `example.com` builds the transport; subsequent requests reuse it.
- `Refresh()` closes connections but keeps the cookie jar and TLS resumption ticket cache. Optionally switches protocol.
- `Fork(n)` returns `n` child sessions sharing the cookie jar and TLS cache but with their own connections. Mimics multiple browser tabs.
- `Warmup(ctx, url)` simulates a real page load: fetches the HTML, parses it, then fetches CSS/JS/image subresources with realistic per-resource priorities and timing.

### `transport/`

The three transports plus the unifying `Transport` wrapper. Each transport handles one protocol family.

- **`HTTP1Transport`**: raw TCP, TLS via uTLS, HTTP/1.1 framing via the `sardanioss/http` fork. Used when a host doesn't speak H2 or H1 is forced.
- **`HTTP2Transport`**: uTLS for ClientHello, then a custom `http2.ClientConn` from the `sardanioss/http` fork. The custom conn is what lets us send Chrome's exact `SETTINGS` order, `PRIORITY` frames, `WINDOW_UPDATE` value, and HPACK indexing policy.
- **`HTTP3Transport`**: `quic-go` from `sardanioss/quic-go` (with the `PRIORITY_UPDATE` fix and Chrome-style initial packet shaping) plus `http3.Transport`.

Other things this package owns:

- **Speculative TLS for proxy CONNECT**: sends the CONNECT request and the TLS ClientHello in a single TCP write, saving one round-trip. Off by default because some proxies choke. Toggled via `WithEnableSpeculativeTLS`.
- **Happy Eyeballs**: iPv4/IPv6 racing in H3 dial functions. The first address that completes the QUIC handshake wins; the loser is closed.
- **`raceH3H2`**: protocol racing. When a host advertises H3 via Alt-Svc but the user hasn't forced a protocol, both an H3 dial and an H2 dial are started in parallel. First successful handshake wins.
- **TLS session ticket cache**: in-memory by default, pluggable via `WithSessionCache(backend, errCb)` for distributed setups (Redis, etc.).
- **Connection pool**: per-transport, with idle timeout. H3 idle is configurable via `WithQuicIdleTimeout`.

### `fingerprint/`

The preset registry and the data structures behind it.

- `presets.go`: Go-defined presets (Chrome 133-146, Firefox 133, Safari 18, iOS, Android variants). Each preset is a `func() *Preset`.
- `embedded/*.json`: JSON-defined presets (Chrome 147, 148 across all platforms). Loaded at package init time and registered alongside the Go presets.
- `custom_preset.go`: JSON parsing + `BuildPreset`. Used both for the embedded JSONs and for user-supplied presets via `LoadPresetFromFile` / `LoadPresetFromJSON`.
- `describe.go`: the inverse of `BuildPreset`. Produces canonical JSON from a `*Preset`. Round-trip stable.
- `client_hello_ids.go`: string-to-uTLS ClientHelloID resolution.
- `akamai.go`: Akamai HTTP/2 fingerprint string parser. The shorthand format `SETTINGS|WINDOW_UPDATE|PRIORITY|PSEUDO` is parsed here.
- `ja3.go`: JA3 string parser and TLS spec builder.
- `headers.go`: header order and value handling.
- `preset_pool.go`: round-robin / random rotation across multiple presets.
- `priority_table_test.go` (and the runtime piece in transport): RFC 9218 priority handling for `sec-fetch-dest` driven priorities.

### `proxy/`

Proxy implementations. Three protocols:

- **HTTP CONNECT**: for `http://` and `https://` proxy URLs. Used by H1 and H2.
- **SOCKS5**: for `socks5://` URLs. Used by H1, H2, and (if the proxy supports UDP-associate) H3.
- **MASQUE**: UDP-tunnelled-over-HTTP/3 proxying. Used for H3 over an HTTP-aware UDP proxy. Implements the CONNECT-UDP method.

Proxy chains are supported (proxy through a proxy). `WithSessionProxy` sets a single URL for all protocols; `WithSessionTCPProxy` + `WithSessionUDPProxy` lets you split (e.g. SOCKS5 for H1/H2, MASQUE for H3).

### `dns/`

DNS resolution layer.

- A/AAAA lookup with caching.
- HTTPS RR lookup for ECH config retrieval. Used by `WithECHFrom` and the default ECH-when-available path.
- Per-resolver overrides (system, DoH, custom).

### `protocol/`

The IPC layer for the daemon binary (`httpcloak-daemon`). Languages other than Go (Python, Node.js, .NET) talk to the daemon over stdin/stdout JSON. Not relevant for direct Go usage, but `protocol/types.go` defines `SessionConfig` which the root package's `NewSession` builds internally.

### `client/`

A second, lower-level client surface that predates the unified root API. Most users should use the root `httpcloak.Client` / `Session` instead. The `client/` package still ships its own `WithX` options for backwards compatibility (e.g. `client.WithPreset`, `client.WithECHConfig`, certificate pinning via `PinCertificate`).

### Other directories

- `bindings/`: language SDKs (Python, Node.js, .NET) that wrap the daemon or link against the C library.
- `pool/`: connection pooling primitives.
- `streaming/`: streaming-body request helpers.
- `extensions/`: uTLS extension shims.

---

## Forked dependencies

httpcloak depends on four forks. Each fork patches the upstream to expose fingerprint-relevant behaviour.

| Fork | Upstream | What we changed |
|---|---|---|
| `sardanioss/http` | `golang.org/x/net/http2` | Custom `http2.ClientConn` that lets us control `SETTINGS` order, `WINDOW_UPDATE`, `PRIORITY` frames, HPACK indexing policy, pseudo-header order, cookie splitting. |
| `sardanioss/utls` | `refraction-networking/utls` | Additional ClientHello presets for newer Chrome / iOS / Safari versions, QUIC ClientHello variants, PSK variants, key share count control. |
| `sardanioss/quic-go` | `quic-go/quic-go` | Chrome-style initial packet shaping, transport parameter order control, `PRIORITY_UPDATE` frame fix, GREASE frame emission. |
| `sardanioss/net` | `golang.org/x/net` | Various small fixes for the `http2` interaction. |

These are pinned in `go.mod`. For local development you can use `go.work` to point at a local checkout of the fork.

---

## Request flow

A typical `Session.Get(ctx, "https://example.com/")` follows this path:

1. **Cookie injection**: `session/` looks up cookies for the URL and adds them as a `Cookie:` header (unless `WithoutCookieJar` is set or the caller passed their own).
2. **Header building**: preset's header order + values + the user's request-level overrides are merged into the final ordered list. `User-Agent`, `sec-ch-ua`, `accept-language`, etc. come from the preset.
3. **Transport selection**: `session/` looks up the per-host transport. If none exists, it builds one based on what the host has advertised (H3 via Alt-Svc cache) or what the user forced.
4. **Connect / dial**: the transport opens a connection if needed. For H1/H2: TCP, then TLS via uTLS with the preset's ClientHello. For H3: UDP, then QUIC handshake with the preset's QUIC ClientHello + transport parameters.
   - With Happy Eyeballs: IPv4 and IPv6 are raced.
   - With protocol racing: H3 and H2 are raced if the host advertises both.
   - With a proxy: CONNECT / SOCKS5 / MASQUE first, then handshake.
5. **DNS / ECH**: handled by `dns/`. ECH HTTPS RR is fetched if not disabled.
6. **Wire send**: the transport writes the request frames (HTTP/2 `HEADERS` + `DATA`, or HTTP/3 equivalent). The frame ordering, settings, priorities all come from the preset.
7. **Response**: read back, parsed, body delivered as `io.ReadCloser`.
8. **Cookie storage**: `Set-Cookie` headers from the response are added to the jar (unless `WithoutCookieJar`).
9. **Redirect**: if the response is 3xx and following is enabled, repeat from step 1 with the new URL.

---

## Threading and concurrency

- `Session` is goroutine-safe for concurrent requests. Internal state (cookie jar, transport map, TLS cache) is mutex-protected.
- `Fork(n)` returns sessions that share the cookie jar mutex with the parent. Concurrent requests across forks contend on the jar but have independent connections.
- Transport `Close()` paths are wrapped in `closeWithTimeout` because QUIC `Close` can block indefinitely on a half-closed UDP socket. This is documented as one of the timeout patterns we keep an eye on.
- Per-request context propagates through to the transport. Cancelling the context cancels the in-flight read / handshake / dial.

---

## Where to look when something's wrong

| Symptom | Look in |
|---|---|
| Wrong JA3 / cipher order | `fingerprint/presets.go` or `fingerprint/embedded/*.json` for the preset's TLS section. Cross-check uTLS ClientHello ID. |
| Wrong HTTP/2 SETTINGS / pseudo order | `fingerprint/presets.go` H2 settings + `chromeH2Config` / `firefoxH2Config` / `safariH2Config`. |
| Wrong header order | Preset's `HeaderOrder` field. Verify against `tls.peet.ws/api/all`. |
| H3 not used | Check the host's Alt-Svc, the preset's `SupportHTTP3`, and whether `WithDisableHTTP3` was called. |
| Proxy doesn't connect | `proxy/`. Check the proxy URL scheme matches what the proxy speaks. |
| Hangs on close | `transport/` close paths. Check the QUIC close timeout wrapping. |
| Timeout not respected | `session/` and `transport/`. Context propagation, deadline computation. |

For everything else: the source is easier to read than this map. Start at `httpcloak.NewSession`, follow the calls.
