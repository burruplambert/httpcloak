# Response to @desperatee

## PRI * HTTP/2.0 Request

That's **completely normal behavior** - it's not a bug!

The `PRI * HTTP/2.0` is the **HTTP/2 Connection Preface** defined in RFC 7540. Every HTTP/2 connection starts with this magic string:

```
PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n
```

The `%2A` you're seeing is just URL-encoded `*` (asterisk). This is how HTTP/2 identifies itself to the server before sending the actual SETTINGS frame and requests. You don't see this on HTTP/1.1 because H1 doesn't have a connection preface.

If you're using a proxy logger or traffic capture tool (like Charles Proxy), it will display this as a "request" but it's just the protocol handshake.

---

## Host Header in resp.Request.Headers

Good catch - this was indeed a bug! I've fixed it.

**What was happening:**
The Host header was being set on Go's internal `*http.Request` but wasn't being copied back to our `Request.Headers` struct that gets returned in `resp.Request`.

**The fix:**
Now when the response is created, we ensure the Host header is included in `req.Headers`:

```go
// Ensure Host is in req.Headers for net/http compatibility
if req.Headers == nil {
    req.Headers = make(map[string][]string)
}
if _, hasHost := req.Headers["Host"]; !hasHost {
    req.Headers["Host"] = []string{parsedURL.Hostname()}
}
```

After updating, this will work:
```go
resp, _ := client.Get(ctx, "https://example.com", nil)
host := resp.Request.GetHeader("Host")  // Returns "example.com"
fmt.Println(resp.Request.Headers)       // Shows Host header
```

---

## Additional Fixes (Found During Investigation)

While fixing the Host header issue, I found and fixed several related case-sensitivity bugs:

### 1. Request.GetHeader() was case-sensitive
```go
// BEFORE: Would fail if header was set with different case
req.GetHeader("host")  // Returns "" if set as "Host"

// AFTER: Works regardless of case
req.GetHeader("host")  // Returns value even if set as "Host"
```

### 2. Accept header lookups were case-sensitive
The mode detection (`FetchModeNavigate` vs `FetchModeCORS`) and Accept header override were case-sensitive. Now they work with any case (`"Accept"`, `"accept"`, `"ACCEPT"`).

### 3. Host header duplication prevention
If you set `"host"` (lowercase) in headers, the library would add another `"Host"` (capitalized) - now it correctly detects existing Host headers case-insensitively.

---

Let me know if you need anything else!
