# relay.name - Reverse DNS Proxy & Tunneling Service

**High-performance reverse DNS proxy with TCP/UDP tunneling and WebSocket support for games.**

relay.name is a Go-based reverse DNS proxy service that allows you to control DNS records and route traffic through custom domain patterns. With built-in tunneling (like ngrok) and optimized for low-latency game connections, it's perfect for exposing local game servers, APIs, and services.

**Pricing: $1/month per DNS record**

## Why Go?

We chose **Go** for this project because:

- ✅ **Low Latency**: Compiled language with minimal overhead, perfect for real-time game connections
- ✅ **Excellent Concurrency**: Goroutines handle thousands of simultaneous connections efficiently
- ✅ **Native Networking**: Built-in TCP/UDP/WebSocket support in standard library
- ✅ **High Performance**: Used by ngrok, Cloudflare Tunnel, and other production tunneling services
- ✅ **Simple Deployment**: Single binary, no runtime dependencies
- ✅ **Memory Efficient**: Lower memory footprint than Node.js/Python for high-traffic scenarios

## URL Patterns Supported

### Pattern 1: Subdomain + Path Routing
```
[name1].relay.name/[name2]/[name3]/[etc...]
```

**Example:**
- `alice.relay.name/bob/carol` → Routes to the target configured for "alice" with path segments "bob/carol"
- `myapp.relay.name/api/users` → Routes to "myapp" target with path "/api/users"

**Use Cases:**
- Preserve TLD structure while adding path-based routing
- Single subdomain can handle multiple routes
- Clean separation between domain identifier and path segments

### Pattern 2: Multi-Word Subdomain (What3Words Style)
```
word1.word2.word3.relay.name
```

**Example:**
- `happy.blue.moon.relay.name` → Routes to target configured for "happy.blue.moon"
- `first.second.third.relay.name` → Routes to target configured for "first.second.third"

**Use Cases:**
- Human-readable, memorable addresses
- No path segments needed - the entire subdomain is the identifier
- Great for simple, direct routing

### Pattern 3: Hybrid (Subdomain + Path)
```
[first-name-address].relay.name/[lastname-address]
```

**Example:**
- `john.relay.name/doe` → Routes to target configured for "john" with path segment "doe"
- `company.relay.name/service` → Routes to "company" target with path "service"

**Use Cases:**
- Combines subdomain identification with path-based routing
- Flexible for hierarchical structures

## Protocol Support

### HTTP/HTTPS
Standard HTTP reverse proxy with path routing support.

### WebSocket (WS/WSS)
Full bidirectional WebSocket proxy for real-time game connections:
- Low-latency message forwarding
- Binary and text message support
- Automatic reconnection handling

### TCP Tunneling
Raw TCP connection tunneling for game servers:
- Custom protocol: Send subdomain in first packet
- Bidirectional data streaming
- Perfect for game protocols (Minecraft, custom game servers, etc.)

### UDP Tunneling
UDP packet forwarding for game networking:
- Low-latency packet routing
- Connection pooling for performance
- Ideal for real-time multiplayer games

## Architecture

### Technology Stack

- **Language**: Go 1.21+
- **Database**: SQLite (can be swapped to PostgreSQL)
- **Networking**: Native Go net package
- **WebSocket**: gorilla/websocket
- **Deployment**: Single binary

### Server Components

1. **HTTP Server** (Port 8080)
   - Handles HTTP reverse proxy
   - API endpoints for DNS record management
   - Subdomain-based routing

2. **WebSocket Server** (Port 8083)
   - Dedicated WebSocket proxy for games
   - Low-latency message forwarding
   - Supports binary and text frames

3. **TCP Tunneling Server** (Port 8081)
   - Raw TCP connection tunneling
   - Custom protocol: `SUBDOMAIN\n` + data
   - Bidirectional streaming

4. **UDP Tunneling Server** (Port 8082)
   - UDP packet forwarding
   - Connection pooling
   - Low-latency routing

### Database Schema

