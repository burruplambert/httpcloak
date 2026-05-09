---
title: ECH
sidebar_position: 2
---

# ECH

ECH is Encrypted Client Hello. Without it, the SNI field in your TLS
handshake is plaintext on the wire. Anyone watching the network (your
ISP, a corporate middlebox, a curious nation-state) can see exactly
which hostname you're hitting even though the rest of the handshake is
encrypted. ECH wraps that ClientHello in a second handshake against the
ECH provider's outer name, so on the wire all an observer sees is a
generic outer SNI like `cloudflare-ech.com`. The real target stays
hidden inside.

httpcloak ships ECH on by default. If the target host publishes an ECH
config in its DNS HTTPS RR record, the lib fetches it, includes the
ECH extension in the ClientHello, and the inner SNI gets encrypted.
If the host doesn't publish one, the lib falls back to a plain
ClientHello and your request still works. No request fails just
because ECH isn't available.

:::info
ECH is still rolling out across the web. Many sites don't publish
HTTPS RR records yet, so ECH falls back gracefully. No failure if the
record is missing, just a normal SNI on the wire.
:::

## How httpcloak picks the ECH config

The order is:

1. If `WithECHFrom(domain)` is set, fetch the HTTPS RR for that
   alternate domain and use its ECH config.
2. Otherwise, fetch the HTTPS RR for the target host and use whatever
   it publishes.
3. If nothing is found, send a normal ClientHello with plaintext SNI.

For H3, this lookup runs in parallel with the A/AAAA DNS resolution
so it adds basically no latency to the first connection. For H2 the
ECH lookup only happens when you explicitly opt in via
`WithECHFrom`. (See the H2 caveat below.)

## Default session, ECH on

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
package main

import (
    "context"
    "fmt"

    "github.com/sardanioss/httpcloak"
)

func main() {
    s := httpcloak.NewSession("chrome-latest")
    defer s.Close()

    resp, err := s.Get(context.Background(), "https://example.com/")
    if err != nil {
        panic(err)
    }
    fmt.Println(resp.StatusCode, resp.Protocol)
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

with httpcloak.Session(preset="chrome-latest") as s:
    r = s.get("https://example.com/")
    print(r.status_code, r.protocol)
```

</TabItem>
<TabItem value="node" label="Node.js">

```js
const { Session } = require('httpcloak');

const s = new Session({ preset: 'chrome-latest' });
const r = await s.get('https://example.com/');
console.log(r.statusCode, r.protocol);
s.close();
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using HttpCloak;

using var s = new Session(preset: "chrome-latest");
var r = await s.GetAsync("https://example.com/");
Console.WriteLine($"{r.StatusCode} {r.Protocol}");
```

</TabItem>
</Tabs>

That's it. No flag needed. If `example.com` published an ECH config
(it doesn't, today), you'd get an encrypted SNI. Since it doesn't,
you get a normal handshake and the request still works.

## Turning ECH off

Disable the DNS HTTPS RR lookup entirely with `WithDisableECH`. The
session will never include the ECH extension in the ClientHello and
will never run the parallel HTTPS RR fetch. Useful when you want to
shave a few milliseconds off the first connection in a known-no-ECH
environment, or when you're debugging and you want a plain SNI on
the wire to stare at.

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
s := httpcloak.NewSession("chrome-latest",
    httpcloak.WithDisableECH(),
)
```

</TabItem>
<TabItem value="python" label="Python">

```python
with httpcloak.Session(preset="chrome-latest", disable_ech=True) as s:
    ...
```

</TabItem>
<TabItem value="node" label="Node.js">

```js
const s = new Session({ preset: 'chrome-latest', disableEch: true });
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using var s = new Session(preset: "chrome-latest", disableEch: true);
```

</TabItem>
</Tabs>

There's no security impact to turning ECH off. You just lose the SNI
privacy bit. Everything else about the handshake is identical.

## Borrowing an ECH config from another domain

Some hosts don't publish their own HTTPS RR record but sit behind a
CDN that does. Cloudflare in particular runs a public ECH endpoint
at `cloudflare-ech.com`. Any Cloudflare-proxied origin can be reached
with that ECH config because the outer handshake terminates at the
Cloudflare edge regardless of which inner hostname you're after.

`WithECHFrom(domain)` tells the lib to fetch the HTTPS RR from
`domain` instead of from the target host. The fetched config is then
used for any request made on this session.

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
s := httpcloak.NewSession("chrome-latest",
    httpcloak.WithECHFrom("cloudflare-ech.com"),
)
defer s.Close()

resp, _ := s.Get(context.Background(), "https://example.com/")
fmt.Println(resp.StatusCode)
```

</TabItem>
<TabItem value="python" label="Python">

```python
with httpcloak.Session(
    preset="chrome-latest",
    ech_from="cloudflare-ech.com",
) as s:
    r = s.get("https://example.com/")
    print(r.status_code)
```

</TabItem>
<TabItem value="node" label="Node.js">

```js
const s = new Session({
  preset: 'chrome-latest',
  echFrom: 'cloudflare-ech.com',
});
const r = await s.get('https://example.com/');
console.log(r.statusCode);
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using var s = new Session(
    preset: "chrome-latest",
    echFrom: "cloudflare-ech.com");

var r = await s.GetAsync("https://example.com/");
Console.WriteLine(r.StatusCode);
```

</TabItem>
</Tabs>

## Verifying ECH actually fired

This is the annoying part. There's no public reflector that prints
"yes, you used ECH". You have a few options:

- Capture the connection in Wireshark with the [keylog
  trick](./tls-keylog) and look for the `encrypted_client_hello`
  extension (type `0xfe0d`) in the ClientHello. Outer SNI will be
  the ECH provider's name, not your target.
- Read your own DNS lookup. The `dns` package exposes
  `FetchECHConfigsBase64(ctx, host)` if you import it directly. If
  it returns a non-empty base64 string, the host publishes ECH and
  the lib will use it.
- Trust the fallback. If `WithECHFrom` is set and the target is
  Cloudflare-fronted, ECH almost certainly fired.

Quick sanity probe in Go:

```go
import hcdns "github.com/sardanioss/httpcloak/dns"

b64, _ := hcdns.FetchECHConfigsBase64(ctx, "cf.erika.cool")
fmt.Println("ECH config len:", len(b64))
```

Empty means the host doesn't publish (or the DNS path is broken).
Non-empty means the lib has a real config to plug into the
ClientHello.

## Caveats

- **ECH on H1/H2 is opt-in only.** The H3 dial path auto-fetches
  the target's HTTPS RR. The H2 path only consults `WithECHFrom`
  and `WithECHConfig`, it does not auto-probe the target. The H1
  path doesn't touch ECH at all. If your session is forced to
  H1/H2, set `WithECHFrom` explicitly. (See the bug filed against
  this in the project's internal bug tracker.)
- ECH requires TLS 1.3. The lib bumps `MinVersion` to 1.3
  automatically when an ECH config is present.
- The HTTPS RR lookup is cached per host for the configured TTL,
  so repeated requests don't re-resolve.
- ECH config rotation: providers rotate keys every few hours.
  httpcloak handles this transparently; if a stale config is
  cached and rejected, the next dial refetches.
