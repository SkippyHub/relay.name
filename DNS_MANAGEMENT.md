# Automatic DNS Management

**Yes! You can manage DNS records programmatically through your Go server.**

When users create DNS records via your API, the server can automatically create/update/delete actual DNS records in Cloudflare, DigitalOcean, or other providers.

## How It Works

1. **User creates DNS record via API**
   ```bash
   POST /api/dns/records
   { "subdomain": "alice", "target": "http://localhost:3000", ... }
   ```

2. **Server saves to database** (SQLite)

3. **Server automatically creates DNS record** in Cloudflare/DigitalOcean
   - Creates: `alice.relay.name` → `YOUR_SERVER_IP`

4. **DNS propagates** (5-60 minutes)

5. **Traffic flows**: `alice.relay.name` → DNS resolves → Your server → Routes to target

## Setup

### Cloudflare (Recommended - Free)

1. **Get Cloudflare API credentials:**
   - Go to https://dash.cloudflare.com/profile/api-tokens
   - Create API token with "Zone:Edit" permissions
   - Or use Global API Key (less secure)

2. **Get Zone ID:**
   - In Cloudflare dashboard, select your domain
   - Zone ID is shown in the right sidebar

3. **Run server with DNS management:**
   ```bash
   ./relay-server \
     -dns-provider=cloudflare \
     -dns-api-key=YOUR_API_KEY \
     -dns-zone-id=YOUR_ZONE_ID \
     -dns-email=your@email.com \
     -server-ip=YOUR_SERVER_IP
   ```

### DigitalOcean

1. **Get API token:**
   - Go to https://cloud.digitalocean.com/account/api/tokens
   - Generate new token

2. **Run server:**
   ```bash
   ./relay-server \
     -dns-provider=digitalocean \
     -dns-api-key=YOUR_TOKEN \
     -dns-zone-id=relay.name \
     -server-ip=YOUR_SERVER_IP
   ```

## Command Line Options

```
-dns-provider      DNS provider: "cloudflare" or "digitalocean" (empty = manual)
-dns-api-key       API key/token from DNS provider
-dns-zone-id       Zone ID (Cloudflare) or domain name (DigitalOcean)
-dns-email         Email (Cloudflare only)
-server-ip         Your server's public IP address
```

## Example Workflow

```bash
# 1. Start server with DNS management
./relay-server \
  -dns-provider=cloudflare \
  -dns-api-key=abc123... \
  -dns-zone-id=xyz789... \
  -dns-email=admin@relay.name \
  -server-ip=1.2.3.4

# 2. User creates DNS record via API
curl -X POST http://localhost:8080/api/dns/records \
  -H "Content-Type: application/json" \
  -d '{
    "subdomain": "mygame",
    "target": "ws://localhost:8080",
    "pattern_type": "multi_word",
    "protocol": "ws",
    "user_id": 1
  }'

# 3. Server automatically:
#    - Saves to database
#    - Creates DNS record: mygame.relay.name → 1.2.3.4
#    - Logs: "Created DNS record: mygame.relay.name -> 1.2.3.4"

# 4. After DNS propagation (5-60 min):
#    - mygame.relay.name resolves to 1.2.3.4
#    - Traffic flows: mygame.relay.name → Your server → Routes to target
```

## Supported Providers

### ✅ Cloudflare
- Free tier available
- Fast DNS resolution
- DDoS protection
- API: Full CRUD support

### ✅ DigitalOcean
- Simple API
- Good for DO infrastructure
- API: Full CRUD support

### 🔜 Coming Soon
- AWS Route 53
- Google Cloud DNS
- Namecheap API (if available)

## Manual Mode (No DNS Provider)

If you don't set `-dns-provider`, the server works in **manual mode**:
- ✅ Still saves DNS records to database
- ✅ Still routes traffic based on subdomain
- ❌ Does NOT create actual DNS records
- You must manually create DNS records in your DNS provider

**Use manual mode if:**
- You want to manage DNS records yourself
- You're using a DNS provider without API access
- You're testing locally

## How DNS Records Are Created

### For Pattern 1: `[name1].relay.name/[path]`
- Creates: `name1.relay.name` → `YOUR_SERVER_IP`
- All paths under `name1` route through your server

### For Pattern 2: `word1.word2.word3.relay.name`
- Creates: `word1.word2.word3.relay.name` → `YOUR_SERVER_IP`
- Full subdomain is the identifier

### For Pattern 3: `[name1].relay.name/[name2]`
- Creates: `name1.relay.name` → `YOUR_SERVER_IP`
- Path routing handled by server

## Important Notes

1. **Wildcard DNS**: You still need a wildcard DNS record (`*.relay.name`) pointing to your server IP. This is created once manually or via API.

2. **DNS Propagation**: New DNS records take 5-60 minutes to propagate globally.

3. **TTL**: DNS records are created with 300 second TTL for fast updates.

4. **Errors**: If DNS provider API fails, the server logs a warning but doesn't fail the request. This allows manual DNS management as fallback.

## Testing

```bash
# Check if DNS record was created
dig mygame.relay.name

# Should return: YOUR_SERVER_IP

# Test the proxy
curl http://mygame.relay.name
```

## Troubleshooting

### "DNS record not created"
- Check API credentials are correct
- Verify Zone ID/domain name is correct
- Check server logs for API errors
- Ensure server IP is correct

### "DNS not resolving"
- Wait 5-60 minutes for propagation
- Check DNS record exists in provider dashboard
- Verify wildcard DNS record exists

### "API rate limits"
- Cloudflare: 1200 requests per 5 minutes
- DigitalOcean: 5000 requests per hour
- Server handles rate limits gracefully

## Security

- Store API keys in environment variables:
  ```bash
  export CLOUDFLARE_API_KEY=your_key
  ./relay-server -dns-api-key=$CLOUDFLARE_API_KEY ...
  ```

- Use API tokens (not global API keys) when possible
- Restrict API token permissions to minimum required

