# HTTPCloak API Reference

This document covers the public API for Go and language bindings.

## Go API

### Quick Start

```go
import "github.com/sardanioss/httpcloak"

// Simple request
response, err := httpcloak.Get("https://example.com")

// With session
session := httpcloak.NewSession(httpcloak.SessionConfig{
    Preset: "chrome-144",
})
defer session.Close()

response, err := session.Get(ctx, "https://example.com", nil)
```

### Session

#### Creating a Session

```go
// session/session.go:55-119
func NewSession(id string, config *protocol.SessionConfig) *Session

// With config
session := httpcloak.NewSession(httpcloak.SessionConfig{
    Preset:          "chrome-144",     // Browser fingerprint
    Proxy:           "socks5://...",   // Proxy URL
    TCPProxy:        "http://...",     // TCP-only proxy
    UDPProxy:        "socks5://...",   // UDP-only proxy
    Timeout:         30,               // Request timeout (seconds)
    FollowRedirects: true,             // Auto-follow redirects
    MaxRedirects:    10,               // Max redirect count
    RetryEnabled:    false,            // Enable retry logic
    MaxRetries:      3,                // Max retry attempts
    RetryWaitMin:    500,              // Min wait between retries (ms)
    RetryWaitMax:    10000,            // Max wait between retries (ms)
    RetryOnStatus:   []int{429, 500},  // Status codes to retry
    ForceHTTP3:      false,            // Force HTTP/3
    DisableHTTP3:    false,            // Disable HTTP/3
    PreferIPv4:      false,            // Prefer IPv4 over IPv6
    ECHConfigDomain: "",               // Domain for ECH config
    ConnectTo:       nil,              // Host mapping for domain fronting
    TLSOnly:         false,            // TLS-only mode (skip preset HTTP headers)
    QuicIdleTimeout: 30,               // QUIC idle timeout in seconds (default: 30)
    LocalAddress:    "",               // Local IP address to bind outgoing connections
    KeyLogFile:      "",               // Path to write TLS keys for Wireshark decryption
})
```

#### Session Methods

```go
// HTTP methods
func (s *Session) Get(ctx context.Context, url string, headers map[string][]string) (*Response, error)
func (s *Session) Post(ctx context.Context, url string, body []byte, headers map[string][]string) (*Response, error)
func (s *Session) Request(ctx context.Context, req *transport.Request) (*Response, error)

// Streaming
func (s *Session) GetStream(ctx context.Context, url string, headers map[string][]string) (*StreamResponse, error)
func (s *Session) PostStream(ctx context.Context, url string, body []byte, headers map[string][]string) (*StreamResponse, error)
func (s *Session) RequestStream(ctx context.Context, req *transport.Request) (*StreamResponse, error)

// Cookie management
func (s *Session) GetCookies() map[string]string
func (s *Session) SetCookie(name, value string)
func (s *Session) SetCookies(cookies map[string]string)
func (s *Session) ClearCookies()

// Proxy management
func (s *Session) SetProxy(proxyURL string)
func (s *Session) SetTCPProxy(proxyURL string)
func (s *Session) SetUDPProxy(proxyURL string)
func (s *Session) GetProxy() string
func (s *Session) GetTCPProxy() string
func (s *Session) GetUDPProxy() string

// Header order customization
func (s *Session) SetHeaderOrder(order []string)  // Set custom header order (nil to reset)
func (s *Session) GetHeaderOrder() []string       // Get current header order

// Cache management
func (s *Session) ClearCache()

// Persistence
func (s *Session) Save(path string) error
func (s *Session) Marshal() ([]byte, error)
func LoadSession(path string) (*Session, error)
func UnmarshalSession(data []byte) (*Session, error)

// Lifecycle
func (s *Session) Close()
func (s *Session) IsActive() bool
func (s *Session) Stats() SessionStats
```

### Request

```go
// transport/transport.go:126-134
type Request struct {
    Method     string
    URL        string
    Headers    map[string][]string // Multi-value headers
    Body       []byte
    BodyReader io.Reader           // For streaming uploads
    Timeout    time.Duration
}

// Methods (client.Request)
func (r *Request) SetHeader(key, value string)    // Set header (replaces existing)
func (r *Request) AddHeader(key, value string)    // Add header (preserves existing)
func (r *Request) GetHeader(key string) string    // Get header (case-insensitive)
```

#### Header Case-Insensitivity

`Request.GetHeader()` is case-insensitive per RFC 7230:

```go
req.Headers = map[string][]string{"content-type": {"application/json"}}
req.GetHeader("Content-Type")  // Returns "application/json"
req.GetHeader("CONTENT-TYPE")  // Returns "application/json"
```

### Response

```go
// transport/transport.go:143-156
type Response struct {
    StatusCode int
    Headers    map[string][]string // Multi-value headers
    Body       io.ReadCloser       // Streaming body
    FinalURL   string
    Timing     *protocol.Timing
    Protocol   string              // "h1", "h2", or "h3"
    History    []*RedirectInfo
}

// Methods
func (r *Response) Close() error
func (r *Response) GetHeader(key string) string    // Case-insensitive
func (r *Response) GetHeaders(key string) []string // Case-insensitive
func (r *Response) Bytes() ([]byte, error)
func (r *Response) Text() (string, error)
```

