---
title: Local Proxy Server
sidebar_position: 5
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Local Proxy Server

You've already got a Python scraper. Or a Node service running on Undici. Or a Playwright setup. Or a .NET app some intern wrote three years ago and nobody wants to touch. Swapping the HTTP client out for a fingerprinted one isn't on the table. `LocalProxy` is the escape hatch: run httpcloak as a tiny HTTP proxy on `127.0.0.1`, point your existing client at it, and every request that goes through gets Chrome-grade TLS fingerprinting on the way out.

It's a drop-in. No SDK install in the target language, no code changes beyond a proxy URL in your client config. Anything that speaks "HTTP proxy" works: `requests`, `fetch`, `curl`, Undici, `HttpClient`, Playwright, your Bash one-liner from 2019.

The combo people actually want this for is Undici and Playwright. Playwright runs a real Chrome, ships legit Chrome cookies, real headers, the lot. But the TLS underneath rides on Undici / Node, which fingerprints as Node and gets flagged anyway. Pretty annoying when you've spent a whole afternoon getting Playwright to look human only for the bot-detection vendor to clock you on the very first byte. Point Playwright at a `LocalProxy` in TLS-only mode and the wire bytes match real Chrome again, your headers stay untouched. Same fix works for any Node app sitting on Undici.

## Quick start

Spin one up. Pick whichever language your control plane lives in:

<Tabs groupId="lang">
<TabItem value="go" label="Go">

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
    if err != nil {
        log.Fatal(err)
    }
    defer lp.Stop()

    log.Printf("listening on :%d", lp.Port())
    select {} // block forever
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
from httpcloak import LocalProxy

proxy = LocalProxy(port=8080, preset="chrome-latest")
print(f"listening on {proxy.proxy_url}")

try:
    input("Press enter to stop...\n")
finally:
    proxy.close()
```

</TabItem>
<TabItem value="node" label="Node.js">

```js
import { LocalProxy } from "httpcloak";

const proxy = new LocalProxy({ port: 8080, preset: "chrome-latest" });
console.log(`listening on ${proxy.proxyUrl}`);

process.on("SIGINT", () => {
    proxy.close();
    process.exit(0);
});
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using HttpCloak;

using var proxy = new LocalProxy(port: 8080, preset: "chrome-latest");
Console.WriteLine($"listening on http://localhost:{proxy.Port}");

Console.WriteLine("Press enter to stop...");
Console.ReadLine();
```

</TabItem>
</Tabs>

Then, from anywhere on the box, point any client at it:

```bash
curl -x http://127.0.0.1:8080 https://tls.peet.ws/api/all
```

The response body holds the JA4, akamai hash, peetprint hash. They'll match real Chrome, not Go's default client. Job done.

:::tip
The proxy binds to `127.0.0.1` only, never `0.0.0.0`. That's deliberate. You don't want a fingerprinting proxy reachable from your LAN by accident. If you need it on a different host, run it inside that host or front it with something like `socat` or an SSH tunnel you actually trust.
:::

## How it works

`LocalProxy` runs two paths inside one server, picked per-request by what the client sends:

- **HTTP-proxy-style request** (`GET http://target/path HTTP/1.1`): the proxy forwards through `Session.DoStream()` and the full TLS+H2 fingerprint stack lights up. This is the path where fingerprinting actually happens.
- **CONNECT tunnel** (`CONNECT target:443 HTTP/1.1`): the proxy opens a raw TCP tunnel to the target. TLS happens between the client and the target. The proxy is just plumbing. Fingerprinting is whatever the client's TLS stack does.

Catch is, every HTTP client out there CONNECT-tunnels for HTTPS by default. Which means by default LocalProxy is just a fancy TCP pipe and your fingerprinting is doing absolutely nothing. There's a header that flips it:

```
X-HTTPCloak-Scheme: https
```

Send an HTTP-proxy-style request with this header on it, and the proxy upgrades the URL to HTTPS internally and runs it through `Session.DoStream()`. Standard HTTP proxy client, real TLS fingerprinting, no CONNECT in the picture. Without this header, your client just CONNECT-tunnels and the bytes on the wire are still your client's TLS stack, not httpcloak's. Pure cosmetics.

Plain HTTP requests (no scheme upgrade) skip Session entirely. They get forwarded by a stock `http.Client` because there's no TLS to fingerprint anyway. Faster, no overhead, no point doing the dance.

