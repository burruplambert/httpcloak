---
title: Streaming Responses
sidebar_position: 5
---

# Streaming Responses

`DoStream()` returns a response whose body is read incrementally. Use it for:

- **Big downloads.** Don't buffer a 2 GB file in RAM.
- **Server-Sent Events.** Long-lived connections that drip events.
- **NDJSON / line-delimited streams.** Read one record at a time.
- **Anything chunked encoding.** When the server doesn't know the content length up front.

The non-streaming `Do()` reads the full body into memory before returning. `DoStream()` returns as soon as the response headers arrive, and you read the body yourself.

:::info
`DoStream` pre-1.6.6 didn't update the cookie jar from the response. If you're on an older version, upgrade or extract Set-Cookie headers manually. Bug fixed in 1.6.6.
:::

## The shape

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs groupId="lang">
<TabItem value="go" label="Go">

`Session.DoStream(ctx, req)` returns a `*StreamResponse`. It implements `io.Reader`, so anything that takes a Reader works (bufio.Scanner, json.Decoder, io.Copy, etc.).

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

`Close()` is mandatory. Defer it the moment you have the stream. Forgetting to close leaks the underlying connection (which means it doesn't go back into the pool, and you eat the dial cost on the next request).

</TabItem>
<TabItem value="python" label="Python">

The Python binding rolls streaming into `get(stream=True)`:

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

Always call `stream.close()` after you're done iterating, otherwise the connection leaks.

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

The body is just bytes coming off the wire. You decide how to split them.

- **Line-delimited.** `bufio.Scanner` (Go), `iter_lines()` (Python), readline loop (Node).
- **Fixed-size chunks.** `Read(buf)` (Go), `iter_content(chunk_size=N)` (Python), `read(N)` (Node).
- **JSON streams.** Wrap in a JSON decoder. Go: `json.NewDecoder(stream).Decode(&v)` in a loop for NDJSON. Python: `for line in r.iter_lines(): obj = json.loads(line)`.
- **Pipe to a file.** Go: `io.Copy(file, stream)`. Python: `for chunk in r.iter_content(8192): f.write(chunk)`.

## Lifetime and Close

The contract: **caller must call Close when done**. There is no automatic GC fallback because the stream wraps real syscall resources (a TCP socket, an H2 stream window, an H3 stream).

Common ways to forget:

- Returning early from a function on an error path without `defer stream.Close()`. Always defer right after the err check.
- Iterating partially then bailing without closing.
- In Python, not using `with`. The non-with form needs an explicit `r.close()`.

Closing partway through is fine. The lib reads-and-discards the rest in the background to keep the underlying connection clean for reuse, or hard-aborts the H2/H3 stream if there's a lot left.

## ContentLength and chunked

`stream.ContentLength` (or `content_length` / `contentLength` in bindings) is `-1` when the server uses chunked transfer encoding (or H2/H3 without an explicit content-length frame). Don't assume it's positive when sizing a download progress bar.

If you actually need to know the size up front: send a `HEAD` request first, read `Content-Length` from the response headers, then `DoStream()` the GET. Most servers send a length on HEAD even when they'd switch to chunked on GET.

## Cookie jar parity (since 1.6.6)

Streaming responses now go through the same cookie extraction path as regular responses. `Set-Cookie` headers from the response (or any in-stream redirect that the lib resolved before handing you the body) end up in the session jar.

Before 1.6.6, streaming bypassed the jar update and you'd silently miss cookies from streamed endpoints. The fix landed in [#5491c85](../changelog) so behavior is now identical between `Do` and `DoStream`.

If you're stuck on an older version and can't upgrade:

```go
// Manual cookie extraction from a streamed response, pre-1.6.6 workaround.
for _, sc := range stream.Headers["Set-Cookie"] {
    // parse sc with net/http or store as raw and inject on next request
}
```

## A note on H2 and H3

When you stream over HTTP/2 or HTTP/3, the underlying transport still uses a single multiplexed connection. So `stream.Close()` doesn't kill the connection itself, just the one stream on it. You can have multiple streaming requests in flight on the same H2 connection at once, which is great for things like SSE + an API call running side by side.

On HTTP/1.1, a streaming response holds the whole TCP connection until you close. Concurrent requests need separate connections. The lib handles connection pooling either way, you don't have to think about it, but be aware that 100 concurrent streams on H1 means 100 TCP connections.
