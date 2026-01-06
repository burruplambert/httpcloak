/**
 * Basic HTTP Requests with httpcloak
 *
 * This example demonstrates:
 * - Simple GET and POST requests
 * - Using module-level functions
 * - Accessing response data
 *
 * Run: node 01_basic_requests.js
 */

const httpcloak = require("httpcloak");

async function main() {
  // Simple GET request
  console.log("=".repeat(60));
  console.log("Example 1: Simple GET Request");
  console.log("-".repeat(60));

  let r = await httpcloak.get("https://httpbin.org/get");
  console.log(`Status: ${r.statusCode}`);
  console.log(`Protocol: ${r.protocol}`);
  console.log(`OK: ${r.ok}`);

  // GET with query parameters
  console.log("\n" + "=".repeat(60));
  console.log("Example 2: GET with Query Parameters");
  console.log("-".repeat(60));

  r = await httpcloak.get("https://httpbin.org/get", {
    params: { search: "httpcloak", page: 1 },
  });
  console.log(`Status: ${r.statusCode}`);
  console.log(`Final URL: ${r.url}`);

  // POST with JSON body
  console.log("\n" + "=".repeat(60));
  console.log("Example 3: POST with JSON Body");
  console.log("-".repeat(60));

  r = await httpcloak.post("https://httpbin.org/post", {
    json: { name: "httpcloak", version: "1.0" },
  });
  console.log(`Status: ${r.statusCode}`);
  const data = r.json();
  console.log(`Echoed JSON:`, data.json);

  // POST with form data
  console.log("\n" + "=".repeat(60));
  console.log("Example 4: POST with Form Data");
  console.log("-".repeat(60));

  r = await httpcloak.post("https://httpbin.org/post", {
    data: { username: "user", password: "pass" },
  });
  console.log(`Status: ${r.statusCode}`);
  const formData = r.json();
  console.log(`Echoed Form:`, formData.form);

  // Custom headers
  console.log("\n" + "=".repeat(60));
  console.log("Example 5: Custom Headers");
  console.log("-".repeat(60));

  r = await httpcloak.get("https://httpbin.org/headers", {
    headers: {
      "X-Custom-Header": "my-value",
      "X-Request-ID": "12345",
    },
  });
  console.log(`Status: ${r.statusCode}`);
  const headers = r.json().headers;
  console.log(`Custom header received: ${headers["X-Custom-Header"]}`);

  // Response helpers
  console.log("\n" + "=".repeat(60));
  console.log("Example 6: Response Helpers");
  console.log("-".repeat(60));

  r = await httpcloak.get("https://httpbin.org/json");
  console.log(`Status Code: ${r.statusCode}`);
  console.log(`Is OK (status < 400): ${r.ok}`);
  console.log(`Content type: ${r.headers["content-type"]}`);
  console.log(`Body length: ${r.content.length} bytes`);

  // Parse JSON
  const jsonData = r.json();
  console.log(`JSON parsed: ${typeof jsonData}`);

  console.log("\n" + "=".repeat(60));
  console.log("All basic examples completed!");
  console.log("=".repeat(60));
}

main().catch(console.error);
