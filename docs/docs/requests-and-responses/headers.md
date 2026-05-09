---
title: Headers
sidebar_position: 2
---

# Headers

Header order matters. Not the values alone, the **order** they go on the wire.

When Chrome makes a request, the headers come out in a specific, deterministic sequence: `sec-ch-ua` first, then `sec-ch-ua-mobile`, then `sec-ch-ua-platform`, then `upgrade-insecure-requests`, then `user-agent`, then `accept`, and so on. That sequence is part of your fingerprint. Anti-bot vendors hash it. If your client sends the same headers but in a different order (or sets a header Chrome wouldn't set, or skips one Chrome always sends), you stand out.

httpcloak ships the canonical header order baked into each preset. Your custom headers slot in at preset-defined positions so you don't break the fingerprint when adding `Authorization` or `X-Anything-Custom`.

:::tip
Chrome being a lil bitch won't show you header order in DevTools. You can check what your client actually puts on the wire at [tls.peet.ws/api/all](https://tls.peet.ws/api/all) (look at the `http2.sent_frames[].headers` array).
:::

## What we set by default

Every preset comes with a built-in set of browser headers. For `chrome-148-linux` (the current default at time of writing), the request goes out with:

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

Different presets carry different defaults (Firefox doesn't send `sec-ch-ua-*`, Safari sends a different `accept-language`, mobile presets flip `sec-ch-ua-mobile` to `?1`, etc.). The exact list per preset lives in `fingerprint/embedded/<preset>.json` if you want to peek.

The lib also auto-rewrites the `sec-fetch-*` cluster based on what kind of request you're making. POST/PUT/PATCH and most XHR-shaped GETs get flipped from navigation mode (`navigate`/`document`/`?1`) to CORS mode (`cors`/`empty`/cross-site, no `sec-fetch-user`). That matches what real browsers do and stops you from looking like a bot hitting an API endpoint with navigation headers.

## Setting custom headers

Two scopes: per-request, or session-wide as a default.

### Per-request

Drop a `Headers` map onto the request. Whatever you set merges into the preset defaults. If your key matches a preset header, yours wins (single-value Set semantics).

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

httpbin echoes back the headers it received. You'll see your `X-My-Header: hello-world` plus the full preset cluster (User-Agent, Accept, sec-ch-ua, etc.) in there.

### Session-wide defaults

If a header should ride along on every request in a session (auth tokens, an `X-API-Key`, a static `Referer`), set it once and forget it.

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

The merge order is: **preset defaults first → your custom headers second**. If your key collides with a preset key (case-insensitive), your value replaces the preset's. New keys get inserted at the position the preset reserved for them, or at the end if the preset doesn't reserve a slot.

The reserved-slot bit matters. The preset's full HPACK position table (separate from the smaller "always emit" set) carves out spots for situational headers like `cache-control`, `content-type`, `content-length`, `cookie`, `origin`, `referer`. So when you add `Content-Type: application/json` on a POST, it ends up at the same offset Chrome would have placed it. Without that, custom headers would just append after `priority`, which is the kind of small drift fingerprinters love.

## Things that won't behave the way you expect

- **Casing.** HTTP/2 and HTTP/3 are lowercase on the wire. The preset stores everything lowercase. If you pass `User-Agent: foo`, the lib normalizes it to `user-agent: foo` for H2/H3. On HTTP/1.1, casing is preserved per the request map.
- **Removing a preset header.** If you really need to drop a default header (say, `Accept-Encoding`), set it to an empty string `""` in your headers map. The lib will skip emitting it.
- **Order of your own custom headers vs each other.** If you add five custom headers that the preset doesn't reserve slots for, they all end up after the preset's slot table, in the order you added them.
- **Cookie.** Don't set `Cookie` directly unless you've thought about it. The session jar handles it. See [Per-Request Cookies](../cookies-and-state/per-request-cookies) for the override path.

## Inspecting what actually went out

The cleanest way to verify your headers and order is to send to [tls.peet.ws/api/all](https://tls.peet.ws/api/all) and look at the `http2.sent_frames` array. Each HEADERS frame lists the headers in the exact order they were transmitted. That's the ground truth for what the wire saw.

httpbin.org/headers is fine for "did my custom header show up?" checks but it gives you a Python dict, not the wire order. Use peet for the order question.

## Header order overrides

If you really know what you're doing and want to reorder how headers get emitted (different preset baseline, custom mobile order, etc.), the session exposes `SetHeaderOrder()` and `GetHeaderOrder()`. Pass a list of lowercase header names. Pass `nil` or an empty slice to reset to the preset's default.

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

This is the nuclear option. Don't reach for it unless your target is doing some weirdo-specific order check that no shipped preset matches. For 99% of cases the preset's order is what you want and changing it just makes you stand out.
