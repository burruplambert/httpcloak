package main

import (
	"context"
	"fmt"
	"time"

	"github.com/sardanioss/httpcloak/client"
)

func main() {
	// Test PSK WITHOUT ECH using cloudflare.com
	url := "https://cloudflare.com/"
	ctx := context.Background()

	c := client.NewClient("chrome-143", client.WithForceHTTP3())
	defer c.Close()

	fmt.Println("=== Request 1 (New Connection - Get Session Ticket) ===")
	fmt.Println("Target:", url)
	resp, err := c.Get(ctx, url, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	body, _ := resp.Bytes()
	fmt.Printf("Status: %d, Protocol: %s, Body len: %d\n\n", resp.StatusCode, resp.Protocol, len(body))

	fmt.Println("Waiting 500ms for session ticket processing...")
	time.Sleep(500 * time.Millisecond)

	fmt.Println("\n=== Closing QUIC connections (keeping session cache) ===")
	c.CloseQUICConnections()
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\n=== Request 2 (New Connection - Should use PSK) ===")
	resp, err = c.Get(ctx, url, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	body, _ = resp.Bytes()
	fmt.Printf("Status: %d, Protocol: %s, Body len: %d\n", resp.StatusCode, resp.Protocol, len(body))

	fmt.Println("\n=== Request 3 (Confirm stable) ===")
	c.CloseQUICConnections()
	time.Sleep(100 * time.Millisecond)
	resp, err = c.Get(ctx, url, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	body, _ = resp.Bytes()
	fmt.Printf("Status: %d, Protocol: %s, Body len: %d\n", resp.StatusCode, resp.Protocol, len(body))

	fmt.Println("\nTest Complete - PSK without ECH")
}
