using System.Net;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace HttpCloak;

/// <summary>
/// A local HTTP proxy server that forwards requests through httpcloak with TLS fingerprinting.
/// Use this with C# HttpClient to transparently apply fingerprinting to all requests.
/// </summary>
/// <example>
/// <code>
/// // Start local proxy
/// using var proxy = new LocalProxy(preset: "chrome-143");
///
/// // Configure HttpClient to use the proxy
/// var handler = new HttpClientHandler
/// {
///     Proxy = new WebProxy($"http://localhost:{proxy.Port}")
/// };
/// var client = new HttpClient(handler);
///
/// // All requests now go through httpcloak with fingerprinting
/// var response = await client.GetAsync("https://example.com");
/// </code>
/// </example>
public sealed class LocalProxy : IDisposable
{
    private readonly long _handle;
    private bool _disposed;

    /// <summary>
    /// Creates and starts a local HTTP proxy with the specified configuration.
    /// </summary>
    /// <param name="port">Port to listen on (0 = auto-select)</param>
    /// <param name="preset">Browser fingerprint preset (default: chrome-143)</param>
    /// <param name="timeout">Request timeout in seconds (default: 30)</param>
    /// <param name="maxConnections">Maximum concurrent connections (default: 1000)</param>
    /// <param name="tcpProxy">Upstream TCP proxy URL (optional)</param>
    /// <param name="udpProxy">Upstream UDP proxy URL (optional)</param>
    public LocalProxy(
        int port = 0,
        string preset = "chrome-143",
        int timeout = 30,
        int maxConnections = 1000,
        string? tcpProxy = null,
        string? udpProxy = null)
    {
        var config = new LocalProxyConfig
        {
            Port = port,
            Preset = preset,
            Timeout = timeout,
            MaxConnections = maxConnections,
            TcpProxy = tcpProxy,
            UdpProxy = udpProxy
        };

        string configJson = JsonSerializer.Serialize(config, LocalProxyJsonContext.Default.LocalProxyConfig);
        _handle = Native.LocalProxyStart(configJson);

        if (_handle < 0)
        {
            throw new HttpCloakException("Failed to start local proxy");
        }
    }

    /// <summary>
    /// Gets the port the proxy is listening on.
    /// </summary>
    public int Port
    {
        get
        {
            ThrowIfDisposed();
            return Native.LocalProxyGetPort(_handle);
        }
    }

    /// <summary>
    /// Gets whether the proxy is currently running.
    /// </summary>
    public bool IsRunning
    {
        get
        {
            if (_disposed) return false;
            return Native.LocalProxyIsRunning(_handle) != 0;
        }
    }

    /// <summary>
    /// Gets the proxy URL for use with HttpClient.
    /// </summary>
    public string ProxyUrl
    {
        get
        {
            ThrowIfDisposed();
            return $"http://localhost:{Port}";
        }
    }

    /// <summary>
    /// Creates a WebProxy instance configured to use this local proxy.
    /// </summary>
    public WebProxy CreateWebProxy()
    {
        ThrowIfDisposed();
        return new WebProxy(ProxyUrl);
    }

    /// <summary>
    /// Creates an HttpClientHandler configured to use this local proxy.
    /// </summary>
    public HttpClientHandler CreateHandler()
    {
        ThrowIfDisposed();
        return new HttpClientHandler
        {
            Proxy = CreateWebProxy(),
            UseProxy = true
        };
    }

    /// <summary>
    /// Gets proxy statistics.
    /// </summary>
    public LocalProxyStats GetStats()
    {
        ThrowIfDisposed();

        IntPtr ptr = Native.LocalProxyGetStats(_handle);
        string? json = Native.PtrToStringAndFree(ptr);

        if (string.IsNullOrEmpty(json))
        {
            return new LocalProxyStats();
        }

        // Check for error
        if (json.Contains("\"error\""))
        {
            return new LocalProxyStats();
        }

        try
        {
            return JsonSerializer.Deserialize(json, LocalProxyJsonContext.Default.LocalProxyStats)
                ?? new LocalProxyStats();
        }
        catch
        {
            return new LocalProxyStats();
        }
    }

    private void ThrowIfDisposed()
    {
        if (_disposed)
        {
            throw new ObjectDisposedException(nameof(LocalProxy));
        }
    }

    /// <summary>
    /// Stops the local proxy server.
    /// </summary>
    public void Dispose()
    {
        if (_disposed) return;
        _disposed = true;

        Native.LocalProxyStop(_handle);
        GC.SuppressFinalize(this);
    }

    ~LocalProxy()
    {
        Dispose();
    }
}

/// <summary>
/// Configuration for creating a local proxy.
/// </summary>
internal class LocalProxyConfig
{
    [JsonPropertyName("port")]
    public int Port { get; set; }

    [JsonPropertyName("preset")]
    public string Preset { get; set; } = "chrome-143";

    [JsonPropertyName("timeout")]
    public int Timeout { get; set; } = 30;

    [JsonPropertyName("max_connections")]
    public int MaxConnections { get; set; } = 1000;

    [JsonPropertyName("tcp_proxy")]
    [JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
    public string? TcpProxy { get; set; }

    [JsonPropertyName("udp_proxy")]
    [JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
    public string? UdpProxy { get; set; }
}

/// <summary>
/// Statistics about the local proxy.
/// </summary>
public class LocalProxyStats
{
    [JsonPropertyName("running")]
    public bool Running { get; set; }

    [JsonPropertyName("port")]
    public int Port { get; set; }

    [JsonPropertyName("active_conns")]
    public long ActiveConnections { get; set; }

    [JsonPropertyName("total_requests")]
    public long TotalRequests { get; set; }

    [JsonPropertyName("preset")]
    public string? Preset { get; set; }

    [JsonPropertyName("max_connections")]
    public int MaxConnections { get; set; }
}

/// <summary>
/// JSON serialization context for LocalProxy types.
/// </summary>
[JsonSerializable(typeof(LocalProxyConfig))]
[JsonSerializable(typeof(LocalProxyStats))]
internal partial class LocalProxyJsonContext : JsonSerializerContext
{
}
