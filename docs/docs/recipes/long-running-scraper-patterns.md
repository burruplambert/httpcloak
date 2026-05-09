---
title: Long-Running Scraper Patterns
sidebar_position: 3
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Long-Running Scraper Patterns

Scrapers that run for days hit two ugly problems:

1. **Connections age out.** Servers and load balancers close idle keep-alive
   connections. Some kill them mid-use. Your nice TLS tickets and 0-RTT
   setup get tossed.
2. **Session fingerprints get tracked.** A scraper that never refreshes its
   TCP connection looks nothing like a real browser. Real browsers cycle
   connections constantly.

The fix is a handful of patterns: periodic `Refresh()`, `Warmup()` at
startup, careful cookie handling, Save/Load across process restarts.

## Pattern 1: Periodic Refresh

Real browsers don't sit on TCP connections for hours. Keepalives time out,
users open new tabs, life happens. Your scraper should mimic that.

`session.Refresh()` drops every live connection but keeps:

- TLS session tickets (next handshake is 0-RTT)
- Cookies
- ECH config
- Preset plus any custom fingerprint overrides

Next request opens a fresh TCP socket but resumes TLS instantly with the
same cookies. From the server's POV that's a returning visitor with a
slightly older session, which is exactly what real browsers look like.

:::warning
Don't refresh after every request. That kills the point of session
continuity. Slow cadence, minutes not seconds.
:::

A solid default is every 2-5 minutes:

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/sardanioss/httpcloak"
)

func main() {
    s := httpcloak.NewSession("chrome-latest", httpcloak.WithSessionTimeout(30*time.Second))
    defer s.Close()

    // Refresh ticker. Reasonable cadence: 2-5 min.
    refreshEvery := 3 * time.Minute
    nextRefresh := time.Now().Add(refreshEvery)

    for i := 0; i < 1000; i++ {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        r, err := s.Get(ctx, "https://example.com/page")
        cancel()
        if err != nil {
            fmt.Println("err:", err)
            time.Sleep(5 * time.Second)
            continue
        }
        r.Close()

        if time.Now().After(nextRefresh) {
            s.Refresh()
            nextRefresh = time.Now().Add(refreshEvery)
            fmt.Println("refreshed")
        }
    }
}
```

</TabItem>
<TabItem value="python" label="Python">

```python
import time
import httpcloak

with httpcloak.Session("chrome-latest", timeout=30) as s:
    refresh_every = 180  # seconds
    next_refresh = time.time() + refresh_every

    for i in range(1000):
        try:
            r = s.get("https://example.com/page")
        except Exception as e:
            print("err:", e)
            time.sleep(5)
            continue

        if time.time() > next_refresh:
            s.refresh()
            next_refresh = time.time() + refresh_every
            print("refreshed")
```

</TabItem>
</Tabs>

How fast is too fast? If your scraper sends one request every 30 seconds,
refreshing every 30 seconds means a fresh TCP/TLS handshake on every single
request. Even with 0-RTT that's wasted RTT. Refresh during idle gaps, not in
front of every request.

## Pattern 2: Warmup at startup

`Warmup(ctx, url)` mimics a real browser page load. Fetches the page HTML,
parses it, then pulls the obvious subresources (CSS, JS, fonts) with
realistic headers, priorities, and timing.

Once warmup runs, the session has:

- TLS session tickets for the target's edge servers
- Cookies set by the page and its subresources (analytics, CDN cookies)
- Cache headers (ETag, Last-Modified) ready for follow-up requests

So your first "real" scrape request looks like the second navigation inside
an already-open tab, which is way less suspicious than a cold first hit.

```go
s := httpcloak.NewSession("chrome-latest")
defer s.Close()

ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()

// One-time warmup. Hit the home page or a typical entry page.
if err := s.Warmup(ctx, "https://example.com"); err != nil {
    log.Printf("warmup soft-fail: %v", err)
    // Don't bail, warmup is opportunistic. If it fails, scrape anyway.
}

// Now run your scrape loop. The session has CDN cookies, tickets,
// and cache state populated.
scrapeLoop(ctx, s)
```

Warmup is opportunistic. Home page fails or hits a challenge? Log it and
keep going. The scrape loop should handle that case on its own.

:::tip
Got a heavy login flow? Do an authenticated warmup once per process: log in,
navigate, save with `Save()`. Later runs just `LoadSession()` from disk and
start scraping right away, no re-login dance.
:::

## Pattern 3: Cookie strategy

By default, sessions ship with a cookie jar. One jar per session, shared
across every request that goes through it.

Three patterns, depending on what you're scraping:

### One session, one target

The default. Shared cookie jar, every request through the same session sees
the same cookies.

```go
s := httpcloak.NewSession("chrome-latest")
// Hit target.com 1000 times. All requests share the cookie jar.
```

### N sessions, N targets (jar isolation)

Hitting N different targets? You almost always want one session per target.
Cookies stay scoped, no leaks across hosts:

```go
sessions := map[string]*httpcloak.Session{
    "site-a.com": httpcloak.NewSession("chrome-latest"),
    "site-b.com": httpcloak.NewSession("chrome-latest"),
    "site-c.com": httpcloak.NewSession("chrome-latest"),
}
defer func() {
    for _, s := range sessions {
        s.Close()
    }
}()

