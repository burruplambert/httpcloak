/**
 * HTTPCloak Node.js Client
 *
 * Provides HTTP client with browser fingerprint emulation.
 */

const koffi = require("koffi");
const path = require("path");
const os = require("os");
const fs = require("fs");

/**
 * Custom error class for HTTPCloak errors
 */
class HTTPCloakError extends Error {
  constructor(message) {
    super(message);
    this.name = "HTTPCloakError";
  }
}

/**
 * Response object returned from HTTP requests
 */
class Response {
  constructor(data) {
    this.statusCode = data.status_code || 0;
    this.headers = data.headers || {};
    this.body = Buffer.from(data.body || "", "utf8");
    this.text = data.body || "";
    this.finalUrl = data.final_url || "";
    this.protocol = data.protocol || "";
  }

  /**
   * Parse response body as JSON
   */
  json() {
    return JSON.parse(this.text);
  }
}

/**
 * Get the platform package name for the current platform
 */
function getPlatformPackageName() {
  const platform = os.platform();
  const arch = os.arch();

  // Map to npm platform names
  let platName;
  if (platform === "darwin") {
    platName = "darwin";
  } else if (platform === "win32") {
    platName = "win32";
  } else {
    platName = "linux";
  }

  let archName;
  if (arch === "x64" || arch === "amd64") {
    archName = "x64";
  } else if (arch === "arm64" || arch === "aarch64") {
    archName = "arm64";
  } else {
    archName = arch;
  }

  return `@httpcloak/${platName}-${archName}`;
}

/**
 * Get the path to the native library
 */
function getLibPath() {
  const platform = os.platform();
  const arch = os.arch();

  // Check environment variable first
  const envPath = process.env.HTTPCLOAK_LIB_PATH;
  if (envPath && fs.existsSync(envPath)) {
    return envPath;
  }

  // Try to load from platform-specific optional dependency
  const packageName = getPlatformPackageName();
  try {
    const libPath = require(packageName);
    if (fs.existsSync(libPath)) {
      return libPath;
    }
  } catch (e) {
    // Optional dependency not installed, fall back to local search
  }

  // Normalize architecture for library name
  let archName;
  if (arch === "x64" || arch === "amd64") {
    archName = "amd64";
  } else if (arch === "arm64" || arch === "aarch64") {
    archName = "arm64";
  } else {
    archName = arch;
  }

  // Determine OS name and extension
  let osName, ext;
  if (platform === "darwin") {
    osName = "darwin";
    ext = ".dylib";
  } else if (platform === "win32") {
    osName = "windows";
    ext = ".dll";
  } else {
    osName = "linux";
    ext = ".so";
  }

  const libName = `libhttpcloak-${osName}-${archName}${ext}`;

  // Search paths (fallback for local development)
  const searchPaths = [
    path.join(__dirname, libName),
    path.join(__dirname, "..", libName),
    path.join(__dirname, "..", "lib", libName),
  ];

  for (const searchPath of searchPaths) {
    if (fs.existsSync(searchPath)) {
      return searchPath;
    }
  }

  throw new HTTPCloakError(
    `Could not find httpcloak library (${libName}). ` +
      `Try: npm install ${packageName}`
  );
}

// Load the native library
let lib = null;

function getLib() {
  if (lib === null) {
    const libPath = getLibPath();
    const nativeLib = koffi.load(libPath);

    lib = {
      httpcloak_session_new: nativeLib.func("httpcloak_session_new", "int64", ["str"]),
      httpcloak_session_free: nativeLib.func("httpcloak_session_free", "void", ["int64"]),
      httpcloak_get: nativeLib.func("httpcloak_get", "str", ["int64", "str", "str"]),
      httpcloak_post: nativeLib.func("httpcloak_post", "str", ["int64", "str", "str", "str"]),
      httpcloak_request: nativeLib.func("httpcloak_request", "str", ["int64", "str"]),
      httpcloak_get_cookies: nativeLib.func("httpcloak_get_cookies", "str", ["int64"]),
      httpcloak_set_cookie: nativeLib.func("httpcloak_set_cookie", "void", ["int64", "str", "str"]),
      httpcloak_free_string: nativeLib.func("httpcloak_free_string", "void", ["str"]),
      httpcloak_version: nativeLib.func("httpcloak_version", "str", []),
      httpcloak_available_presets: nativeLib.func("httpcloak_available_presets", "str", []),
    };
  }
  return lib;
}

/**
 * Parse response from the native library
 */
function parseResponse(result) {
  if (!result) {
    throw new HTTPCloakError("No response received");
  }

  const data = JSON.parse(result);

  if (data.error) {
    throw new HTTPCloakError(data.error);
  }

  return new Response(data);
}

