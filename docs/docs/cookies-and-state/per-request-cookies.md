---
title: Per-Request Cookies
sidebar_position: 4
---

# Per-Request Cookies

Sometimes you just want to slap a `Cookie` header onto a single request without touching the session jar. Maybe you're testing one specific cookie, maybe you've already got the value from somewhere else, or maybe the jar is off and you're driving manually.

httpcloak passes the `Cookie` header through unchanged. Whatever string you give it goes on the wire.

## Setting it on a request

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
req := &httpcloak.Request{
	Method: "GET",
	URL:    "https://httpbin.org/cookies",
	Headers: map[string][]string{
		"Cookie": {"session=abc; lang=en"},
	},
}
r, _ := s.Do(ctx, req)
defer r.Close()
```

</TabItem>
<TabItem value="python" label="Python">

```python
# Option A: explicit Cookie header
r = s.get(
    "https://httpbin.org/cookies",
    headers={"Cookie": "session=abc; lang=en"},
)

# Option B: cookies kwarg (joined into a single Cookie header for you)
r = s.get(
    "https://httpbin.org/cookies",
    cookies={"session": "abc", "lang": "en"},
)
```

</TabItem>
<TabItem value="nodejs" label="Node.js">

```js
// Option A: explicit Cookie header
let r = await s.get("https://httpbin.org/cookies", {
  headers: { Cookie: "session=abc; lang=en" },
});

// Option B: cookies option
r = await s.get("https://httpbin.org/cookies", {
  cookies: { session: "abc", lang: "en" },
});
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
// Option A: explicit Cookie header
var headers = new Dictionary<string, string> {
    { "Cookie", "session=abc; lang=en" }
};
var r = s.Get("https://httpbin.org/cookies", headers: headers);

// Option B: cookies parameter
var cookies = new Dictionary<string, string> {
    { "session", "abc" },
    { "lang", "en" }
};
var r2 = s.Get("https://httpbin.org/cookies", cookies: cookies);
```

</TabItem>
</Tabs>

## How this interacts with the jar

If the jar is on, the lib **merges** your per-request `Cookie` header with whatever the jar would have sent. Your header comes first, jar contents come after, joined with `; `.

That's usually what you want. But if your goal is "use only this one cookie, ignore the jar," you've got two clean ways:

1. Disable the jar with [`WithoutCookieJar()`](./disabling-cookie-jar) for that whole session.
2. Call `ClearCookies()` on the session right before the request, then attach your header.

Don't try to fight the merge by setting `Cookie: ""`. The lib treats empty as "no per-request cookie," not "send nothing," so the jar will still inject.

## Cookie order matters for fingerprinting

The order of cookies in the `Cookie` header is part of your client's fingerprint. Real browsers sort consistently (longer path first, then by creation time, per RFC 6265). httpcloak preserves whatever you give it byte-for-byte.

So if you're hand-rolling cookies and you want to look like a browser, sort them yourself. Don't shuffle them across requests, don't rely on `dict` iteration order in scripts, and double-check your sort matches the order the jar would have produced.

If the jar is doing the work for you, you don't need to think about this. The jar handles the sort. This only matters when you're driving the `Cookie` header manually.

## When per-request beats the jar

A few situations where reaching for a per-request header makes more sense than touching the jar:

- **One-off auth.** A single API call needs a special token cookie that shouldn't stick around for follow-ups.
- **Replaying a captured session.** You've got a `Cookie` string from a browser export; just paste it.
- **Cross-host scenarios.** You want the same cookie sent to two different hosts that the jar's domain rules wouldn't cover automatically.

For everything else, let the jar do its job.