#### Accessing Request from Response (client package)

The `client.Response` includes the original request with all headers that were actually sent:

```go
// client.Response has Request field
resp, _ := client.Do(ctx, &client.Request{
    Method: "GET",
    URL:    "https://example.com",
    Headers: map[string][]string{
        "X-Custom": {"value"},
    },
})

// resp.Request.Headers contains ALL sent headers:
// - User-provided headers (X-Custom)
// - Auto-added headers (User-Agent, Accept, Accept-Encoding, etc.)
// - Sec-Fetch-* headers
// - Cookies, Auth headers
// - Host header

fmt.Println(resp.Request.Headers)
// map[Accept:[text/html,...] Accept-Encoding:[gzip, deflate, br, zstd]
//     Host:[example.com] User-Agent:[Mozilla/5.0...] X-Custom:[value] ...]

resp.Request.GetHeader("User-Agent")  // Returns the User-Agent that was sent
resp.Request.GetHeader("Host")        // Returns "example.com"
```

This is useful for debugging to see exactly what headers were sent to the server.

### Transport

```go
// transport/transport.go:211-233
type Transport struct {
    // Internal transports for each protocol
}

// Methods
func (t *Transport) Do(ctx context.Context, req *Request) (*Response, error)
func (t *Transport) SetProtocol(p Protocol)
func (t *Transport) SetTimeout(timeout time.Duration)
func (t *Transport) SetProxy(proxy *ProxyConfig)
func (t *Transport) SetPreset(presetName string)
func (t *Transport) SetInsecureSkipVerify(skip bool)
func (t *Transport) SetConnectTo(requestHost, connectHost string)
func (t *Transport) SetECHConfig(echConfig []byte)
func (t *Transport) SetECHConfigDomain(domain string)
func (t *Transport) Close()
func (t *Transport) Stats() map[string]interface{}
```

### Presets

```go
// fingerprint/presets.go:582-597
func Get(name string) *Preset
func Available() []string

// Available presets (18 total)
available := fingerprint.Available()
// ["chrome-144", "chrome-144-windows", "chrome-144-linux", "chrome-144-macos",
//  "chrome-143", "chrome-143-windows", ..., "firefox-133", "safari-18",
//  "safari-18-ios", "safari-17-ios", "chrome-144-ios", "chrome-143-ios",
//  "chrome-144-android", "chrome-143-android"]
```

### Session Manager

```go
// session/manager.go:12-23
type Manager struct {
    // Manages multiple sessions
}

func NewManager() *Manager
func (m *Manager) CreateSession(config *protocol.SessionConfig) (string, error)
func (m *Manager) GetSession(sessionID string) (*Session, error)
func (m *Manager) CloseSession(sessionID string) error
func (m *Manager) ListSessions() []SessionStats
func (m *Manager) SessionCount() int
func (m *Manager) Shutdown()
func (m *Manager) SetMaxSessions(max int)
func (m *Manager) SetSessionTimeout(timeout time.Duration)
```

### Client Package

The `client` package provides a high-level HTTP client API with comprehensive configuration options.

```go
import "github.com/sardanioss/httpcloak/client"

// Create client with default settings
c := client.NewClient("chrome-144")
defer c.Close()

// Create client with options
c := client.NewClient("chrome-144",
    client.WithTimeout(60*time.Second),
    client.WithProxy("socks5://proxy:1080"),
    client.WithTLSOnly(),
    client.WithForceHTTP2(),
)
```

#### Client Options

| Option | Description |
|--------|-------------|
| `WithPreset(name)` | Browser fingerprint preset |
| `WithTimeout(d)` | Request timeout |
| `WithProxy(url)` | Proxy URL (http, https, socks5, masque) |
| `WithTCPProxy(url)` | Proxy for TCP traffic (H1/H2) |
| `WithUDPProxy(url)` | Proxy for UDP traffic (H3/MASQUE) |
| `WithForceHTTP1()` | Force HTTP/1.1 only |
| `WithForceHTTP2()` | Force HTTP/2 only |
| `WithForceHTTP3()` | Force HTTP/3 only |
| `WithDisableHTTP3()` | Disable H3, allow H2 with H1 fallback |
| `WithTLSOnly()` | TLS fingerprint only, skip preset HTTP headers |
| `WithRetry(n)` | Enable retry with max attempts |
| `WithRetryConfig(...)` | Configure retry behavior |
| `WithRedirects(follow, max)` | Configure redirect following |
| `WithoutRedirects()` | Disable redirect following |
| `WithInsecureSkipVerify()` | Skip TLS certificate verification |
| `WithPreferIPv4()` | Prefer IPv4 over IPv6 |
| `WithConnectTo(req, conn)` | Domain fronting - map request host to connect host |
| `WithECHConfig(cfg)` | Custom ECH configuration |
| `WithECHFrom(domain)` | Fetch ECH config from alternate domain |
| `WithDisableECH()` | Disable automatic ECH |
| `WithLocalAddress(ip)` | Bind to local IP address (IPv4/IPv6 rotation) |
| `WithKeyLogFile(path)` | Write TLS keys to file for Wireshark decryption |

