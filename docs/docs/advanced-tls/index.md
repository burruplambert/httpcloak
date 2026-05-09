---
title: Advanced TLS
sidebar_position: 1
---

# Advanced TLS

The deeper TLS knobs. ECH, speculative CONNECT, keylogging for Wireshark, and domain fronting. You won't reach for these every day, but when you need them, you really need them.

## In this section

- [ECH](./ech): Encrypted Client Hello. On by default, opt out with `WithDisableECH`.
- [Speculative TLS](./speculative-tls): pipeline CONNECT and ClientHello, save one RTT on every proxied dial.
- [TLS Keylog](./tls-keylog): dump `SSLKEYLOGFILE` for Wireshark when you need to see what's actually on the wire.
- [Domain Fronting](./domain-fronting): when SNI isn't Host, here's how to wire it up.