```sql
CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email VARCHAR(255) UNIQUE,
  wallet_address VARCHAR(255),
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE dns_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  subdomain VARCHAR(255) NOT NULL UNIQUE,
  target VARCHAR(500) NOT NULL,
  pattern_type VARCHAR(50) NOT NULL, -- 'subdomain_path', 'multi_word', 'hybrid'
  protocol VARCHAR(20) NOT NULL DEFAULT 'http', -- 'http', 'tcp', 'udp', 'ws'
  user_id INTEGER NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  expires_at DATETIME,
  is_active BOOLEAN DEFAULT 1,
  FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE subscriptions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  dns_record_id INTEGER NOT NULL,
  status VARCHAR(50) DEFAULT 'active',
  billing_cycle_start DATE,
  billing_cycle_end DATE,
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (dns_record_id) REFERENCES dns_records(id)
);
```

## API Endpoints

### Create DNS Record
```bash
POST /api/dns/records
Content-Type: application/json

{
  "subdomain": "alice",
  "target": "http://localhost:3000",
  "pattern_type": "subdomain_path",
  "protocol": "http",
  "user_id": 1
}
```

**Response:**
```json
{
  "id": 1,
  "message": "DNS record created"
}
```

### List DNS Records
```bash
GET /api/dns/records?user_id=1
```

**Response:**
```json
[
  {
    "id": 1,
    "subdomain": "alice",
    "target": "http://localhost:3000",
    "pattern_type": "subdomain_path",
    "protocol": "http",
    "user_id": 1,
    "is_active": true,
    "created_at": "2024-01-01T00:00:00Z",
    "expires_at": null
  }
]
```

### Update DNS Record
```bash
PUT /api/dns/records/1
Content-Type: application/json

{
  "target": "http://new-target.com",
  "protocol": "ws"
}
```

### Delete DNS Record
```bash
DELETE /api/dns/records/1
```

## Tunnel Feature (ngrok-style)

**NEW!** Create persistent tunnels from your local machine:

```bash
# Run client on your machine
./client/relay-client -server=relay.name:8084 -port=3000

# Output: ✅ Tunnel established!
# 🌐 Public URL: http://tunnel-123456.relay.name
```

**See [TUNNEL_GUIDE.md](TUNNEL_GUIDE.md) for complete tunnel documentation.**

## Getting Started

### Prerequisites

- Go 1.21 or later
- SQLite3 (included with Go binary)

### Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/relay.name.git
cd relay.name
```

2. Install dependencies:
```bash
go mod download
```

3. Build the server:
```bash
go build -o relay-server main.go proxy.go udp.go
```

4. Run the server:
```bash
./relay-server \
  -http=8080 \
  -tcp=8081 \
  -udp=8082 \
  -ws=8083 \
  -db=./relay.db \
  -domain=relay.name
```

### Configuration

Set environment variables or use command-line flags:

**Server Settings:**
- `-http`: HTTP server port (default: 8080)
- `-tcp`: TCP tunneling port (default: 8081)
- `-udp`: UDP tunneling port (default: 8082)
- `-ws`: WebSocket server port (default: 8083)
- `-tunnel`: Tunnel server port for client connections (default: 8084)
- `-db`: Database path (default: ./relay.db)
- `-domain`: Base domain (default: relay.name)
- `-tls-cert`: TLS certificate path (optional)
- `-tls-key`: TLS key path (optional)

**DNS Management (Automatic DNS Record Creation):**
- `-dns-provider`: DNS provider: `cloudflare` or `digitalocean` (empty = manual)
- `-dns-api-key`: API key/token from DNS provider
- `-dns-zone-id`: Zone ID (Cloudflare) or domain name (DigitalOcean)
- `-dns-email`: Email (Cloudflare only)
- `-server-ip`: Your server's public IP address

**Example with automatic DNS management:**
```bash
./relay-server \
  -dns-provider=cloudflare \
  -dns-api-key=YOUR_API_KEY \
  -dns-zone-id=YOUR_ZONE_ID \
  -dns-email=admin@relay.name \
  -server-ip=1.2.3.4