## Options

Pass these to `StartLocalProxy(port, opts...)` (Go) or as kwargs to the binding constructors:

| Option | What it does |
| --- | --- |
| `WithProxyPreset(name)` | The fingerprint preset. `chrome-latest`, `firefox-148`, `safari-tp`, etc. Default is `chrome-146`. |
| `WithProxyTimeout(d)` | Per-request timeout. Default 30s. |
| `WithProxyMaxConnections(n)` | Hard cap on concurrent client connections. Anything above gets dropped at accept. Default 1000. |
| `WithProxyUpstream(tcp, udp)` | Chain through an upstream proxy. SOCKS5 or HTTP for `tcp`, MASQUE for `udp`. Both are optional. |
| `WithProxyTLSOnly()` | Skip the preset's HTTP headers. Pass client headers through unchanged. Use when your client already ships authentic browser headers (Playwright, Undici, real browsers driven by CDP). |
| `WithProxySessionCache(backend, errCb)` | Plug in a distributed TLS session ticket cache. Lets multiple LocalProxy instances share resumption state. |

Pass `0` as the port to let the kernel pick one, then read it back with `lp.Port()`.

## Special headers

The proxy reads four request headers to drive per-request behavior. They get stripped before the request goes upstream.

| Header | What it does |
| --- | --- |
| `X-HTTPCloak-Session` | Routes the request through a registered session by ID. See [Session registry](#session-registry) below. |
| `X-HTTPCloak-TlsOnly` | Per-request override of the TLS-only mode. `"true"` skips preset headers, `"false"` applies them, omitting the header uses the proxy's global setting. |
| `X-HTTPCloak-Scheme` | Set to `"https"` to upgrade an HTTP-proxy-style request to HTTPS with full fingerprinting. The trick that gets fingerprinting working from clients that would otherwise CONNECT-tunnel. |
| `X-Upstream-Proxy` | Per-request upstream proxy override (HTTP-proxy-style requests only). |

For HTTPS / CONNECT requests, the upstream-proxy override goes through `Proxy-Authorization` instead, since `X-Upstream-Proxy` won't survive a CONNECT:

```
Proxy-Authorization: HTTPCloak http://user:pass@upstream.example.com:8080
```

The `HTTPCloak` scheme is the per-request signal. The proxy strips this header before forwarding. Real `Basic` / `Bearer` auth headers pass through untouched.

## TLSOnly mode

This is what TLSOnly is for. It's the whole reason the flag exists.

By default, `LocalProxy` slaps the preset's HTTP headers on every request. User-Agent, sec-ch-ua, Accept-Language, the lot. Right move when your client is stock `requests` or `HttpClient` because those clients ship a User-Agent like `python-requests/2.31.0` which would absolutely cook you on any modern target.

Wrong move when your client's already shipping legit Chrome headers. Playwright runs a real Chrome, real Chrome cookies, real Chrome headers in the right order. If you let `LocalProxy` shove its own headers in, you're stepping on actual real-browser headers with preset stand-ins. The fingerprint gets worse, not better. Bruh.

`WithProxyTLSOnly()` switches that off. The proxy keeps its hands off your headers, just passes them straight through, and only fingerprints the TLS layer. Combined with `X-HTTPCloak-Scheme: https`, you get the actual win:

- Playwright's authentic headers, untouched
- httpcloak's TLS handshake on the wire (uTLS, real Chrome cipher list, extension order, the whole shape)

Wire it up like this from a Node service running Undici:

<Tabs groupId="lang">
<TabItem value="node" label="Node.js (Undici)">

```js
import { LocalProxy } from "httpcloak";
import { fetch, ProxyAgent } from "undici";

const proxy = new LocalProxy({ port: 8080, preset: "chrome-latest", tlsOnly: true });
const dispatcher = new ProxyAgent(proxy.proxyUrl);

// Tell the proxy to upgrade this HTTP request to HTTPS with fingerprinting
const r = await fetch("http://tls.peet.ws/api/all", {
    dispatcher,
    headers: { "X-HTTPCloak-Scheme": "https" },
});

console.log((await r.json()).tls.ja4);
proxy.close();
```

Notice the URL is `http://`, not `https://`. That's deliberate. Plain HTTP plus the scheme-upgrade header keeps the request out of CONNECT and into the fingerprinting path. The proxy sees the `https` upgrade and treats the target as HTTPS.

</TabItem>
<TabItem value="playwright" label="Playwright">

```js
import { chromium } from "playwright";
import { LocalProxy } from "httpcloak";

const proxy = new LocalProxy({ port: 8080, preset: "chrome-latest", tlsOnly: true });

const browser = await chromium.launch({
    proxy: { server: `http://localhost:${proxy.port}` },
});
const ctx = await browser.newContext({
    extraHTTPHeaders: { "X-HTTPCloak-Scheme": "https" },
});
const page = await ctx.newPage();
await page.goto("https://tls.peet.ws/api/all");
console.log(await page.content());