/**
 * Get the httpcloak library version
 */
function version() {
  const nativeLib = getLib();
  return nativeLib.httpcloak_version() || "unknown";
}

/**
 * Get list of available browser presets
 */
function availablePresets() {
  const nativeLib = getLib();
  const result = nativeLib.httpcloak_available_presets();
  if (result) {
    return JSON.parse(result);
  }
  return [];
}

/**
 * HTTP Session with browser fingerprint emulation
 */
class Session {
  /**
   * Create a new session
   * @param {Object} options - Session options
   * @param {string} [options.preset="chrome-143"] - Browser preset to use
   * @param {string} [options.proxy] - Proxy URL (e.g., "http://user:pass@host:port")
   * @param {number} [options.timeout=30] - Request timeout in seconds
   */
  constructor(options = {}) {
    const { preset = "chrome-143", proxy = null, timeout = 30 } = options;

    this._lib = getLib();

    const config = {
      preset,
      timeout,
    };
    if (proxy) {
      config.proxy = proxy;
    }

    this._handle = this._lib.httpcloak_session_new(JSON.stringify(config));

    if (this._handle === 0n || this._handle === 0) {
      throw new HTTPCloakError("Failed to create session");
    }
  }

  /**
   * Close the session and release resources
   */
  close() {
    if (this._handle) {
      this._lib.httpcloak_session_free(this._handle);
      this._handle = 0n;
    }
  }

  // ===========================================================================
  // Synchronous Methods
  // ===========================================================================

  /**
   * Perform a synchronous GET request
   * @param {string} url - Request URL
   * @param {Object} [headers] - Optional custom headers
   * @returns {Response} Response object
   */
  getSync(url, headers = null) {
    const headersJson = headers ? JSON.stringify(headers) : null;
    const result = this._lib.httpcloak_get(this._handle, url, headersJson);
    return parseResponse(result);
  }

  /**
   * Perform a synchronous POST request
   * @param {string} url - Request URL
   * @param {string|Buffer|Object} [body] - Request body
   * @param {Object} [headers] - Optional custom headers
   * @returns {Response} Response object
   */
  postSync(url, body = null, headers = null) {
    if (typeof body === "object" && body !== null && !Buffer.isBuffer(body)) {
      body = JSON.stringify(body);
      headers = headers || {};
      if (!headers["Content-Type"]) {
        headers["Content-Type"] = "application/json";
      }
    }

    if (Buffer.isBuffer(body)) {
      body = body.toString("utf8");
    }

    const headersJson = headers ? JSON.stringify(headers) : null;
    const result = this._lib.httpcloak_post(this._handle, url, body, headersJson);
    return parseResponse(result);
  }

  /**
   * Perform a synchronous custom HTTP request
   * @param {Object} options - Request options
   * @param {string} options.method - HTTP method
   * @param {string} options.url - Request URL
   * @param {Object} [options.headers] - Optional custom headers
   * @param {string|Buffer|Object} [options.body] - Optional request body
   * @param {number} [options.timeout] - Optional request timeout
   * @returns {Response} Response object
   */
  requestSync(options) {
    let { method, url, headers = null, body = null, timeout = null } = options;

    if (typeof body === "object" && body !== null && !Buffer.isBuffer(body)) {
      body = JSON.stringify(body);
      headers = headers || {};
      if (!headers["Content-Type"]) {
        headers["Content-Type"] = "application/json";
      }
    }

    if (Buffer.isBuffer(body)) {
      body = body.toString("utf8");
    }

    const requestConfig = {
      method: method.toUpperCase(),
      url,
    };
    if (headers) requestConfig.headers = headers;
    if (body) requestConfig.body = body;
    if (timeout) requestConfig.timeout = timeout;

    const result = this._lib.httpcloak_request(
      this._handle,
      JSON.stringify(requestConfig)
    );
    return parseResponse(result);
  }

  // ===========================================================================
  // Promise-based Methods
  // ===========================================================================

  /**
   * Perform an async GET request
   * @param {string} url - Request URL
   * @param {Object} [headers] - Optional custom headers
   * @returns {Promise<Response>} Response object
   */
  get(url, headers = null) {
    return new Promise((resolve, reject) => {
      setImmediate(() => {
        try {
          resolve(this.getSync(url, headers));
        } catch (err) {
          reject(err);
        }
      });
    });
  }

  /**
   * Perform an async POST request
   * @param {string} url - Request URL
   * @param {string|Buffer|Object} [body] - Request body
   * @param {Object} [headers] - Optional custom headers
   * @returns {Promise<Response>} Response object
   */
  post(url, body = null, headers = null) {
    return new Promise((resolve, reject) => {
      setImmediate(() => {
        try {
          resolve(this.postSync(url, body, headers));
        } catch (err) {
          reject(err);
        }
      });
    });
  }

