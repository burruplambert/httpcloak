---
title: Auto-Negotiation
sidebar_position: 5
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Auto-Negotiation

By default, you don't pick a protocol. The lib does. It races HTTP/3 and HTTP/2 in parallel and uses whichever connects first, falling back to HTTP/1.1 only if H2 actually fails ALPN. This is the "I don't care, just give me Chrome-shaped bytes on the right protocol" mode.

## What `doAuto` actually does

The dispatcher in `transport/transport.go` is a single function called `doAuto`. Steps:

1. Look up the host in the `protocolSupport` cache. If we already learned this host's best protocol from a prior request, skip the race and just dial that.
2. Otherwise, fire two goroutines: one dials H3 over UDP via QUIC, the other dials H2 over TCP+TLS.
3. Wait for the first one to succeed. Cancel the loser.
4. If H2's TLS handshake came back with `http/1.1` in ALPN (an `ALPNMismatchError`), reuse that same TLS connection for an H1 request. No second handshake.
5. If both attempts time out (default budget around 6 seconds), the lib tries H2 directly as a last resort, then H1.
6. Cache the winning protocol in `protocolSupport[host]` so the next request to the same host skips the race.

The race is in `raceH3H2`. The whole reason it exists is to avoid the 5-second wall you'd hit if you tried H3 first and the network silently dropped UDP/443. With the race, H2 fills in as soon as TCP comes back, usually in under 200ms.

## How H3 gets discovered

Two ways:

- **Alt-Svc**. The first H2 response from a host carries `alt-svc: h3=":443"; ma=86400`. The lib parses it, the `protocolSupport` cache learns "this host speaks H3", and the next request can race H3 against H2 (and H3 will usually win because the QUIC handshake completes in fewer round trips).
- **DNS HTTPS RR**. RFC 9460 HTTPS records advertise ALPN values directly in DNS. If the resolver returns one with `h3` in it, the lib can skip the H2 detour entirely. Whether this fires depends on your DNS config in `dns/`.

If neither hint says H3 is available, the race still includes H3 the first time but H3's handshake is unlikely to land first.

## When H1 actually shows up

H1 is the boring fallback. You land here when:

- The TLS server hello returns `http/1.1` in ALPN. The `ALPNMismatchError` path reuses the connection.
- Both H3 and H2 attempts fail outright and the lib has to try H1 over a fresh TCP connection.
- You forced it with `WithForceHTTP1()` or `RefreshWithProtocol("h1")`.

In normal browsing-like traffic against modern hosts, H1 should be rare.

## Forcing one protocol

Three options at session construction:

- `WithForceHTTP1()`: lock to H1. Skips H2/H3 entirely.
- `WithForceHTTP2()`: lock to H2. Skips H3 and won't fall back to H1 unless ALPN forces it.
- `WithForceHTTP3()`: lock to H3. Hard fails if the host doesn't speak H3.

Plus one for the common middle case:

- `WithDisableHTTP3()`: keep auto-negotiation but never try H3. Equivalent to "old-school client".

For mid-session changes:

- `RefreshWithProtocol("h1" | "h2" | "h3")` closes the connection pool and forces the named protocol from the next request.
- `WithSwitchProtocol("h2")` at construction time queues a protocol switch on the next `Refresh()`. Useful for the warmup-on-H3, serve-on-H2 pattern when you want to share a TLS ticket across protocols.

## Why force one

A handful of real reasons:

- **Tests**. You want predictable behavior. Auto-negotiation can land on H2 or H3 depending on what the target's edge feels like advertising today.
- **Broken H3 at the target**. Some hosts advertise `h3` in Alt-Svc but their UDP port is firewalled, or the QUIC stack is misconfigured. Auto-negotiation handles this by losing the race, but if you're hitting the same host millions of times you might as well skip the H3 attempt entirely with `WithDisableHTTP3()`.
- **Policy**. Your network only allows TCP/443. Force H2.
- **Fingerprint surface**. You specifically want to test the H3 fingerprint your preset emits. Force H3 against a known H3-capable target.

## Code: default vs forced

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/sardanioss/httpcloak"
)

