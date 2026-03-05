package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
)

var (
	httpPort   = flag.Int("http", 8080, "HTTP server port")
	tcpPort    = flag.Int("tcp", 8081, "TCP tunneling port")
	udpPort    = flag.Int("udp", 8082, "UDP tunneling port")
	wsPort     = flag.Int("ws", 8083, "WebSocket server port")
	tunnelPort = flag.Int("tunnel", 8084, "Tunnel server port (for client connections)")
	dbPath     = flag.String("db", "./relay.db", "Database path")
	domain     = flag.String("domain", "relay.name", "Base domain")
	tlsCert    = flag.String("tls-cert", "", "TLS certificate path")
	tlsKey     = flag.String("tls-key", "", "TLS key path")

	// DNS Provider settings
	dnsProvider = flag.String("dns-provider", "", "DNS provider: cloudflare, digitalocean, or empty (manual)")
	dnsAPIKey   = flag.String("dns-api-key", "", "DNS provider API key/token")
	dnsZoneID   = flag.String("dns-zone-id", "", "DNS provider zone ID (Cloudflare) or domain (DigitalOcean)")
	dnsEmail    = flag.String("dns-email", "", "DNS provider email (Cloudflare)")
	serverIP    = flag.String("server-ip", "", "Your server IP address (for DNS records)")
)

type Server struct {
	db          *sql.DB
	tunnels     map[string]*Tunnel
	tunnelsLock sync.RWMutex
	upgrader    websocket.Upgrader
	dnsProvider DNSProvider
	serverIP    string
	tunnelMgr   *TunnelManager
}

type Tunnel struct {
	ID          string
	Subdomain   string
	Target      string
	Protocol    string // "tcp", "udp", "http", "ws"
	Connections map[string]net.Conn
	mu          sync.RWMutex
	CreatedAt   time.Time
	LastActive  time.Time
}

type DNSRecord struct {
	ID          int        `json:"id"`
	Subdomain   string     `json:"subdomain"`
	Target      string     `json:"target"`
	PatternType string     `json:"pattern_type"` // "subdomain_path", "multi_word", "hybrid"
	Protocol    string     `json:"protocol"`     // "http", "tcp", "udp", "ws"
	UserID      int        `json:"user_id"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
}

func NewServer() (*Server, error) {
	db, err := sql.Open("sqlite3", *dbPath+"?_journal_mode=WAL&_foreign_keys=1")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := initDB(db); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	s := &Server{
		db:       db,
		tunnels:  make(map[string]*Tunnel),
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		serverIP: *serverIP,
	}

	// Initialize tunnel manager
	s.tunnelMgr = NewTunnelManager(s)

	// Initialize DNS provider if configured
	if *dnsProvider != "" && *dnsAPIKey != "" && *serverIP != "" {
		switch *dnsProvider {
		case "cloudflare":
			if *dnsZoneID == "" || *dnsEmail == "" {
				return nil, fmt.Errorf("cloudflare requires --dns-zone-id and --dns-email")
			}
			s.dnsProvider = NewCloudflareDNS(*dnsAPIKey, *dnsZoneID, *dnsEmail, *domain)
			log.Printf("DNS Provider: Cloudflare configured for %s", *domain)
		case "digitalocean":
			if *dnsZoneID == "" {
				return nil, fmt.Errorf("digitalocean requires --dns-zone-id (domain name)")
			}
			s.dnsProvider = NewDigitalOceanDNS(*dnsAPIKey, *dnsZoneID)
			log.Printf("DNS Provider: DigitalOcean configured for %s", *dnsZoneID)
		default:
			log.Printf("Warning: Unknown DNS provider '%s', DNS management disabled", *dnsProvider)
		}
	} else {
		log.Printf("DNS Provider: Not configured (manual DNS management)")
	}

	return s, nil
}

func initDB(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email VARCHAR(255) UNIQUE,
		wallet_address VARCHAR(255),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS dns_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		subdomain VARCHAR(255) NOT NULL UNIQUE,
		target VARCHAR(500) NOT NULL,
		pattern_type VARCHAR(50) NOT NULL,
		protocol VARCHAR(20) NOT NULL DEFAULT 'http',
		user_id INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME,
		is_active BOOLEAN DEFAULT 1,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS subscriptions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		dns_record_id INTEGER NOT NULL,
		status VARCHAR(50) DEFAULT 'active',
		billing_cycle_start DATE,
		billing_cycle_end DATE,
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (dns_record_id) REFERENCES dns_records(id)
	);

	CREATE INDEX IF NOT EXISTS idx_subdomain ON dns_records(subdomain);
	CREATE INDEX IF NOT EXISTS idx_user_id ON dns_records(user_id);

	CREATE TABLE IF NOT EXISTS tunnels (
		id VARCHAR(255) PRIMARY KEY,
		subdomain VARCHAR(255) NOT NULL UNIQUE,
		target VARCHAR(500) NOT NULL,
		protocol VARCHAR(20) NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		is_active BOOLEAN DEFAULT 1
	);

	CREATE INDEX IF NOT EXISTS idx_tunnel_subdomain ON tunnels(subdomain);
	`

	_, err := db.Exec(schema)
	return err
}

