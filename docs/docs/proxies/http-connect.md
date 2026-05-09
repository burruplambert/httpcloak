---
title: HTTP CONNECT
sidebar_position: 3
---

# HTTP CONNECT

The classic HTTP proxy. The client opens a TCP connection to the proxy,
sends a `CONNECT host:port HTTP/1.1` request, the proxy replies `200
Connection established`, and from that point on the socket is a raw
tunnel. Your real TLS handshake then runs through the tunnel as if the
proxy weren't there.

This is what HTTPS-through-an-HTTP-proxy actually looks like on the
wire. Squid, mitmproxy in upstream mode, every datacenter proxy
provider, almost every corporate egress proxy: all HTTP CONNECT.

## URL shape

```
http://user:pass@proxy.example.com:8080
https://user:pass@proxy.example.com:8443
```

Use `https://` when the connection from your client to the proxy itself
should be TLS (not super common, but some providers offer it). The
inner request is always TLS regardless because that's the target site's
TLS, decrypted only by the target.

## Setting it up

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
    )
    defer s.Close()

    resp, err := s.Get(context.Background(), "https://httpbin.org/ip")
    if err != nil {
        panic(err)
    }
    fmt.Println(resp.StatusCode, string(resp.Body))
    // {"origin": "<the proxy's egress IP>"}
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

with httpcloak.Session(
    preset="chrome-latest",
    tcp_proxy="http://user:pass@proxy.example.com:8080",
) as s:
    r = s.get("https://httpbin.org/ip")
    print(r.status_code, r.text)
    # {"origin": "<the proxy's egress IP>"}
```

</TabItem>
<TabItem value="node" label="Node.js">

```js
const { Session } = require('httpcloak');

const s = new Session({
  preset: 'chrome-latest',
  tcpProxy: 'http://user:pass@proxy.example.com:8080',
});

const r = await s.get('https://httpbin.org/ip');
console.log(r.statusCode, r.body);
s.close();
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using HttpCloak;

using var s = new Session(
    preset: "chrome-latest",
    tcpProxy: "http://user:pass@proxy.example.com:8080");

var r = await s.GetAsync("https://httpbin.org/ip");
Console.WriteLine($"{r.StatusCode} {r.Body}");
```

</TabItem>
</Tabs>

## What's on the wire

A normal request through an HTTP CONNECT proxy goes:

1. TCP open to `proxy.example.com:8080`.
2. Client sends `CONNECT httpbin.org:443 HTTP/1.1` plus
   `Proxy-Authorization: Basic <base64(user:pass)>` and a host header.
3. Proxy replies `HTTP/1.1 200 Connection established`.
4. Client sends the TLS ClientHello to the same socket. From here the
   proxy is just forwarding bytes.
5. TLS handshake completes against `httpbin.org`. Client sends `GET /ip`.

That's two round-trips before TLS even starts (TCP handshake then
CONNECT exchange), then one or two more for TLS depending on resumption.

## Saving an RTT with speculative TLS

httpcloak can pipeline the CONNECT request with the inner ClientHello.
Same socket, both writes coalesced before any read. Saves one full
round-trip on every fresh proxied connection.

```go
s := httpcloak.NewSession("chrome-latest",
    httpcloak.WithSessionTCPProxy("http://user:pass@proxy.example.com:8080"),
    httpcloak.WithEnableSpeculativeTLS(),
)
```

It's off by default because some proxies (especially older Squid
configurations and a handful of debugging tools) react badly when bytes
arrive before the 200 response is fully read. Test it on your provider
first. Full details and trade-offs in [Speculative
TLS](/advanced-tls/speculative-tls).

## H3 with an HTTP CONNECT TCP proxy

HTTP CONNECT only carries TCP. If you set just `WithSessionTCPProxy`
with an HTTP URL, H3 will dial directly to the target (no proxy on UDP).
Three options:

- Let H3 dial direct (default). Fine on most networks.
- Add a UDP proxy: `WithSessionUDPProxy("masque://...")` or a SOCKS5
  endpoint that supports UDP ASSOCIATE.
- Disable H3: `WithDisableHTTP3()` so the session sticks to H1/H2 and
  everything goes through the HTTP CONNECT proxy.

## Auth

Basic auth is handled for you when credentials are in the URL.
httpcloak base64-encodes `user:pass` and adds
`Proxy-Authorization: Basic ...` to the CONNECT request. If your
password has special characters, URL-encode it:

```
http://user:p%40ss%21@proxy.example.com:8080
```

(That's `p@ss!`.)

## Common errors

- `407 Proxy Authentication Required`: credentials missing or wrong.
  Check the URL has `user:pass@`.
- `403 Forbidden`: proxy reached but rejected the target. Some
  providers block specific destinations. Try a different target to
  confirm.
- `502 Bad Gateway`: proxy couldn't reach the target. Often the proxy's
  upstream having a bad day.
- `failed to resolve proxy host ...`: your DNS can't resolve the proxy
  hostname itself. Most often a typo in the hostname.

## Provider quirks

Some HTTP CONNECT providers are picky about the order or presence of
headers in the CONNECT request. httpcloak sends a minimal CONNECT
(method line, Host header, optional Proxy-Authorization). If a provider
requires a specific User-Agent or extra header on the CONNECT line
itself, that's not currently configurable. File an issue with the
provider name if you hit this.
