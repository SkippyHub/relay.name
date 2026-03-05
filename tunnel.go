package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// TunnelConnection represents an active tunnel connection
type TunnelConnection struct {
	ID           string
	Subdomain    string
	Target       string
	Protocol     string // "http", "tcp", "udp", "ws"
	ClientConn   net.Conn
	TargetConn   net.Conn
	CreatedAt    time.Time
	LastActive   time.Time
	mu           sync.RWMutex
	IsActive     bool
}

// TunnelManager manages all active tunnels
type TunnelManager struct {
	tunnels     map[string]*TunnelConnection
	tunnelsLock sync.RWMutex
	server      *Server
}

func NewTunnelManager(server *Server) *TunnelManager {
	return &TunnelManager{
		tunnels: make(map[string]*TunnelConnection),
		server:  server,
	}
}

// TunnelProtocol messages
type TunnelMessage struct {
	Type      string `json:"type"`      // "register", "heartbeat", "data", "close"
	Subdomain string `json:"subdomain"` // For register
	Protocol  string `json:"protocol"`  // "http", "tcp", "udp", "ws"
	Target    string `json:"target"`   // For register: "localhost:3000"
	Data      []byte `json:"data"`     // For data messages
	ID        string `json:"id"`       // Tunnel ID
}

// HandleTunnelConnection handles a new tunnel client connection
func (tm *TunnelManager) HandleTunnelConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	
	// Read registration message
	if !scanner.Scan() {
		return
	}

	var msg TunnelMessage
	if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
		log.Printf("Tunnel: Invalid registration message: %v", err)
		return
	}

	if msg.Type != "register" {
		log.Printf("Tunnel: Expected register, got %s", msg.Type)
		return
	}

	// Generate tunnel ID
	tunnelID := generateTunnelID()
	
	// Use provided subdomain or generate one
	subdomain := msg.Subdomain
	if subdomain == "" {
		subdomain = generateSubdomain()
	}

	// Check if subdomain is available
	if tm.getTunnelBySubdomain(subdomain) != nil {
		// Try with random suffix
		subdomain = fmt.Sprintf("%s-%s", subdomain, tunnelID[:8])
	}

	// Create tunnel connection
	tunnel := &TunnelConnection{
		ID:        tunnelID,
		Subdomain: subdomain,
		Target:    msg.Target,
		Protocol:  msg.Protocol,
		ClientConn: conn,
		CreatedAt: time.Now(),
		LastActive: time.Now(),
		IsActive:  true,
	}

	// Register tunnel
	tm.tunnelsLock.Lock()
	tm.tunnels[tunnelID] = tunnel
	tm.tunnelsLock.Unlock()

	log.Printf("Tunnel registered: %s -> %s.%s -> %s", tunnelID, subdomain, *domain, msg.Target)

	// Create DNS record if provider is configured
	if tm.server.dnsProvider != nil && tm.server.serverIP != "" {
		if err := tm.server.dnsProvider.CreateRecord(subdomain, tm.server.serverIP); err != nil {
			log.Printf("Warning: Failed to create DNS record for tunnel: %v", err)
		} else {
			log.Printf("Created DNS record: %s.%s -> %s", subdomain, *domain, tm.server.serverIP)
		}
	}

	// Save to database
	tm.saveTunnelToDB(tunnel)

	// Send registration response
	response := TunnelMessage{
		Type:      "registered",
		ID:        tunnelID,
		Subdomain: subdomain,
		Protocol:  msg.Protocol,
	}
	responseJSON, _ := json.Marshal(response)
	fmt.Fprintf(conn, "%s\n", responseJSON)

	// Handle tunnel based on protocol
	switch msg.Protocol {
	case "http", "ws":
		tm.handleHTTPTunnel(tunnel, scanner, conn)
	case "tcp":
		tm.handleTCPTunnel(tunnel, scanner, conn)
	case "udp":
		tm.handleUDPTunnel(tunnel, scanner, conn)
	default:
		log.Printf("Unknown protocol: %s", msg.Protocol)
	}

	// Cleanup
	tm.tunnelsLock.Lock()
	delete(tm.tunnels, tunnelID)
	tm.tunnelsLock.Unlock()
}

func (tm *TunnelManager) handleHTTPTunnel(tunnel *TunnelConnection, scanner *bufio.Scanner, conn net.Conn) {
	// For HTTP/WS, we use a different approach
	// The server will forward HTTP requests to this tunnel
	// We need to read HTTP requests and forward them to the client
	
	// Keep connection alive and handle heartbeat
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				// Send heartbeat
				heartbeat := TunnelMessage{Type: "heartbeat", ID: tunnel.ID}
				heartbeatJSON, _ := json.Marshal(heartbeat)
				if _, err := fmt.Fprintf(conn, "%s\n", heartbeatJSON); err != nil {
					return
				}
			}
		}
	}()

	// Listen for data from client
	for scanner.Scan() {
		var msg TunnelMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}

		if msg.Type == "close" {
			break
		}

		tunnel.mu.Lock()
		tunnel.LastActive = time.Now()
		tunnel.mu.Unlock()
	}
}

