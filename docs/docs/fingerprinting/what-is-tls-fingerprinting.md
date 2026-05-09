---
title: What is TLS Fingerprinting
sidebar_position: 2
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# What is TLS Fingerprinting

A TLS fingerprint is the wire-level signature of a TLS handshake. The TLS ClientHello is a tightly structured message and every client picks a specific cipher list, extension list, supported groups, and signature algorithm set in a specific order. Two clients running different code will pretty much always produce a different ClientHello, byte for byte.

Anti-bot vendors hash that ClientHello and match the hash against an allowlist of known-browser values. Match a real Chrome / Firefox / Safari hash, you pass. Anything else, you're flagged.

The same idea applies one layer up. After the handshake, the H2 connection opens with a SETTINGS frame, a WINDOW_UPDATE, optional PRIORITY frames, and a fixed pseudo-header order on the first request. Browsers all do this differently, and it hashes too.

## Common fingerprint formats

There are three formats you'll see most often:

### JA3

The original. MD5 hash over five comma-separated lists from the ClientHello:

```
TLSVersion,CipherSuites,Extensions,EllipticCurves,PointFormats
```

Example for Chrome 148 with sorted extensions:

```
771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-5-10-11-13-16-17613-18-23-27-35-43-45-51-65037-65281,29-23-24,0
```

JA3 is **deprecated**. Modern Chrome shuffles the order of TLS extensions every connection, which means the raw JA3 string changes every time even when the underlying browser version doesn't. The `ja3_hash` is also unstable for the same reason. Most defenders moved off JA3 a while back.

### JA4

The replacement. JA4 is a compound, more granular fingerprint. Format:

```
t13d1516h2_8daaf6152771_d8a2da3f94cd
```

Roughly:

- `t13` = TLS 1.3
- `d` = TCP (q for QUIC)
- `1516` = 15 ciphers, 16 extensions
- `h2` = ALPN h2
- middle hash = sorted cipher suites
- last hash = sorted extensions, sig algs

JA4 sorts the extension list before hashing, so it's stable across Chrome's extension shuffle. This is what you actually want to verify.

### Akamai HTTP/2 hash

A separate fingerprint covering the H2 layer. It hashes a compact string with four parts:

```
SETTINGS|WINDOW_UPDATE|PRIORITY|PSEUDO_HEADER_ORDER
```

Real Chrome 148 looks like this:

```
1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p
```

The hash captures things like initial window size, header table size, the connection-level window update value, and the order Chrome puts `:method`, `:authority`, `:scheme`, `:path` in. Chrome and Safari pick a different pseudo-header order. Firefox picks a different SETTINGS layout. All of it ends up in the hash.

## Why default Go net/http gets blocked

`net/http` uses Go's standard `crypto/tls` to build the ClientHello. The cipher list, extension list, and supported curves are all bog-standard Go defaults that no real browser produces. The JA4 hash for a default Go client doesn't match any browser's hash. Bot vendors block by exclusion — if the hash isn't on the allowlist, the request is presumed bot.

This is why you can run the same code through `curl --tls-cipher` to manually order ciphers and still get blocked. Chrome doesn't just have a different cipher list, it has a different extension order, a different curve list, different signature algorithms, different ALPN, different cert compression. Reproducing all that is what httpcloak does.

httpcloak emits ClientHello bytes byte-identical to a real Chrome / Firefox / Safari handshake. The H2 SETTINGS, WINDOW_UPDATE, and pseudo-header order match too. So does the order of HTTP headers Chrome sends on the first request.

## Quick verification

Hit `tls.peet.ws/api/all` with httpcloak's `chrome-latest` preset and look at the JA4:

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
    s := httpcloak.NewSession("chrome-latest")
    defer s.Close()

    resp, err := s.Get(context.Background(), "https://tls.peet.ws/api/all")
    if err != nil { panic(err) }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    fmt.Println(string(body))
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

with httpcloak.Session(preset="chrome-latest") as s:
    r = s.get("https://tls.peet.ws/api/all")
    print(r.text)
```

</TabItem>
<TabItem value="node" label="Node.js">

```js
const { Session } = require("httpcloak");

const s = new Session({ preset: "chrome-latest" });
const r = await s.get("https://tls.peet.ws/api/all");
console.log(r.text);
s.close();
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using HttpCloak;

using var s = new Session(preset: "chrome-latest");
var r = await s.GetAsync("https://tls.peet.ws/api/all");
Console.WriteLine(r.Text);
```

</TabItem>
</Tabs>

What you'll see in the response (Chrome 148, captured 2026-05):

```text
ja4:                     t13d1516h2_8daaf6152771_d8a2da3f94cd
peetprint_hash:          1d4ffe9b0e34acac0bd883fa7f79d7b5
akamai_fingerprint_hash: 52d84b11737d980aef856699f885ca86
```

Those three hashes match real Chrome 148 desktop. Run the same request through `net/http` and the JA4 will be something like `t13d1517h2_acb858a92679_eb4d4c4c4f4f` which doesn't match any browser, anywhere.

The `ja3_hash` field will not be stable across runs because Chrome permutes its extension order. `ja4` and `peetprint_hash` will be stable, and that's what you should verify against.

:::info
`tls.peet.ws/api/all` is your friend. It reflects everything: TLS, H2, headers, and the order each was received in. `cf.erika.cool` and `browserleaks.com` are useful when you specifically want to see what Cloudflare's edge sees, since it runs the same TLS terminator as production Cloudflare. Both are safe to test against.
:::

## What the rest of this section covers

- [Presets](./presets) — the bundled Chrome / Firefox / Safari profiles you pick by name.
- [JSON Preset Builder](./json-preset-builder) — describe a preset, mutate the JSON, load it back as a new preset. The proper customization workflow.
- [Custom JA3](./custom-ja3) — when you only want to override the JA3 string, the lightweight path.
- [Akamai Shorthand](./akamai-shorthand) — same idea for the H2 fingerprint.
- [Per-Resource Priority](./per-resource-priority) — RFC 7540 stream weights and RFC 9218 priority headers based on Sec-Fetch-Dest.
