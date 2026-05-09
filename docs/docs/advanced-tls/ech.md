---
title: ECH
sidebar_position: 2
---

# ECH

ECH stands for Encrypted Client Hello. Without it, the SNI in your TLS handshake sits in plaintext on the wire. Your ISP, a corporate middlebox, a curious nation-state, anyone watching can read which hostname you're hitting even though the rest of the handshake is encrypted. ECH wraps the real ClientHello inside a second handshake aimed at an ECH provider's outer name, so all an observer sees is something generic like `cloudflare-ech.com`. The actual target stays hidden inside.

httpcloak ships with ECH on by default. If the target host publishes an ECH config in its DNS HTTPS RR, the lib grabs it, drops the ECH extension into the ClientHello, and the inner SNI gets encrypted. If the host doesn't publish one, you fall back to a plain ClientHello and the request still goes through. Nothing breaks just because ECH isn't available.

:::info
ECH is still rolling out. Plenty of sites haven't published HTTPS RR records yet, so the fallback kicks in often. No failure when the record's missing, just a normal SNI on the wire.
:::

## How httpcloak picks the ECH config

The order's pretty simple:

1. If you set `WithECHFrom(domain)`, fetch the HTTPS RR for that alternate domain and use its ECH config.
2. Otherwise, fetch the HTTPS RR for the target host and use whatever it publishes.
3. If nothing's there, send a normal ClientHello with plaintext SNI.

For H3, this lookup runs in parallel with the A/AAAA DNS resolution, so it adds basically zero latency on the first connection. For H2, the ECH lookup only runs when you opt in via `WithECHFrom`. (See the H2 caveat below.)

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

That's the whole opt-in. No flag needed. If `example.com` published an ECH config (it doesn't, today), you'd get an encrypted SNI. Since it doesn't, you get a plain handshake and the request still works.

## Turning ECH off

`WithDisableECH` kills the DNS HTTPS RR lookup entirely. The session won't include the ECH extension in the ClientHello, won't run the parallel HTTPS RR fetch, won't even try. Handy when you want to shave a few ms off the first connection in a known-no-ECH environment, or when you're debugging and want a plain SNI to stare at.

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

There's no security hit from turning ECH off. You just lose the SNI privacy bit. Everything else about the handshake is identical.

## Borrowing an ECH config from another domain

Some hosts don't publish their own HTTPS RR but they sit behind a CDN that does. Cloudflare in particular runs a public ECH endpoint at `cloudflare-ech.com`. Any Cloudflare-proxied origin can be reached using that ECH config, because the outer handshake terminates at Cloudflare's edge regardless of which inner hostname you're after.

`WithECHFrom(domain)` tells the lib to fetch the HTTPS RR from `domain` instead of from the target host. The fetched config gets used for any request on that session.

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

Yeah, this is the annoying part. There's no public reflector that prints "yes, you used ECH". You've got a few options:

- Capture the connection in Wireshark with the [keylog trick](./tls-keylog) and look for the `encrypted_client_hello` extension (type `0xfe0d`) in the ClientHello. The outer SNI will be the ECH provider's name, not your target.
- Read your own DNS lookup. The `dns` package exposes `FetchECHConfigsBase64(ctx, host)` if you import it directly. Non-empty base64 means the host publishes ECH and the lib will use it.
- Trust the fallback. If `WithECHFrom` is set and the target's Cloudflare-fronted, ECH almost certainly fired.

Quick sanity probe in Go:

```go
import hcdns "github.com/sardanioss/httpcloak/dns"

b64, _ := hcdns.FetchECHConfigsBase64(ctx, "cf.erika.cool")
fmt.Println("ECH config len:", len(b64))
```

Empty means the host doesn't publish (or your DNS path's broken). Non-empty means the lib has a real config to plug into the ClientHello.

## Caveats

- **ECH on H1/H2 is opt-in only.** The H3 dial path auto-fetches the target's HTTPS RR. The H2 path only consults `WithECHFrom` and `WithECHConfig`, it doesn't auto-probe the target. The H1 path doesn't touch ECH at all. If your session's forced to H1/H2, set `WithECHFrom` explicitly. (There's a bug filed against this in the project's internal tracker.)
- ECH requires TLS 1.3. The lib bumps `MinVersion` to 1.3 automatically when an ECH config is present.
- The HTTPS RR lookup is cached per host for the configured TTL, so repeated requests don't re-resolve.
- ECH config rotation: providers rotate keys every few hours. httpcloak handles this transparently. If a stale config gets cached and rejected, the next dial refetches.
