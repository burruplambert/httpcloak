---
title: Multi-Proxy Rotation With State
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Multi-Proxy Rotation With State

Swap proxies without throwing away your TLS state. Same fingerprint across
rotations, no fresh handshake every time you change exits.

## Why session continuity matters

Most rotators nuke the whole client when they swap proxies. New client, new
TCP and TLS handshake, new session ticket, start over. Fine for soft targets.
Leaves a trail everywhere else.

Here's what dies between two "fresh" handshakes:

- **TLS extension order** drifts a bit because of GREASE rotation. Same
  preset, same browser version, but the bytes on the wire don't quite match.
- **Session tickets** are gone. Returning visitors look very different from
  first-time visitors. Looking like a first-time visitor 500 times in a row
  is a dead giveaway.
- **ECH state** resets. If the target uses ECH, you re-fetch the config from
  zero.
- **Cookie jar** resets unless you copy it over.
- **Per-connection tracking** like CF's `__cf_bm` cookie ages weirdly when
  you hop hosts.

Keep ONE session and just swap the proxy under it. Handshake state, tickets,
cookies, ECH, all sticks. Only the IP changes. Clean.

:::tip
Most residential proxy providers don't care about session continuity. But if
you're hitting Cloudflare or anything with session tracking layered on top,
this pattern keeps you from looking like a brand-new visitor every single
request.
:::

## The pattern

1. Spin up one `Session` with your preset (e.g. `chrome-latest`).
2. For each request:
   - Pick a proxy from your pool.
   - Call `session.SetTCPProxy(url)` (plus `SetUDPProxy` if you're on H3).
   - Send the request.
3. Optional: call `session.Refresh()` to drop live connections without
   killing tickets or cookies. Next request gets 0-RTT on the new proxy.

That's the whole thing. The session keeps every piece of state across
rotations.

## Full example: rotating through 3 proxies

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/sardanioss/httpcloak"
)

// In production, load this from a file or your provider's API.
// We use placeholder URLs here so the example doesn't ship credentials.
var proxyPool = []string{
    "http://user1:pass1@proxy1.example.com:8080",
    "http://user2:pass2@proxy2.example.com:8080",
    "http://user3:pass3@proxy3.example.com:8080",
}

