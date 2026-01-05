package main

import (
	"context"
	tls "github.com/sardanioss/utls"
	"fmt"
	"net"
	"time"

	"github.com/sardanioss/quic-go"
	"github.com/sardanioss/quic-go/http3"
	utls "github.com/sardanioss/utls"
)

func main() {
	host := "quic.browserleaks.com"
	port := 443

	fmt.Println("=== Test 1: Without ClientHelloID (should work) ===")
	testWithoutClientHelloID(host, port)

	fmt.Println("\n=== Test 2: With ClientHelloID using quic.DialAddr (should work) ===")
	testDialAddrWithClientHelloID(host, port)

	fmt.Println("\n=== Test 3: With ClientHelloID using quic.Dial + custom UDP (may hang) ===")
	testDialWithClientHelloID(host, port)
}

func testWithoutClientHelloID(host string, port int) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tlsConfig := &tls.Config{
		ServerName: host,
		NextProtos: []string{http3.NextProtoH3},
		MinVersion: tls.VersionTLS13,
	}

	quicConfig := &quic.Config{
		MaxIdleTimeout:  30 * time.Second,
		KeepAlivePeriod: 30 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	fmt.Printf("Dialing %s...\n", addr)

	conn, err := quic.DialAddr(ctx, addr, tlsConfig, quicConfig)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer conn.CloseWithError(0, "done")

	fmt.Printf("Success! Remote: %s, TLS: %s\n", conn.RemoteAddr(), conn.ConnectionState().TLS.ServerName)
}

func testDialAddrWithClientHelloID(host string, port int) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tlsConfig := &tls.Config{
		ServerName: host,
		NextProtos: []string{http3.NextProtoH3},
		MinVersion: tls.VersionTLS13,
	}

	// Use QUIC-specific preset (has "h3" ALPN instead of "h2, http/1.1")
	clientHelloID := &utls.HelloChrome_143_QUIC

	quicConfig := &quic.Config{
		MaxIdleTimeout:  30 * time.Second,
		KeepAlivePeriod: 30 * time.Second,
		ClientHelloID:   clientHelloID,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	fmt.Printf("Dialing %s with ClientHelloID=%v...\n", addr, clientHelloID)

	conn, err := quic.DialAddr(ctx, addr, tlsConfig, quicConfig)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer conn.CloseWithError(0, "done")

	fmt.Printf("Success! Remote: %s, TLS: %s\n", conn.RemoteAddr(), conn.ConnectionState().TLS.ServerName)
}

func testDialWithClientHelloID(host string, port int) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Resolve the address
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		fmt.Printf("Resolve error: %v\n", err)
		return
	}

	// Create UDP connection
	udpConn, err := net.ListenUDP("udp", nil)
	if err != nil {
		fmt.Printf("ListenUDP error: %v\n", err)
		return
	}
	defer udpConn.Close()

	tlsConfig := &tls.Config{
		ServerName: host,
		NextProtos: []string{http3.NextProtoH3},
		MinVersion: tls.VersionTLS13,
	}

	// Use QUIC-specific preset (has "h3" ALPN instead of "h2, http/1.1")
	clientHelloID := &utls.HelloChrome_143_QUIC

	quicConfig := &quic.Config{
		MaxIdleTimeout:  30 * time.Second,
		KeepAlivePeriod: 30 * time.Second,
		ClientHelloID:   clientHelloID,
	}

	fmt.Printf("Dialing %s with custom UDP + ClientHelloID=%v...\n", udpAddr, clientHelloID)

	conn, err := quic.Dial(ctx, udpConn, udpAddr, tlsConfig, quicConfig)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer conn.CloseWithError(0, "done")

	fmt.Printf("Success! Remote: %s, TLS: %s\n", conn.RemoteAddr(), conn.ConnectionState().TLS.ServerName)
}
