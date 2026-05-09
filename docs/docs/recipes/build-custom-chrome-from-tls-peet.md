---
title: Build Custom Chrome From tls.peet.ws
sidebar_position: 2
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Build Custom Chrome From tls.peet.ws

Take a tls.peet.ws capture from a real Chrome session and turn it into a
custom httpcloak preset. Useful when the Chrome version we ship doesn't match
what you need. Ten minutes of work.

:::tip
This is exactly how you support a Chrome version we haven't shipped yet.
Don't wait for a new release, capture, edit JSON, ship.
:::

## When to use this

You should reach for this recipe when:

- A target site checks the `User-Agent` against the major Chrome version
  number, and our shipped `chrome-latest` is one or two majors behind what
  they expect.
- You want to reproduce a specific user's setup (Linux Chrome 145, macOS
  Chrome 147, etc).
- You're debugging a fingerprint mismatch and want to verify what Chrome
  actually sends versus what httpcloak puts on the wire.

You should NOT use this for impersonating a browser we haven't profiled at
the TLS layer. The deltas this recipe handles are header / User-Agent /
sec-ch-ua deltas. If the new Chrome version added a new TLS extension or
shifted the extension order, you need a new utls profile, not just JSON.
Check [Custom JA3](../fingerprinting/custom-ja3) for that case.

## The flow

1. Open Chrome (the version you want to copy). Hit `https://tls.peet.ws/api/all`.
2. Save the response JSON.
3. Run `describe_preset("chrome-latest")` to dump the shipped preset to JSON.
4. Diff the two. Find the deltas (UA string, sec-ch-ua brand list, sometimes
   accept-language).
5. Edit the preset JSON to match the capture.
6. Load it with `load_preset_from_json` under a new name.
7. Hit tls.peet again with the new preset, verify JA4 and peetprint and
   akamai hash all match the original capture.

## Step 1: Capture from real Chrome

Open Chrome, navigate to `https://tls.peet.ws/api/all`, and save the JSON.
On Linux you can also do it from the command line if you have a real Chrome
installed:

```bash
google-chrome --headless --dump-dom https://tls.peet.ws/api/all > capture.json
```

The fields we care about (full response is much bigger):

```json
{
  "http_version": "h2",
  "user_agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
  "tls": {
    "ja3": "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,51-11-43-23-18-0-27-65281-45-16-5-17613-10-35-65037-13,4588-29-23-24,0",
    "ja3_hash": "f33ef28649dda9a281b02e75670c8139",
    "ja4": "t13d1516h2_8daaf6152771_d8a2da3f94cd",
    "peetprint_hash": "1d4ffe9b0e34acac0bd883fa7f79d7b5"
  },
  "http2": {
    "akamai_fingerprint": "1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p",
    "akamai_fingerprint_hash": "52d84b11737d980aef856699f885ca86",
    "sent_frames": [
      {
        "frame_type": "HEADERS",
        "headers": [
          ":method: GET",
          ":authority: tls.peet.ws",
          ":scheme: https",
          ":path: /api/all",
          "sec-ch-ua: \"Chromium\";v=\"148\", \"Google Chrome\";v=\"148\", \"Not/A)Brand\";v=\"99\"",
          "sec-ch-ua-mobile: ?0",
          "sec-ch-ua-platform: \"Linux\"",
          "user-agent: Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
          "accept: text/html,...",
          "..."
        ]
      }
    ]
  }
}
```

The two key things we'll read off:

1. `user_agent`, the exact Chrome version string. Check the major number,
   the platform.
2. `sec-ch-ua` header, the brand-version list. This rotates with Chrome
   majors and is a common giveaway when it's stale.

:::tip
Chrome being a lil bitch won't show you header order. You can check
`tls.peet.ws/api/all` for the exact order under `sent_frames[].headers`.
That's the ground truth for what your target sees.
:::

## Step 2: Describe the shipped preset

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
package main

import (
    "fmt"
    "os"

    "github.com/sardanioss/httpcloak/fingerprint"
)