func (s *Server) getDNSRecord(subdomain string) (*DNSRecord, error) {
	query := `
		SELECT id, subdomain, target, pattern_type, protocol, user_id, is_active, created_at, expires_at
		FROM dns_records
		WHERE subdomain = ? AND is_active = 1
		AND (expires_at IS NULL OR expires_at > datetime('now'))
	`

	var record DNSRecord
	var expiresAt sql.NullTime

	err := s.db.QueryRow(query, subdomain).Scan(
		&record.ID, &record.Subdomain, &record.Target, &record.PatternType,
		&record.Protocol, &record.UserID, &record.IsActive, &record.CreatedAt, &expiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if expiresAt.Valid {
		record.ExpiresAt = &expiresAt.Time
	}

	return &record, nil
}

func (s *Server) parseSubdomain(hostname string) (string, []string) {
	// Remove port if present
	host := strings.Split(hostname, ":")[0]

	// Remove base domain
	baseDomain := "." + *domain
	if !strings.HasSuffix(host, baseDomain) {
		return "", nil
	}

	subdomain := strings.TrimSuffix(host, baseDomain)
	if subdomain == "" {
		return "", nil
	}

	// Split into parts
	parts := strings.Split(subdomain, ".")
	return subdomain, parts
}

// HTTP Proxy Handler
func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	hostname := r.Host
	subdomain, parts := s.parseSubdomain(hostname)

	if subdomain == "" {
		http.Error(w, "Invalid subdomain", http.StatusBadRequest)
		return
	}

	// First check if it's a tunnel
	tunnel := s.tunnelMgr.getTunnelBySubdomain(subdomain)
	if tunnel != nil && tunnel.IsActive {
		// Forward through tunnel
		s.tunnelMgr.ForwardHTTPRequest(subdomain, w, r)
		return
	}

	// Otherwise, check DNS records
	record, err := s.getDNSRecord(subdomain)
	if err != nil {
		log.Printf("Error getting DNS record: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if record == nil {
		http.Error(w, "DNS record not found", http.StatusNotFound)
		return
	}

	// Pattern 1: subdomain_path - [name1].relay.name/[name2]/[name3]
	// Pattern 2: multi_word - word1.word2.word3.relay.name
	// Pattern 3: hybrid - [name1].relay.name/[name2]

	targetURL := record.Target
	path := r.URL.Path

	// For subdomain_path and hybrid, append path to target
	if record.PatternType == "subdomain_path" || record.PatternType == "hybrid" {
		if !strings.HasSuffix(targetURL, "/") && path != "/" {
			targetURL += path
		} else if path != "/" {
			targetURL += strings.TrimPrefix(path, "/")
		}
	}

	// For multi_word, use target as-is (no path routing)
	// The subdomain itself is the identifier

	log.Printf("Proxying %s -> %s (pattern: %s, path: %s, parts: %v)",
		subdomain, targetURL, record.PatternType, path, parts)

	// Create reverse proxy
	proxy := &httpProxy{
		target: targetURL,
	}

	proxy.ServeHTTP(w, r)
}

// WebSocket Handler for Games
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	hostname := r.Host
	subdomain, _ := s.parseSubdomain(hostname)

	if subdomain == "" {
		http.Error(w, "Invalid subdomain", http.StatusBadRequest)
		return
	}

	record, err := s.getDNSRecord(subdomain)
	if err != nil || record == nil {
		http.Error(w, "DNS record not found", http.StatusNotFound)
		return
	}

	if record.Protocol != "ws" && record.Protocol != "http" {
		http.Error(w, "Protocol not supported for WebSocket", http.StatusBadRequest)
		return
	}

	// Upgrade to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Connect to target WebSocket
	targetURL := strings.Replace(record.Target, "http://", "ws://", 1)
	targetURL = strings.Replace(targetURL, "https://", "wss://", 1)

	targetConn, _, err := websocket.DefaultDialer.Dial(targetURL, nil)
	if err != nil {
		log.Printf("Failed to connect to target: %v", err)
		return
	}
	defer targetConn.Close()

	// Bidirectional proxy
	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> Target
	go func() {
		defer wg.Done()
		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := targetConn.WriteMessage(messageType, data); err != nil {
				return
			}
		}
	}()

	// Target -> Client
	go func() {
		defer wg.Done()
		for {
			messageType, data, err := targetConn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(messageType, data); err != nil {
				return
			}
		}
	}()

	wg.Wait()
}

