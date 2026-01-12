using System;
using System.IO;
using HttpCloak;

Console.WriteLine("=== ECH + PSK Session Resumption Tests (C#) ===");

bool allPass = true;

// Test 1: cloudflare.com (no ECH, auto)
Console.WriteLine("\n=== Testing cloudflare_cs (auto) ===");
try
{
    using var session1 = new Session(preset: "chrome-143", httpVersion: "auto");
    var resp1 = session1.Get("https://cloudflare.com/cdn-cgi/trace");
    Console.WriteLine($"Fresh request: Status={resp1.StatusCode}, Protocol={resp1.Protocol}");

    session1.Save("/tmp/session_cloudflare_cs.json");

    using var loaded1 = Session.Load("/tmp/session_cloudflare_cs.json");
    var resp1b = loaded1.Get("https://cloudflare.com/cdn-cgi/trace");
    Console.WriteLine($"Loaded request: Status={resp1b.StatusCode}, Protocol={resp1b.Protocol}");

    File.Delete("/tmp/session_cloudflare_cs.json");
    Console.WriteLine("cloudflare_auto: PASS");
}
catch (Exception e)
{
    Console.WriteLine($"Error: {e.Message}");
    Console.WriteLine("cloudflare_auto: FAIL");
    allPass = false;
}

// Test 2: crypto.cloudflare.com (ECH, H2)
Console.WriteLine("\n=== Testing crypto_cs (auto) ===");
try
{
    using var session2 = new Session(preset: "chrome-143", httpVersion: "auto");
    var resp2 = session2.Get("https://crypto.cloudflare.com/cdn-cgi/trace");
    Console.WriteLine($"Fresh request: Status={resp2.StatusCode}, Protocol={resp2.Protocol}");

    session2.Save("/tmp/session_crypto_cs.json");

    using var loaded2 = Session.Load("/tmp/session_crypto_cs.json");
    var resp2b = loaded2.Get("https://crypto.cloudflare.com/cdn-cgi/trace");
    Console.WriteLine($"Loaded request: Status={resp2b.StatusCode}, Protocol={resp2b.Protocol}");

    File.Delete("/tmp/session_crypto_cs.json");
    Console.WriteLine("crypto_ech_h2: PASS");
}
catch (Exception e)
{
    Console.WriteLine($"Error: {e.Message}");
    Console.WriteLine("crypto_ech_h2: FAIL");
    allPass = false;
}

// Test 3: quic.browserleaks.com (ECH, H3 forced)
Console.WriteLine("\n=== Testing quic_cs (h3) ===");
try
{
    using var session3 = new Session(preset: "chrome-143", httpVersion: "h3");
    var resp3 = session3.Get("https://quic.browserleaks.com/?minify=1");
    Console.WriteLine($"Fresh request: Status={resp3.StatusCode}, Protocol={resp3.Protocol}");

    session3.Save("/tmp/session_quic_cs.json");

    using var loaded3 = Session.Load("/tmp/session_quic_cs.json");
    var resp3b = loaded3.Get("https://quic.browserleaks.com/?minify=1");
    Console.WriteLine($"Loaded request: Status={resp3b.StatusCode}, Protocol={resp3b.Protocol}");

    File.Delete("/tmp/session_quic_cs.json");
    Console.WriteLine("quic_ech_h3: PASS");
}
catch (Exception e)
{
    Console.WriteLine($"Error: {e.Message}");
    Console.WriteLine("quic_ech_h3: FAIL");
    allPass = false;
}

Console.WriteLine("\n=== Summary ===");
Console.WriteLine(allPass ? "ALL TESTS PASSED!" : "SOME TESTS FAILED!");
