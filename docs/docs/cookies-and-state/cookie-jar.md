---
title: Cookie Jar
sidebar_position: 2
---

# Cookie Jar

Every `Session` has an internal cookie jar by default. It stores cookies from `Set-Cookie` response headers and replays them on matching follow-up requests, just like a browser tab.

You don't have to do anything to switch it on. It's already running.

## What gets stored

When a response comes back with one or more `Set-Cookie` headers, httpcloak parses each cookie and saves the full attribute set:

- `name` and `value`
- `domain` (with the host-only flag tracked separately)
- `path`
- `expires` and `max-age`
- `secure`
- `httpOnly`
- `sameSite`

The jar also tracks creation time so cookies with longer paths and older creation timestamps come first when building the `Cookie` header. This matches the RFC 6265 sort order browsers use.

## When the jar sends what

On the next request, the jar walks its stored cookies and picks the ones that match:

- the request **host** (host-only cookies require an exact match, domain cookies match the domain and any subdomain)
- the request **path** (cookie path must be a prefix of the request path with a `/` boundary)
- the request **scheme** (secure cookies only ride HTTPS)
- the **expiry** (anything past its expiry gets skipped)

The matching cookies get glued together into a single `Cookie:` header and sent.

See [Domain and Path Matching](./domain-and-path-matching) for the full rules and the gotchas.

## Lifecycle

Cookies live in memory for as long as the `Session` lives. Close the session and the jar goes with it.

If you want cookies to survive across processes, use `Save()` and `LoadSession()`. They serialise the jar (along with TLS resumption tickets and a few other bits) to a file you can reload later. See [Session save & restore](/connection-lifecycle/session-save-restore) for the full flow.

## Quick example

The example below sets a cookie via `httpbin.org/cookies/set`, then reads `httpbin.org/cookies` to show the jar replayed it on the second request.

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
package main

import (
	"context"
	"fmt"
	"io"

	"github.com/sardanioss/httpcloak"
)

func main() {
	s := httpcloak.NewSession("chrome-146")
	defer s.Close()

	ctx := context.Background()

	r1, _ := s.Get(ctx, "https://httpbin.org/cookies/set?demo=hello")
	r1.Close()

	r2, _ := s.Get(ctx, "https://httpbin.org/cookies")
	body, _ := io.ReadAll(r2.Body)
	r2.Close()
	fmt.Println(string(body))
	// {"cookies": {"demo": "hello"}}
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

with httpcloak.Session(preset="chrome-146") as s:
    s.get("https://httpbin.org/cookies/set?demo=hello")
    r = s.get("https://httpbin.org/cookies")
    print(r.json())
    # {'cookies': {'demo': 'hello'}}
```

</TabItem>
<TabItem value="nodejs" label="Node.js">

```js
const httpcloak = require("httpcloak");

const s = new httpcloak.Session({ preset: "chrome-146" });
try {
  await s.get("https://httpbin.org/cookies/set?demo=hello");
  const r = await s.get("https://httpbin.org/cookies");
  console.log(r.json());
  // { cookies: { demo: 'hello' } }
} finally {
  s.close();
}
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using HttpCloak;

using var s = new Session(preset: "chrome-146");

s.Get("https://httpbin.org/cookies/set?demo=hello");
var r = s.Get("https://httpbin.org/cookies");
Console.WriteLine(r.Text);
// {"cookies": {"demo": "hello"}}
```

</TabItem>
</Tabs>

## Inspecting the jar

The session exposes the current jar contents so you can poke at it:

- `GetCookies()` returns the full list with domain, path, expiry, flags
- `SetCookie(...)` adds or updates a cookie programmatically
- `DeleteCookie(name, domain)` removes one (pass empty domain to wipe across all domains)
- `ClearCookies()` empties the jar

Handy for tests, debugging, or when you want to seed the jar before the first request.

:::info DoStream parity
`DoStream()` also pulls cookies out of streamed responses (fixed in 1.6.6). Older versions of httpcloak that you might find in tutorials online didn't, so if you copy old code, double-check the version. Streaming and non-streaming requests now feed the same jar.
:::

## What the jar does NOT do

- It doesn't honour `__Host-` and `__Secure-` cookie name prefixes with the strict RFC checks. The flags (`Secure`, host-only) are still respected, but the prefix rules aren't enforced separately.
- It doesn't do anything with `SameSite` other than store it. There's no cross-site request tracking, so cookies always go out on requests you make.
- It doesn't garbage-collect expired cookies on every read. They get filtered at send time and during `ClearExpired()`. If you keep a long-lived session and want a clean snapshot, call `ClearExpired()` yourself.

## When you don't want the jar

If you'd rather manage cookies yourself, drop the jar entirely with `WithoutCookieJar()`. See [Disabling the Cookie Jar](./disabling-cookie-jar).
