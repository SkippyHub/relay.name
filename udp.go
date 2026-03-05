package main

import (
	"log"
	"net"
	"strings"
	"sync"
)

// UDP Tunneling Handler
func (s *Server) handleUDPTunnel(conn *net.UDPConn) {
	buffer := make([]byte, 65507) // Max UDP packet size

	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("UDP read error: %v", err)
			continue
		}

		go func(data []byte, clientAddr *net.UDPAddr) {
			// Parse subdomain from first packet
			// Protocol: "SUBDOMAIN\n" followed by data
			lines := strings.SplitN(string(data), "\n", 2)
			if len(lines) < 2 {
				return
			}

			subdomain := strings.TrimSpace(lines[0])
			payload := []byte(lines[1])

			record, err := s.getDNSRecord(subdomain)
			if err != nil || record == nil {
				log.Printf("UDP: DNS record not found for %s", subdomain)
				return
			}

			if record.Protocol != "udp" {
				log.Printf("UDP: Protocol mismatch for %s", subdomain)
				return
			}

			// Parse target (host:port)
			targetAddr, err := net.ResolveUDPAddr("udp", record.Target)
			if err != nil {
				log.Printf("UDP: Failed to resolve target: %v", err)
				return
			}

			// Create connection to target
			targetConn, err := net.DialUDP("udp", nil, targetAddr)
			if err != nil {
				log.Printf("UDP: Failed to connect to target: %v", err)
				return
			}
			defer targetConn.Close()

			// Send payload to target
			if _, err := targetConn.Write(payload); err != nil {
				log.Printf("UDP: Failed to write to target: %v", err)
				return
			}

			// Read response from target
			response := make([]byte, 65507)
			n, err := targetConn.Read(response)
			if err != nil {
				log.Printf("UDP: Failed to read from target: %v", err)
				return
			}

			// Send response back to client
			if _, err := conn.WriteToUDP(response[:n], clientAddr); err != nil {
				log.Printf("UDP: Failed to write to client: %v", err)
				return
			}
		}(buffer[:n], addr)
	}
}

// UDP connection pool for better performance
type UDPPool struct {
	connections map[string]*net.UDPConn
	mu          sync.RWMutex
}

func NewUDPPool() *UDPPool {
	return &UDPPool{
		connections: make(map[string]*net.UDPConn),
	}
}

func (p *UDPPool) Get(target string) (*net.UDPConn, error) {
	p.mu.RLock()
	if conn, ok := p.connections[target]; ok {
		p.mu.RUnlock()
		return conn, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double check
	if conn, ok := p.connections[target]; ok {
		return conn, nil
	}

	addr, err := net.ResolveUDPAddr("udp", target)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}

	p.connections[target] = conn
	return conn, nil
}

func (p *UDPPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, conn := range p.connections {
		conn.Close()
	}
	p.connections = make(map[string]*net.UDPConn)
}


