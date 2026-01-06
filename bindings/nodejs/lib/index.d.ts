/**
 * HTTPCloak Node.js TypeScript Definitions
 */

export class HTTPCloakError extends Error {
  name: "HTTPCloakError";
}

export class Response {
  /** HTTP status code */
  statusCode: number;
  /** Response headers */
  headers: Record<string, string>;
  /** Raw response body as Buffer */
  body: Buffer;
  /** Response body as string */
  text: string;
  /** Final URL after redirects */
  finalUrl: string;
  /** Protocol used (http/1.1, h2, h3) */
  protocol: string;

  /** Parse response body as JSON */
  json<T = any>(): T;
}

export interface SessionOptions {
  /** Browser preset to use (default: "chrome-143") */
  preset?: string;
  /** Proxy URL (e.g., "http://user:pass@host:port") */
  proxy?: string;
  /** Request timeout in seconds (default: 30) */
  timeout?: number;
}

export interface RequestOptions {
  /** HTTP method */
  method: string;
  /** Request URL */
  url: string;
  /** Optional custom headers */
  headers?: Record<string, string>;
  /** Optional request body */
  body?: string | Buffer | Record<string, any>;
  /** Optional request timeout in seconds */
  timeout?: number;
}

export type RequestCallback = (
  error: HTTPCloakError | null,
  response: Response | null
) => void;

export class Session {
  constructor(options?: SessionOptions);

  /** Close the session and release resources */
  close(): void;

  // Synchronous methods
  /** Perform a synchronous GET request */
  getSync(url: string, headers?: Record<string, string>): Response;

  /** Perform a synchronous POST request */
  postSync(
    url: string,
    body?: string | Buffer | Record<string, any>,
    headers?: Record<string, string>
  ): Response;

  /** Perform a synchronous custom HTTP request */
  requestSync(options: RequestOptions): Response;

  // Promise-based methods
  /** Perform an async GET request */
  get(url: string, headers?: Record<string, string>): Promise<Response>;

  /** Perform an async POST request */
  post(
    url: string,
    body?: string | Buffer | Record<string, any>,
    headers?: Record<string, string>
  ): Promise<Response>;

  /** Perform an async custom HTTP request */
  request(options: RequestOptions): Promise<Response>;

  /** Perform an async PUT request */
  put(
    url: string,
    body?: string | Buffer | Record<string, any>,
    headers?: Record<string, string>
  ): Promise<Response>;

  /** Perform an async DELETE request */
  delete(url: string, headers?: Record<string, string>): Promise<Response>;

  /** Perform an async PATCH request */
  patch(
    url: string,
    body?: string | Buffer | Record<string, any>,
    headers?: Record<string, string>
  ): Promise<Response>;

  /** Perform an async HEAD request */
  head(url: string, headers?: Record<string, string>): Promise<Response>;

  /** Perform an async OPTIONS request */
  options(url: string, headers?: Record<string, string>): Promise<Response>;

  // Callback-based methods
  /** Perform a GET request with callback */
  getCb(url: string, callback: RequestCallback): void;
  getCb(
    url: string,
    headers: Record<string, string>,
    callback: RequestCallback
  ): void;

  /** Perform a POST request with callback */
  postCb(url: string, callback: RequestCallback): void;
  postCb(
    url: string,
    body: string | Buffer | Record<string, any>,
    callback: RequestCallback
  ): void;
  postCb(
    url: string,
    body: string | Buffer | Record<string, any>,
    headers: Record<string, string>,
    callback: RequestCallback
  ): void;

  /** Perform a custom request with callback */
  requestCb(options: RequestOptions, callback: RequestCallback): void;

  // Cookie management
  /** Get all cookies from the session */
  getCookies(): Record<string, string>;

  /** Set a cookie in the session */
  setCookie(name: string, value: string): void;

  /** Get cookies as a property */
  readonly cookies: Record<string, string>;
}

/** Get the httpcloak library version */
export function version(): string;

/** Get list of available browser presets */
export function availablePresets(): string[];
