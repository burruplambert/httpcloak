---
title: Warmup
sidebar_position: 2
---

# Warmup

`Warmup(ctx, url)` runs a multi-hop preflight that mimics what a real browser
does when you type a URL and hit enter. It loads the HTML, parses out the
stylesheets, scripts, images and fonts the page references, and fetches them
in parallel with browser-style headers and timing. By the time it returns,
your session has cookies, cache validators, TLS tickets and ECH state that
look like a tab someone has already used.

## Why bother

Cold-start fingerprinting is real. The very first request from a session has
patterns that are hard to fake. Header order is fine, JA3 is fine, but the
*timing* and the *request graph* are bare. There's no Referer chain. The
cookie jar is empty. There are no cache-validation headers. No subresource
fetches. The site never set an `Accept-CH` so you're not sending high-entropy
client hints. None of this individually screams bot, but together it paints
a "this connection just opened to grab one specific endpoint" picture, which
is exactly what bots do.

`Warmup` papers over a lot of that in one call. It:

- Fetches the navigation HTML with proper Sec-Fetch headers.
- Parses the HTML for `<link>`, `<script>`, `<img>` and `<link rel="preload">`
  references.
- Splits them into priority groups: CSS and fonts first, then scripts, then
  images.
- Fires each batch concurrently (up to 6 in flight, matching Chrome's per-host
  H1 connection limit), with realistic 50-300ms gaps between batches.
- Picks up Set-Cookie headers along the way.
- Records `Accept-CH` headers from any host that asks for client hints.
- Caches ETag and Last-Modified so the next request to the same URL ships
  conditional headers.
- Lets the TLS layer cache session tickets for every host hit.

After `Warmup` returns, your follow-up request looks like the user clicked
the next link, not like the connection just popped into existence.

## Common pattern

Warm up on the home page, then go after the actual endpoint you care about.

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
s := httpcloak.NewSession("chrome-latest")
defer s.Close()
ctx := context.Background()

// Warm up against the target. Loads HTML, fetches CSS/JS/images,
// grabs whatever cookies the site sets.
if err := s.Warmup(ctx, "https://tls.peet.ws/api/all"); err != nil {
	panic(err)
}
fmt.Printf("cookies: %d\n", len(s.GetCookies()))

// Now the actual request, with tickets, cookies and a believable history
// already in place.
r, _ := s.Get(ctx, "https://tls.peet.ws/api/all")
defer r.Close()
fmt.Printf("status=%d proto=%s\n", r.StatusCode, r.Protocol)
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

with httpcloak.Session(preset="chrome-latest") as s:
    s.warmup("https://tls.peet.ws/api/all", timeout=60000)
    print("cookies:", len(s.get_cookies()))

    r = s.get("https://tls.peet.ws/api/all")
    print(r.status_code, r.headers.get("content-type"))
```

</TabItem>
<TabItem value="nodejs" label="Node.js">

```javascript
const httpcloak = require("httpcloak");

const s = new httpcloak.Session({ preset: "chrome-latest" });
try {
  s.warmup("https://tls.peet.ws/api/all", { timeout: 60000 });
  console.log("cookies:", s.getCookies().length);

  const r = await s.get("https://tls.peet.ws/api/all");
  console.log(r.statusCode);
} finally {
  s.close();
}
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using HttpCloak;

using var s = new Session(preset: "chrome-latest");

s.Warmup("https://tls.peet.ws/api/all", timeoutMs: 60000);
Console.WriteLine($"cookies: {s.GetCookies().Length}");

var r = s.Get("https://tls.peet.ws/api/all");
Console.WriteLine($"status={r.StatusCode}");
```

</TabItem>
</Tabs>

## What gets populated

| State after Warmup | Source |
| --- | --- |
| Cookies | Set-Cookie from the page and every subresource |
| TLS session tickets | One per origin touched during warmup |
| ECH config cache | DNS HTTPS records seen during the navigation |
| Cache validators (ETag, Last-Modified) | Per-URL response headers |
| Client hints map | `Accept-CH` headers from any host |
| Header order memory | Already preset-driven, no drift |
| Cookie jar size | Whatever the site sets, often 5-20 |

If the warmed page redirects (301/302), the redirect chain is followed and
cookies set along the way all land in the jar with proper domain scoping.

## Edge cases worth knowing

Warmup caps at 50 subresources. Big news sites reference 200+ images on a
single page; we stop at 50 so the call doesn't run forever. Subresource
fetch failures are silent (matches browser behaviour, one broken image
shouldn't fail the page load). If the navigation URL returns something
other than `text/html`, warmup returns nil after the navigation. TLS and
cookies still got populated, just no subresource crawl.

The inter-batch delays (50-150ms before scripts, 100-300ms before images)
are randomised per call so back-to-back warmups don't have identical
timing fingerprints.

## When NOT to warmup

- The endpoint you want is on a different origin from any plausible home
  page. Site A's warmup cookies don't help when you're hitting site B.
- You're in a tight retry loop. Warming up before each retry is overkill.
- The target is a JSON-only API that never sets cookies. The request graph
  just costs you bandwidth.

In those cases skip `Warmup`, or call it once at session creation and
never again.

## Warmup vs Refresh

These are not interchangeable. `Refresh()` cuts connections without
touching state. `Warmup()` does the opposite: leaves connections alone and
pumps state into the session. Common pipeline: `Warmup` once at start, then
periodic `Refresh()` calls.

`Warmup` respects the context. Cancel and the call returns the context
error after the in-flight batch finishes.
