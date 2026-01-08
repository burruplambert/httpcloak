/**
 * HTTPCloak Node.js Client - ESM Module
 *
 * A fetch/axios-compatible HTTP client with browser fingerprint emulation.
 * Provides TLS fingerprinting for HTTP requests.
 */

import { createRequire } from "module";
const require = createRequire(import.meta.url);

// Import the CommonJS module
const cjs = require("./index.js");

// Re-export all named exports
export const Session = cjs.Session;
export const Response = cjs.Response;
export const HTTPCloakError = cjs.HTTPCloakError;
export const Preset = cjs.Preset;
export const configure = cjs.configure;
export const get = cjs.get;
export const post = cjs.post;
export const put = cjs.put;
export const patch = cjs.patch;
export const head = cjs.head;
export const options = cjs.options;
export const request = cjs.request;
export const version = cjs.version;
export const availablePresets = cjs.availablePresets;

// 'delete' is a reserved word in ESM, so we export it specially
const del = cjs.delete;
export { del as delete };

// Default export (the entire module)
export default cjs;
