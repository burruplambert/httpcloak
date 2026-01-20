package httpcloak

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sardanioss/httpcloak/proxy"
)

// LocalProxy is an HTTP proxy server that forwards requests through httpcloak
// sessions with TLS fingerprinting.
//
// Architecture:
// - For HTTP requests: Forwards through httpcloak Session (fingerprinting applied)
// - For HTTPS (CONNECT): Tunnels TCP (client does TLS, fingerprinting via upstream proxy only)
//
// Usage with C# HttpClient:
//
//	proxy := httpcloak.StartLocalProxy(8080, "chrome-143")
//	defer proxy.Stop()
//	// Configure HttpClient to use http://localhost:8080 as proxy
type LocalProxy struct {
	listener net.Listener
	port     int

	// Configuration
	preset         string
	timeout        time.Duration
	maxConnections int
	tcpProxy       string // Upstream proxy for TCP connections
	udpProxy       string // Upstream proxy for UDP connections
	tlsOnly        bool   // TLS-only mode: skip preset HTTP headers

	// Session for making requests (HTTP forwarding with fingerprinting)
	session   *Session
	sessionMu sync.RWMutex

	// Fast HTTP client for plain HTTP forwarding (no fingerprinting overhead)
	httpClient *http.Client
	transport  *http.Transport

	// State
	running      atomic.Bool
	activeConns  atomic.Int64
	totalReqs    atomic.Int64
	shuttingDown atomic.Bool

	// Shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// LocalProxyConfig holds configuration for the local proxy
type LocalProxyConfig struct {
	// Port to listen on (0 = auto-select)
	Port int

	// Browser fingerprint preset (default: chrome-143)
	Preset string

	// Request timeout
	Timeout time.Duration

	// Maximum concurrent connections
	MaxConnections int

	// Upstream proxy (optional)
	TCPProxy string
	UDPProxy string

	// TLSOnly mode: only apply TLS fingerprinting, pass HTTP headers through unchanged.
	// Useful when the client (e.g., Playwright) already provides authentic browser headers.
	TLSOnly bool
}

// LocalProxyOption configures the local proxy
type LocalProxyOption func(*LocalProxyConfig)

// WithProxyPreset sets the browser fingerprint preset
func WithProxyPreset(preset string) LocalProxyOption {
	return func(c *LocalProxyConfig) {
		c.Preset = preset
	}
}

// WithProxyTimeout sets the request timeout
func WithProxyTimeout(d time.Duration) LocalProxyOption {
	return func(c *LocalProxyConfig) {
		c.Timeout = d
	}
}

// WithProxyMaxConnections sets the maximum concurrent connections
func WithProxyMaxConnections(n int) LocalProxyOption {
	return func(c *LocalProxyConfig) {
		c.MaxConnections = n
	}
}

// WithProxyUpstream sets upstream proxy URLs for the httpcloak session
func WithProxyUpstream(tcpProxy, udpProxy string) LocalProxyOption {
	return func(c *LocalProxyConfig) {
		c.TCPProxy = tcpProxy
		c.UDPProxy = udpProxy
	}
}

// WithProxyTLSOnly enables TLS-only mode where only TLS fingerprinting is applied.
// HTTP headers from the client pass through unchanged - useful when using Playwright
// or other browsers that already provide authentic headers.
func WithProxyTLSOnly() LocalProxyOption {
	return func(c *LocalProxyConfig) {
		c.TLSOnly = true
	}
}

// StartLocalProxy creates and starts a local HTTP proxy on the specified port.
// The proxy forwards requests through httpcloak sessions with TLS fingerprinting.
//
// Example:
//
//	proxy, err := httpcloak.StartLocalProxy(8080, httpcloak.WithProxyPreset("chrome-143"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer proxy.Stop()
//	fmt.Printf("Proxy running on port %d\n", proxy.Port())
func StartLocalProxy(port int, opts ...LocalProxyOption) (*LocalProxy, error) {
	config := &LocalProxyConfig{
		Port:           port,
		Preset:         "chrome-143",
		Timeout:        30 * time.Second,
		MaxConnections: 1000,
	}

	for _, opt := range opts {
		opt(config)
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &LocalProxy{
		port:           config.Port,
		preset:         config.Preset,
		timeout:        config.Timeout,
		maxConnections: config.MaxConnections,
		tcpProxy:       config.TCPProxy,
		udpProxy:       config.UDPProxy,
		tlsOnly:        config.TLSOnly,
		ctx:            ctx,
		cancel:         cancel,
	}

	// Create session for HTTP forwarding
	sessionOpts := []SessionOption{
		WithSessionTimeout(config.Timeout),
	}
	if config.TCPProxy != "" {
		sessionOpts = append(sessionOpts, WithSessionTCPProxy(config.TCPProxy))
	}
	if config.UDPProxy != "" {
		sessionOpts = append(sessionOpts, WithSessionUDPProxy(config.UDPProxy))
	}
	if config.TLSOnly {
		sessionOpts = append(sessionOpts, WithTLSOnly())
	}
	p.session = NewSession(config.Preset, sessionOpts...)

	// Create fast HTTP transport for plain HTTP forwarding
	p.transport = &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true, // Let client handle compression
		WriteBufferSize:     64 * 1024,
		ReadBufferSize:      64 * 1024,
		ForceAttemptHTTP2:   false, // Keep HTTP/1.1 for simplicity
	}
	p.httpClient = &http.Client{
		Transport: p.transport,
		Timeout:   config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	// Start the server
	if err := p.start(); err != nil {
		p.session.Close()
		p.transport.CloseIdleConnections()
		cancel()
		return nil, err
	}

	return p, nil
}

// start starts the proxy server
func (p *LocalProxy) start() error {
	if p.running.Load() {
		return errors.New("proxy already running")
	}

	// Listen on localhost only (security)
	addr := fmt.Sprintf("127.0.0.1:%d", p.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	p.listener = listener
	p.running.Store(true)

	// Update port if auto-selected
	if p.port == 0 {
		p.port = listener.Addr().(*net.TCPAddr).Port
	}

	// Start accept loop
	p.wg.Add(1)
	go p.acceptLoop()

	return nil
}

// Stop stops the local proxy server gracefully
func (p *LocalProxy) Stop() error {
	if !p.running.Load() {
		return nil
	}

	p.shuttingDown.Store(true)
	p.cancel()

	if p.listener != nil {
		p.listener.Close()
	}

	// Wait for all connections to finish (with timeout)
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		// Force close after timeout
	}

	// Close session and transport
	if p.session != nil {
		p.session.Close()
	}
	if p.transport != nil {
		p.transport.CloseIdleConnections()
	}

	p.running.Store(false)
	return nil
}

// Port returns the port the proxy is listening on
func (p *LocalProxy) Port() int {
	return p.port
}

// IsRunning returns whether the proxy is running
func (p *LocalProxy) IsRunning() bool {
	return p.running.Load()
}

// Stats returns proxy statistics
func (p *LocalProxy) Stats() map[string]interface{} {
	return map[string]interface{}{
		"running":         p.running.Load(),
		"port":            p.port,
		"active_conns":    p.activeConns.Load(),
		"total_requests":  p.totalReqs.Load(),
		"preset":          p.preset,
		"max_connections": p.maxConnections,
	}
}

// acceptLoop accepts incoming connections
func (p *LocalProxy) acceptLoop() {
	defer p.wg.Done()

	for {
		conn, err := p.listener.Accept()
		if err != nil {
			if p.shuttingDown.Load() {
				return
			}
			continue
		}

		// Check connection limit
		if p.activeConns.Load() >= int64(p.maxConnections) {
			conn.Close()
			continue
		}

		p.activeConns.Add(1)
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			defer p.activeConns.Add(-1)
			p.handleConnection(conn)
		}()
	}
}

// handleConnection handles a single client connection
func (p *LocalProxy) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Set read deadline for initial request
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// Read the HTTP request
	reader := bufio.NewReader(conn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		p.sendError(conn, http.StatusBadRequest, "Bad Request")
		return
	}

	// Clear deadline
	conn.SetReadDeadline(time.Time{})

	p.totalReqs.Add(1)

	// Handle based on method
	if req.Method == http.MethodConnect {
		p.handleCONNECT(conn, req)
	} else {
		p.handleHTTP(conn, req, reader)
	}
}