#### Force Protocol

```go
// Force HTTP/1.1 - useful for servers that don't support H2
c := client.NewClient("chrome-144", client.WithForceHTTP1())

// Force HTTP/2 - no H3 or H1 fallback
c := client.NewClient("chrome-144", client.WithForceHTTP2())

// Force HTTP/3 - QUIC only (requires SOCKS5 or MASQUE proxy if using proxy)
c := client.NewClient("chrome-144", client.WithForceHTTP3())

// Disable H3 but allow H2 with H1 fallback
c := client.NewClient("chrome-144", client.WithDisableHTTP3())
```

#### Local Address Binding (IPv6 Rotation)

```go
// Bind outgoing connections to a specific local IP address
// Useful for IPv6 rotation when you have a large IPv6 prefix
c := client.NewClient("chrome-144", client.WithLocalAddress("2001:db8::1"))

// IPv4 also supported
c := client.NewClient("chrome-144", client.WithLocalAddress("192.168.1.100"))

// Works with all protocols: HTTP/1.1, HTTP/2, and HTTP/3
// If local address is IPv6, only IPv6 targets are dialed (and vice versa)
// Clear error if target has no addresses matching local address family
```

#### TLS Key Logging (Wireshark)

```go
// Write TLS keys to file for Wireshark decryption
c := client.NewClient("chrome-144", client.WithKeyLogFile("/tmp/keys.log"))

// Or use SSLKEYLOGFILE environment variable (global)
// export SSLKEYLOGFILE=/tmp/keys.log

// Works with HTTP/1.1 (TLS 1.3), HTTP/2, and HTTP/3 (QUIC)
// File format: NSS Key Log Format (compatible with Wireshark)
// In Wireshark: Edit → Preferences → Protocols → TLS → (Pre)-Master-Secret log filename
```

#### TLS-Only Mode

```go
// TLS-only mode: use TLS fingerprint but you control all HTTP headers
c := client.NewClient("chrome-144", client.WithTLSOnly())

// Make request with your own headers
resp, err := c.Do(ctx, &client.Request{
    Method: "GET",
    URL:    "https://example.com",
    Headers: map[string][]string{
        "User-Agent": {"MyCustomAgent/1.0"},
        "Accept":     {"text/html"},
    },
})
```

### Cookie Management (CookieJar)

```go
// Get the CookieJar for advanced cookie operations
jar := c.Cookies()

// CookieJar methods
jar.Set(host string, cookie *CookieData, secure bool)  // Set cookie with full attributes
jar.SetSimple(host, name, value string)                // Set simple name=value cookie
jar.Get(u *url.URL) []*CookieData                      // Get cookies for URL
jar.CookieHeader(u *url.URL) string                    // Get Cookie header string
jar.AllCookies() map[string][]*CookieData              // Get all cookies by domain
jar.Count() int                                        // Total cookie count
jar.Clear()                                            // Clear all cookies
jar.ClearDomain(domain string)                         // Clear cookies for domain
jar.ClearExpired()                                     // Remove expired cookies

// CookieData structure
type CookieData struct {
    Name      string
    Value     string
    Domain    string     // Normalized domain (with leading dot for domain cookies)
    HostOnly  bool       // True if cookie should only be sent to exact host
    Path      string
    Expires   *time.Time
    MaxAge    int
    Secure    bool
    HttpOnly  bool
    SameSite  string     // "Strict", "Lax", or "None"
    CreatedAt time.Time
}
```

Cookie matching follows browser behavior:
- Domain cookies (`.example.com`) match subdomains
- Host-only cookies match exact host only
- Path matching uses prefix comparison
- Secure cookies only sent over HTTPS
- Cookies ordered by path length (longer first), then creation time

### QUIC Connection Management

```go
// Close all QUIC connections but keep session caches intact
// Useful for testing 0-RTT session resumption
c.CloseQUICConnections()
```

### Hooks

```go
// client/hooks.go
type PreRequestHook func(req *http.Request) error
type PostResponseHook func(resp *Response) error

type Hooks struct {
    // ...
}

func NewHooks() *Hooks
func (h *Hooks) OnPreRequest(hook PreRequestHook) *Hooks
func (h *Hooks) OnPostResponse(hook PostResponseHook) *Hooks
func (h *Hooks) Clear()
```

### Certificate Pinning

```go
// client/certpin.go:41-52
type CertPinner struct {
    // ...
}

func NewCertPinner() *CertPinner
func (p *CertPinner) AddPin(hash string, opts ...PinOption) *CertPinner
func (p *CertPinner) AddPinFromCertFile(certPath string, opts ...PinOption) error
func (p *CertPinner) AddPinFromPEM(pemData []byte, opts ...PinOption) error
func (p *CertPinner) Verify(host string, certs []*x509.Certificate) error
func (p *CertPinner) Clear()
func (p *CertPinner) HasPins() bool

// Options
func ForHost(host string) PinOption
func IncludeSubdomains() PinOption
```

### DNS Configuration