func hit(label string, opts ...httpcloak.SessionOption) {
	sess := httpcloak.NewSession("chrome-latest", opts...)
	defer sess.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := sess.Get(ctx, "https://tls.peet.ws/api/all")
	if err != nil {
		fmt.Printf("[%s] err: %v\n", label, err)
		return
	}
	defer resp.Close()
	fmt.Printf("[%s] resp.Protocol=%s status=%d\n", label, resp.Protocol, resp.StatusCode)
}

func main() {
	hit("default")                              // h2 against tls.peet
	hit("force-h2", httpcloak.WithForceHTTP2()) // h2
	hit("disable-h3", httpcloak.WithDisableHTTP3())
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

def hit(label, **kwargs):
    with httpcloak.Session(preset="chrome-latest", timeout=30, **kwargs) as sess:
        r = sess.get("https://tls.peet.ws/api/all")
        print(f"[{label}] protocol={r.http_version} status={r.status_code}")

hit("default")
hit("force-h2", force_http2=True)
hit("disable-h3", disable_http3=True)
```

</TabItem>
<TabItem value="node" label="Node.js">

```javascript
const { Session } = require("httpcloak");

async function hit(label, opts) {
  const sess = new Session({ preset: "chrome-latest", timeout: 30, ...opts });
  try {
    const r = await sess.get("https://tls.peet.ws/api/all");
    console.log(`[${label}] protocol=${r.httpVersion} status=${r.statusCode}`);
  } finally {
    sess.close();
  }
}

(async () => {
  await hit("default", {});
  await hit("force-h2", { forceHttp2: true });
  await hit("disable-h3", { disableHttp3: true });
})();
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using HttpCloak;

void Hit(string label, Action<SessionOptions>? configure = null) {
    var opts = new SessionOptions { Preset = "chrome-latest", Timeout = 30 };
    configure?.Invoke(opts);
    using var sess = new Session(opts);
    var r = sess.Get("https://tls.peet.ws/api/all");
    Console.WriteLine($"[{label}] protocol={r.HttpVersion} status={r.StatusCode}");
}

Hit("default");
Hit("force-h2", o => o.ForceHttp2 = true);
Hit("disable-h3", o => o.DisableHttp3 = true);
```

</TabItem>
</Tabs>

Expected output, hitting `tls.peet.ws`:

```
[default] resp.Protocol=h2 status=200
[force-h2] resp.Protocol=h2 status=200
[disable-h3] resp.Protocol=h2 status=200
```

All three land on H2 because tls.peet.ws's UDP/443 port is closed in practice and the lib doesn't get an H3 path to win the race.

## Per-host learning

Once a request to `example.com` comes back on H3, the lib caches that fact. The next request to `example.com` skips the race and dials H3 directly. The cache is keyed by hostname (no port, no path) and lives in `protocolSupport`. If the host stops responding on H3 later, you'll need to recreate the session or call `RefreshWithProtocol("h2")` to evict the cached choice.

There is a planned `BrokenAltSvc` circuit breaker that suppresses H3 attempts after repeated failures to a specific host without you having to restart. It's tracked in our internal docs and not landed yet.

:::tip Auto vs forced for production
For production traffic where you don't care about which protocol you land on, leave it auto. The lib handles Alt-Svc, the H3 race, and ALPN fallback for you. For automation that targets specific bot products, force the protocol your preset is shaped for. Most "chrome-148" presets are tuned for H2 and H3 in parallel, but if you're matching against a capture that was specifically H3, force H3 so you don't accidentally compare an H2 fingerprint to an H3 capture.
:::

## See also

- [HTTP/1.1](./http1) for what H1 negotiates and when it's the right call.
- [HTTP/2](./http2) for the SETTINGS, WINDOW_UPDATE, and Akamai signals on H2.
- [HTTP/3 (QUIC)](./http3-quic) for the QUIC INITIAL packet and PRIORITY_UPDATE.
- [Akamai shorthand](/fingerprinting/akamai-shorthand) for tweaking H2 fingerprint values without rebuilding a preset.