// handleCONNECT handles HTTP CONNECT requests (HTTPS tunneling)
// Note: For CONNECT, we just tunnel - the client does its own TLS.
// Fingerprinting only works if an upstream proxy is configured.
func (p *LocalProxy) handleCONNECT(clientConn net.Conn, req *http.Request) {
	// Parse target host:port
	targetHost := req.Host
	if targetHost == "" {
		targetHost = req.URL.Host
	}

	host, port, err := net.SplitHostPort(targetHost)
	if err != nil {
		host = targetHost
		port = "443"
		targetHost = net.JoinHostPort(host, port)
	}

	// Security check
	if !p.isPortAllowed(port) {
		p.sendError(clientConn, http.StatusForbidden, "Port not allowed")
		return
	}

	// Connect to target
	ctx, cancel := context.WithTimeout(p.ctx, p.timeout)
	defer cancel()

	targetConn, err := p.dialTarget(ctx, host, port)
	if err != nil {
		p.sendError(clientConn, http.StatusBadGateway, fmt.Sprintf("Failed to connect: %v", err))
		return
	}
	defer targetConn.Close()

	// Send 200 Connection Established
	response := "HTTP/1.1 200 Connection Established\r\n\r\n"
	if _, err := clientConn.Write([]byte(response)); err != nil {
		return
	}

	// Bidirectional tunnel
	p.tunnel(clientConn, targetConn)
}

