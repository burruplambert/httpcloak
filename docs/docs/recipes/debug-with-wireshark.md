---
title: Debug With Wireshark
sidebar_position: 4
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Debug With Wireshark

When you really need to see what httpcloak puts on the wire, dump TLS keys
and decrypt in Wireshark. Lowest-level view you can get short of stepping
through the library with a debugger.

:::info
For fingerprinting issues, Wireshark plus keylog is the ground truth.
tls.peet.ws is handy but it's a server-side reconstruction, not the actual
wire bytes. They mostly agree. When they don't, Wireshark wins.
:::

## What you'll see

Once decrypted, you can read:

- The full TLS ClientHello with extension order, GREASE values, key shares,
  the works.
- Every HTTP/2 frame: SETTINGS, WINDOW_UPDATE, HEADERS, PRIORITY,
  PRIORITY_UPDATE.
- HPACK-decoded headers (Wireshark does the HPACK decode for you).
- Every QUIC frame for HTTP/3, including PRIORITY_UPDATE and STREAM frames.
- Server response framing, server settings, connection-level windowing.

For HTTP/2 fingerprint checks specifically: SETTINGS values, settings order,
the first PRIORITY frame on a request stream, header order in HEADERS
frames. All in plain bytes.

## Step 1: Dump TLS keys from your code

Use `WithKeyLogFile` to write the SSLKEYLOGFILE format Wireshark wants.

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/sardanioss/httpcloak"
)

