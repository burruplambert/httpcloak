---
title: Authentication
sidebar_position: 6
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Authentication

You can hand-roll an `Authorization` header just fine. But for anything more interesting than a static value, the auth helpers carry their weight: they know the wire format, they keep the header at the right offset Chrome would put it, and Digest actually does the challenge-response loop for you. Three flavors ship: Basic, Bearer, and Digest.

The Authorization header is also one of those headers fingerprinters watch for ordering drift. Build it through the helpers and it lands in the slot the preset reserved, not bolted onto the end like an afterthought.

## The Auth interface

Every helper satisfies the same tiny interface:

```go
type Auth interface {
    Apply(req *http.Request) error
    HandleChallenge(resp *http.Response, req *http.Request) (bool, error)
}
```

`Apply` writes the Authorization header onto the outgoing request. `HandleChallenge` is the second-pass hook: when the server hits you with a 401, the client calls `HandleChallenge` so the auth scheme can read `WWW-Authenticate`, work out what to send back, and tell the client whether to retry. Basic and Bearer don't have a challenge dance, so their `HandleChallenge` always returns `false`. Digest does the whole nonce-and-cnonce thing.

## Basic

Username and password, base64-encoded, slapped behind `Basic `. That's it.

```go
auth := client.NewBasicAuth("user", "passwd")
// or, on the client itself:
c.SetBasicAuth("user", "passwd")
```

`SetBasicAuth` is the shortcut: it builds a `BasicAuth` and parks it on the client so every request from that point gets the header. `NewBasicAuth` returns the value if you'd rather scope it per-request via `Request.Auth`.

## Bearer

Same shape, different scheme. Pass the token in, get `Authorization: Bearer <token>` on every request.

```go
auth := client.NewBearerAuth("eyJhbGc...")
// or:
c.SetBearerAuth("eyJhbGc...")
```

:::tip
Tokens expire. If yours rotates (OAuth refresh, short-lived JWTs, signed STS creds), don't bake the token into the client at construction. Wrap your refresh logic and call `SetBearerAuth` again whenever the token rolls. Or, if requests come from different identities, skip the session setter entirely and stamp `Request.Auth = client.NewBearerAuth(token)` on each call so a stale token never leaks into the wrong request.
:::

## Digest

Digest is the picky one. The server hands you a `WWW-Authenticate: Digest realm=..., nonce=..., qop=...` on the first 401. You hash username + password + realm into HA1, hash method + URI into HA2, then hash those together with the nonce, a counter, and a cnonce you generate. That hash goes back as the `Authorization: Digest response="..."`. The client does the second leg automatically:

```go
auth := client.NewDigestAuth("user", "passwd")
c.SetAuth(auth)
```

Flow on the wire:

