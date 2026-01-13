package main

import (
	"context"
	"fmt"
	// "os"
	"strings"
	"time"

	"github.com/sardanioss/httpcloak/client"
)

func main() {
	// Disable ECH debug for cleaner output
	// os.Setenv("UTLS_ECH_DEBUG", "1")
	// Test ECH + PSK (0-RTT) with browserleaks
	url := "https://quic.browserleaks.com/?minify=1"
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
	bodyStr := string(body)
	// Extract 0-rtt value
	if strings.Contains(bodyStr, `"0-rtt":true`) {
		fmt.Println("0-RTT: TRUE")
	} else if strings.Contains(bodyStr, `"0-rtt":false`) {
		fmt.Println("0-RTT: FALSE (expected for first request)")
	}
	fmt.Printf("Status: %d, Protocol: %s\n\n", resp.StatusCode, resp.Protocol)

	fmt.Println("Waiting 500ms for session ticket processing...")
	time.Sleep(500 * time.Millisecond)

	fmt.Println("\n=== Closing QUIC connections (keeping session cache) ===")
	c.CloseQUICConnections()
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\n=== Request 2 (New Connection - Should use 0-RTT) ===")
	resp, err = c.Get(ctx, url, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	body, _ = resp.Bytes()
	bodyStr = string(body)
	// Extract 0-rtt value
	if strings.Contains(bodyStr, `"0-rtt":true`) {
		fmt.Println("0-RTT: TRUE (SUCCESS!)")
	} else if strings.Contains(bodyStr, `"0-rtt":false`) {
		fmt.Println("0-RTT: FALSE (session resumption might not have 0-RTT)")
	}
	fmt.Printf("Status: %d, Protocol: %s\n", resp.StatusCode, resp.Protocol)

	fmt.Println("\n=== Request 3 (Confirm stable) ===")
	c.CloseQUICConnections()
	time.Sleep(100 * time.Millisecond)
	resp, err = c.Get(ctx, url, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	body, _ = resp.Bytes()
	bodyStr = string(body)
	if strings.Contains(bodyStr, `"0-rtt":true`) {
		fmt.Println("0-RTT: TRUE (SUCCESS!)")
	} else if strings.Contains(bodyStr, `"0-rtt":false`) {
		fmt.Println("0-RTT: FALSE")
	}
	fmt.Printf("Status: %d, Protocol: %s\n", resp.StatusCode, resp.Protocol)
}
