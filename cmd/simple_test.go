package main

import (
	"context"
	"fmt"
	"time"

	"github.com/sardanioss/httpcloak/client"
)

func main() {
	url := "https://www.google.com/"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := client.NewClient("chrome-143")
	defer c.Close()

	fmt.Println("Testing basic HTTP request...")
	resp, err := c.Get(ctx, url, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	body, _ := resp.Bytes()
	fmt.Printf("Status: %d, Protocol: %s, Body len: %d\n", resp.StatusCode, resp.Protocol, len(body))
	fmt.Println("Success!")
}
