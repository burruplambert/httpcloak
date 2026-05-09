---
title: Local Proxy Server
sidebar_position: 5
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Local Proxy Server

You've already got a Python scraper. Or a Node service. Or a .NET app some intern wrote three years ago and nobody wants to touch. Swapping the HTTP client out for a fingerprinted one isn't on the table. `LocalProxy` is the escape hatch: run httpcloak as a tiny HTTP proxy on `127.0.0.1`, point your existing client at it, and every request that goes through gets Chrome-grade TLS and H2 fingerprinting on the way out.

It's a drop-in. No SDK install in the target language, no code changes beyond a proxy URL in your client config. Anything that speaks "HTTP proxy" works: `requests`, `fetch`, `curl`, Undici, `HttpClient`, Playwright, your Bash one-liner from 2019.

## Quick start

Spin one up in Go and curl it:

```go
package main

import (
    "log"
    "github.com/sardanioss/httpcloak"
)

func main() {
    lp, err := httpcloak.StartLocalProxy(8080,
        httpcloak.WithProxyPreset("chrome-latest"),
    )
    if err != nil { log.Fatal(err) }
    defer lp.Stop()

    log.Printf("listening on :%d", lp.Port())
    select {} // block forever
}
```

Then, from anywhere on the box:

```bash
curl -x http://127.0.0.1:8080 https://tls.peet.ws/api/all
```

The response body holds the JA4, JA3, akamai hash, peetprint hash. They'll match real Chrome, not Go's default client. Job done.

:::tip
The proxy binds to `127.0.0.1` only, never `0.0.0.0`. That's deliberate. You don't want a fingerprinting proxy reachable from your LAN by accident. If you need it on a different host, run it inside that host or front it with something like `socat` or an SSH tunnel you actually trust.
:::

## Options

Pass these to `StartLocalProxy(port, opts...)`:

| Option | What it does |
| --- | --- |
| `WithProxyPreset(name)` | Pick the fingerprint preset (e.g. `chrome-latest`, `firefox-148`, `safari-tp`). Default is `chrome-146`. |
| `WithProxyTimeout(d)` | Per-request timeout. Default 30s. |
| `WithProxyMaxConnections(n)` | Hard cap on concurrent client connections. New ones get dropped above the cap. Default 1000. |
| `WithProxyUpstream(tcp, udp)` | Chain through an upstream proxy (SOCKS5 or HTTP). UDP arg is for H3 / QUIC. |
| `WithProxyTLSOnly()` | Apply TLS+H2 fingerprinting but don't rewrite HTTP headers. Use this when your client already sends authentic browser headers (Playwright, real browsers driven by CDP). |
| `WithProxySessionCache(backend, errCb)` | Plug in a distributed TLS session ticket cache so multiple proxy instances share resumption state. |

Pass `0` as the port to let the kernel pick one, then read it back with `lp.Port()`.

## Lifecycle

The returned `*LocalProxy` is the whole control surface:

| Method | Returns |
| --- | --- |
| `Stop() error` | Graceful shutdown. Closes the listener, waits up to 10s for in-flight requests, closes the underlying session and idle conns. Idempotent. |
| `Port() int` | The port the proxy actually bound to. Useful when you started with `0`. |
| `IsRunning() bool` | True between successful `StartLocalProxy` and `Stop`. |
| `Stats() map[string]interface{}` | Snapshot: `running`, `port`, `active_conns`, `total_requests`, `preset`, `max_connections`, `registered_sessions`. Cheap, no locks held during the call. |

Wire it into your shutdown handler, scrape `Stats()` into Prometheus, you're set.

## Session registry: one proxy, many fingerprints

Sometimes you want the same proxy port to serve different "users", each with their own cookies, IP, and TLS state. That's what the registry is for. You pre-build sessions, register them with an ID, and the client picks one per-request via the `X-HTTPCloak-Session` header.