```go
// dns/cache.go

// SetECHDNSServers configures the DNS servers used for ECH config queries.
// By default, uses Google (8.8.8.8), Cloudflare (1.1.1.1), and Quad9 (9.9.9.9).
// Pass nil or empty slice to reset to defaults.
// Thread-safe.
func SetECHDNSServers(servers []string)

// GetECHDNSServers returns the current DNS servers used for ECH queries.
// Thread-safe.
func GetECHDNSServers() []string
```

### Distributed Session Cache

Enable TLS session sharing across distributed instances via pluggable cache backends (Redis, Memcached, etc.).

```go
// transport/tls_cache.go

// SessionCacheBackend is the interface for distributed TLS session storage
type SessionCacheBackend interface {
    // Get retrieves a TLS session for a host
    // Returns nil, nil if not found
    Get(ctx context.Context, key string) (*TLSSessionState, error)

    // Put stores a TLS session with TTL (typically 24 hours)
    Put(ctx context.Context, key string, session *TLSSessionState, ttl time.Duration) error

    // Delete removes a session (for invalidation)
    Delete(ctx context.Context, key string) error

    // GetECHConfig retrieves ECH config for HTTP/3 support
    GetECHConfig(ctx context.Context, key string) ([]byte, error)

    // PutECHConfig stores ECH config for HTTP/3 support
    PutECHConfig(ctx context.Context, key string, config []byte, ttl time.Duration) error
}

// ErrorCallback is called when cache operations fail
type ErrorCallback func(operation string, key string, err error)

// Cache key format helpers
func FormatSessionCacheKey(preset, protocol, host, port string) string
// Returns: "httpcloak:sessions:{preset}:{protocol}:{host}:{port}"

func FormatECHCacheKey(preset, host, port string) string
// Returns: "httpcloak:ech:{preset}:{host}:{port}"
```

#### Using Session Cache in Go

```go
// httpcloak.go

// WithSessionCache configures a distributed session cache backend
func WithSessionCache(backend transport.SessionCacheBackend, errorCallback transport.ErrorCallback) SessionOption

// Example with custom implementation
type RedisCache struct {
    client *redis.Client
}

func (r *RedisCache) Get(ctx context.Context, key string) (*transport.TLSSessionState, error) {
    data, err := r.client.Get(ctx, key).Bytes()
    if err == redis.Nil {
        return nil, nil // Not found
    }
    if err != nil {
        return nil, err
    }
    var state transport.TLSSessionState
    json.Unmarshal(data, &state)
    return &state, nil
}

func (r *RedisCache) Put(ctx context.Context, key string, session *transport.TLSSessionState, ttl time.Duration) error {
    data, _ := json.Marshal(session)
    return r.client.Set(ctx, key, data, ttl).Err()
}

// ... implement Delete, GetECHConfig, PutECHConfig similarly

// Usage
session := httpcloak.NewSession("chrome-144",
    httpcloak.WithSessionCache(redisCache, func(op, key string, err error) {
        log.Printf("Cache error: %s on %s: %v", op, key, err)
    }),
)
```

#### LocalProxy with Session Cache

```go
proxy, err := httpcloak.StartLocalProxy(0,
    httpcloak.WithProxyPreset("chrome-144"),
    httpcloak.WithProxySessionCache(redisCache, errorCallback),
)
```

### Local Proxy

Local HTTP proxy server for transparent HttpClient integration. Enables any HTTP client to use httpcloak's TLS fingerprinting without FFI limitations.

```go
// local_proxy.go

// Configuration options
type LocalProxyOption func(*LocalProxyConfig)

func WithProxyPreset(preset string) LocalProxyOption      // Browser fingerprint
func WithProxyTimeout(d time.Duration) LocalProxyOption   // Request timeout
func WithProxyMaxConnections(n int) LocalProxyOption      // Max concurrent connections
func WithProxyUpstream(tcpProxy, udpProxy string) LocalProxyOption  // Upstream proxy

// Start a local proxy
func StartLocalProxy(port int, opts ...LocalProxyOption) (*LocalProxy, error)

// Example
proxy, err := httpcloak.StartLocalProxy(0,  // 0 = auto-select port
    httpcloak.WithProxyPreset("chrome-144"),
    httpcloak.WithProxyTimeout(30 * time.Second),
    httpcloak.WithProxyMaxConnections(1000),
)
if err != nil {
    log.Fatal(err)
}
defer proxy.Stop()

fmt.Printf("Proxy running on port %d\n", proxy.Port())
// Configure any HTTP client to use http://127.0.0.1:<port> as proxy
```

#### LocalProxy Methods

```go
// local_proxy.go:256-276

// Port returns the port the proxy is listening on
func (p *LocalProxy) Port() int

// IsRunning returns whether the proxy is running
func (p *LocalProxy) IsRunning() bool

// Stats returns proxy statistics
func (p *LocalProxy) Stats() map[string]interface{}
// Returns: running, port, active_conns, total_requests, preset, max_connections

// Stop stops the local proxy server gracefully
func (p *LocalProxy) Stop() error
```

#### Session Registry

Register multiple sessions for per-request routing via `X-HTTPCloak-Session` header.