await browser.close();
proxy.close();
```

Playwright sets the scheme-upgrade header on every navigation via `extraHTTPHeaders`, then the proxy handles the rest. Real Chrome cookies, real Chrome headers, httpcloak's TLS on the wire.

</TabItem>
<TabItem value="python" label="Python (requests)">

```python
from httpcloak import LocalProxy
import requests

proxy = LocalProxy(port=8080, preset="chrome-latest", tls_only=True)

# requests sends its own User-Agent, but with TLS-only mode, that's what gets used
r = requests.get(
    "http://tls.peet.ws/api/all",
    proxies={"http": proxy.proxy_url, "https": proxy.proxy_url},
    headers={"X-HTTPCloak-Scheme": "https"},
)
print(r.json()["tls"]["ja4"])
proxy.close()
```

</TabItem>
</Tabs>

When NOT to use `TLSOnly`: any time your client doesn't ship authentic browser headers. Stock `requests`, generic Go `net/http`, plain `curl` without `--user-agent`. Those clients leak `python-requests` or `Go-http-client` in their UA and there's no fingerprint trick that saves you from that. Let the preset headers do their job.

## Session registry

Sometimes you want one proxy port to serve different "users", each with their own cookies, IP, and TLS resumption state. That's the registry. Pre-build sessions, register them with an ID, the client picks one per-request via `X-HTTPCloak-Session`.

```go
lp, _ := httpcloak.StartLocalProxy(8080, httpcloak.WithProxyPreset("chrome-latest"))
defer lp.Stop()

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

| Method | What it does |
| --- | --- |
| `RegisterSession(id, *Session) error` | Adds a session under `id`. Errors if `id` is taken. Calls `SetSessionIdentifier(id)` on the session so distributed TLS caches stay isolated per persona. |
| `UnregisterSession(id) *Session` | Removes the session and returns it. Does NOT close it. That's your call, since you might reuse it. |
| `GetSession(id) *Session` | Direct lookup. Returns `nil` if missing. |
| `ListSessions() []string` | All registered IDs. Handy for `/health` endpoints. |

Unknown session IDs return a 400 from the proxy, so typos surface fast instead of silently falling back to the default session.

