# DNS Setup Guide for relay.name

This guide explains how to configure Namecheap to use your own DNS server for relay.name.

## Option 1: Managed DNS Service (Recommended)

### Using Cloudflare (Free & Fast)

1. **Sign up for Cloudflare** (free tier is sufficient)
   - Go to https://cloudflare.com
   - Add your domain `relay.name`

2. **Get Cloudflare Nameservers**
   - Cloudflare will provide nameservers like:
     - `alice.ns.cloudflare.com`
     - `bob.ns.cloudflare.com`

3. **Configure Namecheap**
   - Log into Namecheap
   - Go to Domain List → Manage → Advanced DNS
   - Change from "Namecheap BasicDNS" to "Custom DNS"
   - Enter Cloudflare's nameservers

4. **Set DNS Records in Cloudflare**
   ```
   Type  Name    Content           TTL
   A     @       YOUR_SERVER_IP     Auto
   A     *       YOUR_SERVER_IP     Auto
   ```

5. **Done!** Cloudflare will handle all DNS queries

### Using AWS Route 53

1. **Create Hosted Zone in Route 53**
   ```bash
   aws route53 create-hosted-zone --name relay.name --caller-reference $(date +%s)
   ```

2. **Get Nameservers**
   - Route 53 will provide 4 nameservers

3. **Configure Namecheap** (same as Cloudflare)

4. **Create Records**
   ```bash
   # Root domain
   aws route53 change-resource-record-sets --hosted-zone-id ZONE_ID --change-batch '{
     "Changes": [{
       "Action": "CREATE",
       "ResourceRecordSet": {
         "Name": "relay.name",
         "Type": "A",
         "TTL": 300,
         "ResourceRecords": [{"Value": "YOUR_SERVER_IP"}]
       }
     }]
   }'
   
   # Wildcard
   aws route53 change-resource-record-sets --hosted-zone-id ZONE_ID --change-batch '{
     "Changes": [{
       "Action": "CREATE",
       "ResourceRecordSet": {
         "Name": "*.relay.name",
         "Type": "A",
         "TTL": 300,
         "ResourceRecords": [{"Value": "YOUR_SERVER_IP"}]
       }
     }]
   }'
   ```

### Using DigitalOcean DNS

1. **Create Domain in DigitalOcean**
   - Go to Networking → Domains
   - Add `relay.name`

2. **Get Nameservers**
   - DigitalOcean provides nameservers automatically

3. **Configure Namecheap**

4. **Add Records via UI or API**
   ```bash
   curl -X POST "https://api.digitalocean.com/v2/domains/relay.name/records" \
     -H "Authorization: Bearer YOUR_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "type": "A",
       "name": "@",
       "data": "YOUR_SERVER_IP"
     }'
   
   curl -X POST "https://api.digitalocean.com/v2/domains/relay.name/records" \
     -H "Authorization: Bearer YOUR_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "type": "A",
       "name": "*",
       "data": "YOUR_SERVER_IP"
     }'
   ```

## Option 2: Self-Hosted DNS Server

### Using BIND9 (Linux)

1. **Install BIND9**
   ```bash
   sudo apt-get update
   sudo apt-get install bind9 bind9utils bind9-doc
   ```

2. **Configure BIND9**
   
   Edit `/etc/bind/named.conf.local`:
   ```bash
   zone "relay.name" {
       type master;
       file "/etc/bind/db.relay.name";
   };
   ```

3. **Create Zone File** `/etc/bind/db.relay.name`:
   ```
   $TTL    604800
   @       IN      SOA     ns1.relay.name. admin.relay.name. (
                             2024010101     ; Serial
                             604800         ; Refresh
                              86400         ; Retry
                            2419200         ; Expire
                             604800 )       ; Negative Cache TTL
   
   @       IN      NS      ns1.relay.name.
   @       IN      A       YOUR_SERVER_IP
   *       IN      A       YOUR_SERVER_IP
   ns1     IN      A       YOUR_SERVER_IP
   ```

4. **Start BIND9**
   ```bash
   sudo systemctl start bind9
   sudo systemctl enable bind9
   ```

5. **Configure Namecheap**
   - Use your server's IP as nameserver
   - Or set up secondary nameservers

### Using CoreDNS (Go-based, Lightweight)

1. **Install CoreDNS**
   ```bash
   # Download from https://coredns.io
   # Or use Docker
   docker run -d -p 53:53/udp -p 53:53/tcp \
     -v $(pwd)/Corefile:/Corefile \
     coredns/coredns:latest
   ```

2. **Create Corefile**:
   ```
   .:53 {
       errors
       health
       
       file /etc/coredns/db.relay.name {
           reload 30s
       }
       
       log
   }
   ```