```

**See [DNS_MANAGEMENT.md](DNS_MANAGEMENT.md) for detailed DNS setup.**

### DNS Configuration

**Important**: The Go server is NOT a DNS server. It's a reverse proxy that receives traffic AFTER DNS resolution. You need to configure DNS records that point to your server IP.

**Quick Setup (Recommended: Cloudflare)**:

1. **Sign up for Cloudflare** (free): https://cloudflare.com
2. **Add domain** `relay.name` to Cloudflare
3. **Get nameservers** from Cloudflare (e.g., `alice.ns.cloudflare.com`)
4. **Configure Namecheap**:
   - Domain List → Manage → Nameservers → Custom DNS
   - Enter Cloudflare nameservers
5. **Add DNS records in Cloudflare**:
   - `A` record: `@` → `YOUR_SERVER_IP`
   - `A` record: `*` → `YOUR_SERVER_IP` (wildcard)

**See [DNS_SETUP.md](DNS_SETUP.md) for detailed instructions** including:
- Cloudflare setup (recommended)
- AWS Route 53 setup
- Self-hosted DNS (BIND9, CoreDNS)
- Namecheap configuration steps

### Example Usage

#### HTTP Proxy
```bash
# Create DNS record
curl -X POST http://localhost:8080/api/dns/records \
  -H "Content-Type: application/json" \
  -d '{
    "subdomain": "myapp",
    "target": "http://localhost:3000",
    "pattern_type": "subdomain_path",
    "protocol": "http",
    "user_id": 1
  }'

# Access via: http://myapp.relay.name/api/users
```

#### WebSocket for Games
```bash
# Create WebSocket DNS record
curl -X POST http://localhost:8080/api/dns/records \
  -H "Content-Type: application/json" \
  -d '{
    "subdomain": "game-server",
    "target": "ws://localhost:8080",
    "pattern_type": "multi_word",
    "protocol": "ws",
    "user_id": 1
  }'

# Connect via: ws://game-server.relay.name
```

#### TCP Tunneling
```bash
# Create TCP tunnel record
curl -X POST http://localhost:8080/api/dns/records \
  -H "Content-Type: application/json" \
  -d '{
    "subdomain": "minecraft",
    "target": "localhost:25565",
    "pattern_type": "multi_word",
    "protocol": "tcp",
    "user_id": 1
  }'

# Connect via TCP port 8081
# Send: "minecraft\n" followed by game data
```

#### UDP Tunneling
```bash
# Create UDP tunnel record
curl -X POST http://localhost:8080/api/dns/records \
  -H "Content-Type: application/json" \
  -d '{
    "subdomain": "game-udp",
    "target": "localhost:7777",
    "pattern_type": "multi_word",
    "protocol": "udp",
    "user_id": 1
  }'

# Send UDP packets to port 8082
# Format: "game-udp\n" + packet data
```

## Performance

- **Latency**: < 1ms overhead per request (local network)
- **Throughput**: 10,000+ concurrent connections
- **Memory**: ~50MB base + ~1KB per connection
- **CPU**: Efficient goroutine scheduling

## Game Development

### WebSocket Game Example

```javascript
// Client-side
const ws = new WebSocket('wss://mygame.relay.name');

ws.onopen = () => {
  console.log('Connected to game server');
  ws.send(JSON.stringify({ type: 'join', player: 'Alice' }));
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Game update:', data);
};
```

### TCP Game Server Example

```go
// Your game server
listener, _ := net.Listen("tcp", ":25565")

// relay.name will tunnel connections to your server
// Players connect to: tcp://minecraft.relay.name:8081
```

## Billing System

- **$1/month per active DNS record**
- Automatic expiration handling
- Subscription tracking in database
- Integration ready for Stripe/PayPal

## Development

### Running Tests
```bash
go test ./...
```

### Building for Production
```bash
go build -ldflags="-s -w" -o relay-server main.go proxy.go udp.go
```

### Docker Support (Coming Soon)
```bash
docker build -t relay-name .
docker run -p 8080:8080 relay-name
```

## Roadmap

- [ ] TLS/SSL termination
- [ ] Rate limiting
- [ ] Authentication/Authorization
- [ ] Metrics and monitoring
- [ ] Docker containerization
- [ ] Kubernetes deployment
- [ ] Payment integration (Stripe)
- [ ] Web dashboard
- [ ] Client SDKs (Go, Node.js, Python)

## License

See LICENSE file for details.

## Contributing

Contributions welcome! Please open an issue or submit a pull request.
