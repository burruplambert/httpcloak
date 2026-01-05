# ECH and QUIC Fingerprint Implementation Guide

This document explains the recent changes made to implement proper ECH (Encrypted Client Hello) support and fix QUIC TLS fingerprinting issues in the httpcloak library.

---

## Table of Contents

1. [The Problem We're Solving](#the-problem-were-solving)
2. [What is TLS Fingerprinting?](#what-is-tls-fingerprinting)
3. [What is ECH (Encrypted Client Hello)?](#what-is-ech-encrypted-client-hello)
4. [What is QUIC?](#what-is-quic)
5. [The Issues We Fixed](#the-issues-we-fixed)
6. [Implementation Details](#implementation-details)
7. [File Changes Summary](#file-changes-summary)

---

## The Problem We're Solving

When your program makes HTTPS requests, it leaves behind a "fingerprint" - a unique signature based on how it establishes the secure connection. Websites and security systems can analyze this fingerprint to determine if the request is coming from a real browser or an automated tool.

The goal of httpcloak is to make HTTP requests look exactly like they come from a real browser (like Chrome). This involves mimicking:
- The TLS handshake (how the secure connection is established)
- The HTTP/3 protocol behavior over QUIC
- Various extension orderings and values

We had two main issues:
1. **ECH was not actually negotiating** - it was sending fake/placeholder data instead of real ECH
2. **QUIC connections had wrong TLS extensions** - they included TLS 1.2 legacy extensions that real browsers don't send over QUIC

---

## What is TLS Fingerprinting?

### The Simple Explanation

Think of TLS fingerprinting like a handshake between two people. When you meet someone, your handshake reveals things about you - how firm it is, which hand you use, how you position your fingers. Similarly, when your program connects to a website over HTTPS, the way it says "hello" reveals what software it is.

### The Technical Explanation

When establishing a TLS connection, the client sends a "ClientHello" message. This message contains:
- **Supported cipher suites** - encryption algorithms the client can use
- **Extensions** - additional features and capabilities
- **Supported TLS versions** - which versions of TLS the client supports
- **Supported elliptic curves** - mathematical curves for key exchange

Each browser has a unique combination of these values in a specific order. For example:
- Chrome sends extensions in a certain order
- Firefox sends them in a different order
- Python's requests library has its own distinct pattern

Security systems maintain databases of these fingerprints. When they see a request claiming to be Chrome but with Python's fingerprint, they know something is suspicious.

### Why It Matters

If your fingerprint doesn't match a real browser:
- Websites may block your requests
- CAPTCHAs may appear more frequently
- Anti-bot systems may flag your traffic
- Rate limiting may be stricter

---

## What is ECH (Encrypted Client Hello)?

### The Simple Explanation

When you visit a website, your browser needs to tell the server which website you want (like "I want to visit example.com"). This happens during the TLS handshake in a field called SNI (Server Name Indication).

The problem: This website name is sent in plain text, before encryption is established. Anyone watching your network traffic (ISPs, firewalls, governments) can see which websites you're visiting, even if the actual content is encrypted.

ECH solves this by encrypting the website name itself. It's like sealing your destination address inside an envelope before sending it.

### The Technical Explanation

ECH (Encrypted Client Hello) is a TLS 1.3 extension that encrypts the entire ClientHello message, including the SNI. Here's how it works:

1. **DNS Lookup**: Before connecting, the client queries DNS for the website. Modern DNS returns "HTTPS records" (type 65) which contain an ECH configuration - essentially a public key.

2. **Encryption**: The client uses this public key to encrypt its real ClientHello (the "inner" ClientHello) into an "outer" ClientHello that shows a fake/generic server name.

3. **Decryption**: The server (or a front-end proxy like Cloudflare) has the private key and decrypts the real ClientHello to see the actual destination.

4. **Result**: Network observers only see the outer (fake) server name, not the real website you're visiting.

### GREASE ECH vs Real ECH

- **GREASE ECH**: A placeholder/fake ECH extension with random data. Browsers send this to test that servers properly ignore unknown extensions. It doesn't actually encrypt anything.

- **Real ECH**: Uses the actual ECH configuration from DNS to encrypt the ClientHello. This provides real privacy protection.

Our problem was: We were only sending GREASE ECH (fake), not real ECH. Fingerprint analysis showed `ech_success: false`.

### Why ECH Matters for Fingerprinting

Modern browsers like Chrome now use ECH when available. If your fingerprint shows:
- No ECH at all, or
- Only GREASE ECH when real ECH should be used

...then you don't look like a real browser. Sites that check fingerprints will notice the discrepancy.

---

## What is QUIC?

### The Simple Explanation

QUIC is a new transport protocol that replaces TCP for HTTP/3. Think of TCP as sending a letter through the postal service - reliable but slow, with lots of back-and-forth confirmations. QUIC is like a high-tech courier service that's faster, handles lost packages better, and starts the secure handshake immediately.

### The Technical Explanation

QUIC (Quick UDP Internet Connections) is:
- Built on UDP instead of TCP
- Has TLS 1.3 built directly into the protocol
- Supports multiplexed streams (multiple requests without head-of-line blocking)
- Has improved connection migration and 0-RTT resumption

Key point: **QUIC always uses TLS 1.3**. It cannot use TLS 1.2 or earlier versions.

### Why This Matters for Our Fix

Because QUIC is TLS 1.3 only, certain TLS extensions make no sense in QUIC:

| Extension | Purpose | Why It's TLS 1.2 Only |
|-----------|---------|----------------------|
| `ec_point_formats` | Specifies elliptic curve point formats | TLS 1.3 only uses uncompressed points |
| `extended_master_secret` | Prevents certain MITM attacks | Built into TLS 1.3 by design |
| `renegotiation_info` | Secure renegotiation | TLS 1.3 doesn't support renegotiation |
| `status_request` (OCSP) | Certificate status | Handled differently in TLS 1.3 |
| `signed_certificate_timestamp` | Certificate transparency | Handled differently in TLS 1.3 |

Real browsers don't include these extensions in QUIC connections. Our code was incorrectly adding them.

---

## The Issues We Fixed

### Issue 1: ECH Not Negotiating

**Symptom**: Fingerprint analysis showed `ech_success: false`

**Root Cause**: We were only sending GREASE ECH (placeholder), not fetching and using real ECH configurations from DNS.

**Solution**:
1. Added code to fetch ECH configurations from DNS HTTPS records
2. Passed the ECH config through the entire chain: QUIC pool → QUIC config → connection → crypto setup → uTLS
3. uTLS then uses this config to actually encrypt the ClientHello

### Issue 2: Wrong TLS Extensions in QUIC

**Symptom**: Fingerprint showed extensions like `ec_point_formats`, `renegotiation_info`, `extended_master_secret` in QUIC ClientHello

**Root Cause**: In the uTLS library, the function `makeClientHelloForApplyPreset()` was setting TLS 1.2 legacy extension flags for ALL connections, including QUIC. These flags tell the TLS library to add extensions that shouldn't exist in QUIC/TLS 1.3.

**Solution**: Added a check `isQUIC := c.quic != nil` and only set the legacy extension flags for non-QUIC connections.

---

## Implementation Details

### ECH Config Flow

```
┌─────────────────┐
│   Application   │
│  calls Fetch()  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐     DNS HTTPS Query
│   QUIC Pool     │ ──────────────────────►  DNS Server
│  (quic_pool.go) │ ◄──────────────────────  Returns ECH config
└────────┬────────┘
         │ ECHConfigList
         ▼
┌─────────────────┐
│   quic.Config   │
│  (interface.go) │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  connection.go  │
│   creates conn  │
└────────┬────────┘
         │
         ▼
┌─────────────────────┐
│ NewCryptoSetupClient│
│  (crypto_setup.go)  │
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│   tlsConfigToUtls   │
│ Sets ECH on utls    │
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│    utls.Config      │
│ EncryptedClientHello│
│    ConfigList set   │
└─────────────────────┘
```

### DNS HTTPS Record Fetching

DNS HTTPS records (type 65) are a relatively new DNS record type that can contain:
- IP address hints (for faster connections)
- ALPN protocols supported
- **ECH configuration** (the `ech` parameter)

We use the `miekg/dns` library to query these records:

```go
// Simplified flow
msg := new(dns.Msg)
msg.SetQuestion(dns.Fqdn(host), dns.TypeHTTPS)
response, _ := dns.Exchange(msg, "8.8.8.8:53")

for _, answer := range response.Answer {
    if https, ok := answer.(*dns.HTTPS); ok {
        for _, kv := range https.Value {
            if kv.Key() == dns.SVCB_ECHCONFIG {
                // Found ECH config!
                return kv.(*dns.SVCBECHConfig).ECH
            }
        }
    }
}
```

### QUIC Extension Fix

The fix in `u_handshake_client.go`:

```go
// Before: Legacy extensions set for ALL connections
hello := &clientHelloMsg{
    extendedMasterSecret:             true,  // TLS 1.2 only
    ocspStapling:                     true,  // Different in 1.3
    scts:                             true,  // Different in 1.3
    supportedPoints:                  []uint8{pointFormatUncompressed},
    secureRenegotiationSupported:     true,  // TLS 1.2 only
}

// After: Only set for non-QUIC connections
isQUIC := c.quic != nil

hello := &clientHelloMsg{
    // ... basic fields ...
}

if !isQUIC {
    hello.extendedMasterSecret = true
    hello.ocspStapling = true
    hello.scts = true
    hello.supportedPoints = []uint8{pointFormatUncompressed}
    hello.secureRenegotiationSupported = true
}
```

---

## File Changes Summary

### Repository: sardanioss-utls

| File | Changes |
|------|---------|
| `u_handshake_client.go` | Fixed `makeClientHelloForApplyPreset()` to not add TLS 1.2 legacy extensions for QUIC connections |

### Repository: sardanioss-quic-go

| File | Changes |
|------|---------|
| `interface.go` | Added `ECHConfigList []byte` field to `Config` struct |
| `config.go` | Added `ECHConfigList` to `populateConfig()` |
| `connection.go` | Pass `ECHConfigList` to `NewCryptoSetupClient()` |
| `internal/handshake/crypto_setup.go` | Added `echConfigList` parameter and set it on uTLS config |
| `internal/handshake/crypto_setup_test.go` | Updated tests with nil ECH parameter |
| `fuzzing/handshake/fuzz.go` | Updated with nil ECH parameter |
| `fuzzing/handshake/cmd/corpus.go` | Updated with nil ECH parameter |

### Repository: httpcloak

| File | Changes |
|------|---------|
| `dns/cache.go` | Added `FetchECHConfigs()` and `queryECHFromDNS()` functions |
| `pool/quic_pool.go` | Fetch ECH configs before creating QUIC connections |
| `go.mod` / `go.sum` | Updated quic-go dependency |

---

## Testing

To verify ECH is working:

1. **Check ECH config fetching**:
```go
echConfig, err := dns.FetchECHConfigs(ctx, "cloudflare.com")
// Should return non-nil []byte for sites supporting ECH
```

2. **Check fingerprint**:
Use a fingerprint analysis tool and verify:
- `ech_success: true` (when connecting to ECH-enabled sites)
- No TLS 1.2 legacy extensions in QUIC connections

3. **Sites known to support ECH**:
- cloudflare.com
- crypto.cloudflare.com
- Many sites behind Cloudflare CDN

---

## Summary

We implemented two critical fixes:

1. **Real ECH Support**: The library now fetches actual ECH configurations from DNS and uses them to encrypt the ClientHello, providing privacy and matching real browser behavior.

2. **Correct QUIC Extensions**: QUIC connections no longer include TLS 1.2 legacy extensions that would make the fingerprint obviously non-browser.

These changes make the library's fingerprint much more closely match a real Chrome browser when making HTTP/3 requests over QUIC.
