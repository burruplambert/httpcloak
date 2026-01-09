# HTTPCloak Node.js

Browser fingerprint emulation HTTP client with HTTP/1.1, HTTP/2, and HTTP/3 support.

## Installation

```bash
npm install httpcloak
```

## Quick Start

### Promise-based Usage (Recommended)

```javascript
const { Session } = require("httpcloak");

async function main() {
  const session = new Session({ preset: "chrome-143" });

  try {
    // GET request
    const response = await session.get("https://www.cloudflare.com/cdn-cgi/trace");
    console.log(response.statusCode);
    console.log(response.text);

    // POST request with JSON body
    const postResponse = await session.post("https://api.example.com/data", {
      key: "value",
    });

    // Custom headers
    const customResponse = await session.get("https://example.com", {
      "X-Custom": "value",
    });

    // Concurrent requests
    const responses = await Promise.all([
      session.get("https://example.com/1"),
      session.get("https://example.com/2"),
      session.get("https://example.com/3"),
    ]);
  } finally {
    session.close();
  }
}

main();
```

### Synchronous Usage

```javascript
const { Session } = require("httpcloak");

const session = new Session({ preset: "chrome-143" });

// Sync GET
const response = session.getSync("https://example.com");
console.log(response.statusCode);
console.log(response.text);

// Sync POST
const postResponse = session.postSync("https://api.example.com/data", {
  key: "value",
});

session.close();
```

### Callback-based Usage

```javascript
const { Session } = require("httpcloak");

const session = new Session({ preset: "chrome-143" });

// GET with callback
session.getCb("https://example.com", (err, response) => {
  if (err) {
    console.error("Error:", err.message);
    return;
  }
  console.log(response.statusCode);
  console.log(response.text);
});

// POST with callback
session.postCb(
  "https://api.example.com/data",
  { key: "value" },
  (err, response) => {
    if (err) {
      console.error("Error:", err.message);
      return;
    }
    console.log(response.statusCode);
  }
);
```

## Proxy Support

HTTPCloak supports HTTP, SOCKS5, and HTTP/3 (MASQUE) proxies with full fingerprint preservation.

### HTTP Proxy

```javascript
const { Session } = require("httpcloak");

// Basic HTTP proxy
const session = new Session({
  preset: "chrome-143",
  proxy: "http://host:port",
});

// With authentication
const sessionAuth = new Session({
  preset: "chrome-143",
  proxy: "http://user:pass@host:port",
});

// HTTPS proxy
const sessionHttps = new Session({
  preset: "chrome-143",
  proxy: "https://user:pass@host:port",
});
```

### SOCKS5 Proxy

```javascript
const { Session } = require("httpcloak");

// SOCKS5 proxy (with DNS resolution on proxy)
const session = new Session({
  preset: "chrome-143",
  proxy: "socks5h://host:port",
});

// With authentication
const sessionAuth = new Session({
  preset: "chrome-143",
  proxy: "socks5h://user:pass@host:port",
});

const response = await session.get("https://www.cloudflare.com/cdn-cgi/trace");
console.log(response.protocol); // h3 (HTTP/3 through SOCKS5!)
```

### HTTP/3 MASQUE Proxy

MASQUE (RFC 9484) enables HTTP/3 connections through compatible proxies:

```javascript
const { Session } = require("httpcloak");

// MASQUE proxy (auto-detected for known providers like Bright Data)
const session = new Session({
  preset: "chrome-143",
  proxy: "https://user:pass@brd.superproxy.io:10001",
});

const response = await session.get("https://www.cloudflare.com/cdn-cgi/trace");
console.log(response.protocol); // h3
```

## Advanced Features

### Encrypted Client Hello (ECH)

ECH encrypts the SNI (Server Name Indication) to prevent traffic analysis. Works with all Cloudflare domains:

```javascript
const { Session } = require("httpcloak");

// Enable ECH for Cloudflare domains
const session = new Session({
  preset: "chrome-143",
  echConfigDomain: "cloudflare-ech.com",
});

const response = await session.get("https://www.cloudflare.com/cdn-cgi/trace");
console.log(response.text);
// Output includes: sni=encrypted, http=http/3
```

### Domain Fronting (Connect-To)

Connect to one server while requesting a different domain:

```javascript
const { Session } = require("httpcloak");

// Connect to claude.ai's IP but request www.cloudflare.com
const session = new Session({
  preset: "chrome-143",
  connectTo: { "www.cloudflare.com": "claude.ai" },
});

const response = await session.get("https://www.cloudflare.com/cdn-cgi/trace");
```

### Combined: SOCKS5 + ECH

Get HTTP/3 with encrypted SNI through a SOCKS5 proxy:

```javascript
const { Session } = require("httpcloak");

const session = new Session({
  preset: "chrome-143",
  proxy: "socks5h://user:pass@host:port",
  echConfigDomain: "cloudflare-ech.com",
});

const response = await session.get("https://www.cloudflare.com/cdn-cgi/trace");
// Response shows: http=http/3, sni=encrypted
```

## Cookie Management

```javascript
const { Session } = require("httpcloak");

const session = new Session();

// Get all cookies
const cookies = session.getCookies();
console.log(cookies);

// Set a cookie
session.setCookie("session_id", "abc123");

// Access cookies as property
console.log(session.cookies);

session.close();
```

## Available Presets

```javascript
const { availablePresets } = require("httpcloak");

console.log(availablePresets());
// ['chrome-143', 'chrome-143-windows', 'chrome-143-linux', 'chrome-143-macos',
//  'chrome-131', 'firefox-133', 'safari-18', ...]
```

## Response Object

```javascript
const response = await session.get("https://example.com");

response.statusCode; // number: HTTP status code
response.headers; // object: Response headers
response.body; // Buffer: Raw response body
response.text; // string: Response body as text
response.finalUrl; // string: Final URL after redirects
response.protocol; // string: Protocol used (http/1.1, h2, h3)
response.json(); // Parse response body as JSON
```

## Custom Requests

```javascript
const response = await session.request({
  method: "PUT",
  url: "https://api.example.com/resource",
  headers: { "X-Custom": "value" },
  body: { data: "value" },
  timeout: 60,
});
```

## Error Handling

```javascript
const { Session, HTTPCloakError } = require("httpcloak");

const session = new Session();

try {
  const response = await session.get("https://example.com");
} catch (err) {
  if (err instanceof HTTPCloakError) {
    console.error("HTTPCloak error:", err.message);
  } else {
    console.error("Unknown error:", err);
  }
}

session.close();
```

## TypeScript Support

HTTPCloak includes TypeScript definitions out of the box:

```typescript
import { Session, Response, HTTPCloakError } from "httpcloak";

const session = new Session({ preset: "chrome-143" });

async function fetchData(): Promise<Response> {
  return session.get("https://example.com");
}
```

## Platform Support

- Linux (x64, arm64)
- macOS (x64, arm64)
- Windows (x64, arm64)

## License

MIT
