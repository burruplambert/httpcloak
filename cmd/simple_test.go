package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sardanioss/quic-go"
	"github.com/sardanioss/quic-go/http3"
)

func main() {
	fmt.Println("Testing simple HTTP/3 without ECH...")
	
	transport := &http3.Transport{
		TLSClientConfig: &tls.Config{
			ServerName: "quic.browserleaks.com",
			MinVersion: tls.VersionTLS13,
		},
		QUICConfig: &quic.Config{
			MaxIdleTimeout: 10 * time.Second,
		},
	}
	defer transport.Close()
	
	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://quic.browserleaks.com/?minify=1", nil)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d, Body len: %d\n", resp.StatusCode, len(body))
	fmt.Printf("First 100 chars: %s\n", string(body[:min(100, len(body))]))
}
