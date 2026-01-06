/**
 * Configuration and Browser Presets
 *
 * This example demonstrates:
 * - Using configure() for global defaults
 * - Different browser presets
 * - Forcing HTTP versions
 *
 * Run: node 02_configure_and_presets.js
 */

const httpcloak = require("httpcloak");

async function main() {
  // Configure global defaults
  console.log("=".repeat(60));
  console.log("Example 1: Configure Global Defaults");
  console.log("-".repeat(60));

  httpcloak.configure({
    preset: "chrome-143-linux",
    headers: { "Accept-Language": "en-US,en;q=0.9" },
    timeout: 30,
  });

  let r = await httpcloak.get("https://www.cloudflare.com/cdn-cgi/trace");
  console.log(`Protocol: ${r.protocol}`);
  console.log("First few lines of trace:");
  r.text
    .split("\n")
    .slice(0, 5)
    .forEach((line) => console.log(`  ${line}`));

  // Different browser presets
  console.log("\n" + "=".repeat(60));
  console.log("Example 2: Different Browser Presets");
  console.log("-".repeat(60));

  const presets = [
    "chrome-143",
    "chrome-143-windows",
    "chrome-143-linux",
    "chrome-131",
    "firefox-133",
    "safari-18",
  ];

  for (const preset of presets) {
    const session = new httpcloak.Session({ preset });
    const r = await session.get("https://www.cloudflare.com/cdn-cgi/trace");

    // Parse trace to get HTTP version
    const trace = {};
    r.text.split("\n").forEach((line) => {
      const [key, value] = line.split("=");
      if (key && value) trace[key] = value;
    });

    console.log(
      `${preset.padEnd(25)} | Protocol: ${r.protocol.padEnd(5)} | http=${trace.http || "N/A"}`
    );
    session.close();
  }

  // Force HTTP versions
  console.log("\n" + "=".repeat(60));
  console.log("Example 3: Force HTTP Versions");
  console.log("-".repeat(60));

  const httpVersions = ["auto", "h1", "h2", "h3"];

  for (const version of httpVersions) {
    const session = new httpcloak.Session({
      preset: "chrome-143",
      httpVersion: version,
    });

    try {
      const r = await session.get("https://www.cloudflare.com/cdn-cgi/trace");
      const trace = {};
      r.text.split("\n").forEach((line) => {
        const [key, value] = line.split("=");
        if (key && value) trace[key] = value;
      });

      console.log(
        `httpVersion=${version.padEnd(5)} | Actual Protocol: ${r.protocol.padEnd(5)} | http=${trace.http || "N/A"}`
      );
    } catch (e) {
      console.log(`httpVersion=${version.padEnd(5)} | Error: ${e.message}`);
    } finally {
      session.close();
    }
  }

  // List available presets
  console.log("\n" + "=".repeat(60));
  console.log("Example 4: List Available Presets");
  console.log("-".repeat(60));

  const availablePresets = httpcloak.availablePresets();
  console.log("Available presets:");
  availablePresets.forEach((preset) => console.log(`  - ${preset}`));

  console.log(`\nhttpcloak version: ${httpcloak.version()}`);

  console.log("\n" + "=".repeat(60));
  console.log("Configuration examples completed!");
  console.log("=".repeat(60));
}

main().catch(console.error);
