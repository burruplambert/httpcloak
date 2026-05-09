---
title: HTTP/1.1
sidebar_position: 2
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# HTTP/1.1

H1 is the fallback. Almost every modern host has moved on to H2 or H3, but H1's still alive for legacy targets, internal services, and any case where ALPN refuses to hand you anything else. httpcloak speaks H1 when it has to, and lets you force it when you want.

## When the lib picks H1

Three paths land you on H1:

- The target's TLS server hello returns `http/1.1` in ALPN. No `h2` advertised, no `h3` Alt-Svc. You get H1.
- The auto-negotiation race finds H2 fails with an `ALPNMismatchError`. The lib reuses that same TLS connection and drops to H1 instead of redoing the handshake.
- You forced it. Either `WithForceHTTP1()` at construction or `RefreshWithProtocol("h1")` mid-session.

Why force it? Two reasons. First, predictable behavior in tests. Second, some boxes in front of the origin (older WAFs, internal mTLS gateways) only speak H1, and you don't want the lib burning RTTs trying H2 first.

## What the H1 transport actually does

Raw TCP, then a uTLS handshake with `http/1.1` as the only ALPN entry, then a plain `Request-Line + headers + CRLF + body`. No multiplexing, no header compression, no priority frames. One request per connection at a time, optionally pipelined with `Connection: keep-alive`.

The transport lives in `transport/http1_transport.go`. The interesting part is what gets fingerprinted.

## What gets fingerprinted at H1

Three layers, top to bottom:

1. **TLS handshake**. Same uTLS-backed ClientHello as H2/H3, just with the ALPN extension rewritten to `["http/1.1"]` only. JA3, JA4, peetprint all still apply. See [TLS fingerprinting](/fingerprinting/what-is-tls-fingerprinting).
2. **Header order**. H1's plain text, so header order is exactly the order of bytes you put on the wire. The preset's header order list drives this. Heads up: DevTools won't show you the real order Chrome sends, so check `tls.peet.ws/api/all` if you need ground truth.
3. **`Connection` header behavior**. `keep-alive` vs `close` vs `Upgrade: websocket` is a real fingerprint signal. Chrome on H1 sends `Connection: keep-alive` by default, and the preset matches.

H1 has no SETTINGS, no WINDOW_UPDATE, no PRIORITY frames. So the Akamai H2 hash is empty when you're on H1, and any check that relies on those signals just can't fire.

:::info H1 is also the websocket upgrade path
WebSocket starts as an H1 request with `Upgrade: websocket`. If you need the upgrade flow, you need H1. See [streaming and upgrades](/connection-lifecycle).
:::

## Code: force H1 and verify

Hit `tls.peet.ws/api/all`, assert `http_version` is `HTTP/1.1`, print the JA3.

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sardanioss/httpcloak"
)

func main() {
	sess := httpcloak.NewSession("chrome-latest",
		httpcloak.WithForceHTTP1(),
		httpcloak.WithSessionTimeout(30*time.Second),
	)
	defer sess.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := sess.Get(ctx, "https://tls.peet.ws/api/all")
	if err != nil {
		panic(err)
	}
	defer resp.Close()

	body, _ := resp.Bytes()
	var pr struct {
		HTTPVersion string `json:"http_version"`
		TLS         struct {
			JA3Hash string `json:"ja3_hash"`
		} `json:"tls"`
	}
	json.Unmarshal(body, &pr)

	fmt.Println("resp.Protocol:", resp.Protocol)
	fmt.Println("peet http_version:", pr.HTTPVersion)
	fmt.Println("ja3:", pr.TLS.JA3Hash)
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

with httpcloak.Session(preset="chrome-latest", force_http1=True, timeout=30) as sess:
    r = sess.get("https://tls.peet.ws/api/all")
    body = r.json()
    print("resp protocol:", r.http_version)
    print("peet http_version:", body["http_version"])
    print("ja3:", body["tls"]["ja3_hash"])
```

</TabItem>
<TabItem value="node" label="Node.js">

```javascript
const { Session } = require("httpcloak");

(async () => {
  const sess = new Session({ preset: "chrome-latest", forceHttp1: true, timeout: 30 });
  try {
    const r = await sess.get("https://tls.peet.ws/api/all");
    const body = JSON.parse(r.text);
    console.log("resp protocol:", r.httpVersion);
    console.log("peet http_version:", body.http_version);
    console.log("ja3:", body.tls.ja3_hash);
  } finally {
    sess.close();
  }
})();
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using HttpCloak;
using System.Text.Json;

using var sess = new Session(preset: "chrome-latest", forceHttp1: true, timeout: 30);
var r = sess.Get("https://tls.peet.ws/api/all");
var body = JsonDocument.Parse(r.Text).RootElement;
Console.WriteLine($"resp protocol: {r.HttpVersion}");
Console.WriteLine($"peet http_version: {body.GetProperty("http_version").GetString()}");
Console.WriteLine($"ja3: {body.GetProperty("tls").GetProperty("ja3_hash").GetString()}");
```

</TabItem>
</Tabs>

Expected output:

```
resp.Protocol: h1
peet http_version: HTTP/1.1
ja3: fe202172df94b322cc6e1e888a464d43
```

`resp.Protocol` is the lib's internal label (`h1`, `h2`, `h3`). `http_version` from `tls.peet.ws` is what the server actually saw, so that's your source of truth.

## Switching mid-session

Warm up on H2, then drop to H1 for one specific endpoint with `RefreshWithProtocol`:

```go
sess := httpcloak.NewSession("chrome-latest")
defer sess.Close()

// First request: default auto-negotiation, will land on H2.
sess.Get(ctx, "https://example.com/")

// Switch to H1 for a legacy upgrade-only endpoint.
sess.RefreshWithProtocol("h1")
sess.Get(ctx, "https://legacy.example.com/api/upgrade")
```

`RefreshWithProtocol` drops the existing connection pool. Cookies and the TLS session ticket cache survive the switch.

:::caution H1 with HTTP proxies
If you're going through an HTTP `CONNECT` proxy and the upstream only speaks H1, the lib's speculative-TLS optimization still applies. The ClientHello rides on the same packet as `CONNECT`, saving you an RTT. See [HTTP CONNECT proxies](/proxies/http-connect).
:::
