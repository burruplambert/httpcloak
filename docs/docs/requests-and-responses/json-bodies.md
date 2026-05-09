---
title: JSON Bodies
sidebar_position: 3
---

# JSON Bodies

Most APIs talk JSON. Send a body, parse a body. Bread and butter stuff.

## Sending JSON

The shape: pass a struct (Go) or a dict / object (Python, Node, .NET), the lib serializes it, sets `Content-Type: application/json`, and ships it out.

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs groupId="lang">
<TabItem value="go" label="Go">

In Go you build the body yourself with `encoding/json` and pass an `io.Reader`. The Go API stays explicit so you can stream big payloads instead of buffering them whole.

```go
package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"

    httpcloak "github.com/sardanioss/httpcloak"
)

func main() {
    s := httpcloak.NewSession("chrome-latest")
    defer s.Close()

    payload := map[string]any{"hello": "world", "n": 42}
    body, _ := json.Marshal(payload)

    req := &httpcloak.Request{
        Method: "POST",
        URL:    "https://httpbin.org/post",
        Headers: map[string][]string{
            "Content-Type": {"application/json"},
        },
        Body: bytes.NewReader(body),
    }
    resp, _ := s.Do(context.Background(), req)
    defer resp.Close()

    var parsed struct {
        JSON map[string]any `json:"json"`
    }
    resp.JSON(&parsed)
    fmt.Println(parsed.JSON) // map[hello:world n:42]
}
```

The non-session client has a shortcut too: `client.PostJSON(ctx, url, bodyBytes)` sets the Content-Type for you.

</TabItem>
<TabItem value="python" label="Python">

```python
import httpcloak

s = httpcloak.Session(preset="chrome-latest")

r = s.post(
    "https://httpbin.org/post",
    json={"hello": "world", "n": 42},
)

print(r.json()["json"])  # {"hello": "world", "n": 42}
```

The `json=` kwarg handles serialization and sets `Content-Type: application/json`. Already serialized the body? Use `data=` and set the header yourself.

</TabItem>
<TabItem value="nodejs" label="Node.js">

```js
const { Session } = require("httpcloak");

const s = new Session({ preset: "chrome-latest" });

const r = await s.post("https://httpbin.org/post", {
  json: { hello: "world", n: 42 },
});

console.log(r.json().json); // { hello: "world", n: 42 }
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
using HttpCloak;

using var s = new Session(new SessionOptions { Preset = "chrome-latest" });

var payload = new { hello = "world", n = 42 };
var r = s.PostJson("https://httpbin.org/post", payload);

Console.WriteLine(r.Text);
```

`PostJson` uses `System.Text.Json` under the hood and sets the Content-Type header for you. No ceremony.

</TabItem>
</Tabs>

## Reading JSON

Responses come back as bytes you can parse on demand.

<Tabs groupId="lang">
<TabItem value="go" label="Go">

```go
resp, _ := s.Get(ctx, "https://httpbin.org/json")
defer resp.Close()

var data struct {
    Slideshow struct {
        Title string `json:"title"`
    } `json:"slideshow"`
}
if err := resp.JSON(&data); err != nil {
    // not valid JSON, or read error
}
fmt.Println(data.Slideshow.Title)
```

`resp.JSON(&v)` reads the full body and unmarshals into `v`. The body's buffered after the first read, so you can call `resp.Bytes()` or `resp.Text()` afterward and get the same payload back.

</TabItem>
<TabItem value="python" label="Python">

```python
r = s.get("https://httpbin.org/json")
data = r.json()
print(data["slideshow"]["title"])
```

</TabItem>
<TabItem value="nodejs" label="Node.js">

```js
const r = await s.get("https://httpbin.org/json");
const data = r.json();
console.log(data.slideshow.title);
```

</TabItem>
<TabItem value="dotnet" label=".NET">

```csharp
var r = s.Get("https://httpbin.org/json");
using var doc = JsonDocument.Parse(r.Text);
var title = doc.RootElement
    .GetProperty("slideshow")
    .GetProperty("title")
    .GetString();
```

</TabItem>
</Tabs>

## Pitfalls worth knowing

### Content-Type sniffing

Send a body with no `Content-Type` and something downstream might sniff it and pick one for you. JSON requests **must** carry `Content-Type: application/json`, otherwise:

- Some servers treat the body as form data and 400 you back.
- Some WAFs flag the request as suspicious.

The shortcut wrappers (`PostJson`, `post(json=...)`, etc.) set this for you. Build the request by hand and you're on your own.

### Encoding

JSON is always UTF-8 on the wire. Don't try Latin-1 or UTF-16 even if the server "would accept it." Real browsers send UTF-8. Anti-bot products expect UTF-8.

### Big responses

If you're pulling something multi-MB or bigger, don't buffer it whole. Use the streaming API instead. See [Streaming Responses](./streaming-responses).

`resp.JSON()` and `resp.Bytes()` both pull the entire body into memory. For a 200MB JSON dump, that's gonna hurt.

### Numbers

Go: JSON numbers default to `float64`. Got integer IDs over 2^53 (snowflake IDs and friends)? Use `json.Number` or string-encode them. Python and Node native ints don't have this issue. .NET's `JsonElement.GetInt64()` handles 64-bit cleanly.

### Trailing newlines, BOMs

httpbin and most decent servers don't send these, but if you hit a server that prefixes its JSON with a UTF-8 BOM (`\xef\xbb\xbf`), most parsers choke. Strip the BOM before parsing. Rare, but it pops up on some old Java backends.

## A test pattern that just works

The httpbin echo endpoints are perfect for verifying your client serializes correctly:

- `POST https://httpbin.org/post` echoes the request as JSON, including a parsed `json` field if you sent JSON.
- `PUT https://httpbin.org/put`, `DELETE https://httpbin.org/delete`, `PATCH https://httpbin.org/patch` all do the same for their methods.

So if your client should be sending `{"hello": "world"}`, hit `/post` and check that `response.json["hello"] == "world"`. Missing or wrong type? The lib didn't serialize what you thought, or the Content-Type was off.
