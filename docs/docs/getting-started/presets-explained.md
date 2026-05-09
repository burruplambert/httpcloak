---
title: Presets Explained
sidebar_position: 3
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Presets Explained

A preset is the whole bundle of wire-level fingerprint data for one specific browser build. When you write `NewSession("chrome-148-windows")`, here's what loads:

- The TLS ClientHello: cipher list, extension order, supported groups, ALPN, signature algorithms, key shares, GREASE positions, ECH config, the whole lot.
- HTTP/2 SETTINGS frame values, the WINDOW_UPDATE delta, PRIORITY frame defaults, and the pseudo-header order on every request.
- Default request headers in the exact order Chrome sends them. `sec-ch-ua`, `accept`, `sec-fetch-*`, all of it.
- Per-resource priority (RFC 7540 stream weights for H2, RFC 9218 `priority` headers for H2/H3) keyed off `sec-fetch-dest`.
- For HTTP/3: the QUIC initial parameters and the same H3 SETTINGS frame Chrome ships with.

You don't pick TLS and headers separately. You pick a browser and a build, and the whole bundle moves together. That's the only way to stay self-consistent. A Chrome 148 ClientHello with Firefox header order is going to look busted to anyone fingerprinting hard.

## How to pick one

Start with `chrome-latest`. It tracks the newest Chrome that's shipped. If you need a specific OS variant (some sites gate on `sec-ch-ua-platform`), grab `chrome-latest-windows`, `chrome-latest-linux`, `chrome-latest-macos`, `chrome-latest-android`, or `chrome-latest-ios`.

Need to pin a specific build? Say a target's blocking Chrome 148 specifically and you'd rather look like Chrome 144. Use the explicit version string: `chrome-144-windows`.

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
// Default desktop Chrome, OS picked by the latest alias map
sess := httpcloak.NewSession("chrome-latest")

// Mobile UA + matching TLS for a phone-shaped fingerprint
mobile := httpcloak.NewSession("chrome-148-android")

// Pinned to an older build
old := httpcloak.NewSession("chrome-144-windows")
```

</TabItem>
<TabItem value="python" label="Python">

```python
session = httpcloak.Session(preset="chrome-latest")
mobile  = httpcloak.Session(preset="chrome-148-android")
old     = httpcloak.Session(preset="chrome-144-windows")
```

</TabItem>
<TabItem value="node" label="Node.js">

```javascript
const session = new Session({ preset: "chrome-latest" });
const mobile  = new Session({ preset: "chrome-148-android" });
const old     = new Session({ preset: "chrome-144-windows" });
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using var session = new Session(preset: "chrome-latest");
using var mobile  = new Session(preset: "chrome-148-android");
using var old     = new Session(preset: "chrome-144-windows");
```

</TabItem>
</Tabs>

## What's available

Every preset family ships per-OS variants wherever the underlying browser actually differs by OS. Chrome differs across Windows, Linux, macOS, Android, and iOS (iOS Chrome is WebKit under the hood, totally different stack). Firefox barely differs across desktop OSes, so there's one desktop build.

**Chrome desktop**: 133, 141, 143, 144, 145, 146, 147, 148. Each has `-windows`, `-linux`, `-macos` suffixes (so `chrome-148-windows` and friends). Bare `chrome-148` aliases to whatever OS the library defaults to.

**Chrome mobile**: `chrome-143-android` through `chrome-148-android`, and `chrome-143-ios` through `chrome-148-ios`. Mobile Chrome has its own UA, its own sec-ch-ua values, and on iOS a completely different TLS stack (WebKit again).

**Firefox**: 133 and 148 desktop. No per-OS split.

**Safari**: `safari-18` desktop, `safari-17-ios`, `safari-18-ios`.

**Latest aliases**: `chrome-latest`, `chrome-latest-windows`, `chrome-latest-linux`, `chrome-latest-macos`, `chrome-latest-android`, `chrome-latest-ios`, `firefox-latest`, `safari-latest`, `safari-latest-ios`. These re-point to the newest shipped major on every release. Use them when you don't care about pinning to an exact build.

For the full list with protocol support per preset, see [Reference: Presets](/reference/presets).

## Inheritance: the `based_on` chain

Presets aren't standalone JSON blobs. They form a chain. Crack open `fingerprint/embedded/chrome-148-windows.json` and you'll see something like this:

```json
{
  "version": 1,
  "preset": {
    "name": "chrome-148-windows",
    "based_on": "chrome-147-windows",
    "headers": {
      "user_agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
      "values": {
        "sec-ch-ua": "\"Chromium\";v=\"148\", \"Google Chrome\";v=\"148\", \"Not/A)Brand\";v=\"99\""
      }
    }
  }
}
```

That's the whole file. Chrome 148 windows is just "Chrome 147 windows but bump the UA and sec-ch-ua to 148". TLS settings, H2 settings, header order, priority table, all inherited verbatim.

It's deliberate. Real Chrome rarely changes its TLS or H2 wire format between minor versions. Most of what's "new in Chrome 148" is JS engine and rendering, not network. So a delta only ships when the wire actually changes (cipher reordering, new extension, SETTINGS bump, that kind of thing). Everything else is a UA bump on top of the `based_on` chain.

The chain bottoms out at a "root" preset that carries the full TLS/H2 spec. Loading `chrome-148-windows` walks the chain at startup and merges layer by layer.

Want to see the resolved bundle (post-merge)? There's a describe API. From Go: `fingerprint.Describe("chrome-148-windows")`. From Python: `httpcloak.describe_preset("chrome-148-windows")`.

## Building your own

Drop a JSON file in your app, point httpcloak at it, and you've got a custom preset. Handy when you've captured a real browser session and want to ship that exact fingerprint without hand-editing Go code.

Shape is the same as the embedded files. `based_on` is optional: point it at any registered preset to inherit, or omit it and supply the full TLS+H2 spec yourself.

See [JSON Preset Builder](/fingerprinting/json-preset-builder) for the full schema and a worked example.

## Where to next

- [Reference: Presets](/reference/presets) for the full table of what each preset supports (H1/H2/H3, OS, mobile/desktop).
- [What is TLS Fingerprinting](/fingerprinting/what-is-tls-fingerprinting) for context on why these wire bytes matter.
- [Custom JA3](/fingerprinting/custom-ja3) when you want to override individual TLS bits without rebuilding a whole preset.
