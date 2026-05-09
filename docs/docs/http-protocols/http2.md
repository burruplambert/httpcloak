---
title: HTTP/2
sidebar_position: 3
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# HTTP/2

HTTP/2 is the default for almost every modern host. ALPN negotiates `h2`, the lib opens one TCP connection, multiplexes streams over it, compresses headers with HPACK, and frames everything in binary instead of plain text. If a target advertises `h2` in its TLS server hello, the lib will land here.

The H2 fingerprint is where most modern bot products do their heaviest checking. Header order, SETTINGS values, WINDOW_UPDATE deltas, stream priorities, and the pseudo-header (`:method`, `:authority`, `:scheme`, `:path`) order all combine into the Akamai HTTP/2 hash. Get any of those wrong and you stick out.

## httpcloak's H2 stack

The transport is a custom `http2.ClientConn` from the `sardanioss/net` fork. Same surface as Go's stdlib `net/http2` but with a few additions you need for fingerprinting:

- Per-preset SETTINGS frame values (initial window size, max frame size, max concurrent streams, header table size, enable push, max header list size).
- Configurable initial WINDOW_UPDATE on the connection. Real Chrome bumps this immediately after SETTINGS.
- RFC 7540 stream priority weight + dependency tree per request.
- RFC 9218 priority headers (`priority: u=N, i`) per request, with per-resource-type values driven by the preset's priority table.
- Pseudo-header order matching the preset.

The fork lives in `transport/http2_transport.go` and the SETTINGS / priority data lives in the preset (see [Akamai shorthand](/fingerprinting/akamai-shorthand)).

## What gets fingerprinted at H2

Six signals, listed roughly in the order an Akamai-style fingerprinter parses them:

1. **SETTINGS frame**. The values you send in your first SETTINGS, in the order you send them. Chrome ships `HEADER_TABLE_SIZE=65536`, `ENABLE_PUSH=0`, `INITIAL_WINDOW_SIZE=6291456`, `MAX_HEADER_LIST_SIZE=262144`. Different browsers ship different values and different orders.
2. **WINDOW_UPDATE delta**. Right after SETTINGS, Chrome sends a connection-level `WINDOW_UPDATE` of `15663105` bytes. The exact number is a fingerprint signal.
3. **Stream priorities (RFC 7540)**. The classic priority-frame format with weight and dependency. Deprecated by spec but still emitted by Chrome for backward compatibility, and still checked by fingerprinters.
4. **Priority headers (RFC 9218)**. The newer `priority: u=N, i` HTTP header. httpcloak picks the value per resource type via the priority table. See [per-resource priority](/fingerprinting/per-resource-priority).
5. **Pseudo-header order**. `:method`, `:authority`, `:scheme`, `:path`. Chrome's order is `m,a,s,p`. Some libraries ship `m,s,p,a` or `m,p,s,a` and that alone is enough to flag them.
6. **Regular header order**. Same as H1, but on H2 the order is preserved through HPACK and visible to anyone parsing the wire. This includes any custom headers you add.

The Akamai H2 hash collapses items 1, 2, 3, and 5 into one short string. See [Akamai shorthand](/fingerprinting/akamai-shorthand) for the exact format.

:::info RFC 7540 vs RFC 9218 priorities
RFC 7540 stream priorities (weight + dependency tree) are deprecated in favor of RFC 9218 priority headers. httpcloak ships both, so you stay compatible with old and new servers. Real Chrome 100+ also ships both for the same reason. If you're rolling your own preset, don't omit either.
:::

## Code: capture the H2 fingerprint

Default `chrome-latest` session against `tls.peet.ws/api/all`. The response shows `http_version=h2` plus the H2-specific fields the fingerprinter parsed.

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
		HTTP2       struct {
			AkamaiFingerprint     string `json:"akamai_fingerprint"`
			AkamaiFingerprintHash string `json:"akamai_fingerprint_hash"`
		} `json:"http2"`
	}
	json.Unmarshal(body, &pr)

	fmt.Println("resp.Protocol:", resp.Protocol)
	fmt.Println("http_version:", pr.HTTPVersion)
	fmt.Println("akamai_text:", pr.HTTP2.AkamaiFingerprint)
	fmt.Println("akamai_hash:", pr.HTTP2.AkamaiFingerprintHash)
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

