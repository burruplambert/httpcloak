---
title: Bindings
sidebar_position: 1
---

# Bindings

Same lib, four languages. Go's the native one. The rest call into a cgo-built shared library, so you're getting the exact same wire behaviour everywhere.

## In this section

- [Go](./go): the native API, idiomatic Go
- [Python](./python): a `requests`-shaped wrapper over cgo
- [Node.js](./nodejs): koffi-backed, ESM and CJS both work
- [.NET](./dotnet): P/Invoke wrapper for .NET 8+
