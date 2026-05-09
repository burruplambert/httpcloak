---
title: Refresh
sidebar_position: 1
---

# Refresh

`Refresh()` drops every live connection on a session and keeps the rest of your state. Cookies stay in the jar. TLS tickets stay cached. ECH config doesn't move. The preset name and any fingerprint overrides you set are still there. Only the wires get pulled.

Think of it as a browser tab reload. The next request opens fresh TCP or QUIC sockets, the TLS handshake reuses a stored ticket so it goes 0-RTT, and the cookie header on the new connection is byte-identical to the old one. The server can't easily tell a refresh from a brand-new tab on the same browser. That's the whole point.

## Why this exists

Plenty of anti-bot stacks track connection age. Real browsers don't keep a TCP socket open for hours. The keep-alive timer expires, a new connection opens for the next page load. A scraper that holds one connection alive for six hours sticks out like a sore thumb.

`Refresh()` lets you mimic that without throwing away your cookies or tickets. Run it on a timer. Every two or three minutes is fine. Other times you'll want it: a connection's gone stale, the server's misbehaving, or you want to switch protocols (see [Protocol Switching](./protocol-switching)).

## What survives a Refresh

| State | Survives |
| --- | --- |
| Cookies (full jar with metadata) | Yes |
| TLS 1.3 session tickets | Yes |
| TLS 1.2 session IDs | Yes |
| ECH config cache | Yes |
| Preset name and fingerprint overrides | Yes |
| Header order | Yes |
| Proxy config | Yes |
| Cache-validation headers (ETag / Last-Modified) | Yes |
| Live TCP / QUIC connections | No, all closed |
| In-flight requests | No, cancelled |
| Open streaming responses | No, terminated |

If you call `Refresh()` while a streaming download is mid-flight, that stream dies. There's no graceful drain. Hold onto streaming responses and finish them before refreshing.

## The 0-RTT story

Tickets stay in the cache, so the next handshake after `Refresh()` resumes from the previous TLS state. On TLS 1.3 that's a 0-RTT early-data path. On TLS 1.2 it's session-ID resumption (skips the cert roundtrip but doesn't ship request bytes early). The first request after a refresh is dramatically faster than the first request on a brand-new session.

:::tip
Most long-running scrapers should call `Refresh()` every few minutes. Real browsers do too. A connection that's been alive for hours is one of the cheaper signals an anti-bot stack can use against you.
:::

## Code

The shape's the same in every binding: send some requests, call `Refresh()`, send more.

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
s := httpcloak.NewSession("chrome-latest")
defer s.Close()
ctx := context.Background()

// Round 1 on the original connection.
for i := 0; i < 3; i++ {
	r, _ := s.Get(ctx, "https://tls.peet.ws/api/all")
	fmt.Printf("round1 #%d status=%d\n", i, r.StatusCode)
	r.Close()
}

// Cut the wire. Tickets, cookies, fingerprint state all survive.
s.Refresh()

// Round 2 picks up fresh sockets with TLS resumption.
for i := 0; i < 3; i++ {
	r, _ := s.Get(ctx, "https://tls.peet.ws/api/all")
	fmt.Printf("round2 #%d status=%d\n", i, r.StatusCode)
	r.Close()
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

with httpcloak.Session(preset="chrome-latest") as s:
    # Round 1
    for i in range(3):
        r = s.get("https://tls.peet.ws/api/all")
        print(f"round1 #{i} status={r.status_code}")

    # Cut every connection. Cookies and tickets stay.
    s.refresh()

    # Round 2 picks up clean sockets with TLS resumption.
    for i in range(3):
        r = s.get("https://tls.peet.ws/api/all")
        print(f"round2 #{i} status={r.status_code}")
```

</TabItem>
<TabItem value="nodejs" label="Node.js">

```javascript
const httpcloak = require("httpcloak");

const s = new httpcloak.Session({ preset: "chrome-latest" });
try {
  for (let i = 0; i < 3; i++) {
    const r = await s.get("https://tls.peet.ws/api/all");
    console.log(`round1 #${i} status=${r.statusCode}`);
  }

  s.refresh();

  for (let i = 0; i < 3; i++) {
    const r = await s.get("https://tls.peet.ws/api/all");
    console.log(`round2 #${i} status=${r.statusCode}`);
  }
} finally {
  s.close();
}
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using HttpCloak;

using var s = new Session(preset: "chrome-latest");

for (int i = 0; i < 3; i++)
{
    var r = s.Get("https://tls.peet.ws/api/all");
    Console.WriteLine($"round1 #{i} status={r.StatusCode}");
}

s.Refresh();

for (int i = 0; i < 3; i++)
{
    var r = s.Get("https://tls.peet.ws/api/all");
    Console.WriteLine($"round2 #{i} status={r.StatusCode}");
}
```

</TabItem>
</Tabs>

## What's NOT preserved

- **Live connections.** Every TCP socket, every QUIC connection, gone.
- **Active requests.** Anything in flight gets cancelled. The caller sees a context-cancelled or connection-closed error.
- **Streaming responses.** Body reads fail partway. Drain or close streams before refreshing.

Everything else (jar, tickets, ECH, header order, custom JA3, preset, proxy) sticks around. A save before and after `Refresh()` would only differ in the timestamp.

## When NOT to use it

If you want a totally fresh session (no cookies, no tickets, nothing), don't `Refresh()`. Just close and build a new one. `Refresh()` is the "keep my identity, drop my sockets" tool. For "drop my identity" you build a new `NewSession`.

Heads up: after `Refresh()` the session adds `cache-control: max-age=0` to the next request, mimicking a real browser F5. That hits servers like a deliberate cache-bust. If you don't want that signal, use a fresh session instead.

`Refresh()` and `Close()` both use a timeout-bounded close path on QUIC, so a misbehaving H3 peer can't hang the call forever.
