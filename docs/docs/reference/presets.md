---
title: Presets
sidebar_position: 2
---

# Presets

Every fingerprint preset that ships with httpcloak. Use this as the lookup when you need to know which preset gives which JA3, JA4, Akamai HTTP/2 hash, and User-Agent.

The list below is the registry from `fingerprint/presets.go`. The newer Chrome versions (147, 148) live in the embedded JSON registry (`fingerprint/embedded/*.json`) and inherit from older presets via the `based_on` field.

:::tip
You can always check what a real browser sends today at [tls.peet.ws/api/all](https://tls.peet.ws/api/all). Chrome being a lil bitch won't show you header order in DevTools, so peet is your only source of truth there.
:::

---

## Aliases

The `-latest` aliases follow the newest version we track. They're not separate fingerprints, just pointers.

| Alias | Resolves to |
|---|---|
| `chrome-latest` | `chrome-148` (auto-detects host OS) |
| `chrome-latest-windows` | `chrome-148-windows` |
| `chrome-latest-linux` | `chrome-148-linux` |
| `chrome-latest-macos` | `chrome-148-macos` |
| `chrome-latest-ios` | `chrome-148-ios` |
| `chrome-latest-android` | `chrome-148-android` |
| `firefox-latest` | `firefox-148` |
| `safari-latest` | `safari-18` |
| `safari-latest-ios` | `safari-18-ios` |
| `ios-chrome-latest` | `chrome-148-ios` (back-compat naming) |
| `ios-safari-latest` | `safari-18-ios` (back-compat naming) |
| `android-chrome-latest` | `chrome-148-android` (back-compat naming) |

`chrome-148` (no platform suffix) auto-detects the running OS and dispatches to `chrome-148-windows`, `chrome-148-macos`, or `chrome-148-linux`. Use this if your binary needs to look like the host OS. Use the explicit suffix when you want a Windows fingerprint from a Linux box (the common scraping case).

---

## Captured hashes (verified against tls.peet.ws)

These captures are from the Linux build host on `2026-05-10`. Captured via `NewSession(preset).Get(ctx, "https://tls.peet.ws/api/all")`. The `protocol` column is what tls.peet observed (H2 because peet doesn't advertise H3).

| Preset | Protocol | JA3 hash | JA4 | Akamai HTTP/2 hash | PeetPrint |
|---|---|---|---|---|---|
| `chrome-latest` (resolves to `chrome-148-linux`) | h2 | `51c8a5ff78d815668581664c5789d09c` | `t13d1516h2_8daaf6152771_d8a2da3f94cd` | `52d84b11737d980aef856699f885ca86` | `1d4ffe9b0e34acac0bd883fa7f79d7b5` |
| `chrome-148-windows` | h2 | `f592f2dfba4cdfc1b18ed1f29df8c8b7` | `t13d1516h2_8daaf6152771_d8a2da3f94cd` | `52d84b11737d980aef856699f885ca86` | `1d4ffe9b0e34acac0bd883fa7f79d7b5` |
| `firefox-148` | h2 | `6f7889b9fb1a62a9577e685c1fcfa919` | `t13d1717h2_5b57614c22b0_3cbfd9057e0d` | `6ea73faa8fc5aac76bded7bd238f6433` | `89d89662b21018947a9a46658c4f5ede` |
| `safari-18` | h2 | `c8af4d593e65bd6ba927ef9a0bdef541` | `t13d2013h2_a09f3c656075_7f0f34a4126d` | `90d8353e47699c4c38ecd773e9b5a089` | `62b834de729e78a9f0ebd1dd099314a7` |
| `safari-18-ios` | h2 | `e7c59d91e34d9d83e510732edf732b83` | `t13d2013h2_a09f3c656075_7f0f34a4126d` | `90d8353e47699c4c38ecd773e9b5a089` | `62b834de729e78a9f0ebd1dd099314a7` |

Notes:

- `chrome-latest` and `chrome-148-windows` differ on `ja3_hash` because the cipher / extension order is platform-specific (Linux uses `HelloChrome_148_Linux`, Windows uses `HelloChrome_148_Windows`). JA4 collapses to the same value because JA4 is order-insensitive in the cipher / extension portions.
- `safari-18` and `safari-18-ios` share the JA4 + Akamai because the H2 stack is identical; the JA3 differs because of platform-specific ClientHello extensions.

For other presets, capture them yourself the same way. Or read the JSON in `fingerprint/embedded/<name>.json` for the static parts (UA, sec-ch-ua, header order).

---

## Chrome desktop

The Chrome desktop family. Versions 143-146 are Go-defined in `fingerprint/presets.go`. Versions 147 and 148 are JSON in `fingerprint/embedded/` that inherit from 146 / 147 with only the User-Agent and `sec-ch-ua` brand list changed.

| Preset | UA | `sec-ch-ua` | Notes |
|---|---|---|---|
| `chrome-148-windows` | `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36` | `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"` | Inherits TLS from `chrome-147-windows`. |
| `chrome-148-linux` | `Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36` | `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"` | Inherits TLS from `chrome-147-linux`. |
| `chrome-148-macos` | `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36` | `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"` | Inherits TLS from `chrome-147-macos`. |
| `chrome-147-windows` | `...Chrome/147.0.0.0 Safari/537.36` | `"Google Chrome";v="147", "Chromium";v="147", "Not.A/Brand";v="8"` | Inherits TLS from `chrome-146-windows`. |
| `chrome-147-linux` | `...Chrome/147.0.0.0 Safari/537.36` | `"Google Chrome";v="147", "Chromium";v="147", "Not.A/Brand";v="8"` | Inherits TLS from `chrome-146-linux`. |
| `chrome-147-macos` | `...Chrome/147.0.0.0 Safari/537.36` | `"Google Chrome";v="147", "Chromium";v="147", "Not.A/Brand";v="8"` | Inherits TLS from `chrome-146-macos`. |
| `chrome-146-windows` | `...Chrome/146.0.0.0 Safari/537.36` | `"Google Chrome";v="146", "Chromium";v="146", "Not.A/Brand";v="8"` | Native Go preset. ClientHello: `HelloChrome_146_Windows`. |
| `chrome-146-linux` | `...Chrome/146.0.0.0 Safari/537.36` | `"Google Chrome";v="146", "Chromium";v="146", "Not.A/Brand";v="8"` | Native Go preset. ClientHello: `HelloChrome_146_Linux`. |
| `chrome-146-macos` | `...Chrome/146.0.0.0 Safari/537.36` | `"Google Chrome";v="146", "Chromium";v="146", "Not.A/Brand";v="8"` | Native Go preset. ClientHello: `HelloChrome_146_macOS`. |
| `chrome-145-{windows,linux,macos}` | `...Chrome/145.0.0.0...` | matching brand list | Native Go preset, per-platform ClientHello. |
| `chrome-144-{windows,linux,macos}` | `...Chrome/144.0.0.0...` | matching brand list | Native Go preset, per-platform ClientHello. |
| `chrome-143-{windows,linux,macos}` | `...Chrome/143.0.0.0...` | matching brand list | Native Go preset, per-platform ClientHello. |
| `chrome-141` | `...Chrome/141.0.0.0...` | matching brand list | Auto-detects platform. |
| `chrome-133` | `...Chrome/133.0.0.0...` | matching brand list | Auto-detects platform. |

The unsuffixed `chrome-148` / `chrome-147` / `chrome-146` / `chrome-145` / `chrome-144` / `chrome-143` resolve to the platform-suffixed variant matching the host OS at runtime. For consistent results across machines, use the suffix.

All Chrome desktop presets:

- Support HTTP/3 via `tls.HelloChrome_<v>_QUIC` plus a PSK variant for resumption.
- Use the Chrome H2 fingerprint config (`chromeH2Config`): pseudo-header order `m,a,s,p`, settings order from real Chrome captures, RFC 7540 priorities sent on `HEADERS`.
- Default Akamai: `1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p`.

---

## Chrome iOS

iOS Chrome is a WebKit wrapper, so its TLS fingerprint matches Safari iOS, not desktop Chrome.

| Preset | UA | TLS | Notes |
|---|---|---|---|
| `chrome-148-ios` | `...CriOS/148.0.0.0 Mobile/15E148 Safari/604.1` | `HelloIOS_18` | Inherits from `chrome-146-ios`. |
| `chrome-147-ios` | `...CriOS/147.0.0.0 Mobile/15E148 Safari/604.1` | `HelloIOS_18` | Inherits from `chrome-146-ios`. |
| `chrome-146-ios` | `...CriOS/146.0.0.0 Mobile/15E148 Safari/604.1` | `HelloIOS_18` | Native Go preset. |
| `chrome-145-ios` | `...CriOS/145.0.0.0...` | `HelloIOS_18` | Native Go preset. |
| `chrome-144-ios` | `...CriOS/144.0.0.0...` | `HelloIOS_18` | Native Go preset. |
| `chrome-143-ios` | `...CriOS/143.0.0.0...` | `HelloIOS_18` | Native Go preset. |

All iOS Chrome presets:

- Use the iOS Safari H2 stack (`safariH2Config`): pseudo `m,s,p,a`, `NO_RFC7540_PRIORITIES=1`.
- Support HTTP/3 via `HelloIOS_18_QUIC`.
- Default Akamai: `2:0;4:2097152;3:100;5:16384;9:1|10485760|0|m,s,p,a`.

---

## Chrome Android

Chrome on Android uses Chrome's native TLS stack (not WebKit-restricted like iOS).

| Preset | UA | TLS | Notes |
|---|---|---|---|
| `chrome-148-android` | `Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Mobile Safari/537.36` | `HelloChrome_<v>_Linux` | Inherits from `chrome-147-android`. |
| `chrome-147-android` | `...Chrome/147.0.0.0 Mobile Safari/537.36` | `HelloChrome_<v>_Linux` | Inherits from `chrome-146-android`. |
| `chrome-146-android` | `...Chrome/146.0.0.0 Mobile Safari/537.36` | `HelloChrome_146_Linux` | Native Go preset. |
| `chrome-145-android` | `...Chrome/145.0.0.0...` | `HelloChrome_145_Linux` | Native Go preset. |
| `chrome-144-android` | `...Chrome/144.0.0.0...` | `HelloChrome_144_Linux` | Native Go preset. |
| `chrome-143-android` | `...Chrome/143.0.0.0...` | `HelloChrome_143_Linux` | Native Go preset. |

The `sec-ch-ua-mobile` value is `?1` (vs `?0` for desktop), `sec-ch-ua-platform` is `"Android"`. Default Akamai matches Chrome desktop (because the H2 stack is the same).

---

## Firefox

Firefox uses a totally different TLS extension order from Chrome (compare the JA3s above).

| Preset | UA | TLS | Notes |
|---|---|---|---|
| `firefox-148` | `Mozilla/5.0 (...; rv:148.0) Gecko/20100101 Firefox/148.0` | JA3 mode (no native uTLS Firefox 148) | H3 not supported (no Firefox QUIC fingerprint in uTLS). |
| `firefox-133` | `Mozilla/5.0 (...; rv:133.0) Gecko/20100101 Firefox/133.0` | uTLS `HelloFirefox_133` | H3 not supported. |

All Firefox presets:

- Use the Firefox H2 stack (`firefoxH2Config`): pseudo `m,p,a,s`, `ENABLE_PUSH=0`, custom HPACK ordering.
- Send `TE: trailers` (Chrome doesn't).
- Default Akamai for `firefox-148`: `1:65536;2:0;4:131072;5:16384|12517377|0|m,p,a,s`.
- Don't send `sec-ch-ua` headers (Firefox doesn't implement Client Hints).

---

## Safari (macOS)

| Preset | UA | TLS | Notes |
|---|---|---|---|
| `safari-18` | `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15` | `HelloSafari_18` | H3 via `HelloIOS_18_QUIC` (Safari shares iOS QUIC). |

Safari macOS:

- Pseudo order `m,s,p,a` (different from Chrome and Firefox).
- `NO_RFC7540_PRIORITIES=1`. Safari opts out of stream priorities.
- No `sec-ch-ua`. Header order is shorter than Chrome.
- Default Akamai: `2:0;4:2097152;3:100;5:16384;9:1|10485760|0|m,s,p,a`.

---

## Safari iOS

| Preset | UA | TLS | Notes |
|---|---|---|---|
| `safari-18-ios` | `Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1` | `HelloIOS_18` | H3 via `HelloIOS_18_QUIC`. |
| `safari-17-ios` | `...iPhone OS 17_0...Version/17.0 Mobile...` | `HelloIOS_17` | Older variant. |

iOS Safari shares the H2 fingerprint with macOS Safari but uses a slightly different TLS extension order (visible in the JA3, identical in JA4).

---

## Per-preset details

For the static parts of any preset (UA, sec-ch-ua, header order, H2 settings), the source of truth is the JSON file in `fingerprint/embedded/` (for v147+) or the Go function in `fingerprint/presets.go` (for v146 and earlier).

To dump any preset as canonical JSON:

```go
import "github.com/sardanioss/httpcloak/fingerprint"

j, err := fingerprint.Describe("chrome-148-windows")
// j is the round-trip-stable JSON form
```

The same JSON loads back via `fingerprint.LoadPresetFromJSON` and `fingerprint.BuildPreset` without modification, so this is also how you'd snapshot a preset to disk for diffing across versions.

---

## Picking a preset

Quick guide:

| Goal | Use |
|---|---|
| Just scrape something modern | `chrome-latest` |
| Looking like a Windows desktop user | `chrome-latest-windows` |
| Looking like a phone | `chrome-latest-android` or `safari-latest-ios` |
| Sites that allowlist Firefox quirks (HPACK, TE: trailers) | `firefox-latest` |
| Sites that block Chrome but pass Safari | `safari-latest` |
| Pinning to a specific version for reproducibility | `chrome-148-windows` (no `-latest`) |

For sites that fingerprint the TCP/IP stack (uncommon, but some bot-management products do it), pair the preset with `WithTCPFingerprint(...)` to spoof TTL / window size / MSS.

For everything else: start with `chrome-latest`, capture against `tls.peet.ws/api/all`, compare against a real Chrome from the same OS, file an issue if the JA3 / Akamai / JA4 doesn't match.
