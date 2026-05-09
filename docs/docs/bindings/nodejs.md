---
title: Node.js
sidebar_position: 3
---

# Node.js

The Node.js binding wraps the cgo shared library through [koffi](https://koffi.dev/), a fast FFI lib that doesn't need a node-gyp build step. Both ESM (`import`) and CommonJS (`require`) work. TypeScript types ship in the package, no separate `@types` install.

## Install

```bash
npm install httpcloak
```

The main package pulls in koffi and a per-platform native binary as an `optionalDependencies` entry. So a Linux x64 user transparently gets `@httpcloak/linux-x64`, a macOS arm64 user gets `@httpcloak/darwin-arm64`, and so on. If npm picks the wrong one (rare on weird platforms or in Docker buildx setups), force the right one:

```bash
npm install httpcloak @httpcloak/linux-x64
```

Supported platforms: `linux-x64`, `linux-arm64`, `darwin-x64`, `darwin-arm64`, `win32-x64`. Node 14+.

## Quick start

ESM:

```js
import { Session } from "httpcloak";

const s = new Session({ preset: "chrome-146" });
try {
  const r = await s.get("https://tls.peet.ws/api/all");
  console.log(r.statusCode, r.json().user_agent);
} finally {
  s.close();
}
```

CommonJS:

```js
const { Session } = require("httpcloak");

(async () => {
  const s = new Session({ preset: "chrome-146" });
  try {
    const r = await s.get("https://tls.peet.ws/api/all");
    console.log(r.statusCode);
  } finally {
    s.close();
  }
})();
```

There's no `using` declaration in standard JS yet, so `try / finally` is how you guarantee `close()`. If your TypeScript target is `ES2024+` you can do `using s = new Session(...)` once we ship `Symbol.asyncDispose` support.

## koffi vs napi

Worth flagging upfront: this binding's FFI-based, not a node-native module. Two practical knock-ons:

- **No prebuild compile step.** `npm install` just downloads JS plus the per-platform `.so` / `.dylib` / `.dll`. Faster install, no compiler dependency.
- **Different runtime constraints than typical native modules.** Worker threads can use the binding fine. Buffer ownership rules are spelled out per-method (FastResponse calls out which buffers are pool-managed in its docstring).

If you've worked with koffi before, none of this is news. If you haven't, just know that calling into the lib has a small (microseconds) FFI overhead and the binding handles type marshalling for you.

## `Session`

Constructor takes an options object:

```ts
new Session(options?: SessionOptions);
```

`SessionOptions` (all optional):

```ts
{
  preset?: string;             // "chrome-146" by default
  proxy?: string;
  tcpProxy?: string;
  udpProxy?: string;
  timeout?: number;            // seconds, default 30
  httpVersion?: string;        // "auto" | "h1" | "h2" | "h3"
  verify?: boolean;            // SSL verify, default true
  allowRedirects?: boolean;
  maxRedirects?: number;
  retry?: number;
  retryOnStatus?: number[];
  retryWaitMin?: number;       // ms
  retryWaitMax?: number;       // ms
  preferIpv4?: boolean;
  auth?: [string, string];
  connectTo?: Record<string, string>;
  echConfigDomain?: string;
  tlsOnly?: boolean;
  quicIdleTimeout?: number;    // seconds
  localAddress?: string;
  keyLogFile?: string;
  enableSpeculativeTls?: boolean;
  switchProtocol?: string;
  withoutCookieJar?: boolean;
  ja3?: string;
  akamai?: string;
  extraFp?: Record<string, any>;
}
```

Full per-flag description: [Options reference](/reference/options).

### Async request methods

All return `Promise<Response>`:

```ts
session.get(url, options?): Promise<Response>;
session.post(url, options?): Promise<Response>;
session.put(url, options?): Promise<Response>;
session.delete(url, options?): Promise<Response>;
session.patch(url, options?): Promise<Response>;
session.head(url, options?): Promise<Response>;
session.options(url, options?): Promise<Response>;
session.request(method, url, options?): Promise<Response>;
```

### Sync variants

```ts
session.getSync(url, options?): Response;
session.postSync(url, options?): Response;
session.requestSync(method, url, options?): Response;
```

These block the event loop. Don't use them inside an async server. They're here for scripts and CLIs where you genuinely don't have a loop to worry about.

### `RequestOptions`

```ts
{
  headers?: Record<string, string>;
  body?: string | Buffer | Record<string, any>;
  json?: Record<string, any>;       // serialized + Content-Type added
  data?: Record<string, any>;       // form-urlencoded
  files?: Record<string, Buffer | { filename, content, contentType? }>;
  params?: Record<string, string | number | boolean>;
  cookies?: Record<string, string>;
  auth?: [string, string];
  timeout?: number;                  // seconds
  fetchMode?: "cors" | "no-cors" | "navigate" | "websocket";
}
```

`fetchMode` overrides the auto-detected `Sec-Fetch-Mode` / `Sec-Fetch-Dest` / `Sec-Fetch-Site` triplet. Useful when the auto-sniff gets it wrong (POSTs to CORS endpoints without a JSON Accept header are the usual offender).

### Streaming

```ts
session.getStream(url, options?): StreamResponse;
session.postStream(url, options?): StreamResponse;
session.requestStream(method, url, options?): StreamResponse;
```

`StreamResponse` is async-iterable:

```js
const stream = session.getStream("https://example.com/big.zip");
const out = fs.createWriteStream("big.zip");
for await (const chunk of stream) {
  out.write(chunk);
}
stream.close();
```

It also exposes `readChunk(size)`, `readAll()`, and `iterate(chunkSize)` if you want explicit control.

### Fast path

```ts
session.getFast(url, options?): FastResponse;
session.postFast(url, options?): FastResponse;
session.requestFast(method, url, options?): FastResponse;
session.putFast(url, options?): FastResponse;
session.deleteFast(url, options?): FastResponse;
session.patchFast(url, options?): FastResponse;
```

`FastResponse` is sync-only and uses pool-managed buffers. Call `release()` when you're done so the buffer goes back. Don't hold the buffer across many requests without copying it first.

### Lifecycle

```ts
session.close(): void;
session.refresh(): void;
session.warmup(url, options?): void;
session.fork(n?: number): Session[];
```

`refresh` keeps cookies and TLS tickets, drops connections. `warmup` simulates a real page load. `fork` clones cookies and TLS state into N sibling sessions, each with its own connection pool.

### Persistence

```ts
session.save(path: string): void;
session.marshal(): string;
Session.load(path: string): Session;       // static
Session.unmarshal(data: string): Session;  // static
```

`marshal()` returns JSON you can stuff into a database. `load` and `unmarshal` rebuild the session.

### Cookies

```ts
session.cookies: Record<string, string>;        // deprecated flat shape
session.getCookies(): Record<string, string>;    // deprecated
session.getCookiesDetailed(): Cookie[];
session.getCookie(name): string | null;          // deprecated
session.getCookieDetailed(name): Cookie | null;
session.setCookie(name, value, options?): void;
session.deleteCookie(name, domain?): void;
session.clearCookies(): void;
```

The flat `Record<string, string>` shape will get replaced with `Cookie[]` in a future major. Use `getCookiesDetailed` / `getCookieDetailed` if you want the new shape today.

### Proxies

```ts
session.setProxy(url): void;
session.setTcpProxy(url): void;
session.setUdpProxy(url): void;
session.getProxy(): string;
session.getTcpProxy(): string;
session.getUdpProxy(): string;
session.proxy: string;     // also exposed as a getter/setter property
```

Empty string disables the proxy.

### Header order

```ts
session.setHeaderOrder(order: string[]): void;
session.getHeaderOrder(): string[];
```

Lowercase names. Empty array resets to preset default.

### Misc

```ts
session.headers: Record<string, string>;     // default headers, mutate freely
session.auth: [string, string] | null;       // default auth
session.setSessionIdentifier(sessionId: string): void;
```

## Conventions

- camelCase everywhere. `getCookies`, `setProxy`, `clearCookies`, never `get_cookies`.
- Promises by default. Sync siblings have a `Sync` suffix. Streams and the fast path are sync-only since they handle their own lifecycle.
- TypeScript types ship in the package. `import type { SessionOptions, Response } from "httpcloak"` works without `@types`.
- Errors throw `HTTPCloakError`. `r.raiseForStatus()` throws on `>= 400`.

## `Response`

```ts
{
  statusCode: number;
  headers: Record<string, string>;
  body: Buffer;
  content: Buffer;          // alias of body
  text: string;
  finalUrl: string;
  url: string;              // alias of finalUrl
  protocol: string;         // "http/1.1", "h2", "h3"
  elapsed: number;          // ms
  cookies: Cookie[];
  history: RedirectInfo[];
  ok: boolean;
  reason: string;
  encoding: string | null;
  json<T>(): T;
  raiseForStatus(): void;
}
```

## Concurrency

Fire many concurrent `session.get()` calls from the same `Session` and you're fine. The cgo transport handles parallel dials and the Node side uses koffi's thread-pool offload so the event loop stays responsive.

What that means:

- One session, many `await Promise.all([...])` calls. Fine.
- Worker threads each holding their own `Session`. Also fine.
- Sharing one `Session` across worker threads. Possible but you have to pass it via the koffi pointer dance. Most folks just create one per worker.

For browser-tab-style parallelism with shared cookies, use `session.fork(n)`.

## Custom fingerprints

```js
const s = new Session({
  preset: "chrome-146",
  ja3: "771,4865-4866-4867-49195-49199,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513-21,29-23-24,0",
  akamai: "1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p",
});
```

Setting `ja3` auto-enables TLS-only mode. See [Custom JA3](/fingerprinting/custom-ja3).

## Other exports

```ts
import {
  LocalProxy,
  PresetPool,
  SessionCacheBackend,
  HTTPCloakError,
  configureSessionCache,
  clearSessionCache,
} from "httpcloak";
```

`LocalProxy` runs an HTTP proxy server that applies the fingerprint to whatever HTTP client you point at it. Undici, fetch, curl, anything.

## See also

- [Options reference](/reference/options).
- [Cookies and state](/cookies-and-state).
- [Proxies](/proxies).
