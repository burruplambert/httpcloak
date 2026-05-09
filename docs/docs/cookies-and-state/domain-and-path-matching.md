---
title: Domain and Path Matching
sidebar_position: 5
---

# Domain and Path Matching

The jar runs four checks before a cookie rides along: domain, path, secure, expiry. Get any of them wrong and the cookie either leaks where it shouldn't or silently goes missing where it should have been sent.

This page covers the rules httpcloak's jar follows and the surprises that catch most people out.

## Domain matching

A cookie's domain is set in one of two ways:

- **No `Domain` attribute on `Set-Cookie`.** The cookie is **host-only**. It only goes back to the exact host that set it.
- **`Domain=foo.example.com` (with or without leading dot).** The cookie is a **domain cookie**. It goes back to `foo.example.com` and any subdomain.

Examples with a request to `https://api.example.com/`:

| Stored `Domain` | Type | Sent? |
|---|---|---|
| (none, set by `api.example.com`) | host-only | yes |
| (none, set by `example.com`) | host-only | no |
| `.example.com` | domain | yes |
| `example.com` | domain | yes (modern, see note) |
| `.api.example.com` | domain | yes |
| `.other.com` | domain | no |

:::caution Leading dot, RFC vs reality
Strict RFC 2109 said `Domain=example.com` (no leading dot) meant "exactly example.com." RFC 6265 and every modern browser treat it as if you wrote `.example.com`, so it includes subdomains. When httpcloak parses `Set-Cookie` from a response, it follows the modern behavior: stored with the leading dot internally, matches subdomains.

Heads up: when you call the programmatic `SetCookie()` / `set_cookie()` API yourself, the same string `Domain="example.com"` (no leading dot) is currently stored as **host-only** instead, which is the older RFC 2109 reading. Asymmetry with response-header parsing. If you want a domain cookie via the API, write the leading dot explicitly: `Domain=".example.com"`.
:::

You also can't set a cookie for a domain you don't control. If `api.example.com` returns `Set-Cookie: x=1; Domain=other.com`, the jar rejects it. The request host has to equal the cookie domain or be a subdomain of it.

## Path matching

The path rule is a prefix match with a `/` boundary. A cookie set for `Path=/api` matches:

- `/api` (exact)
- `/api/` (cookie path is a prefix and the next char is `/`)
- `/api/foo`
- `/api/foo/bar`

It does **not** match:

- `/apixyz` (next char isn't `/`)
- `/foo/api` (cookie path isn't a prefix)
- `/` (request path is shorter)

If the cookie path ends in `/`, like `Path=/api/`, the slash boundary is implicit and any request path with that exact prefix matches.

If `Set-Cookie` doesn't include `Path`, httpcloak defaults it to `/`, which matches every request to that domain.

## Secure flag

A cookie with `Secure` only goes out on HTTPS. Period. Even if everything else matches, a plain `http://` request won't see it.

This works the other way too: a server can only **set** a `Secure` cookie over HTTPS. If `Set-Cookie: x=1; Secure` arrives over plain HTTP, the jar rejects it.

## Expiry

The jar checks expiry at send time. A cookie with an `Expires` in the past gets skipped. A session cookie (no `Expires`, no `Max-Age`) lives until the session is closed or `ClearCookies()` is called.

The jar doesn't sweep expired cookies eagerly. Call `ClearExpired()` if you want a clean snapshot.

## Worked example

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
s := httpcloak.NewSession("chrome-146")
defer s.Close()

// host-only scoping: bare domain via the API stores as host-only
s.SetCookie(httpcloak.CookieInfo{
	Name: "scoped", Value: "yes",
	Domain: "httpbin.org", Path: "/cookies",
})

ctx := context.Background()

r1, _ := s.Get(ctx, "https://httpbin.org/cookies")
// matches → {"cookies": {"scoped": "yes"}}
r1.Close()

r2, _ := s.Get(ctx, "https://httpbin.org/anything")
// path /anything doesn't match /cookies → cookie not sent
r2.Close()
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

with httpcloak.Session(preset="chrome-146") as s:
    s.set_cookie(
        name="scoped",
        value="yes",
        domain="httpbin.org",
        path="/cookies",
    )

    r1 = s.get("https://httpbin.org/cookies")
    print(r1.json())
    # {'cookies': {'scoped': 'yes'}}

    r2 = s.get("https://httpbin.org/anything")
    # cookie not sent, path /anything doesn't match /cookies
```

</TabItem>
<TabItem value="nodejs" label="Node.js">

```js
const httpcloak = require("httpcloak");

const s = new httpcloak.Session({ preset: "chrome-146" });
try {
  s.setCookie("scoped", "yes", {
    domain: "httpbin.org",
    path: "/cookies",
  });

  let r = await s.get("https://httpbin.org/cookies");
  console.log(r.json());
  // { cookies: { scoped: 'yes' } }

  r = await s.get("https://httpbin.org/anything");
  // cookie not sent
} finally {
  s.close();
}
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using HttpCloak;

using var s = new Session(preset: "chrome-146");

s.SetCookie("scoped", "yes", domain: "httpbin.org", path: "/cookies");

var r1 = s.Get("https://httpbin.org/cookies");
Console.WriteLine(r1.Text);
// {"cookies": {"scoped": "yes"}}

var r2 = s.Get("https://httpbin.org/anything");
// cookie not sent
```

</TabItem>
</Tabs>

## Common gotchas

- **You meant host-only but typed `Domain`.** Setting `Domain=example.com` makes the cookie ride out to every subdomain. If you only want `example.com` itself, omit `Domain` entirely.
- **Path looks like it matches but doesn't.** `/api` does not match `/apiv2`. The boundary check requires a `/` (or exact equality).
- **Secure cookie set over HTTP.** Some local dev setups proxy through plain HTTP. The jar won't store a `Secure` cookie if the response wasn't HTTPS.
- **Setting `Domain` for someone else's domain.** A response from `a.com` setting `Domain=b.com` gets rejected. Cross-site cookie injection isn't a thing the jar will help you with.
- **Forgetting expiry on long-lived sessions.** A cookie with `Max-Age=10` is gone in ten seconds. The jar will quietly stop sending it. If a server-side flow seems to log you out for no reason, check the cookie expiry first.

:::warning Don't leak cookies to subdomains
If you set `Domain` in your `Cookie` header without thinking, you can leak cookies to subdomains you didn't mean to hit. Match the browser's behavior: leave `Domain` empty on cookies you set yourself when you only want host-only matching. Adding the attribute opts you into subdomain delivery for the rest of the cookie's life.
:::