```go
lp, _ := httpcloak.StartLocalProxy(8080, httpcloak.WithProxyPreset("chrome-latest"))
defer lp.Stop()

// One session per persona, each with its own proxy and cookie jar
alice := httpcloak.NewSession("chrome-latest",
    httpcloak.WithSessionTCPProxy("socks5://user1:pass@residential.example:1080"))
bob := httpcloak.NewSession("firefox-148",
    httpcloak.WithSessionTCPProxy("socks5://user2:pass@residential.example:1080"))

lp.RegisterSession("alice", alice)
lp.RegisterSession("bob", bob)
```

From the client side, just set the header:

```bash
curl -x http://127.0.0.1:8080 \
     -H "X-HTTPCloak-Session: alice" \
     https://example.com/profile
```

Registry methods:

| Method | What it does |
| --- | --- |
| `RegisterSession(id, *Session) error` | Adds a session under `id`. Errors if the ID already exists. Sets the session's identifier so distributed TLS caches stay isolated per persona. |
| `UnregisterSession(id) *Session` | Removes the session and returns it. Does NOT close it: that's your call, since you might reuse it. |
| `GetSession(id) *Session` | Direct lookup. Returns `nil` if missing. |
| `ListSessions() []string` | All registered IDs. Handy for `/health` endpoints. |

Unknown session IDs return a 400 from the proxy, so typos surface fast instead of silently falling back to the default session.

:::info
The registry is just a routing layer. The sessions you register are normal `*Session` values: same options, same cookie jar, same `Refresh()` semantics. You can swap their proxies on the fly with `SetTCPProxy`, and the next request through the registry picks up the change.
:::

## Hitting the proxy from any language

The whole point is that you don't need an httpcloak SDK in the calling language. Standard HTTP-proxy config does it.

<Tabs groupId="lang">
<TabItem value="python" label="Python">

```python
import requests

proxies = {
    "http":  "http://127.0.0.1:8080",
    "https": "http://127.0.0.1:8080",
}

# Per-request session pick
headers = {"X-HTTPCloak-Session": "alice"}

r = requests.get("https://tls.peet.ws/api/all", proxies=proxies, headers=headers)
print(r.json()["tls"]["ja4"])
```

</TabItem>
<TabItem value="node" label="Node.js">

```js
import { fetch, ProxyAgent } from "undici";

const dispatcher = new ProxyAgent("http://127.0.0.1:8080");

const r = await fetch("https://tls.peet.ws/api/all", {
  dispatcher,
  headers: { "X-HTTPCloak-Session": "alice" },
});

console.log((await r.json()).tls.ja4);
```

</TabItem>
<TabItem value="go" label="Go">

```go
proxyURL, _ := url.Parse("http://127.0.0.1:8080")
client := &http.Client{
    Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
}
req, _ := http.NewRequest("GET", "https://tls.peet.ws/api/all", nil)
req.Header.Set("X-HTTPCloak-Session", "alice")
resp, _ := client.Do(req)
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using System.Net;
using System.Net.Http;

var handler = new HttpClientHandler {
    Proxy = new WebProxy("http://127.0.0.1:8080"),
    UseProxy = true,
};
using var client = new HttpClient(handler);
client.DefaultRequestHeaders.Add("X-HTTPCloak-Session", "alice");

var r = await client.GetStringAsync("https://tls.peet.ws/api/all");
Console.WriteLine(r);
```

</TabItem>
<TabItem value="curl" label="curl">

```bash
curl -x http://127.0.0.1:8080 \
     -H "X-HTTPCloak-Session: alice" \
     https://tls.peet.ws/api/all
```

</TabItem>
</Tabs>

For HTTPS targets the proxy uses `Session.DoStream()` under the hood, so you get the full fingerprint stack: TLS, H2 SETTINGS, pseudo-header order, the works. For plain HTTP it just forwards fast since there's no TLS to fingerprint anyway.

## What's next

- [Multi-Proxy Rotation With State](./multi-proxy-rotation-with-state): rotate IPs under a single registered session without burning tickets.
- [Long-Running Scraper Patterns](./long-running-scraper-patterns): lifecycle and refresh strategies that play nicely with the registry.