```go
// RegisterSession registers a session for per-request selection.
// Returns error if session ID already exists.
func (p *LocalProxy) RegisterSession(sessionID string, session *Session) error

// UnregisterSession removes a session from the registry.
// Returns the session if found, nil otherwise. Does NOT close the session.
func (p *LocalProxy) UnregisterSession(sessionID string) *Session

// GetSession returns a registered session by ID.
func (p *LocalProxy) GetSession(sessionID string) *Session

// ListSessions returns all registered session IDs.
func (p *LocalProxy) ListSessions() []string

// Example usage:
session1 := httpcloak.NewSession("chrome-144", httpcloak.WithSessionProxy("socks5://proxy1:1080"))
session2 := httpcloak.NewSession("firefox-133", httpcloak.WithSessionProxy("socks5://proxy2:1080"))

proxy.RegisterSession("session-1", session1)
proxy.RegisterSession("session-2", session2)

// Client sends: X-HTTPCloak-Session: session-1 to use session1
```

#### Per-Request Headers

Headers for per-request control (automatically stripped before forwarding):

| Header | Description | Works with |
|--------|-------------|------------|
| `X-HTTPCloak-TlsOnly` | Override TLS-only mode ("true"/"false") | HTTP |
| `X-HTTPCloak-Session` | Select registered session by ID | HTTP |
| `X-Upstream-Proxy` | Override upstream proxy URL | HTTP |
| `Proxy-Authorization: HTTPCloak <url>` | Override upstream proxy (recommended) | HTTP & CONNECT |

```go
// Example: Per-request proxy rotation
// Client sends headers:
//   X-Upstream-Proxy: http://user:pass@rotating-proxy:8080
// or (works with HTTPS CONNECT):
//   Proxy-Authorization: HTTPCloak http://user:pass@rotating-proxy:8080

// Example: Per-request TLS-only mode
// Client sends headers:
//   X-HTTPCloak-TlsOnly: true
// Overrides proxy's global TLSOnly setting for this request only
```

#### Architecture

- **HTTP requests**: Forwarded through fast pooled `http.Transport` with connection reuse
- **HTTPS (CONNECT)**: TCP tunneling - client does TLS, fingerprinting via upstream proxy only
- **Streaming**: True streaming with 64KB buffers - no memory buffering
- **Performance**: ~3GB/s throughput on localhost

---

## Python Bindings

### Installation

```bash
pip install httpcloak
```

### Quick Start

```python
from httpcloak import Session

session = Session(preset="chrome-144")
response = session.get("https://example.com")

print(response.status_code)  # 200
print(response.protocol)     # "h2"
print(response.text)         # HTML content
```

### Session API

```python
# Create session
session = Session(
    preset="chrome-144",      # Browser fingerprint
    proxy=None,               # Proxy URL
    timeout=30,               # Timeout in seconds
    tls_only=False,           # TLS-only mode (skip preset HTTP headers)
    quic_idle_timeout=30,     # QUIC idle timeout in seconds (for HTTP/3)
    local_address=None,       # Local IP for IPv6 rotation (e.g., "2001:db8::1")
    key_log_file=None,        # Path to write TLS keys for Wireshark
)

# HTTP methods
response = session.get(url, headers=None)
response = session.post(url, body=None, headers=None)
response = session.request(method, url, headers=None, body=None)

# Fast methods (zero-copy, lower memory)
response = session.get_fast(url, headers=None)
response = session.post_fast(url, body=None, headers=None)
response = session.request_fast(method, url, headers=None, body=None)

# Streaming methods
stream = session.get_stream(url, headers=None)
stream = session.post_stream(url, body=None, headers=None)

# Response object
response.status_code    # int
response.headers        # dict
response.body           # bytes
response.text           # str
response.final_url      # str
response.protocol       # str ("h1", "h2", "h3")
response.cookies        # dict (from FastResponse)
response.history        # list (redirect history, from FastResponse)

# Cookie management
session.get_cookies()           # dict
session.set_cookie(name, value)
session.set_cookies(cookies)    # dict
session.clear_cookies()

# Header order customization
session.get_header_order()      # list[str] - Get current header order
session.set_header_order(order) # Set custom order (empty list to reset)

# Lifecycle
session.close()
```

### Distributed Session Cache (Python)

```python
from httpcloak import SessionCacheBackend, configure_session_cache, clear_session_cache

# Option 1: Using configure_session_cache (convenience function)
import redis
r = redis.Redis()

configure_session_cache(
    get=lambda key: r.get(key).decode() if r.get(key) else None,
    put=lambda key, value, ttl: (r.setex(key, ttl, value), 0)[1],
    delete=lambda key: (r.delete(key), 0)[1],
    on_error=lambda op, key, err: print(f"Cache error: {op} on {key}: {err}"),
)

# Now all sessions will use Redis for TLS session storage
session = Session(preset="chrome-144")
session.get("https://example.com")  # Session cached!

# Clear cache backend
clear_session_cache()

# Option 2: Using SessionCacheBackend class
class RedisCache:
    def __init__(self, client):
        self.redis = client

    def get(self, key: str) -> Optional[str]:
        data = self.redis.get(key)
        return data.decode() if data else None

    def put(self, key: str, value: str, ttl_seconds: int) -> int:
        self.redis.setex(key, ttl_seconds, value)
        return 0  # Success

    def delete(self, key: str) -> int:
        self.redis.delete(key)
        return 0

backend = SessionCacheBackend(
    get=cache.get,
    put=cache.put,
    delete=cache.delete,
    get_ech=cache.get,      # For HTTP/3 ECH config
    put_ech=cache.put,      # For HTTP/3 ECH config
    on_error=lambda op, key, err: print(f"Error: {op}")
)
backend.register()

# To unregister
backend.unregister()
```

