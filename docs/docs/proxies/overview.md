---
title: Overview
sidebar_position: 2
---

# Overview

httpcloak speaks four proxy protocols. Pick the one that matches your
upstream and the kind of traffic you're routing.

## What we support

| Type | URL scheme | Carries | Auth | Best for |
|---|---|---|---|---|
| HTTP CONNECT | `http://`, `https://` | TCP (HTTP/1.1, HTTP/2) | Basic | The classic. Datacenter proxies, corporate egress, anything that fronts as a normal HTTP proxy. |
| SOCKS5 | `socks5://`, `socks5h://` | TCP (HTTP/1.1, HTTP/2) | none, user/pass | Residential providers default to this. BrightData, Smartproxy, Oxylabs, SOAX. |
| SOCKS5 with UDP ASSOCIATE | `socks5://` (UDP slot) | UDP (HTTP/3 over QUIC) | none, user/pass | Routing H3 through a SOCKS5 server that supports UDP relay. Less common, not every provider does it. |
| MASQUE (CONNECT-UDP) | `masque://`, `https://` | UDP inside HTTP/3 | Basic | The newer one. Tunnels QUIC inside QUIC. Cloudflare WARP and a few residential providers ship this for H3. |

If you've never thought about which to pick: HTTP CONNECT for HTTP/1.1
and HTTP/2, SOCKS5 if your residential provider gave you a SOCKS5
endpoint, and add a UDP proxy (SOCKS5 UDP or MASQUE) only when you
actually want HTTP/3 to go through the proxy.

## Split-config: TCP and UDP separately

httpcloak lets you configure the TCP proxy and the UDP proxy
independently. Two options, applied to the same session:

- `WithSessionTCPProxy(url)` sends HTTP/1.1 and HTTP/2 through this proxy
- `WithSessionUDPProxy(url)` sends HTTP/3 (QUIC) through this proxy

You'd want this when your TCP proxy doesn't speak UDP. Common patterns:

```go
// HTTP CONNECT for H1/H2 + MASQUE for H3
s := httpcloak.NewSession("chrome-latest",
    httpcloak.WithSessionTCPProxy("http://user:pass@proxy.example.com:8080"),
    httpcloak.WithSessionUDPProxy("masque://user:pass@proxy.example.com:443"),
)

// SOCKS5 for everything (only works if SOCKS5 server supports UDP ASSOCIATE)
s := httpcloak.NewSession("chrome-latest",
    httpcloak.WithSessionTCPProxy("socks5://user:pass@proxy.example.com:1080"),
    httpcloak.WithSessionUDPProxy("socks5://user:pass@proxy.example.com:1080"),
)

// HTTP CONNECT only, kill H3 because no UDP route
s := httpcloak.NewSession("chrome-latest",
    httpcloak.WithSessionTCPProxy("http://user:pass@proxy.example.com:8080"),
    httpcloak.WithDisableHTTP3(),
)
```

If you set only `WithSessionTCPProxy` and don't set a UDP proxy, H3 will
try to dial directly. Most of the time that's fine. If your network
blocks outbound UDP/443, pair it with `WithDisableHTTP3()` so the
session sticks to H1/H2.

There's also `WithSessionProxy(url)` which is the old single-proxy
option. It still works and applies the URL to both TCP and UDP slots
based on scheme. New code should prefer the split form, it's clearer
about what's going where.

## Auth

Auth lives in the URL. Both forms work:

- `http://user:pass@host:port`
- `socks5://user:pass@host:port`
- `masque://user:pass@host:port`

For HTTP CONNECT and MASQUE, this turns into a `Proxy-Authorization:
Basic <base64>` header. For SOCKS5 it goes through RFC 1929 username /
password sub-negotiation. URL-encode the password if it has special
chars, that's standard URL parsing.

## Source-address binding

Independent of any proxy choice you can also pin every dial to a
specific local IP. Use `WithLocalAddress("203.0.113.10")` or
`WithLocalAddrIP(net.IP)` and httpcloak will bind every outgoing socket
(direct or through a proxy) to that address. On Linux it sets
`IP_FREEBIND` so addresses from a routed prefix that aren't on any
interface still work without `CAP_NET_ADMIN`.

See [Source Address Binding](./source-address-binding) for the full
shape, IPv6 prefix rotation tricks, and platform notes.

## Picking the right one

A rough decision tree:

- Datacenter or corporate proxy gave you a host:port and HTTP basic
  auth: HTTP CONNECT.
- Residential provider gave you a SOCKS5 endpoint:
  [SOCKS5](./socks5).
- Same residential provider, you want H3 to also go through the proxy
  and they advertise UDP support: [SOCKS5 UDP](./socks5-udp).
- You want H3 traffic specifically tunneled through Cloudflare WARP or
  a custom MASQUE server: [MASQUE](./masque).
- You want a single source IP no matter the proxy choice:
  [Source Address Binding](./source-address-binding).

## Bindings

Same options exist in Python (`tcp_proxy=`, `udp_proxy=`,
`local_address=`), Node.js (`tcpProxy`, `udpProxy`, `localAddress`),
and .NET (`tcpProxy:`, `udpProxy:`, `localAddress:`). The chapters
that follow show 4-language tabs where the API differs.