func main() {
    j, err := fingerprint.Describe("chrome-latest")
    if err != nil {
        fmt.Println(err); os.Exit(1)
    }
    os.WriteFile("chrome-latest.json", []byte(j), 0644)
    fmt.Printf("wrote %d bytes\n", len(j))
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

j = httpcloak.describe_preset("chrome-latest")
with open("chrome-latest.json", "w") as f:
    f.write(j)
print(f"wrote {len(j)} bytes")
```

</TabItem>
</Tabs>

This dumps the entire shipped preset, fully resolved (no inheritance, no
defaults to chase). You'll see something like:

```json
{
  "version": 1,
  "preset": {
    "name": "chrome-148-linux",
    "tls": {
      "client_hello": "chrome-146-linux",
      "psk_client_hello": "chrome-146-linux-psk",
      "quic_client_hello": "chrome-146-quic",
      "quic_psk_client_hello": "chrome-146-quic-psk"
    },
    "http2": {
      "header_table_size": 65536,
      "initial_window_size": 6291456,
      "max_header_list_size": 262144,
      "settings_order": [1, 2, 4, 6],
      "pseudo_order": [":method", ":authority", ":scheme", ":path"],
      "stream_priority_mode": "chrome",
      "priority_table": { "...": "..." }
    },
    "headers": {
      "user_agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
      "values": {
        "sec-ch-ua": "\"Chromium\";v=\"148\", \"Google Chrome\";v=\"148\", \"Not/A)Brand\";v=\"99\"",
        "sec-ch-ua-mobile": "?0",
        "sec-ch-ua-platform": "\"Linux\"",
        "Accept-Language": "en-US,en;q=0.9"
      },
      "order": [
        {"key": "sec-ch-ua", "value": "..."},
        {"key": "sec-ch-ua-mobile", "value": "..."},
        {"key": "sec-ch-ua-platform", "value": "..."},
        {"key": "upgrade-insecure-requests", "value": "1"},
        {"key": "user-agent", "value": "..."},
        {"key": "accept", "value": "..."}
      ]
    }
  }
}
```

## Step 3: Diff the capture vs the preset

Three places things commonly drift:

| Field | Where in capture | Where in preset |
|-------|------------------|-----------------|
| User-Agent | `user_agent` top-level | `headers.user_agent` |
| sec-ch-ua brand list | inside `sent_frames[].headers` | `headers.values."sec-ch-ua"` |
| sec-ch-ua-platform | same | `headers.values."sec-ch-ua-platform"` |

Less common but worth checking:

- `accept-language`, defaults vary by Chrome locale.
- TLS extensions, if the capture lists an extension that's not in the JA3
  string of the shipped preset, you can't fix it with JSON. That needs a
  utls profile bump. See [What is TLS fingerprinting](../fingerprinting/what-is-tls-fingerprinting).

For our example, both the capture and the shipped preset are Chrome 148 on
Linux, so the deltas are minimal. If your capture is Chrome 150 on macOS,
you'd update:

```json
"user_agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36",

"headers": {
  "values": {
    "sec-ch-ua": "\"Chromium\";v=\"150\", \"Google Chrome\";v=\"150\", \"Not/A)Brand\";v=\"99\"",
    "sec-ch-ua-platform": "\"macOS\""
  }
}
```

## Step 4: Edit and rename

Always rename. `RegisterStrict` (the loader uses this internally) refuses to
shadow a built-in name, so you can't accidentally overwrite `chrome-latest`.

```json
{
  "version": 1,
  "preset": {
    "name": "chrome-148-linux-mine",
    "...": "everything else, edited as needed"
  }
}
```

## Step 5: Load + register

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
package main

import (
    "fmt"
    "os"

    "github.com/sardanioss/httpcloak/fingerprint"
)

func main() {
    data, err := os.ReadFile("chrome-148-linux-mine.json")
    if err != nil { fmt.Println(err); os.Exit(1) }

    p, err := fingerprint.LoadAndBuildPresetFromJSON(data)
    if err != nil { fmt.Println("build:", err); os.Exit(1) }

    if err := fingerprint.RegisterStrict("chrome-148-linux-mine", p); err != nil {
        fmt.Println("register:", err); os.Exit(1)
    }
    fmt.Println("registered chrome-148-linux-mine")
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

with open("chrome-148-linux-mine.json") as f:
    name = httpcloak.load_preset_from_json(f.read())
print(f"registered {name}")
```

</TabItem>
</Tabs>

## Step 6: Verify the round-trip

This is the actual important step. Hit tls.peet again with your new preset
and check the hashes match the original capture.

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "time"

    "github.com/sardanioss/httpcloak"
)

