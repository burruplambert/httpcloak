---
title: Streaming Responses
sidebar_position: 5
---

# Streaming Responses

`DoStream()` returns a response whose body you read incrementally. It returns the moment the response headers arrive, leaving the body for the caller to pull. Plain `Do()` reads the full body into memory before returning, which is fine for small payloads and the wrong call for large ones.

Use streaming for:

- **Big downloads.** A 2 GB file doesn't belong in RAM.
- **Server-Sent Events.** Long-lived connections that drip events.
- **NDJSON / line-delimited streams.** One record at a time.
- **Anything chunked.** When the server doesn't know the content length up front.

:::info
Pre-1.6.6, `DoStream` didn't update the cookie jar from the response. On an older version, upgrade or extract Set-Cookie headers by hand. The bug was fixed in 1.6.6.
:::

## The shape

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs groupId="lang">
<TabItem value="go" label="Go">

`Session.DoStream(ctx, req)` returns a `*StreamResponse` that implements `io.Reader`. Anything that takes a Reader works: bufio.Scanner, json.Decoder, io.Copy.

```go
package main

import (
    "bufio"
    "context"
    "fmt"

    httpcloak "github.com/sardanioss/httpcloak"
)

func main() {
    s := httpcloak.NewSession("chrome-latest")
    defer s.Close()

    stream, err := s.GetStream(context.Background(), "https://httpbin.org/stream/10")
    if err != nil {
        panic(err)
    }
    defer stream.Close()

    fmt.Println("status:", stream.StatusCode)
    fmt.Println("content-length:", stream.ContentLength) // -1 if chunked

    scanner := bufio.NewScanner(stream)
    n := 0
    for scanner.Scan() {
        n++
        fmt.Printf("chunk %d: %s\n", n, scanner.Text())
    }
    fmt.Printf("got %d lines\n", n)
}
```

`Close()` is mandatory. Defer it the second you have the stream. Without it, the underlying connection leaks instead of returning to the pool, and the next request eats the dial cost.

</TabItem>
<TabItem value="python" label="Python">

The Python binding folds streaming into `get(stream=True)`:

```python
import httpcloak

s = httpcloak.Session(preset="chrome-latest")

with s.get("https://httpbin.org/stream/10", stream=True) as r:
    print("status:", r.status_code)
    n = 0
    for line in r.iter_lines():
        n += 1
        print(f"chunk {n}: {line.decode()}")
    print(f"got {n} lines")
```

`iter_lines()` and `iter_content(chunk_size=N)` both work. The `with` block calls Close for you when the body is done.

</TabItem>
<TabItem value="nodejs" label="Node.js">

`session.getStream()` returns a StreamResponse you can iterate with `for await`:

```js
const { Session } = require("httpcloak");

const s = new Session({ preset: "chrome-latest" });

const stream = s.getStream("https://httpbin.org/stream/10");
console.log("status:", stream.statusCode);

let n = 0;
for await (const chunk of stream) {
  n++;
  console.log(`chunk ${n}: ${chunk.toString()}`);
}
stream.close();
console.log(`got ${n} chunks`);
```

Always call `stream.close()` after iterating, otherwise the connection leaks.

</TabItem>
<TabItem value="dotnet" label=".NET">

`Session.GetStream()` (and `RequestStream` for non-GET) returns a `StreamResponse` with a `Stream` body:

```csharp
using HttpCloak;

using var s = new Session(new SessionOptions { Preset = "chrome-latest" });

using var stream = s.GetStream("https://httpbin.org/stream/10");
Console.WriteLine($"status: {stream.StatusCode}");

using var content = stream.GetContentStream();
using var reader = new StreamReader(content);
int n = 0;
string? line;
while ((line = reader.ReadLine()) != null)
{
    n++;
    Console.WriteLine($"chunk {n}: {line}");
}
Console.WriteLine($"got {n} lines");
```

The `using` on the StreamResponse handles Close.

</TabItem>
</Tabs>

## What you can read it as

The body is bytes coming off the wire. The caller decides how to split them.

- **Line-delimited.** `bufio.Scanner` (Go), `iter_lines()` (Python), readline loop (Node).
- **Fixed-size chunks.** `Read(buf)` (Go), `iter_content(chunk_size=N)` (Python), `read(N)` (Node).
- **JSON streams.** Wrap in a JSON decoder. Go: `json.NewDecoder(stream).Decode(&v)` in a loop for NDJSON. Python: `for line in r.iter_lines(): obj = json.loads(line)`.
- **Pipe to a file.** Go: `io.Copy(file, stream)`. Python: `for chunk in r.iter_content(8192): f.write(chunk)`.

## Lifetime and Close

The contract is the caller must call Close when done. There's no GC fallback because the stream wraps real syscall resources: a TCP socket, an H2 stream window, an H3 stream.

Common ways to forget:

- Returning early from a function on an error path without `defer stream.Close()`. Always defer right after the err check.
- Iterating partway and bailing without closing.
- In Python, skipping `with`. The non-with form needs an explicit `r.close()`.

Closing partway through is fine. The lib reads-and-discards the rest in the background to keep the underlying connection clean for reuse, or hard-aborts the H2/H3 stream when there's a lot of body left.

## ContentLength and chunked

`stream.ContentLength` (or `content_length` / `contentLength` in the bindings) is `-1` when the server uses chunked transfer encoding, or H2/H3 without an explicit content-length frame. Don't assume it's positive when sizing a download progress bar.

To know the size up front, fire a `HEAD` request first, read `Content-Length` from the response headers, then `DoStream()` the GET. Most servers send a length on HEAD even when they switch to chunked on GET.

## Cookie jar parity (since 1.6.6)

Streaming responses go through the same cookie extraction path as regular ones. `Set-Cookie` headers from the response, including any in-stream redirect the lib resolved before handing you the body, land in the session jar.

Before 1.6.6, streaming bypassed the jar update and you'd silently miss cookies from streamed endpoints. The fix landed in [#5491c85](../changelog), so `Do` and `DoStream` now behave identically.

For older versions where upgrading isn't an option:

```go
// Manual cookie extraction from a streamed response, pre-1.6.6 workaround.
for _, sc := range stream.Headers["Set-Cookie"] {
    // parse sc with net/http or store as raw and inject on next request
}
```

## A note on H2 and H3

Streaming over HTTP/2 or HTTP/3 still rides on a single multiplexed connection underneath. `stream.Close()` doesn't kill that connection, just the one stream on it. Multiple streaming requests can run in flight on the same H2 connection at once, which works well for SSE plus an API call running side by side.

On HTTP/1.1, a streaming response holds the whole TCP connection until you close. Concurrent requests need separate connections. The lib handles connection pooling either way so the caller doesn't manage it, with the caveat that 100 concurrent streams on H1 means 100 TCP connections.
