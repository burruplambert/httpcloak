using System;
using System.Text.Json;
using HttpCloak;

Console.WriteLine("=== C# 0-RTT Test ===\n");

// Create session with HTTP/3
using var session = new Session(preset: "chrome-143", httpVersion: "h3");

// First request
Console.WriteLine("Request 1 (cold):");
var resp = session.Get("https://quic.browserleaks.com/?minify=1");
var json = JsonDocument.Parse(resp.Text);
var rtt1 = json.RootElement.GetProperty("quic").GetProperty("0-rtt").GetBoolean();
Console.WriteLine($"  0-RTT: {rtt1}");
Console.WriteLine($"  Protocol: {resp.Protocol}");

// Export session
var marshaled = session.Marshal();

// Create new session from marshaled state
Console.WriteLine("\nRequest 2 (restored session):");
using var session2 = Session.Unmarshal(marshaled);
var resp2 = session2.Get("https://quic.browserleaks.com/?minify=1");
var json2 = JsonDocument.Parse(resp2.Text);
var rtt2 = json2.RootElement.GetProperty("quic").GetProperty("0-rtt").GetBoolean();
Console.WriteLine($"  0-RTT: {rtt2}");
Console.WriteLine($"  Protocol: {resp2.Protocol}");

Console.WriteLine($"\n{(rtt2 ? "SUCCESS!" : "FAILED")}");
