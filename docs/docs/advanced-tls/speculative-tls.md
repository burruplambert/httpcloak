---
title: Speculative TLS
sidebar_position: 3
---

# Speculative TLS

When your session goes through an HTTP CONNECT proxy, the normal flow
is two round-trips before any TLS bytes touch the wire:

1. Client opens TCP to the proxy.
2. Client writes `CONNECT target:443 HTTP/1.1` and waits.
3. Proxy writes `HTTP/1.1 200 Connection established` and waits.
4. Client writes the TLS ClientHello.
5. Proxy relays it. Server replies. Handshake continues.

Steps 2-3 cost one full RTT. Steps 4 onward are the real TLS
handshake. On a 50ms-RTT proxy that's 50ms of dead air doing nothing
useful, on every fresh proxied dial.

`WithEnableSpeculativeTLS` collapses that. The CONNECT request and
the inner ClientHello get written to the socket in the same burst,
before the proxy has had a chance to reply with its 200. A
well-behaved proxy reads the CONNECT, sets up the upstream tunnel,
and immediately starts forwarding the bytes that came after the
`\r\n\r\n`. The 200 still gets written back, but the ClientHello
overlaps with it instead of waiting for it. One round-trip saved.

:::tip
Free win for any proxy-heavy workload. If you're making a lot of
fresh proxied dials, turn this on.
:::

## What it looks like on the wire

A non-speculative dial sends two distinct write bursts to the proxy:

```
burst 1: CONNECT httpbin.org:443 HTTP/1.1\r\nHost: httpbin.org:443\r\n\r\n
[wait for 200]
burst 2: \x16\x03\x01... (TLS ClientHello)
```

A speculative dial coalesces:

```
burst 1: CONNECT httpbin.org:443 HTTP/1.1\r\nHost: httpbin.org:443\r\n\r\n\x16\x03\x01... (CONNECT + ClientHello in the same write)
[200 comes back overlapping with the upstream forwarding]
```

The lib still parses the proxy's 200 response correctly, the proxy
still sees a valid CONNECT request. The only thing that changes is
the timing.

## Turning it on

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
    s := httpcloak.NewSession("chrome-latest",
        httpcloak.WithSessionTCPProxy("http://user:pass@proxy.example.com:8080"),
        httpcloak.WithEnableSpeculativeTLS(),
    )
    defer s.Close()

    resp, err := s.Get(context.Background(), "https://httpbin.org/ip")
    if err != nil {
        panic(err)
    }
    fmt.Println(resp.StatusCode)
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

with httpcloak.Session(
    preset="chrome-latest",
    tcp_proxy="http://user:pass@proxy.example.com:8080",
    enable_speculative_tls=True,
) as s:
    r = s.get("https://httpbin.org/ip")
    print(r.status_code)
```

</TabItem>
<TabItem value="node" label="Node.js">

```js
const { Session } = require('httpcloak');

const s = new Session({
  preset: 'chrome-latest',
  tcpProxy: 'http://user:pass@proxy.example.com:8080',
  enableSpeculativeTls: true,
});

const r = await s.get('https://httpbin.org/ip');
console.log(r.statusCode);
s.close();
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using HttpCloak;

using var s = new Session(
    preset: "chrome-latest",
    tcpProxy: "http://user:pass@proxy.example.com:8080",
    enableSpeculativeTls: true);

var r = await s.GetAsync("https://httpbin.org/ip");
Console.WriteLine(r.StatusCode);
```

</TabItem>
</Tabs>

## When it doesn't help

- **No proxy.** Speculative TLS is a proxy CONNECT optimization. With
  a direct dial there's no CONNECT exchange to fold into the
  ClientHello, so the option is a no-op.
- **SOCKS5 proxies.** SOCKS has its own framing and the lib doesn't
  apply the speculative optimization on the SOCKS path. Stick with
  HTTP CONNECT for this win.
- **Already-warm connections.** The savings only apply on the first
  dial. Once the H2 or H1 connection is in the pool, subsequent
  requests reuse it and there's no proxy handshake to optimize.

## When it can break

Some proxies are picky. They expect to read the CONNECT, write the
200, and only then start reading more bytes from the client. If the
client sends extra bytes before the 200 is fully written, the proxy
might:

- Buffer the speculative ClientHello correctly and forward it
  upstream once the tunnel is up. This is the common case.
- Reject the CONNECT outright with a parse error. Rare but seen on
  older Squid setups and a handful of debugging tools.
- Drop the speculative bytes silently, leaving the inner TLS
  handshake stuck waiting for the server's reply. This is the worst
  case.

If you suspect the proxy is misbehaving, turn the option off and
re-test. If a fresh dial without speculative works and with
speculative hangs or errors, that's your answer. File the proxy
brand somewhere so future you remembers.

## Compatibility status by proxy class

- Modern commercial residential and datacenter proxies: works.
  Squid 4+, Tinyproxy, mitmproxy in upstream mode, Bright Data,
  Oxylabs, etc. Verified in production.
- Squid 3.x with old defaults: hit or miss. Test before you trust.
- Corporate egress proxies (BlueCoat, Zscaler, Forcepoint): mostly
  untested in this project. Some inspect the CONNECT carefully and
  may not like extra bytes.
- TLS-terminating MITM proxies: doesn't matter. They terminate
  inside the proxy and re-originate, so the ClientHello you sent
  isn't the one that reaches the target anyway.

## Pairing with other features

Speculative TLS plays well with everything else in the lib:

- Works with H1, H2, and H3-over-MASQUE alike (when the dial path
  is HTTP CONNECT, which for H3 means MASQUE specifically).
- Works with `WithECHFrom`. The ECH-wrapped ClientHello is what
  gets pipelined.
- Works with custom JA3 / JA4 fingerprints.
- Works with session resumption. The speculative ClientHello can
  carry a PSK and resume in one round-trip.

If you're stacking optimizations on a proxy-heavy workload,
speculative TLS plus session resumption plus ECH gives you a fully
private one-RTT-to-data first request. That's a Chrome-class
profile that very few clients ship with by default.
