# Chrome Browser Behavior Documentation

This document details the exact behavior of Google Chrome that must be replicated for accurate browser fingerprint emulation. All findings are based on Chrome 143 analysis against Cloudflare and other fingerprinting services.

---

## Table of Contents

1. [TLS Layer](#tls-layer)
   - [TLS Extension Shuffling](#tls-extension-shuffling)
   - [TLS Fingerprint Details](#tls-fingerprint-details)
   - [JA3/JA4 Fingerprinting](#ja3ja4-fingerprinting)
   - [Session Resumption](#session-resumption)
2. [HTTP/1.1 Protocol](#http11-protocol)
   - [Connection Management](#http11-connection-management)
   - [Request Format](#http11-request-format)
   - [Header Order](#http11-header-order)
   - [Keep-Alive Behavior](#http11-keep-alive-behavior)
   - [Pipelining](#http11-pipelining)
3. [HTTP/2 Protocol](#http2-protocol)
   - [Connection Establishment](#http2-connection-establishment)
   - [Frame Types](#http2-frame-types)
   - [SETTINGS Frame](#http2-settings-frame)
   - [HEADERS Frame](#http2-headers-frame)
   - [HPACK Compression](#http2-hpack-compression)
   - [Stream Management](#http2-stream-management)
   - [Flow Control](#http2-flow-control)
   - [Priority System](#http2-priority-system)
   - [Akamai Fingerprint](#http2-akamai-fingerprint)
4. [HTTP/3 Protocol (QUIC)](#http3-protocol-quic)
   - [QUIC Connection Establishment](#quic-connection-establishment)
   - [Stream Types](#http3-stream-types)
   - [QPACK Compression](#http3-qpack-compression)
   - [Frame Types](#http3-frame-types)
   - [Priority System](#http3-priority-system)
   - [Connection Migration](#http3-connection-migration)
5. [HTTP Headers](#http-headers)
   - [Navigation Headers](#navigation-headers)
   - [Fetch Metadata Headers](#fetch-metadata-headers)
   - [Client Hints](#client-hints)
   - [Cache Control Behavior](#cache-control-behavior)
6. [Protocol Negotiation](#protocol-negotiation)
7. [Cloudflare Detection](#cloudflare-detection)
8. [Bot Detection Triggers](#bot-detection-triggers)

---

## TLS Layer

### TLS Extension Shuffling

Chrome introduced TLS extension shuffling in version 110 to combat fingerprinting.

#### Shuffle Behavior

| Event | Shuffle Action |
|-------|---------------|
| Browser startup | Shuffle once, cache order |
| New tab | Use cached order |
| New connection | Use cached order |
| Session resumption (PSK) | Use cached order |
| Browser restart | New shuffle |

#### Extensions That Are Never Shuffled

These extensions maintain fixed positions:

1. **GREASE Extensions** - Stay in their original positions (positionally invariant)
2. **Padding Extension** - Typically near the end, adjusts ClientHello size
3. **Pre-Shared Key (PSK)** - MUST be last per TLS 1.3 RFC 8446

#### Shuffle Algorithm

```
1. Identify all shuffleable extensions (exclude GREASE, padding, PSK)
2. Generate cryptographically random seed
3. Fisher-Yates shuffle on shuffleable extensions
4. Preserve positions of non-shuffleable extensions
5. Cache result for session lifetime
```

#### Implementation Pattern

```go
// WRONG - Shuffles every connection
tlsConn := utls.UClient(conn, config, utls.HelloChrome_143)

// CORRECT - Shuffle once, reuse
spec, _ := utls.UTLSIdToSpec(utls.HelloChrome_143)  // Shuffles here
cachedSpec := &spec

// For each connection:
tlsConn := utls.UClient(conn, config, utls.HelloCustom)
tlsConn.ApplyPreset(cachedSpec)  // Uses cached shuffle
```

---

### TLS Fingerprint Details

#### Chrome 143 ClientHello Structure

```
ClientHello {
    ProtocolVersion: TLS 1.2 (0x0303)  // Legacy, actual version in extension
    Random: 32 bytes
    SessionID: 32 bytes (SHA-256 of various parameters)
    CipherSuites: [see below]
    CompressionMethods: [null]
    Extensions: [see below]
}
```

#### Cipher Suites (Exact Order)

```
0x?A?A  GREASE
0x1301  TLS_AES_128_GCM_SHA256
0x1302  TLS_AES_256_GCM_SHA384
0x1303  TLS_CHACHA20_POLY1305_SHA256
0xC02B  TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
0xC02F  TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
0xC02C  TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
0xC030  TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
0xCCA9  TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256
0xCCA8  TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256
0xC013  TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA
0xC014  TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA
0x009C  TLS_RSA_WITH_AES_128_GCM_SHA256
0x009D  TLS_RSA_WITH_AES_256_GCM_SHA384
0x002F  TLS_RSA_WITH_AES_128_CBC_SHA
0x0035  TLS_RSA_WITH_AES_256_CBC_SHA
```

#### TLS Extensions (Before Shuffle)

| Extension | ID | Purpose |
|-----------|----|---------|
| GREASE | 0x?A?A | Anti-fingerprinting |
| server_name (SNI) | 0 | Target hostname |
| extended_master_secret | 23 | Security |
| renegotiation_info | 65281 | Security |
| supported_groups | 10 | Curve preferences |
| ec_point_formats | 11 | EC point format |
| session_ticket | 35 | Session resumption |
| application_layer_protocol_negotiation | 16 | ALPN (h2, http/1.1) |
| status_request | 5 | OCSP stapling |
| signature_algorithms | 13 | Sig alg preferences |
| signed_certificate_timestamp | 18 | CT support |
| key_share | 51 | Key exchange |
| psk_key_exchange_modes | 45 | PSK modes |
| supported_versions | 43 | TLS 1.3 |
| compress_certificate | 27 | Cert compression |
| application_settings | 17613 | ALPS |
| GREASE | 0x?A?A | Anti-fingerprinting |
| padding | 21 | Size adjustment |
| pre_shared_key | 41 | PSK (always last) |

#### Supported Groups (Curves)

```
0x?A?A  GREASE
0x6399  X25519MLKEM768 (post-quantum hybrid)
0x001D  X25519
0x0017  secp256r1 (P-256)
0x0018  secp384r1 (P-384)
```

#### Signature Algorithms

```
0x0403  ecdsa_secp256r1_sha256
0x0804  rsa_pss_rsae_sha256
0x0401  rsa_pkcs1_sha256
0x0503  ecdsa_secp384r1_sha384
0x0805  rsa_pss_rsae_sha384
0x0501  rsa_pkcs1_sha384
0x0806  rsa_pss_rsae_sha512
0x0601  rsa_pkcs1_sha512
```

#### ALPN Protocols

For HTTP/2 capable connections:
```
h2
http/1.1
```

For HTTP/1.1 only:
```
http/1.1
```

#### GREASE Values

GREASE (Generate Random Extensions And Sustain Extensibility) uses reserved values:

```
0x0A0A, 0x1A1A, 0x2A2A, 0x3A3A, 0x4A4A, 0x5A5A, 0x6A6A, 0x7A7A,
0x8A8A, 0x9A9A, 0xAAAA, 0xBABA, 0xCACA, 0xDADA, 0xEAEA, 0xFAFA
```

#### GREASE Seed Caching (Critical for HTTP/3)

**CRITICAL**: Like TLS extension shuffling, GREASE values must be CONSISTENT within a session.

| Event | GREASE Behavior |
|-------|-----------------|
| Browser startup | Generate random seed, cache it |
| New tab | Use cached seed |
| New connection (same session) | Use cached seed |
| New QUIC connection | Use cached seed |
| Browser restart | New random seed |

Chrome uses a "GREASE seed" - a set of random values generated once per session that determine all GREASE values:
- Cipher suite GREASE
- Extension GREASE (positions 1 and 2)
- Supported groups GREASE
- Supported versions GREASE

#### Implementation Pattern for GREASE

```go
// WRONG - Different GREASE values per connection (bot signature!)
for each connection {
    spec, _ := utls.UTLSIdToSpec(utls.HelloChrome_143)
    // ApplyPreset generates new GREASE seed each time
    conn.ApplyPreset(&spec)
}

// CORRECT - Cache GREASE seed with spec
spec, _ := utls.UTLSIdToSpec(utls.HelloChrome_143)
cachedSpec := &spec
// First ApplyPreset generates GREASE seed, stores it in spec.GREASESeed
// Subsequent ApplyPreset calls reuse the cached seed

for each connection {
    conn := utls.UClient(nil, config, utls.HelloCustom)
    conn.ApplyPreset(cachedSpec)  // Uses cached GREASE seed
}
```

**Detection Signal**: Cloudflare's `fl` (fingerprint) value will change per-request if GREASE values differ, immediately flagging as bot traffic.

---

### JA3/JA4 Fingerprinting

#### JA3 Hash

JA3 creates an MD5 hash of:
```
TLSVersion,CipherSuites,Extensions,EllipticCurves,EllipticCurveFormats
```

Example:
```
771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513-21,29-23-24,0
```

**Problem**: JA3 preserves extension ORDER, so shuffling changes the hash.

#### JA4 Hash

JA4 improves on JA3:
- Sorts extensions alphabetically (order-independent)
- Includes more metadata
- More resistant to shuffling

Format: `t13d1516h2_8daaf6152771_b0da82dd1658`

---

### Session Resumption

#### TLS 1.3 PSK Resumption

```
Initial Connection:
Client                              Server
  |---- ClientHello ----------------->|
  |<--- ServerHello + Certificate ----|
  |<--- Finished + NewSessionTicket --|
  |---- Finished -------------------->|

Resumed Connection:
Client                              Server
  |---- ClientHello + PSK ----------->|
  |<--- ServerHello + Finished -------|
  |---- Finished -------------------->|
```

Chrome's PSK behavior:
- Stores session tickets per-host
- Attempts 0-RTT when possible
- Falls back to full handshake if PSK rejected
- Uses PSK-specific ClientHello (different extension set)

#### Session Cache

```
Key: ServerName (hostname)
Value: SessionTicket + metadata
TTL: Varies (server-controlled, typically 7 days)
Storage: Disk-backed (survives restart)
```

---

## HTTP/1.1 Protocol

### HTTP/1.1 Connection Management

#### Connection Establishment

```
1. DNS Resolution
2. TCP 3-way handshake (SYN, SYN-ACK, ACK)
3. TLS handshake (if HTTPS)
4. HTTP request/response
```

#### Chrome's Connection Limits

| Limit | Value |
|-------|-------|
| Max connections per host | 6 |
| Max total connections | 256 |
| Connection timeout | 30 seconds |
| Keep-alive timeout | 90 seconds |

---

### HTTP/1.1 Request Format

```
GET /path HTTP/1.1\r\n
Host: example.com\r\n
Connection: keep-alive\r\n
User-Agent: Mozilla/5.0 ...\r\n
Accept: text/html,...\r\n
Accept-Encoding: gzip, deflate, br, zstd\r\n
Accept-Language: en-US,en;q=0.9\r\n
\r\n
```

#### Request Line Components

```
Method SP Request-URI SP HTTP-Version CRLF
```

- **Method**: GET, POST, PUT, DELETE, HEAD, OPTIONS, PATCH
- **SP**: Single space (0x20)
- **Request-URI**: Absolute path with query string
- **HTTP-Version**: `HTTP/1.1`
- **CRLF**: `\r\n` (0x0D 0x0A)

---

### HTTP/1.1 Header Order

Chrome sends headers in a specific order:

```
Host
Connection
Cache-Control (only on refresh)
Upgrade-Insecure-Requests
User-Agent
Accept
Sec-Fetch-Site
Sec-Fetch-Mode
Sec-Fetch-User
Sec-Fetch-Dest
Accept-Encoding
Accept-Language
Cookie (if present)
```

#### Header Formatting

- Header names: Case-insensitive but Chrome uses specific casing
- No space before colon: `Header-Name: value`
- Single space after colon
- CRLF line endings

---

### HTTP/1.1 Keep-Alive Behavior

#### Connection Header

```
Connection: keep-alive      # Keep connection open (default in HTTP/1.1)
Connection: close           # Close after response
```

#### Keep-Alive Parameters

Chrome's keep-alive timing:
```
Keep-Alive: timeout=90, max=100
```

- **timeout**: Idle timeout in seconds
- **max**: Maximum requests per connection

#### Connection Reuse Pattern

```
Request 1 -----> [Connection opens]
Response 1 <----
[Idle period]
Request 2 -----> [Same connection]
Response 2 <----
[Idle > timeout]
Request 3 -----> [New connection]
```

---

### HTTP/1.1 Pipelining

**Chrome does NOT use HTTP/1.1 pipelining** due to head-of-line blocking issues.

Pipelining would look like:
```
Request 1 ----->
Request 2 ----->
Request 3 ----->
<----- Response 1
<----- Response 2
<----- Response 3
```

Chrome instead opens multiple parallel connections (up to 6 per host).

---

## HTTP/2 Protocol

### HTTP/2 Connection Establishment

#### Connection Preface

After TLS handshake with ALPN=h2, client sends:

```
PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n
```

This is the "connection preface" - 24 bytes exactly:
```
0x505249202a20485454502f322e300d0a0d0a534d0d0a0d0a
```

Followed immediately by SETTINGS frame.

#### Full Handshake Sequence

```
Client                              Server
  |---- Connection Preface ---------->|
  |---- SETTINGS Frame -------------->|
  |---- WINDOW_UPDATE Frame --------->|
  |<--- SETTINGS Frame ---------------|
  |<--- WINDOW_UPDATE Frame ----------|
  |---- SETTINGS ACK ---------------->|
  |<--- SETTINGS ACK -----------------|
  |---- HEADERS Frame --------------->|
  |<--- HEADERS Frame ----------------|
  |<--- DATA Frame(s) ----------------|
```

---

### HTTP/2 Frame Types

All HTTP/2 frames have this structure:

```
+-----------------------------------------------+
|                Length (24 bits)               |
+---------------+---------------+---------------+
|  Type (8 bits)| Flags (8 bits)|
+-+-------------+---------------+-------------------------------+
|R|                Stream Identifier (31 bits)                  |
+=+=============================================================+
|                   Frame Payload (variable)                    |
+---------------------------------------------------------------+
```

#### Frame Type Values

| Type | ID | Purpose |
|------|----|---------|
| DATA | 0x0 | Request/response body |
| HEADERS | 0x1 | HTTP headers |
| PRIORITY | 0x2 | Stream priority (deprecated) |
| RST_STREAM | 0x3 | Terminate stream |
| SETTINGS | 0x4 | Connection parameters |
| PUSH_PROMISE | 0x5 | Server push |
| PING | 0x6 | Keepalive/RTT |
| GOAWAY | 0x7 | Graceful shutdown |
| WINDOW_UPDATE | 0x8 | Flow control |
| CONTINUATION | 0x9 | Header continuation |

---

### HTTP/2 SETTINGS Frame

#### Chrome's SETTINGS Frame

```
Frame Header:
  Length: 24 bytes (6 bytes × 4 settings)
  Type: 0x4 (SETTINGS)
  Flags: 0x0
  Stream ID: 0 (connection-level)

Payload (in this exact order):
  HEADER_TABLE_SIZE (0x1): 65536
  ENABLE_PUSH (0x2): 0
  INITIAL_WINDOW_SIZE (0x4): 6291456
  MAX_HEADER_LIST_SIZE (0x6): 262144
```

#### Wire Format (Hex)

```
00 00 18        # Length: 24 bytes
04              # Type: SETTINGS
00              # Flags: none
00 00 00 00     # Stream ID: 0

00 01           # HEADER_TABLE_SIZE
00 01 00 00     # 65536

00 02           # ENABLE_PUSH
00 00 00 00     # 0 (disabled)

00 04           # INITIAL_WINDOW_SIZE
00 60 00 00     # 6291456

00 06           # MAX_HEADER_LIST_SIZE
00 04 00 00     # 262144
```

#### Settings Chrome Does NOT Send

| Setting | ID | Why Not Sent |
|---------|----|--------------|
| MAX_CONCURRENT_STREAMS | 0x3 | Uses server's limit |
| MAX_FRAME_SIZE | 0x5 | Default 16384 is fine |

#### SETTINGS Order Significance

**The order of settings in the frame is fingerprinted!**

Chrome order: `1, 2, 4, 6`
Firefox order: `1, 2, 3, 4, 5, 6`

Wrong order = immediate bot detection.

---

### HTTP/2 HEADERS Frame

#### Frame Structure

```
Frame Header:
  Length: variable
  Type: 0x1 (HEADERS)
  Flags: END_STREAM (0x1), END_HEADERS (0x4), PRIORITY (0x20)
  Stream ID: odd number (client-initiated)

Payload:
  [Pad Length (8 bits)]           # if PADDED flag
  [E (1 bit) + Stream Dep (31)]   # if PRIORITY flag
  [Weight (8 bits)]               # if PRIORITY flag
  Header Block Fragment           # HPACK encoded
  [Padding]                       # if PADDED flag
```

#### Chrome's HEADERS Frame Flags

For first request on connection:
```
Flags: END_STREAM | END_HEADERS | PRIORITY (0x25)
```

For subsequent requests:
```
Flags: END_STREAM | END_HEADERS (0x05)
```

#### Priority Parameters (When PRIORITY Flag Set)

```
E (Exclusive): 1
Stream Dependency: 0
Weight: 255 (wire format, actual weight = 256)
```

---

### HTTP/2 HPACK Compression

#### HPACK Overview

HPACK uses three techniques:
1. **Static Table**: 61 pre-defined header name/value pairs
2. **Dynamic Table**: Connection-specific learned entries
3. **Huffman Encoding**: Optional compression of literals

#### Static Table (First 15 Entries)

| Index | Header Name | Header Value |
|-------|-------------|--------------|
| 1 | :authority | |
| 2 | :method | GET |
| 3 | :method | POST |
| 4 | :path | / |
| 5 | :path | /index.html |
| 6 | :scheme | http |
| 7 | :scheme | https |
| 8 | :status | 200 |
| ... | ... | ... |

#### HPACK Encoding Types

```
1. Indexed Header Field (1-bit prefix: 1)
   +---+---+---+---+---+---+---+---+
   | 1 |        Index (7+)         |
   +---+---------------------------+

2. Literal with Incremental Indexing (2-bit prefix: 01)
   +---+---+---+---+---+---+---+---+
   | 0 | 1 |      Index (6+)       |
   +---+---+-----------------------+
   |       Value (length + string) |
   +-------------------------------+

3. Literal without Indexing (4-bit prefix: 0000)
   +---+---+---+---+---+---+---+---+
   | 0 | 0 | 0 | 0 |  Index (4+)   |
   +---+---+-----------------------+
   |       Value (length + string) |
   +-------------------------------+

4. Literal Never Indexed (4-bit prefix: 0001)
   +---+---+---+---+---+---+---+---+
   | 0 | 0 | 0 | 1 |  Index (4+)   |
   +---+---+-----------------------+
   |       Value (length + string) |
   +-------------------------------+
```

#### Chrome's HPACK Indexing Policy

Chrome uses specific indexing for different headers:

| Header | Indexing Type |
|--------|---------------|
| :method | Indexed (static table) |
| :authority | Literal with indexing |
| :scheme | Indexed (static table) |
| :path | Literal without indexing |
| user-agent | Literal with indexing |
| accept | Literal with indexing |
| accept-encoding | Literal with indexing |
| cookie | Literal never indexed |

**Why this matters**: Different indexing policies affect:
- Dynamic table growth
- Subsequent header block sizes
- Overall fingerprint

---

### HTTP/2 Stream Management

#### Stream States

```
                          +--------+
                  send PP |        | recv PP
                 ,--------|  idle  |--------.
                /         |        |         \
               v          +--------+          v
        +----------+          |           +----------+
        |          |          | send H /  |          |
,------| reserved |          | recv H    | reserved |------.
|      | (local)  |          |           | (remote) |      |
|      +----------+          v           +----------+      |
|          |             +--------+             |          |
|          |     recv ES |        | send ES    |          |
|   send H |     ,-------|  open  |-------.    | recv H   |
|          |    /        |        |        \   |          |
|          v   v         +--------+         v  v          |
|      +----------+          |           +----------+      |
|      |   half   |          |           |   half   |      |
|      |  closed  |          | send R /  |  closed  |      |
|      | (remote) |          | recv R    | (local)  |      |
|      +----------+          |           +----------+      |
|           |                |                 |           |
|           | send ES /      |       recv ES / |           |
|           | send R /       v        send R / |           |
|           | recv R     +--------+   recv R   |           |
| send R /  `----------->|        |<-----------'  send R / |
| recv R                 | closed |               recv R   |
`----------------------->|        |<-----------------------'
                         +--------+
```

#### Stream ID Assignment

- Client-initiated streams: Odd numbers (1, 3, 5, ...)
- Server-initiated streams: Even numbers (2, 4, 6, ...)
- Stream 0: Connection control stream

#### Chrome's Stream Usage

```
Stream 1:  First request
Stream 3:  Second request
Stream 5:  Third request
...
```

Chrome creates new streams for each request but reuses the connection.

---

### HTTP/2 Flow Control

#### Window Update Mechanism

HTTP/2 uses per-stream and connection-level flow control:

```
Initial window size: Configured in SETTINGS (Chrome: 6291456)
Connection window:   Sum of all streams + connection overhead

WINDOW_UPDATE Frame:
  Length: 4 bytes
  Type: 0x8
  Flags: 0x0
  Stream ID: 0 (connection) or specific stream

  Payload:
    Window Size Increment (31 bits)
```

#### Chrome's WINDOW_UPDATE

Sent immediately after SETTINGS:

```
00 00 04        # Length: 4 bytes
08              # Type: WINDOW_UPDATE
00              # Flags: none
00 00 00 00     # Stream ID: 0 (connection-level)

00 ef 00 01     # Increment: 15663105
```

This value (15663105) is specifically fingerprinted!

#### Flow Control Calculation

```
Connection window = 65535 (default) + 15663105 = 15728640
Stream window = 6291456 (from SETTINGS)
```

---

### HTTP/2 Priority System

#### Original Priority (Deprecated)

The original HTTP/2 priority used stream dependencies:

```
Stream 1: Weight 256, depends on 0
Stream 3: Weight 256, depends on 0
Stream 5: Weight 256, depends on 0
```

Chrome's priority frame (when PRIORITY flag set in HEADERS):
```
E (Exclusive): 1
Stream Dependency: 0
Weight: 255 (= 256 in actual weight)
```

#### Extensible Priorities (RFC 9218)

Modern Chrome uses Extensible Priorities via `priority` header:

```
priority: u=0, i
```

Parameters:
- `u=N`: Urgency (0-7, lower = more urgent)
- `i`: Incremental (can be rendered incrementally)

Urgency mapping:
| Content Type | Urgency |
|-------------|---------|
| HTML document | u=0 |
| CSS | u=1 |
| Fonts | u=2 |
| JavaScript | u=3 |
| Images | u=4 |
| Prefetch | u=7 |

---

### HTTP/2 Akamai Fingerprint

#### Fingerprint Format

```
SETTINGS|WINDOW_UPDATE|PRIORITY|PSEUDO_HEADER_ORDER
```

#### Chrome's Fingerprint

```
1:65536;2:0;4:6291456;6:262144|15663105|0:256:e|m,a,s,p
```

Breaking this down:

| Component | Value | Meaning |
|-----------|-------|---------|
| SETTINGS | 1:65536;2:0;4:6291456;6:262144 | Setting ID:value pairs |
| WINDOW_UPDATE | 15663105 | Connection window increment |
| PRIORITY | 0:256:e | StreamDep:Weight:Exclusive |
| PSEUDO_HEADER_ORDER | m,a,s,p | :method,:authority,:scheme,:path |

#### Pseudo-Header Order Details

Chrome order: **m,a,s,p**
```
:method: GET
:authority: example.com
:scheme: https
:path: /page
```

Other browsers:
- Firefox: m,p,a,s
- Safari: m,s,a,p

**CRITICAL**: Alphabetical order (a,m,p,s) is an immediate bot signature.

---

## HTTP/3 Protocol (QUIC)

### QUIC Connection Establishment

#### Initial Handshake

```
Client                              Server
  |---- Initial[0]: CRYPTO --------->|  (ClientHello)
  |<--- Initial[0]: CRYPTO -----------|  (ServerHello)
  |<--- Handshake[0]: CRYPTO ---------|  (EncryptedExtensions, Cert, Verify, Finish)
  |---- Handshake[0]: CRYPTO -------->|  (Finished)
  |---- Short Header: STREAM -------->|  (HTTP/3 request)
```

#### QUIC Packet Types

| Type | Purpose | Encryption |
|------|---------|------------|
| Initial | Connection setup | Initial keys |
| 0-RTT | Early data | PSK-derived |
| Handshake | Key exchange | Handshake keys |
| Short Header | Application data | Traffic keys |

#### Transport Parameters

Chrome's QUIC transport parameters:

```
max_idle_timeout: 30000ms
max_udp_payload_size: 1472
initial_max_data: 15728640
initial_max_stream_data_bidi_local: 6291456
initial_max_stream_data_bidi_remote: 6291456
initial_max_stream_data_uni: 6291456
initial_max_streams_bidi: 100
initial_max_streams_uni: 100
active_connection_id_limit: 4
```

---

### HTTP/3 Stream Types

#### Unidirectional Stream Types

| Type | ID | Purpose |
|------|----|---------|
| Control | 0x00 | HTTP/3 settings/frames |
| Push | 0x01 | Server push |
| QPACK Encoder | 0x02 | Dynamic table updates |
| QPACK Decoder | 0x03 | Acknowledgments |

#### Stream ID Allocation

```
Client-initiated bidirectional:  0, 4, 8, 12, ...    (N*4)
Server-initiated bidirectional:  1, 5, 9, 13, ...    (N*4 + 1)
Client-initiated unidirectional: 2, 6, 10, 14, ...   (N*4 + 2)
Server-initiated unidirectional: 3, 7, 11, 15, ...   (N*4 + 3)
```

#### Chrome's Initial Streams

On connection establishment, Chrome opens:

```
Stream 2 (client uni): Control stream - sends SETTINGS
Stream 6 (client uni): QPACK encoder stream
Stream 10 (client uni): QPACK decoder stream
Stream 0 (client bidi): First HTTP request
```

---

### HTTP/3 QPACK Compression

#### QPACK vs HPACK

| Feature | HPACK | QPACK |
|---------|-------|-------|
| Dynamic table | Synchronized | Asynchronous |
| Out-of-order | No | Yes |
| Stream blocking | Head-of-line | Per-stream |

#### QPACK Static Table

Similar to HPACK but with 99 entries, including:
- HTTP/3 specific pseudo-headers
- Common header values

#### QPACK Encoder Stream

Chrome sends dynamic table updates on stream 6:

```
Insert With Name Reference:
  +---+---+---+---+---+---+---+---+
  | 1 | T |    Name Index (6+)    |
  +---+---+-----------------------+
  |       Value (length + string) |
  +-------------------------------+
```

#### QPACK Decoder Stream

Chrome sends acknowledgments on stream 10:

```
Section Acknowledgment:
  +---+---+---+---+---+---+---+---+
  | 1 |      Stream ID (7+)       |
  +---+---------------------------+
```

---

### HTTP/3 Frame Types

#### HTTP/3 Frame Structure

```
+-------------------------------------------+
|           Type (variable-length int)       |
+-------------------------------------------+
|           Length (variable-length int)     |
+-------------------------------------------+
|           Frame Payload (variable)         |
+-------------------------------------------+
```

#### Frame Types

| Type | ID | Purpose |
|------|----|---------|
| DATA | 0x00 | Request/response body |
| HEADERS | 0x01 | HTTP headers |
| CANCEL_PUSH | 0x03 | Cancel server push |
| SETTINGS | 0x04 | Connection settings |
| PUSH_PROMISE | 0x05 | Server push |
| GOAWAY | 0x07 | Graceful shutdown |
| MAX_PUSH_ID | 0x0D | Push limit |

#### HTTP/3 SETTINGS Frame

Chrome's HTTP/3 SETTINGS (sent on control stream):

```
SETTINGS {
  QPACK_MAX_TABLE_CAPACITY (0x01): 16384
  MAX_FIELD_SECTION_SIZE (0x06): 262144
  QPACK_BLOCKED_STREAMS (0x07): 100
}
```

#### PRIORITY_UPDATE Frame

Chrome uses PRIORITY_UPDATE for dynamic priority:

```
PRIORITY_UPDATE {
  Type: 0xF0700 or 0xF0701
  Prioritized Element ID: Stream ID
  Priority Field Value: "u=0, i"
}
```

---

### HTTP/3 Priority System

#### Extensible Priorities in HTTP/3

Same as HTTP/2 but sent as:
1. `priority` header in HEADERS frame
2. PRIORITY_UPDATE frame for changes

#### Chrome's HTTP/3 Priority Behavior

```
Initial request: priority header in HEADERS
Priority change: PRIORITY_UPDATE frame
Default: u=3, i (non-urgent, incremental)
Document: u=0, i (highest urgency)
```

---

### HTTP/3 Connection Migration

#### Connection Migration Support

Chrome supports QUIC connection migration:

```
1. Client detects network change (WiFi -> Cellular)
2. Client sends PATH_CHALLENGE on new path
3. Server responds with PATH_RESPONSE
4. Connection continues on new path
```

#### Connection ID

Multiple connection IDs allow migration without revealing identity:

```
Initial CID: Random 8-20 bytes
New CIDs: Requested via NEW_CONNECTION_ID frame
Retire old: RETIRE_CONNECTION_ID frame
```

---

## HTTP Headers

### Navigation Headers

#### Complete Header Set for Navigation

```http
GET / HTTP/2
:method: GET
:authority: example.com
:scheme: https
:path: /
sec-ch-ua: "Google Chrome";v="143", "Chromium";v="143", "Not A(Brand";v="24"
sec-ch-ua-mobile: ?0
sec-ch-ua-platform: "Windows"
upgrade-insecure-requests: 1
user-agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36
accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7
sec-fetch-site: none
sec-fetch-mode: navigate
sec-fetch-user: ?1
sec-fetch-dest: document
accept-encoding: gzip, deflate, br, zstd
accept-language: en-US,en;q=0.9
priority: u=0, i
```

#### Header Order (CRITICAL)

This exact order must be maintained:

```
1. :method
2. :authority
3. :scheme
4. :path
5. sec-ch-ua
6. sec-ch-ua-mobile
7. sec-ch-ua-platform
8. upgrade-insecure-requests
9. user-agent
10. accept
11. sec-fetch-site
12. sec-fetch-mode
13. sec-fetch-user
14. sec-fetch-dest
15. accept-encoding
16. accept-language
17. cookie (if present)
18. priority
19. origin (if applicable)
20. referer (if applicable)
```

---

### Fetch Metadata Headers

#### sec-fetch-site Values

| Value | Meaning | Example |
|-------|---------|---------|
| none | Direct navigation | User typed URL |
| same-origin | Same origin | example.com -> example.com/page |
| same-site | Same site (cross-origin) | sub.example.com -> example.com |
| cross-site | Different site | google.com -> example.com |

#### sec-fetch-mode Values

| Value | Meaning | When Used |
|-------|---------|-----------|
| navigate | Navigation | Page loads, form submissions |
| cors | CORS request | fetch() with CORS |
| no-cors | No-CORS request | Simple requests, images |
| same-origin | Same-origin | Restricted to same origin |
| websocket | WebSocket | WebSocket connections |

#### sec-fetch-user Values

| Value | Meaning |
|-------|---------|
| ?1 | User-activated (click, enter) |
| (absent) | Not user-activated |

#### sec-fetch-dest Values

| Value | Content Type |
|-------|-------------|
| document | HTML page |
| script | JavaScript |
| style | CSS |
| image | Images |
| font | Fonts |
| audio | Audio files |
| video | Video files |
| worker | Web workers |
| empty | fetch(), XHR |

---

### Client Hints

#### Low-Entropy Hints (Always Sent)

```
sec-ch-ua: "Google Chrome";v="143", "Chromium";v="143", "Not A(Brand";v="24"
sec-ch-ua-mobile: ?0
sec-ch-ua-platform: "Windows"
```

These are sent on EVERY request without opt-in.

#### High-Entropy Hints (Server Opt-In Required)

Server must send `Accept-CH` header first:

```
Accept-CH: sec-ch-ua-arch, sec-ch-ua-bitness, sec-ch-ua-full-version-list, sec-ch-ua-model, sec-ch-ua-platform-version
```

Only then Chrome sends:

```
sec-ch-ua-arch: "x86"
sec-ch-ua-bitness: "64"
sec-ch-ua-full-version-list: "Google Chrome";v="143.0.6194.0", "Chromium";v="143.0.6194.0", "Not A(Brand";v="24.0.0.0"
sec-ch-ua-model: ""
sec-ch-ua-platform-version: "10.0.0"
```

**BOT DETECTION**: Sending high-entropy hints WITHOUT receiving Accept-CH = bot signature.

#### Not A Brand Format

The "Not A Brand" value changes per Chrome version:

| Chrome Version | Not A Brand |
|----------------|-------------|
| 131 | Not_A Brand";v="24 |
| 133 | Not_A Brand";v="24 |
| 143 | Not A(Brand";v="24 |

---

### Cache Control Behavior

#### Critical: Navigation vs Refresh

| User Action | Cache-Control Header |
|-------------|---------------------|
| Click link | **NOT SENT** |
| Type URL + Enter | **NOT SENT** |
| F5 (Refresh) | `max-age=0` |
| Ctrl+F5 (Hard Refresh) | `no-cache` |
| Back/Forward | **NOT SENT** |
| Bookmark | **NOT SENT** |

**BOT DETECTION CRITICAL**: Sending `cache-control: max-age=0` on every request is one of the most obvious bot signatures. Real users clicking links do NOT send this header.

#### Pragma Header

- Chrome does NOT send `Pragma: no-cache` on navigation
- Only sent on hard refresh along with `cache-control: no-cache`

---

## Protocol Negotiation

### ALPN (Application-Layer Protocol Negotiation)

#### Chrome's ALPN Preference

```
h2          # HTTP/2 (preferred for HTTPS)
http/1.1    # Fallback
```

For HTTP/3 capable servers:
```
h3          # HTTP/3 (discovered via Alt-Svc)
```

### Alt-Svc Header

Servers advertise HTTP/3 support:

```
alt-svc: h3=":443"; ma=86400, h3-29=":443"; ma=86400
```

Chrome behavior:
1. First request uses HTTP/2
2. Receives Alt-Svc header
3. Subsequent requests try HTTP/3
4. Falls back to HTTP/2 if QUIC fails

### Protocol Preference Order

```
1. HTTP/3 (if Alt-Svc received and QUIC succeeds)
2. HTTP/2 (if server supports via ALPN)
3. HTTP/1.1 (fallback)
```

---

## Cloudflare Detection

### Cloudflare Trace Endpoint

`https://www.cloudflare.com/cdn-cgi/trace` returns:

```
fl=679f16
h=www.cloudflare.com
ip=1.2.3.4
ts=1234567890.123
visit_scheme=https
uag=Mozilla/5.0 ...
colo=LAX
sliver=none
http=http/3
loc=US
tls=TLSv1.3
sni=plaintext
warp=off
gateway=off
rbi=off
kex=X25519MLKEM768
```

### Field Meanings

| Field | Meaning | Bot Indicator? |
|-------|---------|----------------|
| fl | Fingerprint bucket | YES - should be consistent |
| sliver | A/B test bucket | NO |
| http | Protocol used | NO |
| tls | TLS version | Minor |
| kex | Key exchange | NO |
| colo | Datacenter | NO |

### Fingerprint Consistency (fl)

**CRITICAL**: The `fl` value should remain CONSTANT across all requests from the same browser session.

```
Good (real browser):
  Request 1: fl=679f16
  Request 2: fl=679f16
  Request 3: fl=679f16

Bad (bot):
  Request 1: fl=679f16
  Request 2: fl=453f12
  Request 3: fl=202f89
```

Varying `fl` indicates:
- New TLS connection each request (possible)
- Different TLS fingerprint each time (BOT!)

---

## Bot Detection Triggers

### Critical (Immediate Detection)

| Issue | Why It's Critical |
|-------|-------------------|
| TLS extension order changes per-request | Impossible in real browsers |
| Alphabetical pseudo-header order (a,m,p,s) | No browser does this |
| cache-control: max-age=0 on navigation | Only sent on refresh |
| High-entropy client hints unsolicited | Requires Accept-CH first |
| Missing sec-fetch-* headers | All modern browsers send these |
| Wrong SETTINGS order | Exact order is fingerprinted |

### High Risk

| Issue | Detection Risk |
|-------|---------------|
| Wrong SETTINGS values | High |
| Wrong window update value | High |
| Wrong stream priority | High |
| Missing GREASE | High |
| Static GREASE values | High (should randomize) |

### Medium Risk

| Issue | Detection Risk |
|-------|---------------|
| Wrong header order | Medium |
| Wrong accept header | Medium |
| Missing priority header | Medium |
| Wrong user-agent format | Medium |

### Behavioral Indicators

| Behavior | Risk |
|----------|------|
| Perfect request timing | Medium |
| No cookie handling | High |
| No redirect following | High |
| Ignoring Set-Cookie | High |
| Same headers every request | Medium |
| No referer when expected | Medium |

---

## Implementation Checklist

### TLS Layer
- [ ] Use platform-specific TLS presets
- [ ] Shuffle extensions once per Client
- [ ] Cache shuffled spec for session
- [ ] Cache GREASE seed for session (NOT per connection!)
- [ ] Support PSK resumption
- [ ] Use correct cipher suite order
- [ ] Use correct supported groups (X25519MLKEM768 first)

### HTTP/2 Layer
- [ ] Send connection preface
- [ ] SETTINGS in correct order (1, 2, 4, 6)
- [ ] SETTINGS with correct values
- [ ] WINDOW_UPDATE = 15663105
- [ ] Pseudo-header order: m,a,s,p
- [ ] Stream weight 256, exclusive
- [ ] Proper HPACK indexing policy
- [ ] Maintain header order

### HTTP/3 Layer
- [ ] Open control stream first
- [ ] Open QPACK encoder/decoder streams
- [ ] Send SETTINGS on control stream
- [ ] Use PRIORITY_UPDATE frames
- [ ] Support 0-RTT
- [ ] Proper QPACK encoding

### Headers
- [ ] Remove cache-control from navigation
- [ ] Only low-entropy client hints by default
- [ ] Correct sec-fetch-* values
- [ ] Maintain strict header order
- [ ] Proper accept header for content type
- [ ] Include priority header

### Session
- [ ] Implement TLS session caching
- [ ] Support PSK resumption
- [ ] Reuse HTTP/2 connections
- [ ] Proper connection pooling
- [ ] Cookie handling
- [ ] Redirect following

---

## References

- RFC 8446 - TLS 1.3
- RFC 7540 - HTTP/2
- RFC 9114 - HTTP/3
- RFC 9000 - QUIC
- RFC 7541 - HPACK
- RFC 9204 - QPACK
- RFC 9218 - Extensible Priorities
- Chromium Source Code
- Akamai HTTP/2 Fingerprinting Research
- Cloudflare Bot Detection Documentation
