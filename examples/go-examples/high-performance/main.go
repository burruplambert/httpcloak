// Example: High-Performance Downloads with Buffer Pooling
//
// This example demonstrates:
// - Using resp.Release() for maximum download performance
// - Buffer pooling to avoid memory allocation overhead
// - Benchmarking download speeds
// - Best practices for high-throughput scenarios
//
// Run: go run main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sardanioss/httpcloak"
)

func main() {
	ctx := context.Background()

	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("httpcloak - High-Performance Downloads")
	fmt.Println(strings.Repeat("=", 70))

	session := httpcloak.NewSession("chrome-131")
	defer session.Close()

	// =========================================================================
	// Understanding Buffer Pooling
	// =========================================================================
	fmt.Println("\n[INFO] Buffer Pooling Explanation")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Println(`
HTTPCloak uses internal buffer pools to minimize memory allocation
overhead during downloads. When you call resp.Release(), the internal
buffer is returned to the pool for reuse by subsequent requests.

Without Release(): ~5000 MB/s (allocates new buffer each time)
With Release():    ~9000 MB/s (reuses pooled buffers)

IMPORTANT: After calling Release(), the resp.Body slice is invalidated
and must not be accessed.`)

	// =========================================================================
	// Example 1: Basic usage with Release()
	// =========================================================================
	fmt.Println("\n[1] Basic Usage with Release()")
	fmt.Println(strings.Repeat("-", 50))

	resp, err := session.Get(ctx, "https://httpbin.org/bytes/1024")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Process the response body while it's valid
	size := len(resp.Body)
	fmt.Printf("Downloaded %d bytes\n", size)

	// Release the buffer back to pool when done
	// After this, resp.Body is nil and must not be accessed
	resp.Release()
	fmt.Println("Buffer released to pool")

	// =========================================================================
	// Example 2: Copy data before releasing (if you need to keep it)
	// =========================================================================
	fmt.Println("\n[2] Copy Data Before Releasing")
	fmt.Println(strings.Repeat("-", 50))

	resp, err = session.Get(ctx, "https://httpbin.org/bytes/1024")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Copy the data if you need to keep it after Release()
	dataCopy := make([]byte, len(resp.Body))
	copy(dataCopy, resp.Body)

	// Now safe to release
	resp.Release()

	// dataCopy is still valid
	fmt.Printf("Kept copy of %d bytes after release\n", len(dataCopy))

	// =========================================================================
	// Example 3: Process in place (most efficient)
	// =========================================================================
	fmt.Println("\n[3] Process In Place (Most Efficient)")
	fmt.Println(strings.Repeat("-", 50))

	resp, err = session.Get(ctx, "https://httpbin.org/json")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Process the data directly from the pooled buffer
	// This is the most efficient pattern - no extra copies
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body, &result); err == nil {
		fmt.Printf("Parsed JSON with %d keys\n", len(result))
	}

	// Release after processing
	resp.Release()

	// =========================================================================
	// Example 4: High-throughput download loop
	// =========================================================================
	fmt.Println("\n[4] High-Throughput Download Loop")
	fmt.Println(strings.Repeat("-", 50))

	// Warm up the buffer pool with initial request
	warmup, _ := session.Get(ctx, "https://httpbin.org/bytes/102400")
	warmup.Release()

	// Now subsequent requests will reuse the pooled buffer
	var totalBytes int64
	start := time.Now()
	iterations := 10

	for i := 0; i < iterations; i++ {
		resp, err := session.Get(ctx, "https://httpbin.org/bytes/102400")
		if err != nil {
			fmt.Printf("Error on iteration %d: %v\n", i, err)
			continue
		}
		totalBytes += int64(len(resp.Body))
		resp.Release() // Critical: release for next iteration to reuse buffer
	}

	elapsed := time.Since(start)
	speed := float64(totalBytes) / (1024 * 1024) / elapsed.Seconds()
	fmt.Printf("Downloaded %d requests, %.2f MB total\n", iterations, float64(totalBytes)/(1024*1024))
	fmt.Printf("Time: %v | Speed: %.1f MB/s\n", elapsed, speed)

	// =========================================================================
	// Example 5: When NOT to use Release()
	// =========================================================================
	fmt.Println("\n[5] When NOT to Use Release()")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Println(`
Don't use Release() when:
- You need to store resp.Body for later use
- You're passing resp.Body to another goroutine
- You need the data after the function returns

In these cases, either:
1. Don't call Release() (GC will clean up)
2. Copy the data first, then Release()`)

	// Example: storing response for later
	resp, _ = session.Get(ctx, "https://httpbin.org/bytes/1024")
	storedData := resp.Body // Keep reference - don't release!
	fmt.Printf("Stored %d bytes for later use (not releasing)\n", len(storedData))

	// =========================================================================
	// Example 6: Streaming for very large files
	// =========================================================================
	fmt.Println("\n[6] Use Streaming for Very Large Files")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Println(`
For files larger than 100MB, use streaming instead:

    resp, _ := session.GetStream(ctx, url)
    defer resp.Close()

    for {
        chunk, err := resp.ReadChunk(1024 * 1024)
        if err == io.EOF { break }
        processChunk(chunk)
    }

Streaming doesn't load the entire file into memory.`)

	// =========================================================================
	// Best Practices Summary
	// =========================================================================
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("BEST PRACTICES SUMMARY")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println(`
1. ALWAYS call resp.Release() in high-throughput scenarios
2. Process data BEFORE calling Release()
3. Copy data if you need it after Release()
4. Warm up the pool with a request of similar size
5. Use streaming (GetStream) for files > 100MB
6. Don't call Release() if storing resp.Body for later

Performance comparison (100MB downloads):
  - Without Release(): ~5000 MB/s
  - With Release():    ~9000 MB/s (1.8x faster)
`)
}
