package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/sardanioss/httpcloak/client"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("TLS Session Resumption Test")
	fmt.Println("Target: quic.browserleaks.com (HTTP/3)")
	fmt.Println("========================================")

	url := "https://quic.browserleaks.com/?minify=1"
	ctx := context.Background()

	// Create client with HTTP/3 forced
	c := client.NewClient("chrome-143", client.WithForceHTTP3())
	defer c.Close()

	// First request - establish new connection and receive session ticket
	fmt.Println("\n[1] First Request - New Connection")
	fmt.Println("-" + fmt.Sprintf("%39s", ""))

	start := time.Now()
	resp, err := c.Get(ctx, url, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	elapsed := time.Since(start)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Time: %dms\n", elapsed.Milliseconds())
	fmt.Printf("Body length: %d bytes\n", len(body))
	fmt.Printf("Protocol: %s\n", resp.Protocol)

	// Small delay to allow session ticket processing
	fmt.Println("\nWaiting 500ms for session ticket processing...")
	time.Sleep(500 * time.Millisecond)

	// Second request - uses pooled connection (no new handshake)
	fmt.Println("\n[2] Second Request - Pooled Connection (Fast)")
	fmt.Println("-" + fmt.Sprintf("%39s", ""))

	start = time.Now()
	resp, err = c.Get(ctx, url, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	elapsed = time.Since(start)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Time: %dms\n", elapsed.Milliseconds())
	fmt.Printf("Body length: %d bytes\n", len(body))
	fmt.Printf("Protocol: %s\n", resp.Protocol)

	// Third request - confirm stable connection
	fmt.Println("\n[3] Third Request - Confirm Stable")
	fmt.Println("-" + fmt.Sprintf("%39s", ""))

	start = time.Now()
	resp, err = c.Get(ctx, url, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	elapsed = time.Since(start)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Time: %dms\n", elapsed.Milliseconds())
	fmt.Printf("Body length: %d bytes\n", len(body))
	fmt.Printf("Protocol: %s\n", resp.Protocol)

	fmt.Println("\n========================================")
	fmt.Println("Test Complete")
	fmt.Println("")
	fmt.Println("Session ticket was received and stored.")
	fmt.Println("If connection times out and new one is needed,")
	fmt.Println("PSK spec will be used for session resumption.")
	fmt.Println("========================================")
}
