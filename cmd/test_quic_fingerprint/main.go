package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sardanioss/httpcloak/client"
)

// H3Response represents the relevant parts of browserleaks response
type H3Response struct {
	JA4    string `json:"ja4"`
	H3Hash string `json:"h3_hash"`
	H3Text string `json:"h3_text"`
	HTTP3  []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		StreamID int    `json:"stream_id"`
		Length   int    `json:"length"`
		Settings []struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Value int64  `json:"value"`
		} `json:"settings,omitempty"`
	} `json:"http3"`
}

func main() {
	// Create client with Chrome fingerprint - will use HTTP/3 by default
	c := client.NewClient("chrome-143-windows",
		client.WithTimeout(30*time.Second),
	)
	defer c.Close()

	ctx := context.Background()

	fmt.Println("Testing QUIC/HTTP3 fingerprint against browserleaks...")
	fmt.Println("Endpoint: https://quic.browserleaks.com/?minify=1")
	fmt.Println()

	resp, err := c.Get(ctx, "https://quic.browserleaks.com/?minify=1", nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Protocol: %s\n", resp.Protocol)
	fmt.Println()

	// Parse and display key fingerprint info
	var h3resp H3Response
	if err := json.Unmarshal(resp.Body, &h3resp); err == nil {
		fmt.Println("=== Fingerprint Summary ===")
		fmt.Printf("JA4: %s\n", h3resp.JA4)
		fmt.Printf("H3 Hash: %s\n", h3resp.H3Hash)
		fmt.Printf("H3 Text: %s\n", h3resp.H3Text)
		fmt.Println()
		fmt.Println("=== HTTP/3 SETTINGS ===")
		for _, frame := range h3resp.HTTP3 {
			if frame.Name == "SETTINGS" {
				fmt.Printf("Frame Length: %d bytes\n", frame.Length)
				for _, s := range frame.Settings {
					fmt.Printf("  Setting 0x%x (%s): %d\n", s.ID, s.Name, s.Value)
				}
			}
		}
	}

	fmt.Println()
	fmt.Println("=== Full Response ===")
	fmt.Println(string(resp.Body))
}