func main() {
    keyLogPath := "/tmp/sslkeys.log"
    // Make sure the file is fresh, old keys won't decrypt new traffic.
    os.Remove(keyLogPath)

    s := httpcloak.NewSession("chrome-latest",
        httpcloak.WithKeyLogFile(keyLogPath),
        httpcloak.WithSessionTimeout(30*time.Second),
    )
    defer s.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    r, err := s.Get(ctx, "https://tls.peet.ws/api/all")
    if err != nil {
        fmt.Println("err:", err); os.Exit(1)
    }
    defer r.Close()
    fmt.Println("status:", r.StatusCode, "proto:", r.Protocol)
    fmt.Println("keys written to:", keyLogPath)
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import os
import httpcloak

keylog = "/tmp/sslkeys.log"
if os.path.exists(keylog):
    os.remove(keylog)

with httpcloak.Session("chrome-latest", key_log_file=keylog, timeout=30) as s:
    r = s.get("https://tls.peet.ws/api/all")
    print(f"status={r.status_code} proto={r.protocol}")
    print(f"keys written to {keylog}")
```

</TabItem>
</Tabs>

`WithKeyLogFile` overrides the global `SSLKEYLOGFILE` env var for this one
session. Want to set it once for every session in the process? Export the
env var before running:

```bash
export SSLKEYLOGFILE=/tmp/sslkeys.log
go run main.go
```

httpcloak picks it up at startup automatically.

## Step 2: Verify the keylog file

After your program runs, the file should look like this (one line per
secret, NSS keylog format):

```
CLIENT_HANDSHAKE_TRAFFIC_SECRET <client_random> <secret>
SERVER_HANDSHAKE_TRAFFIC_SECRET <client_random> <secret>
CLIENT_TRAFFIC_SECRET_0 <client_random> <secret>
SERVER_TRAFFIC_SECRET_0 <client_random> <secret>
```

Four lines per TLS 1.3 connection. Fewer than that and the connection
didn't complete. Zero lines and either the file path is wrong or the
session died before hitting the application data stage.

Quick sanity check:

```bash
$ wc -l /tmp/sslkeys.log
4 /tmp/sslkeys.log

$ awk '{print $1}' /tmp/sslkeys.log | sort -u
CLIENT_HANDSHAKE_TRAFFIC_SECRET
CLIENT_TRAFFIC_SECRET_0
SERVER_HANDSHAKE_TRAFFIC_SECRET
SERVER_TRAFFIC_SECRET_0
```

That's a healthy single-connection keylog.

For QUIC / HTTP/3, you'll see the same four labels plus maybe
`EARLY_TRAFFIC_SECRET` if 0-RTT fired. Recent Wireshark versions read QUIC
and TLS keys from the same file.

## Step 3: Capture traffic

Start Wireshark BEFORE you run your program. Otherwise you miss the
ClientHello and the rest of the trace is uselessly opaque.

Useful capture filters:

| Filter | What it captures |
|--------|------------------|
| `tcp port 443` | All HTTPS over TCP (H1, H2) |
| `udp port 443` | All QUIC (H3) |
| `tcp port 443 or udp port 443` | Both |
| `host tls.peet.ws` | Just traffic to a specific host |

For headless work, `tshark` is the CLI version:

```bash
sudo tshark -i any -f "tcp port 443 or udp port 443" -w /tmp/capture.pcapng
```

Run your Go / Python program while tshark captures, then Ctrl-C tshark.

## Step 4: Point Wireshark at the keylog

In Wireshark:

1. **Edit** → **Preferences** → **Protocols** → **TLS**.
2. **(Pre)-Master-Secret log filename** → `/tmp/sslkeys.log`.
3. OK.

Wireshark re-decodes the capture right there. Any TLS connection whose
`client_random` matches a line in the keylog gets fully decrypted.

QUIC needs zero extra setup. Same TLS keylog config covers it.

## Step 5: Useful display filters

After decryption, the filters worth knowing:

| Filter | Shows |
|--------|-------|
| `tls.handshake.type == 1` | Just ClientHello frames |
| `http2` | All HTTP/2 frames |
| `http2.type == 4` | HTTP/2 SETTINGS frames |
| `http2.type == 2` | HTTP/2 PRIORITY frames |
| `http2.type == 1` | HTTP/2 HEADERS frames |
| `http2.type == 12` | HTTP/2 PRIORITY_UPDATE (RFC 9218) |
| `quic` | All QUIC frames |
| `http3` | All HTTP/3 frames |
| `http3.frame_type == 0xf0700` | H3 PRIORITY_UPDATE |
| `tls.handshake.extension.type` | Group by extension type |

## Things to look for

### TLS ClientHello

Click the ClientHello, expand **Secure Sockets Layer** → **TLS** →
**Handshake** → **Extensions**. You should see:

- Extension order matching your preset's JA4 / peetprint.
- GREASE extension at position 0 (Chrome-style presets only).
- `key_share` carrying the same curves as your preset's `key_share_curves`
  list. Modern Chrome ships GREASE + X25519MLKEM768 + X25519.
- ALPN listing `h2` and `http/1.1` (or just `h3` for QUIC).

### HTTP/2 SETTINGS

Filter: `http2.type == 4`. The first SETTINGS frame from your client
should match your preset's settings:

- Setting 1 (HEADER_TABLE_SIZE): 65536 for Chrome.
- Setting 2 (ENABLE_PUSH): 0.
- Setting 4 (INITIAL_WINDOW_SIZE): 6291456 for Chrome.
- Setting 6 (MAX_HEADER_LIST_SIZE): 262144 for Chrome.

These should land in the order your preset's `settings_order` specifies.
Wireshark shows them sequentially, so order is visible right in the tree
view.

### HTTP/2 PRIORITY on first request stream

For RFC 7540 priorities (Chrome shape), the HEADERS frame on stream 1
carries a PRIORITY flag (`0x20`). Expand **HyperText Transfer Protocol 2**
→ **Stream** → **Header**. Look for:

- `Stream Dependency`: 0
- `Weight`: 256 (Chrome) or whatever your preset says
- `Exclusive Bit`: set

If `stream_priority_mode` is `chrome` and this priority isn't showing up on
the first request, something's busted.

### HTTP/3 PRIORITY_UPDATE

For HTTP/3, hunt for QUIC stream frames carrying H3 PRIORITY_UPDATE
(frame type 0xf0700). They should reference the actual request stream ID,
not stream 0. Older versions had a regression where the request stream
wasn't referenced correctly. Wireshark is the most direct way to confirm
your install actually behaves.

### HPACK / QPACK header order

Click the HEADERS / QPACK encoder stream. Wireshark decodes the headers
into a list. The order should match your preset's `hpack_header_order`
(H2) or QPACK ordering (H3). Pseudo-headers come first, specifically in
this order:

```
:method  :authority  :scheme  :path
```

That's the Chrome `pseudo_order`. Other shapes are visible too. Safari, for
instance, puts `:scheme` before `:authority`.

## tshark for CI

Want to assert these things in tests instead of eyeballing them:

```bash
# Extract just the SETTINGS frames as JSON
tshark -r capture.pcapng -Y "http2.type == 4" \
  -T fields -e http2.settings.identifier -e http2.settings.value

# Print all extension types in the first ClientHello
tshark -r capture.pcapng -Y "tls.handshake.type == 1" \
  -T fields -e tls.handshake.extension.type
```

Wire those into a test harness, capture a known-good run, compare extension
types and order on every CI run.

## Common gotchas

**Old keylog file.** Reusing a keylog from a previous run won't decrypt new
connections because the `client_random` is different. Always start with a
fresh file, or delete before each run.

**Capture started after the handshake.** Start tshark / Wireshark
mid-connection and you'll miss the ClientHello. The keys are still valid,
but Wireshark needs the handshake bytes to tie keys to the connection.

**HTTP/3 falling back to TCP.** If H3 dial fails and the library falls back
to H2, you'll see TCP instead of UDP. Not a bug, but worth knowing. Check
the `Protocol` field of your response: `h3` means UDP, `h2` means TCP.

**Network namespace mismatch.** Code running inside a container or netns
while tshark runs on the host? You won't see the traffic. Run tshark in the
same namespace.

## Related

- [Akamai Shorthand](../fingerprinting/akamai-shorthand), what the SETTINGS
  values mean
- [Per-resource Priority](../fingerprinting/per-resource-priority), what
  PRIORITY frames are for
- [What is TLS Fingerprinting](../fingerprinting/what-is-tls-fingerprinting)
 , the ClientHello you're inspecting
