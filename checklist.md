# HTTPCloak Bindings Feature Checklist

## Python Bindings

### Session Management
- [x] Session class with context manager (`with` statement)
- [x] Session.close() method
- [x] Default session for module-level functions
- [x] configure() function for global defaults

### HTTP Methods
- [x] GET
- [x] POST
- [x] PUT
- [x] DELETE
- [x] PATCH
- [x] HEAD
- [x] OPTIONS
- [x] Generic request() method

### Request Options
- [x] Custom headers per request
- [x] Default headers via session.headers
- [x] Query parameters (`params=`)
- [x] JSON body (`json=`)
- [x] Form data (`data=` with dict)
- [x] Raw body (`data=` with str/bytes)
- [x] Basic auth (`auth=(user, pass)`)
- [x] Timeout per request
- [x] Proxy support (session-level)
- [x] Force HTTP version (`http_version="h1"/"h2"/"h3"/"auto"`)
- [ ] allow_redirects parameter
- [ ] verify SSL parameter (skip cert verification)
- [ ] max_redirects parameter
- [ ] File uploads (multipart/form-data)

### Response Object
- [x] status_code
- [x] headers
- [x] text (string body)
- [x] content (bytes body)
- [x] json() method
- [x] url (final URL after redirects)
- [x] ok property (status < 400)
- [x] raise_for_status() method
- [x] protocol (h2, h3, etc.)
- [ ] encoding property
- [ ] history (redirect chain)
- [ ] elapsed (request duration)
- [ ] cookies (response cookies)

### Cookie Management
- [x] session.get_cookies()
- [x] session.set_cookie(name, value)
- [x] session.cookies property

### Async Support
- [x] get_async()
- [x] post_async()
- [x] request_async()

### Module-level Functions
- [x] httpcloak.get()
- [x] httpcloak.post()
- [x] httpcloak.put()
- [x] httpcloak.delete()
- [x] httpcloak.patch()
- [x] httpcloak.head()
- [x] httpcloak.options()
- [x] httpcloak.request()
- [x] httpcloak.configure()

### Utility
- [x] available_presets()
- [x] version()

---

## Node.js Bindings

### Session Management
- [x] Session class
- [x] session.close() method
- [x] Default session for module-level functions
- [x] configure() function for global defaults

### HTTP Methods (Promise-based)
- [x] get()
- [x] post()
- [x] put()
- [x] delete()
- [x] patch()
- [x] head()
- [x] options()
- [x] request()

### HTTP Methods (Sync)
- [x] getSync()
- [x] postSync()
- [x] requestSync()

### Request Options
- [x] Custom headers per request
- [x] Default headers via session.headers
- [x] Query parameters (`params`)
- [x] JSON body (`json`)
- [x] Form data (`data`)
- [x] Basic auth (`auth: [user, pass]`)
- [x] Timeout per request
- [x] Proxy support (session-level)
- [x] Force HTTP version (`httpVersion: "h1"/"h2"/"h3"/"auto"`)

### Response Object
- [x] statusCode
- [x] headers
- [x] text
- [x] body (Buffer)
- [x] content (Buffer alias)
- [x] json() method
- [x] finalUrl
- [x] url (alias for finalUrl)
- [x] protocol
- [x] ok property
- [x] raiseForStatus() method

### Cookie Management
- [x] session.getCookies()
- [x] session.setCookie(name, value)
- [x] session.cookies getter

### Module-level Functions
- [x] httpcloak.get()
- [x] httpcloak.post()
- [x] httpcloak.put()
- [x] httpcloak.delete()
- [x] httpcloak.patch()
- [x] httpcloak.head()
- [x] httpcloak.options()
- [x] httpcloak.request()
- [x] httpcloak.configure()

### Utility
- [x] availablePresets()
- [x] version()

---

## Remaining Tasks (Low Priority)

### Nice to Have
- [ ] File upload support (multipart)
- [ ] Response.history for redirect chain
- [ ] Skip SSL verification option
- [ ] Retry logic with backoff
- [ ] Response.elapsed for timing
- [ ] allow_redirects parameter

---

## Notes

### Available Presets
- chrome-143, chrome-143-windows, chrome-143-linux, chrome-143-macos
- chrome-131, chrome-131-windows, chrome-131-linux, chrome-131-macos
- firefox-133
- safari-18

### HTTP Version Options
- `"auto"` - Auto-detect (H3 -> H2 -> H1 fallback) [default]
- `"h1"` or `"http1"` - Force HTTP/1.1
- `"h2"` or `"http2"` - Force HTTP/2 (disables H3)
- `"h3"` or `"http3"` - Prefer HTTP/3 (same as auto)

### Usage Examples

**Python:**
```python
import httpcloak

# Simple usage
r = httpcloak.get("https://example.com")

# With configuration
httpcloak.configure(
    preset="chrome-143-linux",
    http_version="h2",
    auth=("user", "pass"),
)
r = httpcloak.get("https://api.example.com")

# With session
with httpcloak.Session(preset="firefox-133", http_version="h1") as session:
    r = session.get("https://example.com", params={"q": "test"})
```

**Node.js:**
```javascript
const httpcloak = require("httpcloak");

// Simple usage
const r = await httpcloak.get("https://example.com");

// With configuration
httpcloak.configure({
    preset: "chrome-143-linux",
    httpVersion: "h2",
    auth: ["user", "pass"],
});
const r = await httpcloak.get("https://api.example.com");

// With session
const session = new httpcloak.Session({ preset: "firefox-133", httpVersion: "h1" });
const r = await session.get("https://example.com", { params: { q: "test" } });
session.close();
```
