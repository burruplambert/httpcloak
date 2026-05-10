---
title: Advanced TLS
sidebar_position: 1
---

# Advanced TLS

This section covers the deeper TLS knobs in httpcloak: ECH, speculative CONNECT, keylogging for Wireshark, domain fronting, and certificate pinning. Most sessions never touch any of these. The ones that do tend to need them sharply, so each chapter is self-contained.

## In this section

- [ECH](./ech): Encrypted Client Hello. On by default, opt out with `WithDisableECH`.
- [Speculative TLS](./speculative-tls): pipeline CONNECT and ClientHello, save one RTT on every proxied dial.
- [TLS Keylog](./tls-keylog): dump `SSLKEYLOGFILE` for Wireshark when you need to see what's actually on the wire.
- [Domain Fronting](./domain-fronting): when SNI isn't Host, here's how to wire it up.
