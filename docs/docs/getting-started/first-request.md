---
title: First Request
sidebar_position: 2
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# First Request

httpcloak is an HTTP client that puts the same bytes on the wire as a real browser. Same TLS ClientHello, same HTTP/2 SETTINGS frame, same header order, same priority frames. If a site fingerprints your client, httpcloak shows up looking like Chrome (or Firefox, or Safari) instead of Go's net/http or Python requests.

This page is the four-line "does it work" check. Pick your language, copy the snippet, run it, you should get a 200 back from `tls.peet.ws/api/all` with a Chrome-shaped fingerprint in the response.

## The snippet

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

	fmt.Println("status:", resp.StatusCode)
	fmt.Println("protocol:", resp.Protocol)

	body, _ := resp.Text()
	fmt.Println(body)
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

with httpcloak.Session(preset="chrome-latest", timeout=30) as session:
    r = session.get("https://tls.peet.ws/api/all")
    print("status:", r.status_code)
    print("protocol:", r.http_version)
    print(r.text)
```

</TabItem>
<TabItem value="node" label="Node.js">

```javascript
const { Session } = require("httpcloak");

(async () => {
  const session = new Session({ preset: "chrome-latest", timeout: 30 });
  try {
    const r = await session.get("https://tls.peet.ws/api/all");
    console.log("status:", r.statusCode);
    console.log("protocol:", r.httpVersion);
    console.log(r.text);
  } finally {
    session.close();
  }
})();
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using HttpCloak;

using var session = new Session(preset: "chrome-latest", timeout: 30);
var r = session.Get("https://tls.peet.ws/api/all");
Console.WriteLine($"status: {r.StatusCode}");
Console.WriteLine($"protocol: {r.HttpVersion}");
Console.WriteLine(r.Text);
```

</TabItem>
</Tabs>

## What you should see

The full response is a chunky JSON blob with TLS, HTTP/2, and header data. Trimmed to the parts that matter:

```json
{
  "http_version": "h2",
  "tls": {
    "ja3_hash": "55ecc08008f90a8b2a5c5289ab0f8b69",
    "ja4": "t13d1516h2_8daaf6152771_d8a2da3f94cd"
  },
  "http2": {
    "akamai_fingerprint": "1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p",
    "akamai_fingerprint_hash": "52d84b11737d980aef856699f885ca86"
  },
  "user_agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
}
```

A few things to notice:

- `http_version` is `h2`. Chrome speaks HTTP/2 by default to anything ALPN-capable, so we did too. HTTP/3 will kick in if the server advertises it via Alt-Svc. Force one with `WithForceHTTP2()` / `WithForceHTTP3()` if you want.
- `ja4` is stable across runs for the same preset. `ja3_hash` is not, because Chrome shuffles GREASE extension values per ClientHello and that ends up in the JA3 string. JA4 strips GREASE. Use JA4 for matching, not JA3.
- `akamai_fingerprint_hash` is the H2 SETTINGS + WINDOW_UPDATE + PRIORITY + pseudo-header-order combo. It should match what real Chrome 148 sends.

:::tip tls.peet.ws is your friend
Bookmark `tls.peet.ws/api/all`. Anytime you change a preset, add a custom JA3, or wonder why a target is still flagging you, hit this endpoint and diff the response against a real browser. Chrome being a lil bitch won't show you header order in DevTools, you can check tls.peet.ws/api/all for it.
:::

## Where to next

- [Presets Explained](./presets-explained) for what `chrome-latest` actually ships with and how to pick a different one.
- [Common Options](./common-options) for timeouts, retries, redirects, and the boring stuff every client has.
- [Fingerprinting overview](/fingerprinting) once you want to start tweaking the wire bytes yourself.
