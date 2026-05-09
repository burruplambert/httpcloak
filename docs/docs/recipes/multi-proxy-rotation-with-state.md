---
title: Multi-Proxy Rotation With State
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Multi-Proxy Rotation With State

Rotate through a pool of proxies while keeping the **same** TLS session state.
Your fingerprint stays consistent across rotations, you don't burn through a
fresh handshake every time you swap exits.

## Why session continuity matters

Most rotators throw away the entire client when they switch proxies. They spin
up a new client, do a fresh TCP and TLS handshake, get a brand new session
ticket, and start over. That's fine for low-effort targets, but it leaves a
trail.

What changes between two "fresh" handshakes:

- **TLS extension order** can drift slightly because of GREASE rotation. Same
  preset, same browser version, but bytes on the wire differ.
- **Session tickets** are gone. Returning visitors look very different from
  first-time visitors to a server. If your scraper looks like a first-time
  visitor 500 times in a row, that's a tell.
- **ECH state** resets. If the target uses ECH, you re-fetch the config every
  time.
- **Cookie jar** resets unless you copy it across.
- **Per-connection tracking** like CF's `__cf_bm` cookie ages oddly when you
  hop hosts.

Keeping ONE session and just swapping the proxy under it solves all of that.
The handshake state, the tickets, the cookies, the ECH config, everything
sticks. Only the IP changes.

:::tip
Most residential proxy providers don't care about session continuity. But if
you're hitting a target that does (Cloudflare, anything with session-tracking
on top), this pattern saves you from looking like a fresh user every single
request.
:::

## The pattern

1. Create one `Session` with your preset (e.g. `chrome-latest`).
2. For each request:
   - Pick a proxy from your pool.
   - Call `session.SetTCPProxy(url)` (and `SetUDPProxy` if you're using H3).
   - Send the request.
3. Optionally call `session.Refresh()` to drop the live connections without
   killing tickets or cookies. Next request gets 0-RTT on the new proxy.

That's it. The session keeps every piece of state across rotations.

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

See [/bindings/python](../bindings/python) for the full Python API.

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

What does NOT survive: the live socket itself. That's the point. You want a
new TCP connection through the new proxy IP, but you want to bring all the
fingerprint and cookie state with you.

## Rotation strategies

### Round-robin (simplest)

```go
proxy := proxyPool[i%len(proxyPool)]
```

Cheap, predictable, works for most cases.

### Sticky-by-host

If your scraper hits multiple hosts and you want to keep one proxy per host,
use a small map:

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

Useful when servers correlate the IP a session originated from with later
requests. If you start a CF challenge on IP A and finish it on IP B, that's
suspicious.

### Rotate-on-error

Keep the same proxy until you get a 403 / 429 / connection error, then move
on. Cheaper than rotating every request, and only burns through proxies when
something goes wrong.

```go
err := doRequest(s)
if err != nil || isBadStatus(resp.StatusCode) {
    s.SetTCPProxy(nextProxy())
    s.Refresh()
}
```

## H3 / QUIC notes

If you're using HTTP/3 through a MASQUE proxy, set the UDP proxy too:

```go
s.SetTCPProxy("http://user:pass@http-proxy:8080")
s.SetUDPProxy("masque://user:pass@masque-proxy:443")
```

Most providers only do TCP. If you call `SetTCPProxy` and leave `SetUDPProxy`
empty, H3 falls back to direct UDP, which can leak your real IP. Either set
both or force H1/H2 with `WithForceHTTP2()`.

## Combining with Save / LoadSession

If your run takes hours and you want to survive a process restart:

```go
// Periodically:
s.Save("/var/lib/scraper/state.json")

// On startup:
s, _ := httpcloak.LoadSession("/var/lib/scraper/state.json")
s.SetTCPProxy(currentProxy)
```

Saves the cookie jar, ticket cache, ECH state. Reloads as if you never
stopped. See [Long-Running Scraper Patterns](./long-running-scraper-patterns)
for the full pattern.

## Common mistakes

**Creating a new session per proxy.** This throws away tickets, cookies,
everything. The whole point of this recipe is one session, many proxies.

**Forgetting Refresh().** If you don't call `Refresh()` between requests, the
existing TCP/TLS connection stays open through the OLD proxy, even though
you set a new one. `SetTCPProxy` only affects the NEXT new connection. If
you want the IP change to take effect immediately, call `Refresh()`.

**Mixing UDP and TCP proxies wrong.** H3 needs `SetUDPProxy`. H1/H2 needs
`SetTCPProxy`. If you only set one and your protocol racing picks the other,
you'll bypass the proxy without knowing.

## Related

- [Refresh](../connection-lifecycle/refresh), what `Refresh()` actually does
- [Proxies overview](../proxies/overview), supported proxy types
- [SOCKS5](../proxies/socks5), SOCKS5 specifics
- [MASQUE](../proxies/masque), UDP / H3 proxying
