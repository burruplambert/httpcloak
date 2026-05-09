---
title: Debug With Wireshark
sidebar_position: 4
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Debug With Wireshark

When you genuinely need to see what httpcloak puts on the wire, dump TLS
keys and decrypt in Wireshark. This is the lowest-level view you can get
short of stepping through the library with a debugger.

:::info
If you're debugging fingerprinting issues, Wireshark + keylog is the ground
truth. tls.peet.ws is convenient but it's a server-side reconstruction, not
the actual wire bytes. They almost always agree, but when they don't,
Wireshark wins.
:::

## What you'll see

Once decrypted, you can read:

- The full TLS ClientHello, with extension order, GREASE values, key
  shares, the works.
- Every HTTP/2 frame: SETTINGS, WINDOW_UPDATE, HEADERS, PRIORITY,
  PRIORITY_UPDATE.
- HPACK-decoded headers (Wireshark does the HPACK decode for you).
- Every QUIC frame for HTTP/3, including PRIORITY_UPDATE and STREAM frames.
- Server response framing, server settings, connection-level windowing.

For HTTP/2 fingerprint checks specifically: SETTINGS values, settings
order, the first PRIORITY frame on a request stream, header order in HEADERS
frames. All of it in plain bytes.

## Step 1: Dump TLS keys from your code

Use `WithKeyLogFile` to write the SSLKEYLOGFILE format Wireshark expects.

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

`WithKeyLogFile` overrides the global `SSLKEYLOGFILE` environment variable
for this session. If you'd rather set it once for all sessions in the
process, just export the env var before running:

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

Four lines per TLS 1.3 connection. If you see fewer, the connection didn't
complete. If you see zero, the file path is wrong or the session failed
before reaching the application data stage.

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

For QUIC / HTTP/3, you'll see the same four labels plus possibly
`EARLY_TRAFFIC_SECRET` if 0-RTT was used. Wireshark handles QUIC and TLS
keys from the same file since recent versions.

## Step 3: Capture traffic

Start Wireshark BEFORE you run your program (otherwise you miss the
ClientHello).

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

Wireshark re-decodes the existing capture immediately. Any TLS connection
whose `client_random` matches a line in the keylog gets fully decrypted.

For QUIC, no extra setup needed. The same TLS keylog config works.

## Step 5: Useful display filters

After decryption, here are the filters worth knowing:

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
- `key_share` containing the same curves as your preset's `key_share_curves`
  list. For modern Chrome that's GREASE + X25519MLKEM768 + X25519.
- ALPN listing `h2` and `http/1.1` (or just `h3` for QUIC).

### HTTP/2 SETTINGS

Filter: `http2.type == 4`. The first SETTINGS frame from your client
should match your preset's settings:

- Setting 1 (HEADER_TABLE_SIZE): 65536 for Chrome.
- Setting 2 (ENABLE_PUSH): 0.
- Setting 4 (INITIAL_WINDOW_SIZE): 6291456 for Chrome.
- Setting 6 (MAX_HEADER_LIST_SIZE): 262144 for Chrome.

These should appear in the order specified by your preset's
`settings_order`. Wireshark shows the settings sequentially, so order is
visible in the tree view.

### HTTP/2 PRIORITY on first request stream

For RFC 7540 priorities (Chrome shape), the HEADERS frame on stream 1
carries a PRIORITY flag (`0x20`). Expand **HyperText Transfer Protocol 2**
→ **Stream** → **Header**. Look for:

- `Stream Dependency`: 0
- `Weight`: 256 (Chrome) or whatever your preset specifies
- `Exclusive Bit`: set

If your `stream_priority_mode` is `chrome` and you're not seeing this
priority on the first request, something's wrong.

### HTTP/3 PRIORITY_UPDATE

For HTTP/3, look for QUIC stream frames carrying H3 PRIORITY_UPDATE
(frame type 0xf0700). These should reference the actual request stream ID,
not stream 0. There was a regression in older versions where the request
stream wasn't referenced correctly, verifying this in Wireshark is the
direct way to confirm your install behaves.

### HPACK / QPACK header order

Click the HEADERS / QPACK encoder stream. Wireshark decodes the headers
into a list. The order should match your preset's `hpack_header_order`
(for H2) or QPACK ordering (for H3). Pseudo-headers come first,
specifically in the order:

```
:method  :authority  :scheme  :path
```

That's the Chrome `pseudo_order`. Other orders are visible too, Safari
puts `:scheme` before `:authority`, for instance.

## tshark for CI

If you want to assert these things in tests instead of eyeballing them:

```bash
# Extract just the SETTINGS frames as JSON
tshark -r capture.pcapng -Y "http2.type == 4" \
  -T fields -e http2.settings.identifier -e http2.settings.value

# Print all extension types in the first ClientHello
tshark -r capture.pcapng -Y "tls.handshake.type == 1" \
  -T fields -e tls.handshake.extension.type
```

You can wire those into a test harness. Capture a known-good run, compare
extension types and order on every CI run.

## Common gotchas

**Old keylog file.** If you re-use a keylog from a previous run, Wireshark
won't decrypt the new connections, different `client_random`. Always start
with a fresh file or delete before each run.

**Capture started after handshake.** If you start tshark / Wireshark
mid-connection, you'll miss the ClientHello. The keys are still valid, but
Wireshark needs the handshake bytes to associate keys with the connection.

**HTTP/3 over TCP fallback.** If your H3 dial fails and the library falls
back to H2, you'll see TCP traffic instead of UDP. That's not a bug, but
it's worth knowing. Check the `Protocol` field of your response, `h3`
means UDP, `h2` means TCP.

**Network namespace mismatch.** If your code runs in a container or netns
and tshark runs on the host, you won't see the traffic. Run tshark inside
the same namespace.

## Related

- [Akamai Shorthand](../fingerprinting/akamai-shorthand), what the SETTINGS
  values mean
- [Per-resource Priority](../fingerprinting/per-resource-priority), what
  PRIORITY frames are for
- [What is TLS Fingerprinting](../fingerprinting/what-is-tls-fingerprinting)
 , the ClientHello you're inspecting