func (tm *TunnelManager) handleTCPTunnel(tunnel *TunnelConnection, scanner *bufio.Scanner, conn net.Conn) {
	// For TCP, establish connection to target
	targetConn, err := net.Dial("tcp", tunnel.Target)
	if err != nil {
		log.Printf("Tunnel: Failed to connect to target %s: %v", tunnel.Target, err)
		return
	}
	defer targetConn.Close()

	tunnel.mu.Lock()
	tunnel.TargetConn = targetConn
	tunnel.mu.Unlock()

	// Bidirectional proxy
	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> Target
	go func() {
		defer wg.Done()
		io.Copy(targetConn, conn)
	}()

	// Target -> Client
	go func() {
		defer wg.Done()
		io.Copy(conn, targetConn)
	}()

	wg.Wait()
}

func (tm *TunnelManager) handleUDPTunnel(tunnel *TunnelConnection, scanner *bufio.Scanner, conn net.Conn) {
	// UDP tunneling is more complex, using TCP connection as control channel
	// Actual UDP packets would be sent separately
	log.Printf("UDP tunnel not fully implemented yet")
}

// ForwardHTTPRequest forwards an HTTP request through a tunnel
func (tm *TunnelManager) ForwardHTTPRequest(subdomain string, w http.ResponseWriter, r *http.Request) {
	tm.tunnelsLock.RLock()
	var tunnel *TunnelConnection
	for _, t := range tm.tunnels {
		if t.Subdomain == subdomain && t.IsActive {
			tunnel = t
			break
		}
	}
	tm.tunnelsLock.RUnlock()

	if tunnel == nil {
		http.Error(w, "Tunnel not found", http.StatusNotFound)
		return
	}

	// Serialize HTTP request
	reqData, err := serializeHTTPRequest(r)
	if err != nil {
		http.Error(w, "Failed to serialize request", http.StatusInternalServerError)
		return
	}

	// Send request through tunnel
	msg := TunnelMessage{
		Type: "http_request",
		ID:   tunnel.ID,
		Data: reqData,
	}
	msgJSON, _ := json.Marshal(msg)
	
	tunnel.mu.Lock()
	tunnel.LastActive = time.Now()
	conn := tunnel.ClientConn
	tunnel.mu.Unlock()

	// Send request
	if _, err := fmt.Fprintf(conn, "%s\n", msgJSON); err != nil {
		http.Error(w, "Tunnel connection lost", http.StatusBadGateway)
		return
	}

	// Read response (simplified - would need proper HTTP parsing)
	// For now, we'll use a timeout
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		var response TunnelMessage
		if err := json.Unmarshal(scanner.Bytes(), &response); err == nil && response.Type == "http_response" {
			// Parse and write HTTP response
			deserializeHTTPResponse(w, response.Data)
			return
		}
	}

	http.Error(w, "Tunnel timeout", http.StatusGatewayTimeout)
}

func (tm *TunnelManager) getTunnelBySubdomain(subdomain string) *TunnelConnection {
	tm.tunnelsLock.RLock()
	defer tm.tunnelsLock.RUnlock()
	
	for _, tunnel := range tm.tunnels {
		if tunnel.Subdomain == subdomain {
			return tunnel
		}
	}
	return nil
}

func (tm *TunnelManager) saveTunnelToDB(tunnel *TunnelConnection) {
	query := `
		INSERT INTO tunnels (id, subdomain, target, protocol, created_at, is_active)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			subdomain = excluded.subdomain,
			target = excluded.target,
			protocol = excluded.protocol,
			is_active = excluded.is_active
	`
	
	_, err := tm.server.db.Exec(query,
		tunnel.ID,
		tunnel.Subdomain,
		tunnel.Target,
		tunnel.Protocol,
		tunnel.CreatedAt,
		tunnel.IsActive,
	)
	
	if err != nil {
		log.Printf("Failed to save tunnel to database: %v", err)
	}
}

// Helper functions
func generateTunnelID() string {
	return fmt.Sprintf("tun_%d", time.Now().UnixNano())
}

func generateSubdomain() string {
	return fmt.Sprintf("tunnel-%d", time.Now().Unix()%1000000)
}

func serializeHTTPRequest(r *http.Request) ([]byte, error) {
	// Simplified serialization - in production, use proper HTTP/1.1 format
	data := map[string]interface{}{
		"method":  r.Method,
		"url":     r.URL.String(),
		"headers": r.Header,
		"body":    nil, // Would need to read body
	}
	return json.Marshal(data)
}

func deserializeHTTPResponse(w http.ResponseWriter, data []byte) {
	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return
	}
	
	// Simplified - would need proper HTTP response parsing
	if status, ok := resp["status"].(float64); ok {
		w.WriteHeader(int(status))
	}
	
	if headers, ok := resp["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				w.Header().Set(k, vs)
			}
		}
	}
	
	if body, ok := resp["body"].(string); ok {
		w.Write([]byte(body))
	}
}

