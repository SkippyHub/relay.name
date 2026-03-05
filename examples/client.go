package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
)

// Example: Create a DNS record via API
func createDNSRecord(subdomain, target, patternType, protocol string) error {
	url := "http://localhost:8080/api/dns/records"
	
	data := map[string]interface{}{
		"subdomain":   subdomain,
		"target":      target,
		"pattern_type": patternType,
		"protocol":    protocol,
		"user_id":     1,
	}

	jsonData, _ := json.Marshal(data)
	resp, err := http.Post(url, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Response: %s\n", body)
	return nil
}

// Example: TCP tunnel client
func tcpTunnelClient(subdomain, target string) error {
	conn, err := net.Dial("tcp", "localhost:8081")
	if err != nil {
		return err
	}
	defer conn.Close()

	// Send subdomain
	fmt.Fprintf(conn, "%s\n", subdomain)

	// Read response
	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		response := scanner.Text()
		if strings.HasPrefix(response, "ERROR") {
			return fmt.Errorf(response)
		}
		fmt.Printf("Connected! Response: %s\n", response)
	}

	// Now you can send/receive data
	go func() {
		io.Copy(os.Stdout, conn)
	}()

	io.Copy(conn, os.Stdin)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  create <subdomain> <target> <pattern_type> <protocol>")
		fmt.Println("  tunnel <subdomain>")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "create":
		if len(os.Args) != 6 {
			fmt.Println("Usage: create <subdomain> <target> <pattern_type> <protocol>")
			os.Exit(1)
		}
		err := createDNSRecord(os.Args[2], os.Args[3], os.Args[4], os.Args[5])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("DNS record created successfully!")

	case "tunnel":
		if len(os.Args) != 3 {
			fmt.Println("Usage: tunnel <subdomain>")
			os.Exit(1)
		}
		err := tcpTunnelClient(os.Args[2], "")
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}