  /**
   * Perform an async custom HTTP request
   * @param {Object} options - Request options
   * @returns {Promise<Response>} Response object
   */
  request(options) {
    return new Promise((resolve, reject) => {
      setImmediate(() => {
        try {
          resolve(this.requestSync(options));
        } catch (err) {
          reject(err);
        }
      });
    });
  }

  /**
   * Perform an async PUT request
   * @param {string} url - Request URL
   * @param {string|Buffer|Object} [body] - Request body
   * @param {Object} [headers] - Optional custom headers
   * @returns {Promise<Response>} Response object
   */
  put(url, body = null, headers = null) {
    return this.request({ method: "PUT", url, body, headers });
  }

  /**
   * Perform an async DELETE request
   * @param {string} url - Request URL
   * @param {Object} [headers] - Optional custom headers
   * @returns {Promise<Response>} Response object
   */
  delete(url, headers = null) {
    return this.request({ method: "DELETE", url, headers });
  }

  /**
   * Perform an async PATCH request
   * @param {string} url - Request URL
   * @param {string|Buffer|Object} [body] - Request body
   * @param {Object} [headers] - Optional custom headers
   * @returns {Promise<Response>} Response object
   */
  patch(url, body = null, headers = null) {
    return this.request({ method: "PATCH", url, body, headers });
  }

  /**
   * Perform an async HEAD request
   * @param {string} url - Request URL
   * @param {Object} [headers] - Optional custom headers
   * @returns {Promise<Response>} Response object
   */
  head(url, headers = null) {
    return this.request({ method: "HEAD", url, headers });
  }

  /**
   * Perform an async OPTIONS request
   * @param {string} url - Request URL
   * @param {Object} [headers] - Optional custom headers
   * @returns {Promise<Response>} Response object
   */
  options(url, headers = null) {
    return this.request({ method: "OPTIONS", url, headers });
  }

  // ===========================================================================
  // Callback-based Methods
  // ===========================================================================

  /**
   * Perform a GET request with callback
   * @param {string} url - Request URL
   * @param {Object|Function} [headersOrCallback] - Headers or callback
   * @param {Function} [callback] - Callback function (err, response)
   */
  getCb(url, headersOrCallback, callback) {
    let headers = null;
    let cb = callback;

    if (typeof headersOrCallback === "function") {
      cb = headersOrCallback;
    } else {
      headers = headersOrCallback;
    }

    setImmediate(() => {
      try {
        const response = this.getSync(url, headers);
        cb(null, response);
      } catch (err) {
        cb(err, null);
      }
    });
  }

  /**
   * Perform a POST request with callback
   * @param {string} url - Request URL
   * @param {string|Buffer|Object} [body] - Request body
   * @param {Object|Function} [headersOrCallback] - Headers or callback
   * @param {Function} [callback] - Callback function (err, response)
   */
  postCb(url, body, headersOrCallback, callback) {
    let headers = null;
    let cb = callback;

    if (typeof headersOrCallback === "function") {
      cb = headersOrCallback;
    } else {
      headers = headersOrCallback;
      cb = callback;
    }

    if (typeof body === "function") {
      cb = body;
      body = null;
    }

    setImmediate(() => {
      try {
        const response = this.postSync(url, body, headers);
        cb(null, response);
      } catch (err) {
        cb(err, null);
      }
    });
  }

  /**
   * Perform a custom request with callback
   * @param {Object} options - Request options
   * @param {Function} callback - Callback function (err, response)
   */
  requestCb(options, callback) {
    setImmediate(() => {
      try {
        const response = this.requestSync(options);
        callback(null, response);
      } catch (err) {
        callback(err, null);
      }
    });
  }

  // ===========================================================================
  // Cookie Management
  // ===========================================================================

  /**
   * Get all cookies from the session
   * @returns {Object} Cookies as key-value pairs
   */
  getCookies() {
    const result = this._lib.httpcloak_get_cookies(this._handle);
    if (result) {
      return JSON.parse(result);
    }
    return {};
  }

  /**
   * Set a cookie in the session
   * @param {string} name - Cookie name
   * @param {string} value - Cookie value
   */
  setCookie(name, value) {
    this._lib.httpcloak_set_cookie(this._handle, name, value);
  }

  /**
   * Get cookies as a property
   */
  get cookies() {
    return this.getCookies();
  }
}

module.exports = {
  Session,
  Response,
  HTTPCloakError,
  version,
  availablePresets,
};
