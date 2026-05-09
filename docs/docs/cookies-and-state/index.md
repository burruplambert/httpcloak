---
title: Cookies & State
sidebar_position: 1
---

# Cookies & State

The session jar handles cookies for you. This section covers how it works, when you'd want to switch it off, and how to hand-roll a `Cookie` header for one-off calls.

## In this section

- [Cookie Jar](./cookie-jar): how the internal jar works, what gets stored, when it sends what
- [Disabling the Cookie Jar](./disabling-cookie-jar): WithoutCookieJar() and when to actually do that
- [Per-Request Cookies](./per-request-cookies): ad-hoc Cookie headers for one-off calls
- [Domain and Path Matching](./domain-and-path-matching): the quirks of cookies matching the next URL