### Async Support

```python
import asyncio
from httpcloak import AsyncSession

async def main():
    async with AsyncSession(preset="chrome-144") as session:
        response = await session.get("https://example.com")
        print(response.text)

asyncio.run(main())
```

### Available Presets

```python
from httpcloak import available_presets

print(available_presets())
# ['chrome-144', 'chrome-144-windows', ..., 'safari-18', 'ios-safari-18', ...]
```

---

## Node.js Bindings

### Installation

```bash
npm install httpcloak
```

### Quick Start

```javascript
const { Session } = require("httpcloak");

const session = new Session({ preset: "chrome-144" });

const response = await session.get("https://example.com");
console.log(response.statusCode);  // 200
console.log(response.protocol);    // "h2"
console.log(response.text);        // HTML content

session.close();
```

### Session API

```javascript
// Create session
const session = new Session({
    preset: "chrome-144",    // Browser fingerprint
    proxy: null,             // Proxy URL
    timeout: 30,             // Timeout in seconds
    tlsOnly: false,          // TLS-only mode (skip preset HTTP headers)
    quicIdleTimeout: 30,     // QUIC idle timeout in seconds (for HTTP/3)
    localAddress: null,      // Local IP for IPv6 rotation (e.g., "2001:db8::1")
    keyLogFile: null,        // Path to write TLS keys for Wireshark
});

// HTTP methods (Promise-based)
const response = await session.get(url, headers);
const response = await session.post(url, body, headers);
const response = await session.request({ method, url, headers, body });

// Fast methods (zero-copy, lower memory)
const response = await session.getFast(url, headers);
const response = await session.postFast(url, body, headers);
const response = await session.requestFast({ method, url, headers, body });

// Response object
response.statusCode     // number
response.headers        // object
response.body           // Buffer
response.text           // string
response.finalUrl       // string
response.protocol       // string ("h1", "h2", "h3")
response.cookies        // object (from FastResponse)
response.history        // array (redirect history, from FastResponse)

// Cookie management
session.getCookies()              // object
session.setCookie(name, value)
session.setCookies(cookies)       // object
session.clearCookies()

// Header order customization
session.getHeaderOrder()          // string[] - Get current header order
session.setHeaderOrder(order)     // Set custom order (empty array to reset)

// Lifecycle
session.close();
```

### Distributed Session Cache (Node.js)

Node.js supports both **sync** and **async** cache callbacks. Use async callbacks for non-blocking Redis operations.

```javascript
const { SessionCacheBackend, configureSessionCache, clearSessionCache } = require('httpcloak');
const Redis = require('ioredis');

const redis = new Redis();

// Option 1: Async callbacks (recommended for Node.js)
// Callbacks can return Promises - the library handles them internally
configureSessionCache({
    get: async (key) => await redis.get(key),
    put: async (key, value, ttlSeconds) => {
        await redis.setex(key, ttlSeconds, value);
        return 0;  // Success
    },
    delete: async (key) => {
        await redis.del(key);
        return 0;
    },
    onError: (operation, key, error) => {
        console.error(`Cache error: ${operation} on ${key}: ${error}`);
    }
});

// Now all sessions will use Redis for TLS session storage
const session = new Session({ preset: 'chrome-144' });
await session.get('https://example.com');  // Session cached!

// Clear cache backend
clearSessionCache();

// Option 2: Using SessionCacheBackend class
const cache = new SessionCacheBackend({
    get: (key) => redis.get(key),
    put: (key, value, ttlSeconds) => {
        redis.setex(key, ttlSeconds, value);
        return 0;
    },
    delete: (key) => {
        redis.del(key);
        return 0;
    },
    getEch: (key) => redis.get(key),      // For HTTP/3 ECH config
    putEch: (key, value, ttlSeconds) => {
        redis.setex(key, ttlSeconds, value);
        return 0;
    },
    onError: (op, key, err) => console.error(`Error: ${op}`)
});

cache.register();

// To unregister
cache.unregister();
```

---

## C Library Exports

The shared library exports C functions for FFI bindings.

### Build

```bash
cd bindings/clib
./build.sh
# Output: dist/libhttpcloak.so (Linux), dist/libhttpcloak.dylib (macOS)
```

### Exports

