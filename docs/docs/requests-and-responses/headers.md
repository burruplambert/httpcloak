---
title: Headers
sidebar_position: 2
---

# Headers

Header order matters. Not just the values, the **order** they hit the wire.

Chrome ships its headers in a fixed sequence: `sec-ch-ua` first, then `sec-ch-ua-mobile`, then `sec-ch-ua-platform`, then `upgrade-insecure-requests`, then `user-agent`, then `accept`, and so on down the list. That sequence is part of your fingerprint. Anti-bot vendors hash it. Send the same headers in a different order (or add one Chrome wouldn't, or skip one Chrome always sends) and you stand out.

httpcloak bakes the canonical order into each preset. Your custom headers slot into preset-reserved positions so adding `Authorization` or `X-Anything-Custom` doesn't blow up the fingerprint.

:::tip
DevTools won't show you header order, so you're flying blind there. Hit [tls.peet.ws/api/all](https://tls.peet.ws/api/all) and check the `http2.sent_frames[].headers` array. That's the actual wire order.
:::

## What ships by default

Every preset carries its own browser header set. For `chrome-148-linux` (today's default), the request goes out as:

| Position | Header | Example value |
|---|---|---|
| 1 | `sec-ch-ua` | `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"` |
| 2 | `sec-ch-ua-mobile` | `?0` |
| 3 | `sec-ch-ua-platform` | `"Linux"` |
| 4 | `upgrade-insecure-requests` | `1` |
| 5 | `user-agent` | `Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 ...` |
| 6 | `accept` | `text/html,application/xhtml+xml,...` |
| 7 | `sec-fetch-site` | `none` |
| 8 | `sec-fetch-mode` | `navigate` |
| 9 | `sec-fetch-user` | `?1` |
| 10 | `sec-fetch-dest` | `document` |
| 11 | `accept-encoding` | `gzip, deflate, br, zstd` |
| 12 | `accept-language` | `en-US,en;q=0.9` |
| 13 | `priority` | `u=0, i` |

Different presets ship different defaults. Firefox skips `sec-ch-ua-*` entirely, Safari sends a different `accept-language`, mobile presets flip `sec-ch-ua-mobile` to `?1`, and so on. Want the full list? Peek at `fingerprint/embedded/<preset>.json`.

The lib also auto-rewrites the `sec-fetch-*` cluster based on what kind of request you're firing. POST/PUT/PATCH and most XHR-shaped GETs get flipped from navigation mode (`navigate`/`document`/`?1`) to CORS mode (`cors`/`empty`/cross-site, no `sec-fetch-user`). Real browsers do the same, so you don't end up looking like a bot hitting an API endpoint with navigation-style headers.

## Setting custom headers

Two scopes: per-request, or session-wide as a default.

### Per-request

Drop a `Headers` map on the request. Whatever you set merges into the preset defaults. If your key matches a preset header, yours wins (single-value Set semantics).

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
package main

import (
    "context"
    "fmt"

    httpcloak "github.com/sardanioss/httpcloak"
)

func main() {
    s := httpcloak.NewSession("chrome-latest")
    defer s.Close()

    req := &httpcloak.Request{
        Method: "GET",
        URL:    "https://httpbin.org/headers",
        Headers: map[string][]string{
            "X-My-Header":   {"hello-world"},
            "Authorization": {"Bearer xxx"},
        },
    }
    resp, _ := s.Do(context.Background(), req)
    defer resp.Close()

    body, _ := resp.Text()
    fmt.Println(body)
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

s = httpcloak.Session(preset="chrome-latest")

r = s.get(
    "https://httpbin.org/headers",
    headers={
        "X-My-Header": "hello-world",
        "Authorization": "Bearer xxx",
    },
)
print(r.text)
```

</TabItem>
<TabItem value="nodejs" label="Node.js">

```js
const { Session } = require("httpcloak");

const s = new Session({ preset: "chrome-latest" });

const r = await s.get("https://httpbin.org/headers", {
  headers: {
    "X-My-Header": "hello-world",
    "Authorization": "Bearer xxx",
  },
});
console.log(r.text);
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using HttpCloak;

using var s = new Session(new SessionOptions { Preset = "chrome-latest" });

var headers = new Dictionary<string, string> {
    { "X-My-Header", "hello-world" },
    { "Authorization", "Bearer xxx" }
};
var r = s.Get("https://httpbin.org/headers", headers: headers);
Console.WriteLine(r.Text);
```

</TabItem>
</Tabs>

httpbin echoes back the headers it saw. You'll spot your `X-My-Header: hello-world` plus the full preset cluster (User-Agent, Accept, sec-ch-ua, the lot).

### Session-wide defaults

If a header should ride on every request in a session (auth tokens, an `X-API-Key`, a static `Referer`), set it once and forget about it.

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
// Go has no built-in WithHeaders for session defaults.
// Closure-wrap the session and inject headers in your wrapper:
type apiClient struct {
    s    *httpcloak.Session
    auth string
}

func (c *apiClient) Get(ctx context.Context, url string) (*httpcloak.Response, error) {
    return c.s.Do(ctx, &httpcloak.Request{
        Method: "GET",
        URL:    url,
        Headers: map[string][]string{
            "Authorization": {c.auth},
        },
    })
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
s = httpcloak.Session(preset="chrome-latest")
s.headers.update({"Authorization": "Bearer xxx"})

# now every s.get / s.post / s.request includes the Authorization header
r = s.get("https://httpbin.org/headers")
```

</TabItem>
<TabItem value="nodejs" label="Node.js">

```js
const s = new Session({ preset: "chrome-latest" });
s.headers["Authorization"] = "Bearer xxx";

const r = await s.get("https://httpbin.org/headers");
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
// .NET binding doesn't expose a session-default headers bag. Pass headers per request
// or wrap the session in your own class that injects defaults.
```

</TabItem>
</Tabs>

## How merge works

Merge order: **preset defaults first, your custom headers second**. If your key collides with a preset key (case-insensitive), your value wins. New keys land at whatever position the preset reserved for them, or at the end if the preset doesn't reserve a slot.

The reserved-slot bit is what matters. The preset's full HPACK position table (separate from the smaller "always emit" set) carves out spots for situational headers like `cache-control`, `content-type`, `content-length`, `cookie`, `origin`, `referer`. So when you add `Content-Type: application/json` on a POST, it lands at the same offset Chrome would've placed it. Skip that and your custom headers just pile up after `priority`, which is exactly the kind of small drift fingerprinters love.

## Things that don't behave the way you'd expect

- **Casing.** HTTP/2 and HTTP/3 are lowercase on the wire. The preset stores everything lowercase. Pass `User-Agent: foo` and the lib normalizes it to `user-agent: foo` for H2/H3. On HTTP/1.1, casing is preserved per the request map.
- **Removing a preset header.** Need to drop a default header (say, `Accept-Encoding`)? Set it to `""` in your headers map. The lib won't emit it.
- **Custom headers vs each other.** Add five custom headers the preset doesn't reserve slots for, and they all pile up at the end in the order you added them.
- **Cookie.** Don't set `Cookie` directly unless you've thought it through. The session jar handles it. See [Per-Request Cookies](../cookies-and-state/per-request-cookies) if you really need to override.

## Inspecting what actually went out

Cleanest verification path: send to [tls.peet.ws/api/all](https://tls.peet.ws/api/all) and read the `http2.sent_frames` array. Each HEADERS frame lists the headers in the exact order they hit the wire. That's the ground truth.

httpbin.org/headers is fine for "did my custom header show up?" checks, but it gives you a Python dict, not the wire order. For order, use peet.

## Header order overrides

If you really know what you're doing and want to reorder how headers get emitted (different preset baseline, custom mobile order, whatever), the session exposes `SetHeaderOrder()` and `GetHeaderOrder()`. Pass a list of lowercase header names. Pass `nil` or an empty slice to reset to the preset's default.

```go
s := httpcloak.NewSession("chrome-latest")
defer s.Close()

s.SetHeaderOrder([]string{
    "user-agent",
    "accept",
    "accept-language",
    "accept-encoding",
    "x-my-header",
})
```

Nuclear option. Don't reach for it unless your target is running some weirdo-specific order check that no shipped preset matches. 99% of the time the preset's order is what you want, and changing it just makes you stand out.
