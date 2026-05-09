---
title: Refresh
sidebar_position: 1
---

# Refresh

`Refresh()` severs every live connection on a session while keeping the rest of
the session intact. Cookies stay in the jar, TLS session tickets stay in the
cache, ECH config keeps its place, the preset name and any custom fingerprint
overrides are untouched. Only the wires get pulled.

Think of it as a browser tab reload. New TCP or QUIC sockets open on the next
request, the TLS handshake gets to use a stored ticket so it goes through
0-RTT, and the cookie header on the new connection looks identical to the
previous one. The server has no easy way to spot a refresh from a brand-new
tab on the same browser, which is the whole point.

## Why this exists

Plenty of anti-bot stacks track connection age. Real browsers don't keep a
TCP connection open for hours; the keep-alive timer expires and a new
connection opens for the next page load. A scraper that holds one
connection open for six hours stands out hard.

`Refresh()` lets you imitate that without throwing away cookies or tickets.
Run it on a timer. Every two or three minutes is reasonable. Other use
cases: a connection has gone stale, the server is misbehaving, or you want
to switch protocols (see [Protocol Switching](./protocol-switching)).

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

If you call `Refresh()` while a streaming download is mid-flight, that stream
ends. There is no graceful drain. Hold onto streaming responses and finish
them before refreshing.

## The 0-RTT story

Because tickets stay in the cache, the next handshake after `Refresh()`
resumes the TLS state from before. On TLS 1.3 that's a 0-RTT early-data
path; on TLS 1.2 it's session-ID resumption (skips the cert roundtrip but
doesn't ship request bytes early). The next request after a refresh is
dramatically faster than the first request on a brand-new session.

:::tip
Most long-running scrapers should call `Refresh()` every few minutes. Real
browsers do too. A connection that's been alive for hours is one of the
cheaper signals an anti-bot stack can use against you.
:::

## Code

The shape is the same in every binding: send some requests, call `Refresh()`,
send more.

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

- **Live connections.** Every TCP socket, every QUIC connection, closed.
- **Active requests.** Anything in flight gets cancelled. Caller sees a
  context-cancelled or connection-closed error.
- **Streaming responses.** Stream body reads fail partway. Drain or close
  streams before refreshing.

Everything else (jar, tickets, ECH, header order, custom JA3, preset, proxy)
stays put. A save before and after `Refresh()` would only differ in
timestamp.

## When NOT to use it

If you want a totally fresh session (no cookies, no tickets, no nothing),
don't `Refresh()`, just close and build a new one. `Refresh()` is the "keep
my identity, drop my sockets" tool. For "drop my identity" build a new
`NewSession`.

Marker side effect: after `Refresh()` the session adds `cache-control:
max-age=0` to the next request, mimicking a real browser F5. That hits
servers like a deliberate cache-bust. If you don't want that signal, use a
fresh session instead.

`Refresh()` and `Close()` both use a timeout-bounded close path on QUIC, so
a misbehaving H3 peer can't hang the call forever.