The cache-key isolation thing matters way more than it sounds. `RegisterSession` calls `session.SetSessionIdentifier(id)` under the hood, which stops alice's TLS tickets from accidentally getting reused on bob's connection (assuming you've wired up the distributed cache, next section). Without the identifier, two sessions hitting the same host collide in the cache and you end up with bob looking like alice on the wire. Which is fine until alice gets banned and suddenly bob's banned too. Each registered session gets its own namespace, no cross-contamination.

:::info
The registry is a routing layer, not a state layer. Sessions you register stay normal `*Session` values. Same options, same cookie jar, same `Refresh()` semantics. You can swap their proxies on the fly with `SetTCPProxy`, the next request through the registry picks up the change.
:::

## Distributed TLS session cache

Run more than one `LocalProxy` (load balancer, horizontal scale, whatever) and each instance starts cold. First request to any new host eats a full TLS handshake. Real Chrome dodges this by caching session tickets locally and resuming with 0-RTT next visit. Single-instance LocalProxy does the same. But the cache lives in process memory, so two instances means two cold starts, three means three. Doesn't scale.

`WithProxySessionCache` plugs in an external backend. All instances share resumption state, the second-instance hit gets 0-RTT just like the first one. Real Chrome behavior across a fleet, basically.

The interface is small. `transport.SessionCacheBackend` only needs a `Get` and `Put`:

```go
type SessionCacheBackend interface {
    Get(key string) ([]byte, bool, error)
    Put(key string, value []byte, ttl time.Duration) error
}
```

A Redis-backed implementation looks like this:

```go
type RedisCache struct {
    client *redis.Client
}

func (r *RedisCache) Get(key string) ([]byte, bool, error) {
    val, err := r.client.Get(context.Background(), key).Bytes()
    if err == redis.Nil {
        return nil, false, nil
    }
    if err != nil {
        return nil, false, err
    }
    return val, true, nil
}

func (r *RedisCache) Put(key string, value []byte, ttl time.Duration) error {
    return r.client.Set(context.Background(), key, value, ttl).Err()
}

// Wire it in:
lp, _ := httpcloak.StartLocalProxy(8080,
    httpcloak.WithProxyPreset("chrome-latest"),
    httpcloak.WithProxySessionCache(&RedisCache{client: redisClient}, func(err error) {
        log.Printf("session cache error: %v", err)
    }),
)
```

The error callback fires whenever the backend coughs (network blip, Redis OOM, whatever). It's advisory, not fatal. The proxy doesn't crash on cache misses, it just logs the error and falls back to a fresh handshake. Pipe the callback into your logger or metrics system and forget it exists.

Cache keys carry the session identifier when set (see registry section above), so a multi-tenant proxy where alice and bob both go through the same Redis stays clean. Their tickets never get crossed.

## Lifecycle and stats

The returned `*LocalProxy` is the whole control surface:

| Method | What it returns / does |
| --- | --- |
| `Stop() error` | Graceful shutdown. Closes the listener, waits up to 10s for in-flight requests, closes the underlying session and idle conns. Idempotent. |
| `Port() int` | The port the proxy actually bound to. Useful when you started with `0`. |
| `IsRunning() bool` | True between successful start and `Stop`. |
| `Stats() map[string]interface{}` | Snapshot. Cheap, no locks held during the call. |

`Stats()` returns:

| Key | Type | Meaning |
| --- | --- | --- |
| `running` | `bool` | Whether the listener is up. |
| `port` | `int` | Bound port. |
| `active_conns` | `int64` | Connections currently being served. |
| `total_requests` | `int64` | Lifetime request count. |
| `preset` | `string` | The preset name. |
| `max_connections` | `int` | The cap from `WithProxyMaxConnections`. |
| `registered_sessions` | `int` | Count of entries in the session registry. |

Wire `Stop()` into your shutdown handler, scrape `Stats()` into Prometheus on a 15-second interval, you're set. The Node and .NET bindings expose the same surface as `getStats()` and `GetStats()` returning a typed object.

## Multi-proxy pattern

You can run multiple `LocalProxy` instances in the same process, different ports, each pinned to a different browser. Comes in handy when one app talks to two targets that expect different browsers, or when you want to A/B fingerprints behind a feature flag without redeploying.

```go
chrome, _ := httpcloak.StartLocalProxy(8080, httpcloak.WithProxyPreset("chrome-latest"))
firefox, _ := httpcloak.StartLocalProxy(8081, httpcloak.WithProxyPreset("firefox-148"))
safari, _ := httpcloak.StartLocalProxy(8082, httpcloak.WithProxyPreset("safari-18"))
defer chrome.Stop()
defer firefox.Stop()
defer safari.Stop()
```

Then route from the client by port:

```python
import requests

CHROME = "http://127.0.0.1:8080"
FIREFOX = "http://127.0.0.1:8081"

# Chrome for the API
api = requests.get("https://api.example.com/...", proxies={"https": CHROME})

# Firefox for the legacy site that hates Chrome
legacy = requests.get("https://legacy.example.com/...", proxies={"https": FIREFOX})
```

Each instance is its own thing. Own connection pool, own cookies, own stats counter. Three proxies cost roughly 3x the memory of one and one extra goroutine per accept loop, which is nothing at proxy scale. Don't lose sleep over it.

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

For the Undici / Playwright drop-in path with `TLSOnly`, see the [TLSOnly mode](#tlsonly-mode) section above.

## What's next

- [Multi-Proxy Rotation With State](./multi-proxy-rotation-with-state): rotate IPs under a single registered session without burning tickets.
- [Long-Running Scraper Patterns](./long-running-scraper-patterns): lifecycle and refresh strategies that play nicely with the registry.
