---
title: Session Save and Restore
sidebar_position: 4
---

# Session Save and Restore

`Save(path)` writes the full session state to disk as JSON. `LoadSession(path)` reads it back into a working session. Cookies, TLS session tickets, ECH configs, the preset name, custom fingerprint overrides, proxy config, all of it survives the round trip.

## Why this exists

**Long-running scrapers that survive restarts.** Crashes, reboots, deploys. Without persistence, the new process starts cold every time: empty jar, no tickets, full handshake to every host. Save on shutdown, load on startup, the new process picks up where the old one left off.

**Distributing a warmed-up session.** One process does the auth and warmup, saves the result, then N workers call `LoadSession` on the same blob. Every worker boots with the same identity and ticket cache, no repeated logins.

**Cold-start caching for short-lived processes.** CLI tools that connect once and exit can save the session so the next invocation skips the full TLS handshake.

## File format

The file is UTF-8 JSON, parseable by any JSON library. Schema version is 5; v3 and v4 files still load. Top-level keys: `version`, `created_at`, `updated_at`, `config`, `cookies` (keyed by domain), `tls_sessions` (keyed by `h1:`/`h2:`/`h3:` plus origin) and `ech_configs` (base64-encoded per host).

The file is written with `0600` permissions because it carries live session credentials. Don't commit these to git, don't ship them in cleartext, treat them like a password. People have uploaded these to public S3 buckets. Don't do that.

## Code

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
// Save phase
s := httpcloak.NewSession("chrome-latest")
ctx := context.Background()
r, _ := s.Get(ctx, "https://httpbin.org/cookies/set/my-id/abc123")
r.Close()
if err := s.Save("session.json"); err != nil {
	panic(err)
}
s.Close()

// Load phase, possibly in a different process
s2, err := httpcloak.LoadSession("session.json")
if err != nil {
	panic(err)
}
defer s2.Close()
// The cookie jar already has my-id=abc123 from the previous run.
r2, _ := s2.Get(ctx, "https://httpbin.org/cookies")
defer r2.Close()
```

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

# In one process:
with httpcloak.Session(preset="chrome-latest") as s:
    r = s.get("https://httpbin.org/cookies/set/my-id/abc123")
    s.save("session.json")

# Later, in a fresh process:
s = httpcloak.Session.load("session.json")
try:
    r = s.get("https://httpbin.org/cookies")
    print(r.text)  # my-id=abc123 still there
finally:
    s.close()
```

</TabItem>
<TabItem value="nodejs" label="Node.js">

```javascript
const httpcloak = require("httpcloak");

// Save phase
{
  const s = new httpcloak.Session({ preset: "chrome-latest" });
  await s.get("https://httpbin.org/cookies/set/my-id/abc123");
  s.save("session.json");
  s.close();
}

// Load phase (could be a totally separate process)
{
  const s = httpcloak.Session.load("session.json");
  const r = await s.get("https://httpbin.org/cookies");
  console.log(await r.text());
  s.close();
}
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using HttpCloak;

// Save phase
using (var s = new Session(preset: "chrome-latest"))
{
    s.Get("https://httpbin.org/cookies/set/my-id/abc123");
    s.Save("session.json");
}

// Load phase
using (var s = Session.Load("session.json"))
{
    var r = s.Get("https://httpbin.org/cookies");
    Console.WriteLine(r.Text);
}
```

</TabItem>
</Tabs>

## In-memory variant

When a file on disk isn't the right destination (writing to a database, shipping bytes across the network, embedding in a config blob), `Marshal()` and `UnmarshalSession()` move the same payload through memory instead. The data shape is identical, returned as a JSON string or byte slice. Every binding ships the pair: `Marshal()` / `Unmarshal()` in Go and .NET, `marshal()` / `Session.unmarshal()` in Python and Node.

```go
blob, err := s.Marshal()
// store blob in your DB / cache / wherever

s2, err := httpcloak.UnmarshalSession(blob)
defer s2.Close()
```

## What survives, what doesn't

| State | Survives Save/Load |
| --- | --- |
| Cookies (with domain, path, expiry, samesite, etc) | Yes |
| TLS 1.3 session tickets | Yes |
| TLS 1.2 session IDs | Yes |
| ECH config cache | Yes |
| Preset name and config | Yes |
| Proxy URL | Yes |
| Custom JA3, Akamai fingerprint, header order | Yes |
| Live connections | No, the load creates a fresh transport |
| In-flight requests | No, obvious |
| Cache-validation headers (ETag, Last-Modified) | No, currently per-session memory only |
| The session ID | New one is generated on load |

The cache validators are a known gap. If a workflow leans heavily on If-None-Match to look browser-like, it re-fetches full responses on the first hit after a load. Fine for most cases.

## Ticket expiry caveat

TLS session tickets have a server-controlled lifetime. Most CDNs hand out tickets that expire within 24 hours. Save a session today, load it a week later, and the tickets are stale. Stale tickets don't error; they just downgrade to a full handshake on the next request. Cookies have their own server-set expiry and the session honours that.

The further apart save and load are, the less the ticket cache buys you. After a couple of days against aggressive CDNs, the tickets are mostly dead weight, while the cookie jar still pulls its weight.

## Versioning and safety

The save format is versioned. v5 is current; v3 and v4 still load. A newer file opened by an older library returns `session file version N is newer than supported version 5`. `ValidateSessionFile(path)` (or its binding equivalent) is a cheap pre-load sanity check.

Don't load session files from untrusted sources. The saved blob carries the preset config (proxy URL, ECH domain, fingerprint overrides), which gets applied verbatim on load. A malicious file can pivot a session in ways the caller didn't intend. Treat these like config files in your own repo, not user input.
