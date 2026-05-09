---
title: Disabling the Cookie Jar
sidebar_position: 3
---

# Disabling the Cookie Jar

Sometimes you don't want the session managing cookies for you. Maybe you've got a database somewhere acting as your single source of truth, maybe you want every request fully isolated, or maybe you're just debugging and the auto-replay is getting in the way.

`WithoutCookieJar()` (added in 1.6.6) flips the jar off completely. With it set:

- `Set-Cookie` headers from responses are **not** stored
- The jar is **not** consulted when building the next request's `Cookie` header
- `GetCookies()` will return an empty list

Caller-provided `Cookie` headers still pass through untouched. Only the auto-injection from the internal jar is suppressed.

## When to actually do this

A few real reasons to turn it off:

- **You manage cookies yourself.** App-level cookie store in Redis, Postgres, etc. You read from there, build the `Cookie` header per request, and don't want the lib doing anything else.
- **You want each request fully independent.** Useful for fan-out crawling where two requests on the same session shouldn't share state.
- **You're debugging.** When you're trying to figure out why a response sets a weird cookie, having the jar silently swallow and replay it makes things harder. Turn it off, watch the raw headers.

If none of those apply, just leave the jar on. It's there for a reason.

## Code

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
	s := httpcloak.NewSession("chrome-146", httpcloak.WithoutCookieJar())
	defer s.Close()

	ctx := context.Background()

	r1, _ := s.Get(ctx, "https://httpbin.org/cookies/set?demo=hello")
	r1.Close()

	r2, _ := s.Get(ctx, "https://httpbin.org/cookies")
	body, _ := io.ReadAll(r2.Body)
	r2.Close()
	fmt.Println(string(body))
	// {"cookies": {}} (jar didn't store anything)

	fmt.Println(s.GetCookies()) // []
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

with httpcloak.Session(preset="chrome-146", without_cookie_jar=True) as s:
    s.get("https://httpbin.org/cookies/set?demo=hello")
    r = s.get("https://httpbin.org/cookies")
    print(r.json())
    # {'cookies': {}}
```

</TabItem>
<TabItem value="nodejs" label="Node.js">

```js
const httpcloak = require("httpcloak");

const s = new httpcloak.Session({
  preset: "chrome-146",
  withoutCookieJar: true,
});
try {
  await s.get("https://httpbin.org/cookies/set?demo=hello");
  const r = await s.get("https://httpbin.org/cookies");
  console.log(r.json());
  // { cookies: {} }
} finally {
  s.close();
}
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using HttpCloak;

using var s = new Session(preset: "chrome-146", withoutCookieJar: true);

s.Get("https://httpbin.org/cookies/set?demo=hello");
var r = s.Get("https://httpbin.org/cookies");
Console.WriteLine(r.Text);
// {"cookies": {}}
```

</TabItem>
</Tabs>

## Managing cookies yourself

With the jar off, you're the one driving. Two main patterns:

1. **Send a `Cookie` header per request.** Build the string yourself, attach it to the request headers. See [Per-Request Cookies](./per-request-cookies) for examples.
2. **Pull `Set-Cookie` out of the response.** Each `Response` exposes its raw headers; parse `Set-Cookie` yourself and stash the result wherever you want.

:::info Headers still pass through
Even with the jar off, the `Cookie` header you set on a request still goes out as you wrote it. The flag only suppresses the lib's auto-injection, not your manual headers.
:::

## Mixing modes

You can't toggle `WithoutCookieJar` mid-session. It's a session option, set once at construction. If you need both modes (jar-on for one workflow, jar-off for another), spin up two sessions.

If you need shared TLS state across the two, use `Fork()` to create a sibling that carries the same TLS resumption cache but starts fresh on cookies. Note: forks share the same cookie jar pointer, so if you fork from a jar-enabled parent, both share the jar. To genuinely separate the cookie state, build a fresh session.
