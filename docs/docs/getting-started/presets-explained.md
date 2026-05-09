---
title: Presets Explained
sidebar_position: 3
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Presets Explained

A preset is the full bundle of wire-level fingerprint data for one specific browser build. When you write `NewSession("chrome-148-windows")`, you're loading:

- The TLS ClientHello: cipher list, extension order, supported groups, ALPN, signature algorithms, key shares, GREASE positions, ECH config, the whole lot.
- HTTP/2 SETTINGS frame values, the WINDOW_UPDATE delta, PRIORITY frame defaults, and the pseudo-header order on every request.
- Default request headers in the exact order Chrome sends them, including `sec-ch-ua`, `accept`, `sec-fetch-*`, and so on.
- Per-resource priority (RFC 7540 stream priorities for H2, RFC 9218 `priority` headers for H2/H3) keyed by `sec-fetch-dest`.
- For HTTP/3, the QUIC initial parameters and the same H3 SETTINGS frame Chrome ships with.

You don't pick TLS and headers separately. You pick a browser and a build, and the whole bundle moves together. That's the only way to stay self-consistent. A Chrome 148 ClientHello with Firefox header order is going to look weird to anyone fingerprinting hard.

## How to pick one

Start with `chrome-latest`. It tracks the newest Chrome we've shipped. If you need a specific OS variant (sites that gate on `sec-ch-ua-platform`), pick `chrome-latest-windows`, `chrome-latest-linux`, `chrome-latest-macos`, `chrome-latest-android`, or `chrome-latest-ios`.

If you need to pin a build (e.g., a target only blocks Chrome 148, you want to look like Chrome 144), use the explicit version string: `chrome-144-windows`.

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

Every preset family ships per-OS variants where the underlying browser actually has OS-specific differences. Chrome differs across Windows, Linux, macOS, Android, and iOS (because iOS Chrome is WebKit under the hood). Firefox doesn't really differ across desktop OSes so we ship one desktop build.

**Chrome desktop**: 133, 141, 143, 144, 145, 146, 147, 148. Each has `-windows`, `-linux`, `-macos` suffixes (so `chrome-148-windows` etc). The bare `chrome-148` is an alias for the OS the library defaults to.

**Chrome mobile**: `chrome-143-android` through `chrome-148-android`, and `chrome-143-ios` through `chrome-148-ios`. Mobile Chrome has its own UA, sec-ch-ua values, and on iOS, a totally different TLS stack (WebKit).

**Firefox**: 133 and 148 desktop. No per-OS split.

**Safari**: `safari-18` desktop, `safari-17-ios`, `safari-18-ios`.

**Latest aliases**: `chrome-latest`, `chrome-latest-windows`, `chrome-latest-linux`, `chrome-latest-macos`, `chrome-latest-android`, `chrome-latest-ios`, `firefox-latest`, `safari-latest`, `safari-latest-ios`. These re-point to the newest shipped major when we publish a new release. Use them when you don't care about pinning to an exact build.

For the full list with protocol support per preset, see [Reference: Presets](/reference/presets).

## Inheritance: the `based_on` chain

Presets aren't standalone JSON blobs. They form a chain. Open `fingerprint/embedded/chrome-148-windows.json` and you'll see something like:

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

That's it. The Chrome 148 windows preset is "Chrome 147 windows but bump the UA and sec-ch-ua to 148". TLS settings, H2 settings, header order, priority table, all inherited verbatim.

This is by design. Real Chrome rarely changes its TLS or H2 wire format between minor versions. Most of what's "new in Chrome 148" is JS engine and rendering, not network. So we only ship a delta when the wire actually changed (cipher reordering, new extension, SETTINGS bump etc), and for everything else we just bump the UA string in the `based_on` chain.

The chain bottoms out at a "root" preset with the full TLS/H2 spec. Loading `chrome-148-windows` walks the chain at startup and merges layer by layer.

If you want to see the resolved bundle for any preset (post-merge), there's a describe API in the library. From Go: `fingerprint.Describe("chrome-148-windows")`. From Python: `httpcloak.describe_preset("chrome-148-windows")`.

## Building your own

You can drop a JSON file in your app, point httpcloak at it, and now you have a custom preset. Useful when you've captured a real browser session and want to ship that exact fingerprint without hand-editing Go code.

The shape is the same as the embedded files, with `based_on` optional (point it at any registered preset to inherit, or omit and supply the full TLS+H2 spec yourself).

See [JSON Preset Builder](/fingerprinting/json-preset-builder) for the full schema and a worked example.

## Where to next

- [Reference: Presets](/reference/presets) for the full table of what each preset supports (H1/H2/H3, OS, mobile/desktop).
- [What is TLS Fingerprinting](/fingerprinting/what-is-tls-fingerprinting) for context on why these wire bytes matter.
- [Custom JA3](/fingerprinting/custom-ja3) once you want to override individual TLS bits without rebuilding a whole preset.
