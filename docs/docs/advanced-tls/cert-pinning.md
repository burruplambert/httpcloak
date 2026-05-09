---
title: Certificate Pinning
sidebar_position: 5
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Certificate Pinning

Cert pinning means: trust this exact cert (or its public key), nobody else. Even if a CA somewhere in the chain gets popped, even if a corporate proxy injects its own root, even if your friendly anti-bot vendor is silently MITMing every request you send, the handshake fails because the key on the wire isn't the one you nailed down.

That last case is the one that bites red teamers. Plenty of "transparent" inspection boxes on internal networks own a trusted root, so the standard cert chain validates fine. Pin the SPKI and the whole game falls apart for them.

## Pin types

Two flavors:

| Type | What it hashes | When to use |
|---|---|---|
| `PinTypeSHA256` | SHA256 of the cert's Subject Public Key Info (SPKI) | Default. Survives cert renewals as long as the keypair stays the same. |
| `PinTypeCertificate` | SHA256 of the full DER cert | Stricter. Breaks the second the cert renews, even with the same key. |

SPKI hashing is what HPKP and Chrome's static pin list use. Stick with SPKI unless you have a reason not to.

There's also a file-based path: load a PEM cert off disk and httpcloak grabs the SPKI for you. Same pin type under the hood, just less copy-pasting hashes around.

## Client-level pinning

The fastest way. The `*client.Client` exposes pin methods directly:

```go
import "github.com/sardanioss/httpcloak/client"

c := client.NewClient("chrome-latest")

// Pin by base64 SPKI hash
c.PinCertificate("YSxNUV05SLc2H4Z6kOXWCsUPPMenylyBVtogFlUiByE=", client.ForHost("example.com"))

// Or load it from a PEM file
_ = c.PinCertificateFromFile("/etc/ssl/example.com.crt", client.ForHost("example.com"))

// Drop everything
c.ClearPins()

// Get the underlying pinner if you want raw control
pinner := c.CertPinner()
```

`PinCertificate` is the one you'll reach for 90% of the time. Pass the base64 SPKI hash, optionally scope it with `ForHost(...)` and `IncludeSubdomains()`, done.

`PinCertificateFromFile` parses a PEM cert and extracts the SPKI for you. Useful when you've got the cert sitting in a file and don't want to pipe it through openssl.

`ClearPins` wipes every pin on the client. `CertPinner` hands you the pinner so you can call `AddPin`, `GetPins`, `HasPins` directly.

## Standalone CertPinner

You can also build a pinner outside any client:

```go
p := client.NewCertPinner()

p.AddPin("YSxNUV05SLc2H4Z6kOXWCsUPPMenylyBVtogFlUiByE=",
    client.ForHost("example.com"),
    client.IncludeSubdomains(),
)

_ = p.AddPinFromCertFile("/etc/ssl/backup.crt", client.ForHost("example.com"))

_ = p.AddPinFromPEM(pemBytes, client.ForHost("api.example.com"))

// Verify yourself, given a chain
err := p.Verify("example.com", peerCerts)
```

When do you reach for the standalone version vs Client-attached? Use Client-attached when httpcloak is doing the request and you want pinning enforced automatically on every response. Use standalone when you're verifying chains from somewhere else (a stored cert dump, a different transport, a custom dial) and you want to call `Verify` yourself.

`AddPin` takes flexible input. You can pass `sha256/...` prefixes, raw hex, or base64. The lib normalizes it down to base64 internally:

```go
p.AddPin("sha256/YSxNUV05SLc2H4Z6kOXWCsUPPMenylyBVtogFlUiByE=")  // works (with prefix)
p.AddPin("612c4d51 5d3948b7 361f867a 90e5d60a c50f3cc7 a7ca5c81 56da2016 55220721")  // works (hex, after stripping spaces)
p.AddPin("YSxNUV05SLc2H4Z6kOXWCsUPPMenylyBVtogFlUiByE=")  // works (raw base64)
```

## Pin scoping with PinOption

Pins default to "all hosts". Almost always wrong. Two options narrow scope:

| Option | Effect |
|---|---|
| `client.ForHost("example.com")` | Pin only fires when `host == "example.com"` |
| `client.IncludeSubdomains()` | Pin also fires for `*.example.com` (used together with `ForHost`) |

Combine them:

```go
c.PinCertificate(spkiHash,
    client.ForHost("example.com"),
    client.IncludeSubdomains(),
)
```

Skip both options and the pin applies globally. Every TLS connection through this client checks against it. That's almost never what you want.

## Pin failure handling

When verification fails, you get back a `*client.CertPinError` with the host and both sides of the mismatch:

```go
resp, err := c.Do(ctx, req)
if err != nil {
    var pinErr *client.CertPinError
    if errors.As(err, &pinErr) {
        fmt.Printf("pin failure on %s\n", pinErr.Host)
        fmt.Printf("expected: %v\n", pinErr.ExpectedHashes)
        fmt.Printf("actual:   %v\n", pinErr.ActualHashes)
    }
}
```