1. Request fires with no Authorization header (Digest doesn't pre-emptively send).
2. Server replies 401 with the `WWW-Authenticate` challenge.
3. `HandleChallenge` parses realm, nonce, qop, opaque, algorithm.
4. Client retries the same request, this time with the computed `Digest response=...`.
5. Server returns the real response.

The nonce-counter (`nc`) increments on every reuse, so the same `DigestAuth` value can ride multiple requests without re-challenging. Discard it once the credentials change.

## Custom schemes via SetAuth

If your target speaks something weird (HMAC-SHA256 signed requests, AWS SigV4, a custom token-and-timestamp scheme), implement the `Auth` interface yourself and hand it to `SetAuth`:

```go
type myAuth struct { secret string }

func (a *myAuth) Apply(req *http.Request) error {
    sig := sign(req, a.secret)
    req.Header.Set("Authorization", "MyScheme "+sig)
    return nil
}

func (a *myAuth) HandleChallenge(resp *http.Response, req *http.Request) (bool, error) {
    return false, nil
}

c.SetAuth(&myAuth{secret: "..."})
```

`Apply` runs once per request right before send. If your signature depends on the body, hash it from the request before Apply returns. If it depends on a timestamp, generate a fresh one each call.

## Try it: Basic auth against httpbin

httpbin's `/basic-auth/user/passwd` endpoint returns 401 without credentials and 200 with the right ones. Quick way to confirm the helper's actually wiring the header.

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
package main

import (
    "context"
    "fmt"
    "io"

    "github.com/sardanioss/httpcloak/client"
)

func main() {
    c := client.NewClient("chrome-latest")
    defer c.Close()

    ctx := context.Background()

    // Without auth: 401
    resp, _ := c.Get(ctx, "https://httpbin.org/basic-auth/user/passwd", nil)
    fmt.Println("no-auth:", resp.StatusCode)
    resp.Body.Close()

    // With BasicAuth: 200
    c.SetBasicAuth("user", "passwd")
    resp, _ = c.Get(ctx, "https://httpbin.org/basic-auth/user/passwd", nil)
    fmt.Println("with-auth:", resp.StatusCode)
    body, _ := io.ReadAll(resp.Body)
    resp.Body.Close()
    fmt.Println(string(body))
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

with httpcloak.Session(preset="chrome-latest") as s:
    # Without auth: 401
    r = s.get("https://httpbin.org/basic-auth/user/passwd")
    print("no-auth:", r.status_code)

    # With auth tuple: 200
    r = s.get(
        "https://httpbin.org/basic-auth/user/passwd",
        auth=("user", "passwd"),
    )
    print("with-auth:", r.status_code)
    print(r.text)
```

</TabItem>
<TabItem value="nodejs" label="Node.js">

```js
const { Session } = require("httpcloak");

const s = new Session({ preset: "chrome-latest" });

// Without auth: 401
let r = await s.get("https://httpbin.org/basic-auth/user/passwd");
console.log("no-auth:", r.status);

// With Authorization header: 200
const creds = Buffer.from("user:passwd").toString("base64");
r = await s.get("https://httpbin.org/basic-auth/user/passwd", {
  headers: { Authorization: `Basic ${creds}` },
});
console.log("with-auth:", r.status);
console.log(r.text);

s.close();
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using HttpCloak;
using System;
using System.Text;

using var s = new Session(new SessionOptions { Preset = "chrome-latest" });

// Without auth: 401
var r = s.Get("https://httpbin.org/basic-auth/user/passwd");
Console.WriteLine($"no-auth: {r.StatusCode}");

// With auth header: 200
var creds = Convert.ToBase64String(Encoding.ASCII.GetBytes("user:passwd"));
var headers = new Dictionary<string, string> {
    { "Authorization", $"Basic {creds}" }
};
r = s.Get("https://httpbin.org/basic-auth/user/passwd", headers: headers);
Console.WriteLine($"with-auth: {r.StatusCode}");
Console.WriteLine(r.Text);
```

</TabItem>
</Tabs>

Run the Go example and you'll see:

```text
no-auth: 401
with-auth: 200
{
  "authenticated": true,
  "user": "user"
}
```

## Per-request vs session-level

Two scopes, same idea as headers.

**Session-level** (the `Set*Auth` family on the client) parks an Auth on the client. Every request from that point on gets it applied automatically. Good for "this whole script is one identity" situations.

**Per-request** (the `Auth` field on `Request`) overrides the session-level Auth for one call. If you set both, the request-level wins. Use it when one client juggles multiple identities, or when most of your traffic is anonymous and only a few endpoints need credentials.

```go
// session default
c.SetBearerAuth("anon-token")

// this single request uses a different token
resp, _ := c.Do(ctx, &client.Request{
    Method: "GET",
    URL:    "https://api.example.com/admin",
    Auth:   client.NewBearerAuth("admin-token"),
})
```

One safety bit baked into the redirect handler: if a redirect crosses origins, the client drops `Authorization` (and `Proxy-Authorization`) before following. Same rule for HTTPS to HTTP downgrades. Real browsers do this and so does httpcloak, so credentials don't leak to whatever domain the response decided to bounce you to.
