package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	serverAddr = flag.String("server", "relay.name:8084", "Relay server address")
	port       = flag.Int("port", 3000, "Local port to tunnel")
	protocol   = flag.String("protocol", "http", "Protocol: http, tcp, udp, ws")
	subdomain  = flag.String("subdomain", "", "Custom subdomain (optional)")
)

type TunnelMessage struct {
	Type      string `json:"type"`
	Subdomain string `json:"subdomain"`
	Protocol  string `json:"protocol"`
	Target    string `json:"target"`
	Data      []byte `json:"data"`
	ID        string `json:"id"`
}

func main() {
	flag.Parse()

	if *port == 0 {
		log.Fatal("Port is required")
	}

	target := fmt.Sprintf("localhost:%d", *port)
	log.Printf("Connecting to %s...", *serverAddr)
	log.Printf("Tunneling %s://%s", *protocol, target)

	// Connect to tunnel server
	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send registration
	regMsg := TunnelMessage{
		Type:      "register",
		Subdomain: *subdomain,
		Protocol:  *protocol,
		Target:    target,
	}
	regJSON, _ := json.Marshal(regMsg)
	fmt.Fprintf(conn, "%s\n", regJSON)

	// Read registration response
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		log.Fatal("Failed to read registration response")
	}

	var response TunnelMessage
	if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
		log.Fatalf("Invalid response: %v", err)
	}

	if response.Type != "registered" {
		log.Fatalf("Registration failed: %s", response.Type)
	}

	log.Printf("✅ Tunnel established!")
	log.Printf("🌐 Public URL: http://%s.%s", response.Subdomain, getDomain(*serverAddr))
	log.Printf("📡 Protocol: %s", response.Protocol)
	log.Printf("🎯 Target: %s", target)
	log.Printf("")
	log.Printf("Press Ctrl+C to stop")

	// Handle based on protocol
	switch *protocol {
	case "http", "ws":
		handleHTTPTunnel(conn, scanner, target)
	case "tcp":
		handleTCPTunnel(conn, target)
	default:
		log.Fatalf("Protocol %s not supported", *protocol)
	}
}

func handleHTTPTunnel(conn net.Conn, scanner *bufio.Scanner, target string) {
	// Start local HTTP server
	localServer := &http.Server{
		Addr:    target,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Serialize request
			reqData, err := httputil.DumpRequest(r, true)
			if err != nil {
				http.Error(w, "Failed to serialize request", http.StatusInternalServerError)
				return
			}

			// Send through tunnel
			msg := TunnelMessage{
				Type: "http_request",
				Data: reqData,
			}
			msgJSON, _ := json.Marshal(msg)
			fmt.Fprintf(conn, "%s\n", msgJSON)

			// Read response (simplified - would need proper timeout handling)
			conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			if scanner.Scan() {
				var response TunnelMessage
				if err := json.Unmarshal(scanner.Bytes(), &response); err == nil {
					if response.Type == "http_response" {
						// Parse and write response
						writeHTTPResponse(w, response.Data)
						return
					}
				}
			}

			http.Error(w, "Tunnel timeout", http.StatusGatewayTimeout)
		}),
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		localServer.Close()
		conn.Close()
		os.Exit(0)
	}()

	log.Printf("Local server listening on %s", target)
	if err := localServer.ListenAndServe(); err != nil {
		log.Fatalf("Local server error: %v", err)
	}
}

func handleTCPTunnel(conn net.Conn, target string) {
	// For TCP, we proxy directly
	targetConn, err := net.Dial("tcp", target)
	if err != nil {
		log.Fatalf("Failed to connect to target: %v", err)
	}
	defer targetConn.Close()

	// Bidirectional proxy
	var done sync.WaitGroup
	done.Add(2)

	go func() {
		defer done.Done()
		io.Copy(targetConn, conn)
	}()

	go func() {
		defer done.Done()
		io.Copy(conn, targetConn)
	}()

	done.Wait()
}

func writeHTTPResponse(w http.ResponseWriter, data []byte) {
	// Simplified - would need proper HTTP response parsing
	// For now, just write the data
	w.Write(data)
}

func getDomain(addr string) string {
	parts := strings.Split(addr, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return "relay.name"
}