The `ActualHashes` list contains the SPKI hash of every cert in the peer chain, leaf first. Handy for figuring out whether the wrong cert showed up, or whether the right cert just rotated to a new key.

## How to capture a pin

The one-liner. Pipe the cert into openssl, extract the public key, hash the DER, base64-encode it:

```bash
echo | openssl s_client -servername example.com -connect example.com:443 2>/dev/null \
  | openssl x509 -pubkey -noout \
  | openssl pkey -pubin -outform DER \
  | openssl dgst -sha256 -binary \
  | base64
```

Output (example.com, captured 2026-05-10):

```
YSxNUV05SLc2H4Z6kOXWCsUPPMenylyBVtogFlUiByE=
```

That's the value you feed `PinCertificate`. Run it once per target, stash the hash somewhere, ship it.

## End-to-end example

This Go program captures example.com's SPKI on the fly via openssl, pins it, and confirms the request lands. Then it swaps in a bogus pin and checks the verification fails:

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "io"
    "os/exec"
    "strings"

    "github.com/sardanioss/httpcloak/client"
)

func captureSPKI(host string) (string, error) {
    cmd := exec.Command("bash", "-c", fmt.Sprintf(
        `echo | openssl s_client -servername %s -connect %s:443 2>/dev/null `+
            `| openssl x509 -pubkey -noout `+
            `| openssl pkey -pubin -outform DER `+
            `| openssl dgst -sha256 -binary `+
            `| base64`, host, host))
    out, err := cmd.Output()
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(string(out)), nil
}

func main() {
    ctx := context.Background()
    host := "example.com"

    spki, err := captureSPKI(host)
    if err != nil {
        panic(err)
    }
    fmt.Printf("captured SPKI: %s\n", spki)

    // Pin the real hash, request should succeed
    c := client.NewClient("chrome-latest")
    c.PinCertificate(spki, client.ForHost(host))

    req := &client.Request{Method: "GET", URL: "https://" + host + "/"}
    resp, err := c.Do(ctx, req)
    if err != nil {
        panic(err)
    }
    io.Copy(io.Discard, resp.Body)
    resp.Body.Close()
    fmt.Printf("pinned request: status=%d\n", resp.StatusCode)

    // Swap to a bogus pin, request should fail with CertPinError
    c.ClearPins()
    c.PinCertificate("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", client.ForHost(host))

    _, err = c.Do(ctx, req)
    var pinErr *client.CertPinError
    if errors.As(err, &pinErr) {
        fmt.Printf("pin failure on %s\n", pinErr.Host)
        fmt.Printf("expected: %v\n", pinErr.ExpectedHashes)
        fmt.Printf("got:      %v\n", pinErr.ActualHashes)
    } else {
        fmt.Println("expected CertPinError, got:", err)
    }
}
```

</TabItem>
<TabItem value="python" label="Python">

Pinning is Go-only right now. The Python binding doesn't surface `PinCertificate` yet. If you need pinning from Python, run a local httpcloak proxy with pinning configured on the Go side and point Python at it. Tracking issue if you want to bump priority: open a GH issue.

</TabItem>
<TabItem value="node" label="Node.js">

Same as Python. The Node binding doesn't expose pin APIs yet. Wrap a Go-side httpcloak local proxy with pins enforced and route Node traffic through it.

</TabItem>
<TabItem value="dotnet" label=".NET">

.NET binding doesn't expose pin APIs yet either. Same workaround applies: Go-side local proxy with pins, .NET points at it.

</TabItem>
</Tabs>

Sample output, run against example.com on 2026-05-10:

```text
captured SPKI: YSxNUV05SLc2H4Z6kOXWCsUPPMenylyBVtogFlUiByE=
pinned request: status=200
pin failure on example.com
expected: [AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=]
got:      [YSxNUV05SLc2H4Z6kOXWCsUPPMenylyBVtogFlUiByE= Kt2bkYM55rPaGBFYxTLlq8AIJqapRcc1eKjai8GUPO0= OXyj9ngbqO9cjLeO/+t9Ggl2EP4JTnVWHq4LEwhFM9w= G/ANXI8TwJTdF+AFBM8IiIUPEv0Gf6H5LA/b9guG4yE=]
```

First request: 200, pin matched. Second: `CertPinError`, peer chain hashes leaked into `ActualHashes` so you can see exactly what showed up.

:::warning
Pins go stale. Sites rotate certs, sometimes on a schedule (Let's Encrypt is 90 days), sometimes after an incident, and your hardcoded SPKI hash dies the moment the keypair changes. Build a refresh path: re-capture the hash on a cron, or pin multiple SPKIs (current + next) at once, or fall back gracefully when `CertPinError` shows up. A pinned client that 100% fails after cert rotation is worse than no pin at all.
:::