3. **Create Zone File** `/etc/coredns/db.relay.name`:
   ```
   $ORIGIN relay.name.
   $TTL 300
   
   @       IN      SOA     ns1.relay.name. admin.relay.name. (
                             2024010101
                             3600
                             1800
                             604800
                             300 )
   
   @       IN      NS      ns1.relay.name.
   @       IN      A       YOUR_SERVER_IP
   *       IN      A       YOUR_SERVER_IP
   ns1     IN      A       YOUR_SERVER_IP
   ```

## Option 3: Dynamic DNS with API Integration

You can integrate DNS updates directly into your Go server using DNS provider APIs.

### Cloudflare API Integration

Add to your Go server:

```go
// dns_api.go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type CloudflareDNS struct {
    APIKey   string
    ZoneID   string
    Email    string
}

func (cf *CloudflareDNS) CreateRecord(subdomain, ip string) error {
    url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", cf.ZoneID)
    
    data := map[string]interface{}{
        "type":    "A",
        "name":    subdomain,
        "content": ip,
        "ttl":     300,
    }
    
    jsonData, _ := json.Marshal(data)
    req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
    req.Header.Set("X-Auth-Email", cf.Email)
    req.Header.Set("X-Auth-Key", cf.APIKey)
    req.Header.Set("Content-Type", "application/json")
    
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

## Namecheap Configuration Steps

1. **Log into Namecheap**
   - Go to https://www.namecheap.com
   - Sign in to your account

2. **Navigate to Domain List**
   - Click "Domain List" in the left sidebar
   - Find `relay.name` and click "Manage"

3. **Change Nameservers**
   - Scroll to "Nameservers" section
   - Select "Custom DNS" (instead of "Namecheap BasicDNS")
   - Enter your nameservers (from Cloudflare, Route 53, or your own server)
   - Click the checkmark to save

4. **Wait for Propagation**
   - DNS changes can take 24-48 hours to propagate globally
   - Use `dig relay.name` or `nslookup relay.name` to check

## Testing DNS Configuration

```bash
# Check root domain
dig relay.name
dig @8.8.8.8 relay.name

# Check wildcard
dig *.relay.name
dig @8.8.8.8 *.relay.name

# Check specific subdomain
dig alice.relay.name
dig @8.8.8.8 alice.relay.name

# Using nslookup
nslookup relay.name
nslookup *.relay.name
```

## Required DNS Records

For relay.name to work, you need:

1. **Root A Record**
   ```
   relay.name → YOUR_SERVER_IP
   ```

2. **Wildcard A Record**
   ```
   *.relay.name → YOUR_SERVER_IP
   ```

3. **Optional: Nameserver Records** (if self-hosting)
   ```
   ns1.relay.name → YOUR_SERVER_IP
   ns2.relay.name → YOUR_SERVER_IP (or secondary server)
   ```

## Quick Setup Script

Save this as `setup-dns.sh`:

```bash
#!/bin/bash

SERVER_IP="YOUR_SERVER_IP"
DOMAIN="relay.name"

echo "Setting up DNS for $DOMAIN"
echo "Server IP: $SERVER_IP"
echo ""
echo "1. Configure Namecheap to use custom nameservers"
echo "2. Add these DNS records:"
echo ""
echo "   Type: A"
echo "   Name: @"
echo "   Value: $SERVER_IP"
echo ""
echo "   Type: A"
echo "   Name: *"
echo "   Value: $SERVER_IP"
echo ""
echo "3. Wait 24-48 hours for propagation"
echo ""
echo "Test with: dig $DOMAIN"
```

## Recommended Setup

**For Production:**
- Use **Cloudflare** (free, fast, DDoS protection)
- Set wildcard A record: `*.relay.name → YOUR_SERVER_IP`
- Configure Namecheap to use Cloudflare nameservers

**For Development:**
- Use `/etc/hosts` for local testing:
  ```
  127.0.0.1 relay.name
  127.0.0.1 *.relay.name
  ```

## Troubleshooting

### DNS Not Propagating
- Wait 24-48 hours
- Clear DNS cache: `sudo dscacheutil -flushcache` (macOS)
- Use different DNS server: `dig @8.8.8.8 relay.name`

### Wildcard Not Working
- Ensure `*.relay.name` record exists
- Check TTL (use 300 seconds for faster updates)
- Verify nameservers are correct in Namecheap

### Server Not Responding
- Check firewall allows ports 80, 443, 8080-8083
- Verify server is running: `./relay-server`
- Test locally: `curl http://localhost:8080`