```c
// bindings/clib/httpcloak.go

// Session management
char* session_create(char* config_json);
void session_close(char* session_id);

// HTTP methods
char* session_get(char* session_id, char* url, char* headers_json);
char* session_post(char* session_id, char* url, char* body, char* headers_json);
char* session_request(char* session_id, char* request_json);

// Cookie management
char* session_get_cookies(char* session_id);
void session_set_cookie(char* session_id, char* name, char* value);
void session_clear_cookies(char* session_id);

// Header order customization
char* httpcloak_session_get_header_order(int64_t handle);    // Returns JSON array
char* httpcloak_session_set_header_order(int64_t handle, char* order_json);

// Local Proxy
int64_t httpcloak_local_proxy_start(char* config_json);
// config_json: {"port": 0, "preset": "chrome-144", "timeout": 30, "maxConnections": 1000}
// Returns: handle (>0 success, <=0 error)

void httpcloak_local_proxy_stop(int64_t handle);
int httpcloak_local_proxy_get_port(int64_t handle);
int httpcloak_local_proxy_is_running(int64_t handle);  // Returns 1 if running, 0 otherwise
char* httpcloak_local_proxy_get_stats(int64_t handle); // Returns JSON stats

// Distributed Session Cache Callbacks
// Register callbacks for distributed session storage
void httpcloak_set_session_cache_callbacks(
    session_cache_get_callback get,      // char* (*)(const char* key)
    session_cache_put_callback put,      // int (*)(const char* key, const char* value_json, int64_t ttl_seconds)
    session_cache_delete_callback del,   // int (*)(const char* key)
    ech_cache_get_callback ech_get,      // char* (*)(const char* key)
    ech_cache_put_callback ech_put,      // int (*)(const char* key, const char* value_base64, int64_t ttl_seconds)
    session_cache_error_callback on_error // void (*)(const char* operation, const char* key, const char* error)
);

// Clear all session cache callbacks
void httpcloak_clear_session_cache_callbacks();

// Memory management
void free_string(char* str);

// Utility
char* available_presets();
```

---

## .NET Bindings

.NET bindings use P/Invoke to call the C library.

> **Note:** C# bindings have feature parity for core functionality (Session, LocalProxy, HttpCloakHandler).
> The following advanced features are available in Python/Node.js but not yet in C#:
> - Zero-copy response optimization (getFast/postFast)
> - Streaming uploads for large files
> - Distributed session cache (SessionCacheBackend)

### Example

```csharp
using HttpCloak;

var session = new Session(preset: "chrome-144");
var response = session.Get("https://example.com");

Console.WriteLine(response.StatusCode);
Console.WriteLine(response.Protocol);
Console.WriteLine(response.Text);

session.Dispose();
```

### Session API

```csharp
// Constructor with all options
var session = new Session(
    preset: "chrome-144",      // Browser fingerprint
    proxy: null,               // Proxy URL
    timeout: 30,               // Timeout in seconds
    tlsOnly: false,            // TLS-only mode (skip preset HTTP headers)
    quicIdleTimeout: 30,       // QUIC idle timeout in seconds (for HTTP/3)
    localAddress: null,        // Local IP for IPv6 rotation (e.g., "2001:db8::1")
    keyLogFile: null           // Path to write TLS keys for Wireshark
);

// HTTP methods
Response Get(string url, Dictionary<string, string[]> headers = null)
Response Post(string url, byte[] body = null, Dictionary<string, string[]> headers = null)

// Fast methods (zero-copy, lower memory)
FastResponse GetFast(string url, Dictionary<string, string[]> headers = null)
FastResponse PostFast(string url, byte[] body = null, Dictionary<string, string[]> headers = null)
FastResponse RequestFast(string method, string url, byte[] body = null, Dictionary<string, string[]> headers = null)

// Cookie management
Dictionary<string, string> GetCookies()
void SetCookie(string name, string value)
void SetCookies(Dictionary<string, string> cookies)
void ClearCookies()

// Header order customization
string[] GetHeaderOrder()          // Get current header order
void SetHeaderOrder(string[] order) // Set custom order (null to reset)

// Persistence
void Save(string path)
static Session Load(string path)

// Lifecycle
void Dispose()
```

### LocalProxy

Local HTTP proxy for transparent HttpClient integration. Enables using httpcloak's TLS fingerprinting with standard .NET HttpClient without FFI limitations.

```csharp
using HttpCloak;

// Create and start a local proxy
using var proxy = new LocalProxy(
    port: 0,              // 0 = auto-select available port
    preset: "chrome-144", // Browser fingerprint
    timeout: 30,          // Request timeout in seconds
    maxConnections: 1000  // Max concurrent connections
    // tcpProxy: "socks5://user:pass@proxy.example.com:1080"  // Optional upstream proxy
);

// Get proxy information
Console.WriteLine($"Proxy URL: {proxy.ProxyUrl}");   // http://127.0.0.1:<port>
Console.WriteLine($"Port: {proxy.Port}");
Console.WriteLine($"Running: {proxy.IsRunning}");

// Create HttpClient that uses the proxy
var handler = proxy.CreateHandler();  // Returns configured HttpClientHandler
using var client = new HttpClient(handler);

// Make requests - they go through httpcloak
var response = await client.GetAsync("https://example.com");

// Or use WebProxy directly
var webProxy = proxy.CreateWebProxy();  // Returns System.Net.WebProxy

// Get statistics
LocalProxyStats stats = proxy.GetStats();
Console.WriteLine($"Total requests: {stats.TotalRequests}");
Console.WriteLine($"Active connections: {stats.ActiveConnections}");
```