// TCP Tunneling Handler
func (s *Server) handleTCPTunnel(listener net.Listener) {
	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("TCP accept error: %v", err)
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()

			// Read subdomain from first packet (custom protocol)
			buffer := make([]byte, 1024)
			n, err := conn.Read(buffer)
			if err != nil {
				return
			}

			// Simple protocol: "SUBDOMAIN\n"
			lines := strings.Split(string(buffer[:n]), "\n")
			if len(lines) == 0 {
				return
			}

			subdomain := strings.TrimSpace(lines[0])
			record, err := s.getDNSRecord(subdomain)
			if err != nil || record == nil {
				conn.Write([]byte("ERROR: DNS record not found\n"))
				return
			}

			if record.Protocol != "tcp" {
				conn.Write([]byte("ERROR: Protocol mismatch\n"))
				return
			}

			// Parse target (host:port)
			targetConn, err := net.Dial("tcp", record.Target)
			if err != nil {
				conn.Write([]byte("ERROR: Failed to connect to target\n"))
				return
			}
			defer targetConn.Close()

			// Proxy remaining data
			if n > len(lines[0])+1 {
				targetConn.Write(buffer[len(lines[0])+1 : n])
			}

			// Bidirectional proxy
			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				ioCopy(targetConn, conn)
			}()

			go func() {
				defer wg.Done()
				ioCopy(conn, targetConn)
			}()

			wg.Wait()
		}(clientConn)
	}
}