func main() {
    // ONE session for the whole run. Proxy is set per-request below.
    s := httpcloak.NewSession("chrome-latest",
        httpcloak.WithSessionTimeout(30*time.Second),
    )
    defer s.Close()

    targets := []string{
        "https://tls.peet.ws/api/all",
        "https://tls.peet.ws/api/all",
        "https://tls.peet.ws/api/all",
        "https://tls.peet.ws/api/all",
    }

    for i, url := range targets {
        // Round-robin pick. Swap for random / weighted / sticky-by-host
        // depending on what your target wants.
        proxy := proxyPool[i%len(proxyPool)]
        s.SetTCPProxy(proxy)

        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        resp, err := s.Get(ctx, url)
        cancel()
        if err != nil {
            fmt.Printf("[req %d] proxy=%s err=%v\n", i, proxy, err)
            continue
        }
        body, _ := resp.Text()
        resp.Close()
        fmt.Printf("[req %d] proxy=%s status=%d body_len=%d\n",
            i, proxy, resp.StatusCode, len(body))

        // Refresh between requests to drop the live connection.
        // Tickets and cookies survive, next request resumes 0-RTT
        // on whatever proxy is set at that point.
        s.Refresh()
    }
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

PROXY_POOL = [
    "http://user1:pass1@proxy1.example.com:8080",
    "http://user2:pass2@proxy2.example.com:8080",
    "http://user3:pass3@proxy3.example.com:8080",
]

with httpcloak.Session("chrome-latest", timeout=30) as s:
    for i in range(4):
        proxy = PROXY_POOL[i % len(PROXY_POOL)]
        s.set_tcp_proxy(proxy)

        try:
            r = s.get("https://tls.peet.ws/api/all")
            print(f"[req {i}] proxy={proxy} status={r.status_code}")
        except Exception as e:
            print(f"[req {i}] proxy={proxy} err={e}")

        s.refresh()
```

Full Python API lives at [/bindings/python](../bindings/python).

</TabItem>
</Tabs>

## What survives a rotation

After `SetTCPProxy(newProxy)` (with or without `Refresh()`):

| State | Survives? | Notes |
|-------|-----------|-------|
| TLS session tickets | Yes | 0-RTT on next handshake |
| Cookie jar | Yes | Same jar, same cookies |
| ECH config | Yes | No re-fetch needed |
| Header order, preset config | Yes | Session-level, not per-conn |
| HTTP/2 connection | No (after Refresh) | Drops cleanly, reopens on next req |
| HTTP/3 connection | No (after Refresh) | Same |
| TCP socket | No (after Refresh) | Reopens through new proxy |

The live socket is the only thing that dies, and that's the point. You want a
new TCP connection through the new proxy IP, with all the fingerprint and
cookie state riding along.

## Rotation strategies

### Round-robin (simplest)

```go
proxy := proxyPool[i%len(proxyPool)]
```

Cheap, predictable, works for most cases.

### Sticky-by-host

Hitting multiple hosts and want one proxy per host? Use a small map:

```go
hostProxy := map[string]string{}

for _, url := range urls {
    host := parseHost(url)
    if _, ok := hostProxy[host]; !ok {
        hostProxy[host] = pickFromPool()
    }
    s.SetTCPProxy(hostProxy[host])
    // ... send
}
```

Handy when servers correlate the IP a session started on with later requests.
Start a CF challenge on IP A, finish it on IP B, that's a red flag.

### Rotate-on-error

Stick with the same proxy until you get a 403 / 429 / connection error, then
move. Way cheaper than rotating every request, and you only burn proxies when
something actually breaks.

```go
err := doRequest(s)
if err != nil || isBadStatus(resp.StatusCode) {
    s.SetTCPProxy(nextProxy())
    s.Refresh()
}
```

## H3 / QUIC notes

Running HTTP/3 through a MASQUE proxy? Set the UDP proxy too:

```go
s.SetTCPProxy("http://user:pass@http-proxy:8080")
s.SetUDPProxy("masque://user:pass@masque-proxy:443")
```

Most providers only do TCP. If you set `SetTCPProxy` and leave `SetUDPProxy`
empty, H3 quietly falls back to direct UDP, which leaks your real IP. Either
wire both up or force H1/H2 with `WithForceHTTP2()`.

## Combining with Save / LoadSession

For runs that go for hours and need to survive a process restart:

```go
// Periodically:
s.Save("/var/lib/scraper/state.json")

// On startup:
s, _ := httpcloak.LoadSession("/var/lib/scraper/state.json")
s.SetTCPProxy(currentProxy)
```

Saves the cookie jar, ticket cache, ECH state. Reloads like you never
stopped. The full pattern lives in
[Long-Running Scraper Patterns](./long-running-scraper-patterns).

## Common mistakes

**Spinning up a new session per proxy.** Trashes tickets, cookies, the lot.
The whole point of this recipe is one session, many proxies.

**Forgetting Refresh().** Skip `Refresh()` between requests and the existing
TCP/TLS connection keeps chugging along through the OLD proxy, even after you
set a new one. `SetTCPProxy` only kicks in on the NEXT new connection. Want
the IP swap right now? Call `Refresh()`.

**Mixing UDP and TCP proxies.** H3 needs `SetUDPProxy`. H1/H2 needs
`SetTCPProxy`. Set only one and let protocol racing pick the other, and
you'll bypass the proxy without realising.

## Related

- [Refresh](../connection-lifecycle/refresh), what `Refresh()` actually does
- [Proxies overview](../proxies/overview), supported proxy types
- [SOCKS5](../proxies/socks5), SOCKS5 specifics
- [MASQUE](../proxies/masque), UDP / H3 proxying