// handleHTTP handles plain HTTP requests using fast direct forwarding
func (p *LocalProxy) handleHTTP(clientConn net.Conn, req *http.Request, reader *bufio.Reader) {
	// Build target URL
	targetURL := req.URL.String()
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		if req.URL.Host != "" {
			targetURL = "http://" + req.URL.Host + req.URL.RequestURI()
		} else if req.Host != "" {
			targetURL = "http://" + req.Host + req.URL.RequestURI()
		} else {
			p.sendError(clientConn, http.StatusBadRequest, "Missing host")
			return
		}
	}

	// Create outgoing request
	ctx, cancel := context.WithTimeout(p.ctx, p.timeout)
	defer cancel()

	outReq, err := http.NewRequestWithContext(ctx, req.Method, targetURL, req.Body)
	if err != nil {
		p.sendError(clientConn, http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	// Copy headers (skip hop-by-hop)
	for key, values := range req.Header {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			outReq.Header.Add(key, value)
		}
	}
	outReq.ContentLength = req.ContentLength

	// Execute request using fast http.Client
	resp, err := p.httpClient.Do(outReq)
	if err != nil {
		p.sendError(clientConn, http.StatusBadGateway, fmt.Sprintf("Request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	// Use buffered writer for better performance
	bufWriter := bufio.NewWriterSize(clientConn, 64*1024)

	// Write status line
	fmt.Fprintf(bufWriter, "HTTP/1.1 %d %s\r\n", resp.StatusCode, resp.Status)

	// Write headers (skip hop-by-hop)
	for key, values := range resp.Header {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			fmt.Fprintf(bufWriter, "%s: %s\r\n", key, value)
		}
	}
	bufWriter.WriteString("\r\n")
	bufWriter.Flush()

	// Stream body with large buffer
	if resp.Body != nil {
		buf := make([]byte, 64*1024) // 64KB buffer
		io.CopyBuffer(clientConn, resp.Body, buf)
	}
}

// dialTarget connects to the target, optionally through upstream proxy
func (p *LocalProxy) dialTarget(ctx context.Context, host, port string) (net.Conn, error) {
	targetAddr := net.JoinHostPort(host, port)

	// If upstream SOCKS5 proxy configured, use it
	if p.tcpProxy != "" && proxy.IsSOCKS5URL(p.tcpProxy) {
		socks5Dialer, err := proxy.NewSOCKS5Dialer(p.tcpProxy)
		if err != nil {
			return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
		}
		return socks5Dialer.DialContext(ctx, "tcp", targetAddr)
	}

	// Direct connection
	dialer := &net.Dialer{
		Timeout:   p.timeout,
		KeepAlive: 30 * time.Second,
	}
	return dialer.DialContext(ctx, "tcp", targetAddr)
}

// tunnel performs bidirectional data transfer with large buffers
func (p *LocalProxy) tunnel(client, target net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Large buffer for high throughput
	const bufSize = 64 * 1024 // 64KB

	// Client -> Target
	go func() {
		defer wg.Done()
		buf := make([]byte, bufSize)
		io.CopyBuffer(target, client, buf)
		if tc, ok := target.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()

	// Target -> Client
	go func() {
		defer wg.Done()
		buf := make([]byte, bufSize)
		io.CopyBuffer(client, target, buf)
		if tc, ok := client.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()

	wg.Wait()
}

// isPortAllowed checks if a port is allowed
func (p *LocalProxy) isPortAllowed(port string) bool {
	blocked := map[string]bool{
		"25": true, "465": true, "587": true, // SMTP
		"23": true, // Telnet
	}
	return !blocked[port]
}

// sendError sends an HTTP error response
func (p *LocalProxy) sendError(conn net.Conn, status int, message string) {
	response := fmt.Sprintf("HTTP/1.1 %d %s\r\nContent-Type: text/plain\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
		status, http.StatusText(status), len(message), message)
	conn.Write([]byte(response))
}

// isHopByHopHeader returns true for hop-by-hop headers that shouldn't be forwarded
func isHopByHopHeader(header string) bool {
	hopByHop := map[string]bool{
		"Connection":          true,
		"Keep-Alive":          true,
		"Proxy-Authenticate":  true,
		"Proxy-Authorization": true,
		"Proxy-Connection":    true,
		"Te":                  true,
		"Trailer":             true,
		"Transfer-Encoding":   true,
		"Upgrade":             true,
	}
	return hopByHop[http.CanonicalHeaderKey(header)]
}
