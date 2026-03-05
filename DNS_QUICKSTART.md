# DNS Quick Start for Namecheap

## The Simplest Way: Cloudflare (5 minutes)

### Step 1: Sign up for Cloudflare (Free)
1. Go to https://dash.cloudflare.com/sign-up
2. Add your domain: `relay.name`
3. Cloudflare will scan your existing DNS records

### Step 2: Get Your Nameservers
Cloudflare will show you nameservers like:
```
alice.ns.cloudflare.com
bob.ns.cloudflare.com
```

### Step 3: Configure Namecheap
1. Log into Namecheap: https://www.namecheap.com
2. Go to **Domain List** → Click **Manage** next to `relay.name`
3. Scroll to **Nameservers** section
4. Change from **"Namecheap BasicDNS"** to **"Custom DNS"**
5. Enter Cloudflare's nameservers (2 nameservers)
6. Click the **checkmark** to save

### Step 4: Add DNS Records in Cloudflare
1. In Cloudflare dashboard, go to **DNS** → **Records**
2. Add these two records:

   **Record 1: Root Domain**
   - Type: `A`
   - Name: `@` (or `relay.name`)
   - IPv4 address: `YOUR_SERVER_IP`
   - Proxy status: DNS only (gray cloud)
   - TTL: Auto

   **Record 2: Wildcard**
   - Type: `A`
   - Name: `*` (asterisk)
   - IPv4 address: `YOUR_SERVER_IP`
   - Proxy status: DNS only (gray cloud)
   - TTL: Auto

### Step 5: Wait & Test
- DNS changes take 5-60 minutes to propagate
- Test with: `dig relay.name` or `nslookup relay.name`
- Or use: `./scripts/check-dns.sh relay.name YOUR_SERVER_IP`

## How It Works

**Important**: Your Go server (`relay-server`) is NOT a DNS server. Here's the flow:

1. **DNS Resolution** (handled by Cloudflare/your DNS provider):
   - User types: `alice.relay.name`
   - DNS server returns: `YOUR_SERVER_IP`

2. **HTTP Request** (handled by your Go server):
   - Browser connects to `YOUR_SERVER_IP:80` (or 443 for HTTPS)
   - HTTP request includes `Host: alice.relay.name` header
   - Your Go server reads the `Host` header
   - Extracts subdomain: `alice`
   - Looks up routing in database
   - Proxies request to configured target

3. **Routing** (handled by your Go server):
   - Server parses subdomain from `Host` header
   - Queries SQLite database for matching record
   - Routes traffic based on protocol (HTTP/WS/TCP/UDP)

## Alternative: Use Namecheap's DNS (No Custom Nameservers)

If you want to keep using Namecheap's DNS:

1. In Namecheap, go to **Domain List** → **Manage** → **Advanced DNS**
2. Add these records:

   **Record 1:**
   - Type: `A Record`
   - Host: `@`
   - Value: `YOUR_SERVER_IP`
   - TTL: Automatic

   **Record 2:**
   - Type: `A Record`
   - Host: `*`
   - Value: `YOUR_SERVER_IP`
   - TTL: Automatic

**Note**: Namecheap's DNS is fine, but Cloudflare offers:
- Faster DNS resolution
- DDoS protection
- Free SSL certificates
- Better performance globally

## Testing Your Setup

```bash
# 1. Check DNS resolution
dig relay.name
dig *.relay.name
dig test.relay.name

# 2. Check if server is running
curl http://localhost:8080/api/dns/records

# 3. Create a test DNS record
curl -X POST http://localhost:8080/api/dns/records \
  -H "Content-Type: application/json" \
  -d '{
    "subdomain": "test",
    "target": "http://httpbin.org",
    "pattern_type": "subdomain_path",
    "protocol": "http",
    "user_id": 1
  }'

# 4. Test the proxy (once DNS is configured)
curl http://test.relay.name/get
```

## Troubleshooting

### "DNS not resolving"
- Wait 5-60 minutes for propagation
- Check nameservers are correct in Namecheap
- Verify DNS records exist in Cloudflare/Namecheap
- Use `dig @8.8.8.8 relay.name` to test

### "Connection refused"
- Check server is running: `./relay-server`
- Check firewall allows ports 80, 443, 8080-8083
- Verify server IP is correct in DNS records

### "Subdomain not found"
- Check database has a record: `curl http://localhost:8080/api/dns/records`
- Verify subdomain matches exactly (case-sensitive)
- Check record is active: `is_active = 1`

## Next Steps

1. ✅ Configure DNS (Cloudflare recommended)
2. ✅ Start server: `./relay-server`
3. ✅ Create DNS records via API
4. ✅ Test with curl/browser
5. ✅ Set up SSL/TLS (Cloudflare can handle this)

## Need Help?

- See [DNS_SETUP.md](DNS_SETUP.md) for detailed instructions
- See [README.md](README.md) for full documentation
- Run `./scripts/check-dns.sh` to diagnose issues