type peet struct {
    HTTPV string `json:"http_version"`
    UA    string `json:"user_agent"`
    TLS   struct {
        Ja4      string `json:"ja4"`
        PeetHash string `json:"peetprint_hash"`
    } `json:"tls"`
    HTTP2 struct {
        AkamaiHash string `json:"akamai_fingerprint_hash"`
    } `json:"http2"`
}

func capture(preset string) peet {
    s := httpcloak.NewSession(preset, httpcloak.WithSessionTimeout(30*time.Second))
    defer s.Close()
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    r, _ := s.Get(ctx, "https://tls.peet.ws/api/all")
    defer r.Close()
    b, _ := r.Bytes()
    var p peet
    json.Unmarshal(b, &p)
    return p
}

func main() {
    base := capture("chrome-latest")
    mine := capture("chrome-148-linux-mine")

    fmt.Printf("ja4   base=%s mine=%s match=%v\n", base.TLS.Ja4, mine.TLS.Ja4, base.TLS.Ja4 == mine.TLS.Ja4)
    fmt.Printf("peet  base=%s mine=%s match=%v\n", base.TLS.PeetHash, mine.TLS.PeetHash, base.TLS.PeetHash == mine.TLS.PeetHash)
    fmt.Printf("akama base=%s mine=%s match=%v\n", base.HTTP2.AkamaiHash, mine.HTTP2.AkamaiHash, base.HTTP2.AkamaiHash == mine.HTTP2.AkamaiHash)
    fmt.Printf("ua    base=%s\n        mine=%s\n", base.UA, mine.UA)

    if base.TLS.Ja4 != mine.TLS.Ja4 || base.TLS.PeetHash != mine.TLS.PeetHash || base.HTTP2.AkamaiHash != mine.HTTP2.AkamaiHash {
        os.Exit(1)
    }
    fmt.Println("PASS")
}
```

Run this end-to-end. You should see `PASS`. Real output from running this
recipe against the live tls.peet endpoint:

```
ja4   base=t13d1516h2_8daaf6152771_d8a2da3f94cd mine=t13d1516h2_8daaf6152771_d8a2da3f94cd match=true
peet  base=1d4ffe9b0e34acac0bd883fa7f79d7b5 mine=1d4ffe9b0e34acac0bd883fa7f79d7b5 match=true
akama base=52d84b11737d980aef856699f885ca86 mine=52d84b11737d980aef856699f885ca86 match=true
PASS
```

## Why JA3 might differ

If you compare JA3 hashes between two captures (with the same preset), they
won't match. JA3 includes raw TLS extension IDs, and Chrome rotates GREASE
values on every connection. So JA3 is unstable by design.

JA4, peetprint, and akamai hashes are GREASE-normalised. They're the right
metrics for "did my preset roundtrip correctly". If JA4 matches and
peetprint matches, you're golden, even if JA3 is different on every request.

:::warning
Don't use JA3 hash matching as your CI pass criterion. It will flake. Use
JA4 instead.
:::

## What this recipe doesn't cover

- **TLS extension order changes**: if Chrome 150 adds a new extension or
  reorders extensions, JSON edits won't help. utls needs a profile bump.
- **HTTP/2 frame ordering**: shipped presets cover all the common Chrome
  shapes. If you're seeing a frame shape the shipped preset doesn't have,
  open an issue.
- **HTTP/3**: same idea. The shipped `quic_client_hello` references handle
  the QUIC handshake bytes. JSON edits affect headers, not the QUIC layer.

For those, the path is updating utls + sardanioss/net, not authoring JSON.

## Related

- [JSON Preset Builder](../fingerprinting/json-preset-builder), full JSON
  schema reference
- [Presets](../fingerprinting/presets), what we ship
- [Custom JA3](../fingerprinting/custom-ja3), bypassing the preset system
- [Akamai Shorthand](../fingerprinting/akamai-shorthand), HTTP/2 fingerprint
  format
