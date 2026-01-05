package main

import (
	"fmt"
	"io"
	"net"
	"time"

	tls "github.com/sardanioss/utls"
	"github.com/sardanioss/httpcloak/fingerprint"
)

func main() {
	fmt.Println("Testing TLS Session Resumption Fingerprinting")
	fmt.Println("==============================================")
	fmt.Println()

	// Use Chrome 143 preset which has PSK support
	preset := fingerprint.Chrome143Linux()
	fmt.Printf("Using preset: %s\n", preset.Name)
	fmt.Printf("ClientHelloID: %s (%s)\n", preset.ClientHelloID.Client, preset.ClientHelloID.Version)
	fmt.Printf("PSKClientHelloID: %s (%s)\n", preset.PSKClientHelloID.Client, preset.PSKClientHelloID.Version)
	fmt.Println()

	host := "www.microsoft.com"
	serverAddr := host + ":443"

	// Create a session cache that persists across connections
	sessionCache := tls.NewLRUClientSessionCache(32)

	// First connection - should use regular ClientHello (no PSK)
	fmt.Println("=== First Connection (No Session) ===")
	tlsConfig := &tls.Config{
		ServerName:             host,
		ClientSessionCache:     sessionCache,
		OmitEmptyPsk:           true,
		SessionTicketsDisabled: false,
	}

	tcpConn, err := net.DialTimeout("tcp", serverAddr, 10*time.Second)
	if err != nil {
		fmt.Printf("TCP connect failed: %v\n", err)
		return
	}

	// First connection uses non-PSK ClientHello
	tlsConn := tls.UClient(tcpConn, tlsConfig, preset.ClientHelloID)
	tlsConn.SetSessionCache(sessionCache)

	err = tlsConn.Handshake()
	if err != nil {
		fmt.Printf("TLS handshake failed: %v\n", err)
		return
	}

	fmt.Printf("TLS Version: 0x%04x\n", tlsConn.ConnectionState().Version)
	fmt.Printf("Cipher Suite: 0x%04x\n", tlsConn.ConnectionState().CipherSuite)
	fmt.Printf("Used PSK: %v\n", tlsConn.HandshakeState.State13.UsingPSK)
	fmt.Println()

	// Trigger a read to receive NewSessionTicket
	tlsConn.SetReadDeadline(time.Now().Add(1 * time.Second))
	tlsConn.Read(make([]byte, 1024))
	tlsConn.Close()

	// Check if session was cached
	if session, ok := sessionCache.Get(host); ok && session != nil {
		fmt.Println("Session ticket was cached successfully!")
	} else {
		fmt.Println("No session ticket was cached.")
	}
	fmt.Println()

	// Small delay
	time.Sleep(500 * time.Millisecond)

	// Second connection - should use PSK ClientHello (session resumption)
	fmt.Println("=== Second Connection (With Cached Session) ===")

	tcpConn2, err := net.DialTimeout("tcp", serverAddr, 10*time.Second)
	if err != nil {
		fmt.Printf("TCP connect failed: %v\n", err)
		return
	}

	// Check if we should use PSK variant
	clientHelloID := preset.ClientHelloID
	if session, ok := sessionCache.Get(host); ok && session != nil {
		fmt.Println("Found cached session - using PSK ClientHello variant")
		clientHelloID = preset.PSKClientHelloID
	}

	tlsConfig2 := &tls.Config{
		ServerName:             host,
		ClientSessionCache:     sessionCache,
		OmitEmptyPsk:           true,
		SessionTicketsDisabled: false,
	}

	tlsConn2 := tls.UClient(tcpConn2, tlsConfig2, clientHelloID)
	tlsConn2.SetSessionCache(sessionCache)

	err = tlsConn2.Handshake()
	if err != nil {
		fmt.Printf("TLS handshake failed: %v\n", err)
		return
	}

	fmt.Printf("TLS Version: 0x%04x\n", tlsConn2.ConnectionState().Version)
	fmt.Printf("Cipher Suite: 0x%04x\n", tlsConn2.ConnectionState().CipherSuite)
	fmt.Printf("Used PSK: %v\n", tlsConn2.HandshakeState.State13.UsingPSK)
	fmt.Println()

	// Make an HTTP request
	fmt.Println("=== Making HTTP/1.1 Request ===")
	req := fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nUser-Agent: %s\r\nConnection: close\r\n\r\n", host, preset.UserAgent)
	tlsConn2.Write([]byte(req))

	resp, _ := io.ReadAll(tlsConn2)
	if len(resp) > 200 {
		fmt.Printf("Response (first 200 bytes): %s...\n", string(resp[:200]))
	} else {
		fmt.Printf("Response: %s\n", string(resp))
	}
	fmt.Println()

	tlsConn2.Close()

	// Summary
	fmt.Println("=== Session Resumption Summary ===")
	if tlsConn2.HandshakeState.State13.UsingPSK {
		fmt.Println("SUCCESS: Session resumption (PSK) was used!")
		fmt.Println("The TLS fingerprint shows pre_shared_key extension was sent and accepted.")
	} else {
		fmt.Println("INFO: Session resumption was NOT used.")
		fmt.Println("The ClientHello included pre_shared_key extension, but server didn't accept it.")
		fmt.Println("This is normal behavior - server decides whether to use PSK resumption.")
	}
}
