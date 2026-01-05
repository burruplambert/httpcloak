package main

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	http "github.com/sardanioss/http"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"github.com/sardanioss/httpcloak/dns"
	"github.com/sardanioss/httpcloak/fingerprint"
	"github.com/sardanioss/httpcloak/pool"
)

// time is used for context timeout

func main() {
	// Create DNS cache
	dnsCache := dns.NewCache()

	// Use Chrome 143 preset with QUIC fingerprinting
	preset := fingerprint.Chrome143()
	fmt.Printf("Using preset: %s\n", preset.Name)
	fmt.Printf("QUIC ClientHelloID: %+v\n", preset.QUICClientHelloID)

	// Create QUIC manager with the preset
	quicManager := pool.NewQUICManager(preset, dnsCache)
	defer quicManager.Close()

	// Get a connection to browserleaks
	host := "quic.browserleaks.com"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get connection from pool
	conn, err := quicManager.GetConn(ctx, host, "443")
	if err != nil {
		fmt.Printf("Failed to get connection: %v\n", err)
		return
	}
	fmt.Printf("Got connection to %s\n", host)

	// Create HTTP request - use /fp endpoint for fingerprint summary
	req, err := http.NewRequestWithContext(ctx, "GET", "https://quic.browserleaks.com/fp", nil)
	if err != nil {
		fmt.Printf("Failed to create request: %v\n", err)
		return
	}

	// Set Chrome-like headers from preset
	for key, value := range preset.Headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("User-Agent", preset.UserAgent)

	// Use the HTTP/3 transport from the connection
	resp, err := conn.HTTP3RT.RoundTrip(req)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Handle compressed responses
	var reader io.Reader = resp.Body
	switch resp.Header.Get("Content-Encoding") {
	case "br":
		reader = brotli.NewReader(resp.Body)
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			fmt.Printf("Failed to create gzip reader: %v\n", err)
			return
		}
	case "zstd":
		zr, err := zstd.NewReader(resp.Body)
		if err != nil {
			fmt.Printf("Failed to create zstd reader: %v\n", err)
			return
		}
		defer zr.Close()
		reader = zr
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		fmt.Printf("Failed to read response: %v\n", err)
		return
	}

	fmt.Printf("\nHTTP/3 Response (Status: %s, Encoding: %s):\n%s\n", resp.Status, resp.Header.Get("Content-Encoding"), string(body))
}
