---
title: Protocol Switching
sidebar_position: 3
---

# Protocol Switching

Sessions can hop between HTTP/1.1, HTTP/2, and HTTP/3 mid-flight. `RefreshWithProtocol("h1" | "h2" | "h3" | "auto")` drops every live connection then re-handshakes on whatever protocol you ask for. The setting sticks, so future plain `Refresh()` calls also use the new protocol until you switch again.

For sessions that should never auto-negotiate in the first place, pass `WithSwitchProtocol("h2")` (or h1 / h3) to `NewSession`. Calling `Refresh()` on that session always lands on the configured protocol.

## Why you'd want this

**Force H2 because the target's H3 is broken.** Some CDNs ship garbage on their HTTP/3 endpoint while H2 works fine. The auto-negotiator picks H3 when it's available, so you'd never know unless you forced H2.

**Force H3 because the target's H2 blocks you.** Common on Cloudflare. The H2 path runs through more middleware where bot detection lives, while H3 often gets lighter filtering, especially on plans where the operator hasn't enabled Bot Management for HTTP/3.

**Test which protocol gets through.** Anti-bot tooling sometimes treats H1, H2 and H3 differently. Lock to one and rerun the same scrape to isolate the variable.

Bonus pattern: warm tickets on H3, then switch to H2 for the workload. The kept ticket means H2 starts from a resumed handshake.

:::info
The auto-negotiation logic (`raceH3H2` internally) races H3 and H2 in parallel and picks whichever wins. Great for latency, unpredictable across runs. Lock to one protocol if you need predictable behaviour, or if you're debugging a site-specific issue and need to know exactly which path the next request will take.
:::

## Code

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
s := httpcloak.NewSession("chrome-latest")
defer s.Close()
ctx := context.Background()

// Auto-negotiate first. Most likely picks H2.
r, _ := s.Get(ctx, "https://tls.peet.ws/api/all")
fmt.Printf("auto: %s\n", r.Protocol)
r.Close()

// Force H2. Useful when a site's H3 is flaky.
s.RefreshWithProtocol("h2")
r, _ = s.Get(ctx, "https://tls.peet.ws/api/all")
fmt.Printf("h2: %s\n", r.Protocol)
r.Close()

// Force H1. Heavy, slow, sometimes the only path that works.
s.RefreshWithProtocol("h1")
r, _ = s.Get(ctx, "https://tls.peet.ws/api/all")
fmt.Printf("h1: %s\n", r.Protocol)
r.Close()

// Back to auto.
s.RefreshWithProtocol("auto")
```

</TabItem>
<TabItem value="python" label="Python">

```python
with httpcloak.Session(preset="chrome-latest") as s:
    s.get("https://tls.peet.ws/api/all")          # auto
    s.refresh(switch_protocol="h2")
    s.get("https://tls.peet.ws/api/all")          # h2
    s.refresh(switch_protocol="h1")
    s.get("https://tls.peet.ws/api/all")          # h1
```

</TabItem>
<TabItem value="nodejs" label="Node.js">

```javascript
const s = new httpcloak.Session({ preset: "chrome-latest" });
try {
  await s.get("https://tls.peet.ws/api/all");     // auto
  s.refresh("h2");
  await s.get("https://tls.peet.ws/api/all");     // h2
  s.refresh("h1");
  await s.get("https://tls.peet.ws/api/all");     // h1
} finally {
  s.close();
}
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using var s = new Session(preset: "chrome-latest");
s.Get("https://tls.peet.ws/api/all");             // auto
s.Refresh(switchProtocol: "h2");
s.Get("https://tls.peet.ws/api/all");             // h2
s.Refresh(switchProtocol: "h1");
s.Get("https://tls.peet.ws/api/all");             // h1
```

</TabItem>
</Tabs>

## Lock at construction time

To skip auto-negotiation entirely, set the protocol when you build the session. `Refresh()` then always lands on that protocol.

```go
s := httpcloak.NewSession("chrome-latest", httpcloak.WithSwitchProtocol("h2"))
```

Python: `Session(preset=..., switch_protocol="h2")`. Node.js: `new Session({ preset, switchProtocol: "h2" })`. .NET: `new Session(preset, switchProtocol: "h2")`.

There's also `WithForceHTTP1`, `WithForceHTTP2`, `WithForceHTTP3` and `WithDisableHTTP3` that pin the protocol from request one (no `Refresh()` needed). Use those for a session that never speaks anything but the chosen protocol.

## H3 caveats

Three things to know before you `RefreshWithProtocol("h3")`:

- The preset has to support H3. If it doesn't, you get `preset %q does not support HTTP/3` and the switch is refused.
- The host has to actually serve H3. Forcing H3 against a host that doesn't advertise it just times out. There's no automatic fallback when forced.
- H3 over a UDP-blocking proxy is a non-starter. Most corporate networks and plenty of SOCKS5 proxies don't pass UDP. Use `WithForceHTTP2` instead.

If you want H3 sometimes and H2 other times, keep two sessions rather than toggling. Every switch throws away the active connection.

## Protocol values

`RefreshWithProtocol` accepts: `"h1"`/`"http1"`/`"1"` for HTTP/1.1, `"h2"`/`"http2"`/`"2"` for HTTP/2, `"h3"`/`"http3"`/`"3"` for HTTP/3, and `"auto"` (or empty string) to race H3 vs H2 with fallback to H1. Anything else returns an error.