// Route each request to the right session by host.
```

Bonus: each session warms up independently against its own target, and you
Save/Load each one separately.

### Manual cookie management

Got weird requirements? Sharing one cookie across multiple sessions, A/B
testing two cookie sets against the same site, that kind of thing. Use
`WithoutCookieJar()` and set the `Cookie` header yourself on every request:

```go
s := httpcloak.NewSession("chrome-latest", httpcloak.WithoutCookieJar())

req := &httpcloak.Request{
    Method: "GET",
    URL:    "https://example.com/page",
    Headers: map[string][]string{
        "Cookie": {"session=abc123; csrf=xyz"},
    },
}
resp, _ := s.Do(ctx, req)
```

You lose automatic cookie tracking. Set-Cookie response headers are now
your problem to parse. Most scrapers should NOT go this route.

## Pattern 4: Save / LoadSession across restarts

Scraper runs for hours and might crash or restart? Save state to disk
periodically. On startup, load it. Crashes survive without losing warmed-up
tickets, ECH config, or cookies.

```go
const stateFile = "/var/lib/scraper/state.json"

func main() {
    var s *httpcloak.Session

    // Try to load from disk. If it fails, start fresh.
    if loaded, err := httpcloak.LoadSession(stateFile); err == nil {
        s = loaded
        log.Println("loaded state from disk")
    } else {
        s = httpcloak.NewSession("chrome-latest")
        // Cold-start warmup.
        ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
        s.Warmup(ctx, "https://example.com")
        cancel()
    }
    defer s.Close()

    // Save every 5 minutes.
    saveTicker := time.NewTicker(5 * time.Minute)
    defer saveTicker.Stop()

    go func() {
        for range saveTicker.C {
            if err := s.Save(stateFile); err != nil {
                log.Printf("save: %v", err)
            }
        }
    }()

    // Save on shutdown.
    defer func() {
        if err := s.Save(stateFile); err != nil {
            log.Printf("save on shutdown: %v", err)
        }
    }()

    scrapeLoop(s)
}
```

Don't want a file? Use `Marshal()` / `UnmarshalSession()` to serialise to
bytes (e.g. stash it in Redis):

```go
data, _ := s.Marshal()
redisClient.Set(ctx, "scraper:state", data, 24*time.Hour)

// Later:
data, _ := redisClient.Get(ctx, "scraper:state").Bytes()
s, _ := httpcloak.UnmarshalSession(data)
```

## Putting it together

The full long-running pattern in pseudocode shape:

```go
func main() {
    s := loadOrCreate("state.json")
    defer s.Close()
    defer s.Save("state.json")

    refreshEvery := 3 * time.Minute
    saveEvery := 5 * time.Minute

    nextRefresh := time.Now().Add(refreshEvery)
    nextSave := time.Now().Add(saveEvery)

    for {
        url, ok := nextWorkItem()
        if !ok {
            break
        }

        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        resp, err := s.Get(ctx, url)
        cancel()

        if err != nil {
            handleError(err)
            continue
        }
        process(resp)
        resp.Close()

        if time.Now().After(nextRefresh) {
            s.Refresh()
            nextRefresh = time.Now().Add(refreshEvery)
        }
        if time.Now().After(nextSave) {
            s.Save("state.json")
            nextSave = time.Now().Add(saveEvery)
        }
    }
}

func loadOrCreate(path string) *httpcloak.Session {
    if s, err := httpcloak.LoadSession(path); err == nil {
        return s
    }
    s := httpcloak.NewSession("chrome-latest")
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()
    s.Warmup(ctx, "https://example.com")
    return s
}
```

Tune cadences to your target. Faster scrapers want longer refresh intervals
(don't cycle TCP under sub-second request load). Slower scrapers running
once a minute can refresh every 5-10 minutes no problem.

## Things to NOT do

- **Don't refresh on every request.** You're tossing the connection benefit
  you just paid for.
- **Don't save state on every request.** Disk IO will dominate. Every few
  minutes is plenty.
- **Don't share one session across very different targets.** Cookies leak
  across hosts in the jar. One session per target is safer.
- **Don't forget `resp.Close()`.** Body leaks tie up connection resources,
  which makes Refresh way less effective.

## Forking sessions for parallel scrapes

Want N parallel workers all hitting the same target with the same warm
session state? Use `Fork(n)`:

```go
s := httpcloak.NewSession("chrome-latest")
s.Warmup(ctx, "https://example.com")

workers := s.Fork(8)
var wg sync.WaitGroup
for _, w := range workers {
    wg.Add(1)
    go func(w *httpcloak.Session) {
        defer wg.Done()
        defer w.Close()
        runWorkerLoop(w)
    }(w)
}
wg.Wait()
s.Close()
```

Each forked session shares the cookie jar and the TLS ticket cache (so all
8 workers get 0-RTT on the first request) but keeps its own connection
state. Basically 8 browser tabs sharing one profile.

## Related

- [Refresh](../connection-lifecycle/refresh), what `Refresh()` does
- [Warmup](../connection-lifecycle/warmup), full warmup mechanics
- [Session Save/Restore](../connection-lifecycle/session-save-restore), Save / Load API
- [Cookies and State](../cookies-and-state/), cookie jar internals
