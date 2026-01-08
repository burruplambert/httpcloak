using HttpCloak;
using System.Text.Json;

Console.WriteLine("=== C# Sync Test ===");
using var session = new Session(preset: Presets.Chrome143Windows, httpVersion: "h3");
var resp = session.Get("https://quic.browserleaks.com/?minify=1");
var data = JsonDocument.Parse(resp.Text).RootElement;
Console.WriteLine($"ja4: {data.GetProperty("ja4").GetString()}");
Console.WriteLine($"h3_hash: {data.GetProperty("h3_hash").GetString()}");
Console.WriteLine($"h3_text: {data.GetProperty("h3_text").GetString()}");
Console.WriteLine($"ECH: {data.GetProperty("tls").GetProperty("ech").GetProperty("ech_success").GetBoolean()}");
Console.WriteLine($"Protocol: {resp.Protocol}");

Console.WriteLine("\n=== C# Async Test ===");
using var asyncSession = new Session(preset: Presets.Chrome143);

// Single async request
Console.WriteLine("Testing single async GET...");
var asyncResp = await asyncSession.GetAsync("https://httpbin.org/get");
Console.WriteLine($"Async GET: {asyncResp.StatusCode}, Protocol: {asyncResp.Protocol}");

// Concurrent async requests
Console.WriteLine("Testing concurrent async requests...");
var tasks = new[]
{
    asyncSession.GetAsync("https://httpbin.org/get"),
    asyncSession.GetAsync("https://httpbin.org/ip"),
    asyncSession.GetAsync("https://httpbin.org/headers")
};
var results = await Task.WhenAll(tasks);
Console.WriteLine($"Concurrent results: [{string.Join(", ", results.Select(r => r.StatusCode))}]");

Console.WriteLine("All async tests passed!");