#### LocalProxy API

```csharp
// Constructor
public LocalProxy(
    int port = 0,              // Port (0 = auto-select)
    string preset = "chrome-144",
    int timeout = 30,          // Seconds
    int maxConnections = 1000,
    string? tcpProxy = null,   // Upstream TCP proxy
    string? udpProxy = null,   // Upstream UDP proxy
    bool tlsOnly = false       // TLS-only mode: skip preset HTTP headers, only apply TLS fingerprint
)

// Properties
int Port { get; }              // Listening port
bool IsRunning { get; }        // Whether proxy is running
string ProxyUrl { get; }       // Full proxy URL (http://127.0.0.1:<port>)

// Methods
WebProxy CreateWebProxy()              // Create System.Net.WebProxy
HttpClientHandler CreateHandler()      // Create configured handler
LocalProxyStats GetStats()             // Get statistics
void Dispose()                         // Stop and cleanup

// Session Registry (for per-request routing)
void RegisterSession(string sessionId, Session session)  // Register session with ID
bool UnregisterSession(string sessionId)                 // Unregister session, returns true if found

// Example: Session Registry usage
var session1 = new Session(preset: "chrome-144");
var session2 = new Session(preset: "firefox-134");

proxy.RegisterSession("user-1", session1);
proxy.RegisterSession("user-2", session2);

// Clients use X-HTTPCloak-Session header to select:
// X-HTTPCloak-Session: user-1 -> uses session1
// X-HTTPCloak-Session: user-2 -> uses session2

// Stats class
public class LocalProxyStats
{
    public bool Running { get; }
    public int Port { get; }
    public string Preset { get; }
    public long ActiveConnections { get; }
    public long TotalRequests { get; }
    public int MaxConnections { get; }
}
```

### HttpCloakHandler

`HttpCloakHandler` provides seamless integration with `HttpClient` using `LocalProxy` internally. It offers TRUE streaming with no memory buffering.

```csharp
using HttpCloak;

// Basic usage
using var handler = new HttpCloakHandler(preset: "chrome-144");
using var client = new HttpClient(handler);

var response = await client.GetAsync("https://example.com");

// With configuration
using var handler = new HttpCloakHandler(
    preset: "chrome-144",           // Browser fingerprint
    proxy: "socks5://proxy:1080",   // Upstream proxy
    tcpProxy: null,                 // TCP-only proxy (H1/H2)
    udpProxy: null,                 // UDP-only proxy (H3)
    timeout: 60,                    // Timeout in seconds
    maxConnections: 500             // Max concurrent connections
);

// Access underlying LocalProxy
handler.Proxy.GetStats();

// Create from existing LocalProxy (shared)
using var proxy = new LocalProxy(preset: "chrome-144");
using var handler1 = new HttpCloakHandler(proxy);  // Doesn't own proxy
using var handler2 = new HttpCloakHandler(proxy);  // Both share same proxy

// Properties
string ProxyUrl { get; }              // Proxy URL
LocalProxy Proxy { get; }             // Underlying LocalProxy
LocalProxyStats GetStats()            // Get proxy statistics
```

---

## Error Handling

### Transport Errors

```go
// transport/errors.go
type TransportError struct {
    Op       string   // Operation that failed
    Host     string   // Target host
    Port     string   // Target port
    Protocol string   // h1, h2, h3
    Cause    error    // Underlying error
    Category error    // Error category (ErrNetwork, ErrTLS, etc.)
}

// Error categories
var (
    ErrNetwork  = errors.New("network error")
    ErrTLS      = errors.New("TLS error")
    ErrProxy    = errors.New("proxy error")
    ErrProtocol = errors.New("protocol error")
    ErrTimeout  = errors.New("timeout error")
)
```

### Session Errors

```go
// session/session.go:27
var ErrSessionClosed = errors.New("session is closed")
```

### Pool Errors

```go
// pool/pool.go:21-24
var (
    ErrPoolClosed    = errors.New("connection pool is closed")
    ErrNoConnections = errors.New("no available connections")
)
```

---

## File References

| Component | File |
|-----------|------|
| Main API | `httpcloak.go` |
| Session | `session/session.go` |
| Transport | `transport/transport.go` |
| Manager | `session/manager.go` |
| Presets | `fingerprint/presets.go` |
| Client Package | `client/client.go` |
| Client Options | `client/options.go` |
| Hooks | `client/hooks.go` |
| Cert Pinning | `client/certpin.go` |
| DNS/ECH | `dns/cache.go` |
| Local Proxy | `local_proxy.go` |
| Errors | `transport/errors.go` |
| C Library | `bindings/clib/httpcloak.go` |
| Python | `bindings/python/httpcloak/client.py` |
| Node.js | `bindings/nodejs/lib/index.js` |
| .NET | `bindings/dotnet/HttpCloak/Session.cs` |
| .NET LocalProxy | `bindings/dotnet/HttpCloak/LocalProxy.cs` |
