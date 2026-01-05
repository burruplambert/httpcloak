package main

import (
	"encoding/json"
	"fmt"
	"io"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
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
	fmt.Println("Testing bogdanfinn/tls-client fingerprint against browserleaks...")
	fmt.Println("Endpoint: https://quic.browserleaks.com/?minify=1")
	fmt.Println()

	// Create options with best Chrome profile
	// HTTP/3 is enabled by default in bogdanfinn/tls-client
	options := []tls_client.HttpClientOption{
		tls_client.WithClientProfile(profiles.Chrome_133), // Latest Chrome profile (Chrome 133)
		tls_client.WithRandomTLSExtensionOrder(),
		tls_client.WithNotFollowRedirects(),
		tls_client.WithTimeoutSeconds(30),
		// HTTP/3 is ON by default, no need to enable it
	}

	// Create client
	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}

	// Create request
	req, err := http.NewRequest("GET", "https://quic.browserleaks.com/?minify=1", nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return
	}

	// Set Chrome 133 headers
	req.Header = http.Header{
		"accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
		"accept-encoding":           {"gzip, deflate, br, zstd"},
		"accept-language":           {"en-US,en;q=0.9"},
		"cache-control":             {"max-age=0"},
		"priority":                  {"u=0, i"},
		"sec-ch-ua":                 {`"Google Chrome";v="133", "Chromium";v="133", "Not_A Brand";v="24"`},
		"sec-ch-ua-mobile":          {"?0"},
		"sec-ch-ua-platform":        {`"Linux"`},
		"sec-fetch-dest":            {"document"},
		"sec-fetch-mode":            {"navigate"},
		"sec-fetch-site":            {"none"},
		"sec-fetch-user":            {"?1"},
		"upgrade-insecure-requests": {"1"},
		"user-agent":                {"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36"},
		http.HeaderOrderKey: {
			"accept",
			"accept-encoding",
			"accept-language",
			"cache-control",
			"priority",
			"sec-ch-ua",
			"sec-ch-ua-mobile",
			"sec-ch-ua-platform",
			"sec-fetch-dest",
			"sec-fetch-mode",
			"sec-fetch-site",
			"sec-fetch-user",
			"upgrade-insecure-requests",
			"user-agent",
		},
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Protocol: %s\n", resp.Proto)
	fmt.Println()

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading body: %v\n", err)
		return
	}

	// Parse and display key fingerprint info
	var h3resp H3Response
	if err := json.Unmarshal(body, &h3resp); err == nil {
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
	fmt.Println(string(body))
}