func ioCopy(dst, src net.Conn) {
	buffer := make([]byte, 32*1024)
	for {
		n, err := src.Read(buffer)
		if n > 0 {
			if _, err := dst.Write(buffer[:n]); err != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// API Handlers
func (s *Server) handleCreateDNSRecord(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Subdomain   string `json:"subdomain"`
		Target      string `json:"target"`
		PatternType string `json:"pattern_type"`
		Protocol    string `json:"protocol"`
		UserID      int    `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Protocol == "" {
		req.Protocol = "http"
	}

	query := `
		INSERT INTO dns_records (subdomain, target, pattern_type, protocol, user_id)
		VALUES (?, ?, ?, ?, ?)
	`

	result, err := s.db.Exec(query, req.Subdomain, req.Target, req.PatternType, req.Protocol, req.UserID)
	if err != nil {
		log.Printf("Error creating DNS record: %v", err)
		http.Error(w, "Failed to create DNS record", http.StatusInternalServerError)
		return
	}

	id, _ := result.LastInsertId()

	// Create actual DNS record if provider is configured
	if s.dnsProvider != nil && s.serverIP != "" {
		fullSubdomain := req.Subdomain
		if err := s.dnsProvider.CreateRecord(fullSubdomain, s.serverIP); err != nil {
			log.Printf("Warning: Failed to create DNS record in provider: %v", err)
			// Don't fail the request, DNS might be managed manually
		} else {
			log.Printf("Created DNS record: %s.%s -> %s", fullSubdomain, *domain, s.serverIP)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      id,
		"message": "DNS record created",
	})
}

func (s *Server) handleListDNSRecords(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	query := `
		SELECT id, subdomain, target, pattern_type, protocol, user_id, is_active, created_at, expires_at
		FROM dns_records
		WHERE (? = '' OR user_id = ?)
		ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query, userID, userID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var records []DNSRecord
	for rows.Next() {
		var record DNSRecord
		var expiresAt sql.NullTime

		err := rows.Scan(
			&record.ID, &record.Subdomain, &record.Target, &record.PatternType,
			&record.Protocol, &record.UserID, &record.IsActive, &record.CreatedAt, &expiresAt,
		)
		if err != nil {
			continue
		}

		if expiresAt.Valid {
			record.ExpiresAt = &expiresAt.Time
		}

		records = append(records, record)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(records)
}

func (s *Server) handleUpdateDNSRecord(w http.ResponseWriter, r *http.Request, idStr string) {
	var req struct {
		Target      string `json:"target"`
		PatternType string `json:"pattern_type"`
		Protocol    string `json:"protocol"`
		IsActive    *bool  `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Build update query dynamically
	updates := []string{}
	args := []interface{}{}

	if req.Target != "" {
		updates = append(updates, "target = ?")
		args = append(args, req.Target)
	}
	if req.PatternType != "" {
		updates = append(updates, "pattern_type = ?")
		args = append(args, req.PatternType)
	}
	if req.Protocol != "" {
		updates = append(updates, "protocol = ?")
		args = append(args, req.Protocol)
	}
	if req.IsActive != nil {
		updates = append(updates, "is_active = ?")
		args = append(args, *req.IsActive)
	}

	if len(updates) == 0 {
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	args = append(args, idStr)
	query := fmt.Sprintf("UPDATE dns_records SET %s WHERE id = ?", strings.Join(updates, ", "))

	result, err := s.db.Exec(query, args...)
	if err != nil {
		log.Printf("Error updating DNS record: %v", err)
		http.Error(w, "Failed to update DNS record", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "DNS record not found", http.StatusNotFound)
		return
	}

	// Update DNS record if provider is configured
	if s.dnsProvider != nil && s.serverIP != "" {
		// Get subdomain from database to update DNS
		var subdomain string
		s.db.QueryRow("SELECT subdomain FROM dns_records WHERE id = ?", idStr).Scan(&subdomain)
		if subdomain != "" {
			if err := s.dnsProvider.UpdateRecord(subdomain, s.serverIP); err != nil {
				log.Printf("Warning: Failed to update DNS record in provider: %v", err)
			} else {
				log.Printf("Updated DNS record: %s.%s -> %s", subdomain, *domain, s.serverIP)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "DNS record updated",
	})
}

func (s *Server) handleDeleteDNSRecord(w http.ResponseWriter, r *http.Request, idStr string) {
	// Soft delete by setting is_active = 0
	query := "UPDATE dns_records SET is_active = 0 WHERE id = ?"
	result, err := s.db.Exec(query, idStr)
	if err != nil {
		log.Printf("Error deleting DNS record: %v", err)
		http.Error(w, "Failed to delete DNS record", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "DNS record not found", http.StatusNotFound)
		return
	}

	// Delete DNS record if provider is configured
	if s.dnsProvider != nil {
		// Get subdomain from database before deletion
		var subdomain string
		s.db.QueryRow("SELECT subdomain FROM dns_records WHERE id = ?", idStr).Scan(&subdomain)
		if subdomain != "" {
			if err := s.dnsProvider.DeleteRecord(subdomain); err != nil {
				log.Printf("Warning: Failed to delete DNS record in provider: %v", err)
			} else {
				log.Printf("Deleted DNS record: %s.%s", subdomain, *domain)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "DNS record deleted",
	})
}

func main() {
	flag.Parse()

	server, err := NewServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer server.db.Close()

	// HTTP Server (for HTTP proxy and API)
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/", server.handleHTTP)
	httpMux.HandleFunc("/api/dns/records", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			server.handleCreateDNSRecord(w, r)
		case http.MethodGet:
			server.handleListDNSRecords(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	httpMux.HandleFunc("/api/dns/records/", func(w http.ResponseWriter, r *http.Request) {
		// Extract ID from path
		path := strings.TrimPrefix(r.URL.Path, "/api/dns/records/")
		parts := strings.Split(path, "/")
		if len(parts) == 0 || parts[0] == "" {
			http.Error(w, "Invalid record ID", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodPut:
			server.handleUpdateDNSRecord(w, r, parts[0])
		case http.MethodDelete:
			server.handleDeleteDNSRecord(w, r, parts[0])
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// WebSocket Server (for games)
	wsMux := http.NewServeMux()
	wsMux.HandleFunc("/", server.handleWebSocket)

	// Start servers
	var wg sync.WaitGroup

	// HTTP Server
	wg.Add(1)
	go func() {
		defer wg.Done()
		addr := fmt.Sprintf(":%d", *httpPort)
		log.Printf("HTTP server listening on %s", addr)
		if err := http.ListenAndServe(addr, httpMux); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// WebSocket Server
	wg.Add(1)
	go func() {
		defer wg.Done()
		addr := fmt.Sprintf(":%d", *wsPort)
		log.Printf("WebSocket server listening on %s", addr)
		if err := http.ListenAndServe(addr, wsMux); err != nil {
			log.Fatalf("WebSocket server error: %v", err)
		}
	}()

	// TCP Tunneling Server
	wg.Add(1)
	go func() {
		defer wg.Done()
		addr := fmt.Sprintf(":%d", *tcpPort)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("TCP server error: %v", err)
		}
		log.Printf("TCP tunneling server listening on %s", addr)
		server.handleTCPTunnel(listener)
	}()

	// UDP Tunneling Server
	wg.Add(1)
	go func() {
		defer wg.Done()
		addr := fmt.Sprintf(":%d", *udpPort)
		udpAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			log.Fatalf("UDP server error: %v", err)
		}
		conn, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			log.Fatalf("UDP server error: %v", err)
		}
		defer conn.Close()
		log.Printf("UDP tunneling server listening on %s", addr)
		server.handleUDPTunnel(conn)
	}()

	// Tunnel Server (for client connections)
	wg.Add(1)
	go func() {
		defer wg.Done()
		addr := fmt.Sprintf(":%d", *tunnelPort)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("Tunnel server error: %v", err)
		}
		defer listener.Close()
		log.Printf("Tunnel server listening on %s", addr)
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Tunnel accept error: %v", err)
				continue
			}
			go server.tunnelMgr.HandleTunnelConnection(conn)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// TODO: Implement graceful shutdown for all servers
	_ = ctx

	log.Println("Server stopped")
}
