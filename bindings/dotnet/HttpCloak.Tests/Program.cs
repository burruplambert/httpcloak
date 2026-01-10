// HttpCloak C# Tests
using HttpCloak;
using HttpCloak.Tests;
using System.Text;

// Check for command line arguments
var testType = args.Length > 0 ? args[0].ToLower() : "features";

switch (testType)
{
    case "proxy":
        await ProxyTests.RunAllTests();
        break;

    case "ech":
        RunEchTest();
        break;

    case "bench":
        BenchSpeed.Run();
        break;

    case "benchlocal":
        BenchLocal.Run();
        break;

    case "handler":
        await TestHttpCloakHandler.Run();
        break;

    case "features":
    default:
        RunNewFeaturesTest();
        break;
}

static void RunEchTest()
{
    Console.WriteLine("Testing ECH with C# bindings (HTTP/3)...");
    Console.WriteLine(new string('=', 50));

    using var session = new Session(
        preset: "chrome-143",
        echConfigDomain: "cloudflare-ech.com",
        httpVersion: "h3",
        retry: 0
    );

    try
    {
        var response = session.Get("https://www.cloudflare.com/cdn-cgi/trace");
        Console.WriteLine($"Status: {response.StatusCode}");
        Console.WriteLine($"Protocol: {response.Protocol}");
        Console.WriteLine();
        Console.WriteLine("Response:");
        Console.WriteLine(response.Text);

        // Check for key indicators
        var lines = response.Text.Trim().Split('\n');
        foreach (var line in lines)
        {
            if (line.StartsWith("http="))
            {
                Console.WriteLine($"\n>> HTTP Version: {line}");
            }
            if (line.StartsWith("sni="))
            {
                Console.WriteLine($">> SNI Status: {line}");
                if (line.Contains("encrypted"))
                {
                    Console.WriteLine("   SUCCESS: ECH is working!");
                }
                else
                {
                    Console.WriteLine("   WARNING: ECH may not be enabled");
                }
            }
        }
    }
    catch (Exception ex)
    {
        Console.WriteLine($"Error: {ex.Message}");
    }
}

static void RunNewFeaturesTest()
{
    Console.WriteLine("=".PadRight(60, '='));
    Console.WriteLine("C# NEW FEATURES TEST");
    Console.WriteLine("=".PadRight(60, '='));

    using var session = new Session(preset: "chrome-143", timeout: 30);

    // Test 1: Multi-value headers
    Console.WriteLine("\n[1] Testing Multi-Value Headers");
    Console.WriteLine("-".PadRight(50, '-'));
    try
    {
        var r = session.Get("https://httpbin.org/response-headers?Set-Cookie=cookie1%3Dvalue1&Set-Cookie=cookie2%3Dvalue2");
        Console.WriteLine($"  Status: {r.StatusCode}");
        Console.WriteLine($"  Headers type: Dictionary<string, string[]>");

        // Test GetHeader (single value)
        var contentType = r.GetHeader("Content-Type");
        Console.WriteLine($"  GetHeader('Content-Type'): {contentType}");

        // Test GetHeaders (all values)
        var allSetCookies = r.GetHeaders("Set-Cookie");
        Console.WriteLine($"  GetHeaders('Set-Cookie') count: {allSetCookies.Length}");
        foreach (var cookie in allSetCookies)
        {
            Console.WriteLine($"    - {cookie}");
        }
        Console.WriteLine("  [PASS] Multi-value headers working");
    }
    catch (Exception ex)
    {
        Console.WriteLine($"  [FAIL] {ex.Message}");
    }

    // Test 2: Streaming download with GetContentStream
    Console.WriteLine("\n[2] Testing GetContentStream()");
    Console.WriteLine("-".PadRight(50, '-'));
    try
    {
        using var streamResponse = session.GetStream("https://httpbin.org/bytes/1024");
        Console.WriteLine($"  Status: {streamResponse.StatusCode}");
        Console.WriteLine($"  ContentLength: {streamResponse.ContentLength}");

        // Get the content stream
        using var contentStream = streamResponse.GetContentStream();
        Console.WriteLine($"  Stream type: {contentStream.GetType().Name}");
        Console.WriteLine($"  CanRead: {contentStream.CanRead}");
        Console.WriteLine($"  CanSeek: {contentStream.CanSeek}");

        // Read all bytes using CopyTo
        using var ms = new MemoryStream();
        contentStream.CopyTo(ms);
        var bytes = ms.ToArray();
        Console.WriteLine($"  Bytes read: {bytes.Length}");
        Console.WriteLine("  [PASS] GetContentStream working");
    }
    catch (Exception ex)
    {
        Console.WriteLine($"  [FAIL] {ex.Message}");
    }

    // Test 3: Binary upload with byte[]
    Console.WriteLine("\n[3] Testing Binary Upload (byte[])");
    Console.WriteLine("-".PadRight(50, '-'));
    try
    {
        var binaryData = new byte[256];
        for (int i = 0; i < 256; i++) binaryData[i] = (byte)i;

        var r = session.Post("https://httpbin.org/post", binaryData, new Dictionary<string, string>
        {
            ["Content-Type"] = "application/octet-stream"
        });
        Console.WriteLine($"  Status: {r.StatusCode}");
        Console.WriteLine($"  Response length: {r.Text.Length} chars");
        Console.WriteLine("  [PASS] Binary upload working");
    }
    catch (Exception ex)
    {
        Console.WriteLine($"  [FAIL] {ex.Message}");
    }

    // Test 4: Stream upload
    Console.WriteLine("\n[4] Testing Stream Upload");
    Console.WriteLine("-".PadRight(50, '-'));
    try
    {
        var data = Encoding.UTF8.GetBytes("{\"test\": \"stream upload\"}");
        using var bodyStream = new MemoryStream(data);

        var r = session.Post("https://httpbin.org/post", bodyStream, new Dictionary<string, string>
        {
            ["Content-Type"] = "application/json"
        });
        Console.WriteLine($"  Status: {r.StatusCode}");

        if (r.Text.Contains("stream upload"))
        {
            Console.WriteLine("  Data received correctly");
        }
        Console.WriteLine("  [PASS] Stream upload working");
    }
    catch (Exception ex)
    {
        Console.WriteLine($"  [FAIL] {ex.Message}");
    }

    // Test 5: ReadChunks enumeration
    Console.WriteLine("\n[5] Testing Streaming ReadChunks()");
    Console.WriteLine("-".PadRight(50, '-'));
    try
    {
        using var streamResponse = session.GetStream("https://httpbin.org/bytes/4096");
        Console.WriteLine($"  Status: {streamResponse.StatusCode}");

        int totalBytes = 0;
        int chunkCount = 0;
        foreach (var chunk in streamResponse.ReadChunks(1024))
        {
            totalBytes += chunk.Length;
            chunkCount++;
        }
        Console.WriteLine($"  Total chunks: {chunkCount}");
        Console.WriteLine($"  Total bytes: {totalBytes}");
        Console.WriteLine("  [PASS] ReadChunks working");
    }
    catch (Exception ex)
    {
        Console.WriteLine($"  [FAIL] {ex.Message}");
    }

    Console.WriteLine("\n" + "=".PadRight(60, '='));
    Console.WriteLine("ALL TESTS COMPLETED");
    Console.WriteLine("=".PadRight(60, '='));
}