with httpcloak.Session(preset="chrome-latest", timeout=30) as sess:
    r = sess.get("https://tls.peet.ws/api/all")
    body = r.json()
    print("resp protocol:", r.http_version)
    print("http_version:", body["http_version"])
    print("akamai_text:", body["http2"]["akamai_fingerprint"])
    print("akamai_hash:", body["http2"]["akamai_fingerprint_hash"])
```

</TabItem>
<TabItem value="node" label="Node.js">

```javascript
const { Session } = require("httpcloak");

(async () => {
  const sess = new Session({ preset: "chrome-latest", timeout: 30 });
  try {
    const r = await sess.get("https://tls.peet.ws/api/all");
    const body = JSON.parse(r.text);
    console.log("resp protocol:", r.httpVersion);
    console.log("http_version:", body.http_version);
    console.log("akamai_text:", body.http2.akamai_fingerprint);
    console.log("akamai_hash:", body.http2.akamai_fingerprint_hash);
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

using var sess = new Session(preset: "chrome-latest", timeout: 30);
var r = sess.Get("https://tls.peet.ws/api/all");
var body = JsonDocument.Parse(r.Text).RootElement;
Console.WriteLine($"resp protocol: {r.HttpVersion}");
Console.WriteLine($"http_version: {body.GetProperty("http_version").GetString()}");
var h2 = body.GetProperty("http2");
Console.WriteLine($"akamai_text: {h2.GetProperty("akamai_fingerprint").GetString()}");
Console.WriteLine($"akamai_hash: {h2.GetProperty("akamai_fingerprint_hash").GetString()}");
```

</TabItem>
</Tabs>

Expected output:

```
resp.Protocol: h2
http_version: h2
akamai_text: 1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p
akamai_hash: 52d84b11737d980aef856699f885ca86
```

Reading that `akamai_text` left-to-right:

- `1:65536;2:0;4:6291456;6:262144` is the SETTINGS frame. Setting 1 (`HEADER_TABLE_SIZE`), setting 2 (`ENABLE_PUSH`), setting 4 (`INITIAL_WINDOW_SIZE`), setting 6 (`MAX_HEADER_LIST_SIZE`).
- `15663105` is the connection-level `WINDOW_UPDATE` increment Chrome sends right after SETTINGS.
- `0` is the priority-frame block. Empty for chrome-148+ because Chrome stopped sending RFC 7540 priority frames on streams it owns. Older presets emit `1:1:0:256,...` here.
- `m,a,s,p` is the pseudo-header order: `:method`, `:authority`, `:scheme`, `:path`.

The hash at the end is just MD5 of the text. Match it against a known-good Chrome capture and you're good.

## Forcing H2

Most of the time the lib picks H2 on its own. Force it when you want predictable behavior in tests or when the target's H3 is busted:

```go
sess := httpcloak.NewSession("chrome-latest", httpcloak.WithForceHTTP2())
```

If you only want to suppress H3 but keep the H2/H1 fallback chain, use `WithDisableHTTP3()` instead. That's what most production code wants because it covers servers that mis-advertise `h3` in Alt-Svc.

```go
sess := httpcloak.NewSession("chrome-latest", httpcloak.WithDisableHTTP3())
```

## Switching mid-session

Same shape as H1. `RefreshWithProtocol("h2")` closes the pool and forces H2 from the next request onwards. Cookies and TLS tickets survive the switch.

:::tip Diff your H2 fingerprint
After every preset change, hit `tls.peet.ws/api/all` and diff the `akamai_fingerprint` text against a real Chrome capture. The hash is fine for a quick sanity check, but the text shows you exactly which knob drifted. The order of fields inside the SETTINGS block is part of the fingerprint, so a diff that says `4` and `6` swapped position will not show up in the hash if both values stayed the same in some hash implementations.
:::
