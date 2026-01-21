# Issue: resp.Request.Headers missing Host header

**Status: FIXED**

## Reporter
@desperatee via GitHub issue

## Problem
```go
resp.Request.GetHeader("Host")  // returns ""
fmt.Println(resp.Request.Headers)  // doesn't show Host header
```

Unlike net/http behavior, breaks code when ported to httpcloak.

## Root Cause
The Host header is set on `httpReq.Header` (Go's standard `*http.Request`) in both:
- `applyTLSOnlyHeaders()` at line 1389
- `applyModeHeaders()` at line 1432

But it's NOT copied back to `req.Headers` (our custom `*Request` struct).

When Response is created at line 1005:
```go
response := &Response{
    ...
    Request: req,  // req.Headers doesn't have Host!
    ...
}
```

The user's original `req.Headers` is returned, which never had Host added to it.

## Expected Behavior (net/http)
In Go's standard library, `http.Request.Host` is a separate field, and when you access `req.Header.Get("Host")`, Go's http package returns `req.Host` transparently.

## Fix Options

### Option 1: Add Host to req.Headers before returning Response
In `client.go` around line 998, before creating the Response:
```go
// Ensure Host is in req.Headers for net/http compatibility
if req.Headers == nil {
    req.Headers = make(map[string][]string)
}
if _, hasHost := req.Headers["Host"]; !hasHost {
    req.Headers["Host"] = []string{parsedURL.Hostname()}
}
```

### Option 2: Update Request.GetHeader to check Host specially
```go
func (r *Request) GetHeader(key string) string {
    // For Host, check both "Host" and extract from URL if needed
    if strings.EqualFold(key, "Host") {
        if values := r.Headers["Host"]; len(values) > 0 {
            return values[0]
        }
        // Try to extract from URL
        if r.URL != "" {
            if parsed, err := url.Parse(r.URL); err == nil {
                return parsed.Hostname()
            }
        }
    }
    if values := r.Headers[key]; len(values) > 0 {
        return values[0]
    }
    return ""
}
```

### Recommended: Option 1
Simpler, more predictable, and ensures `resp.Request.Headers` contains all sent headers.

## Files to Modify
- `client/client.go`: Add Host to req.Headers before creating Response

## Testing
```go
resp, _ := client.Get(ctx, "https://example.com", nil)
host := resp.Request.GetHeader("Host")
// Should return "example.com"
```
