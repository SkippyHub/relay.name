# Tunnel Feature Guide

**ngrok-style tunneling is now available!** Create persistent tunnels from your local machine to the internet.

## How It Works

```
Your PC → relay-client → relay.name server → Internet users
         (persistent tunnel)
```

1. **Run client** on your local machine
2. **Client connects** to relay.name server (persistent connection)
3. **Server assigns** subdomain automatically (or use custom)
4. **Server creates** DNS record automatically (if DNS provider configured)
5. **Traffic flows**: Internet → relay.name → tunnel → your localhost

## Quick Start

### 1. Start the Server

```bash
./relay-server \
  -tunnel=8084 \
  -dns-provider=cloudflare \
  -dns-api-key=YOUR_KEY \
  -dns-zone-id=YOUR_ZONE \
  -dns-email=your@email.com \
  -server-ip=YOUR_SERVER_IP
```

### 2. Run the Client

```bash
# Build the client
cd client
go build -o relay-client main.go

# Run tunnel
./relay-client -server=relay.name:8084 -port=3000

# Output:
# ✅ Tunnel established!
# 🌐 Public URL: http://tunnel-123456.relay.name
# 📡 Protocol: http
# 🎯 Target: localhost:3000
```

### 3. Access Your Service

```bash
# Your local service is now accessible at:
curl http://tunnel-123456.relay.name
```

## Client Usage

### Basic Usage

```bash
# Tunnel localhost:3000
./relay-client -server=relay.name:8084 -port=3000

# Custom subdomain
./relay-client -server=relay.name:8084 -port=3000 -subdomain=myapp

# Different protocol
./relay-client -server=relay.name:8084 -port=3000 -protocol=tcp
```

### Command Line Options

```
-server      Relay server address (default: relay.name:8084)
-port        Local port to tunnel (required)
-protocol    Protocol: http, tcp, udp, ws (default: http)
-subdomain   Custom subdomain (optional, auto-generated if not provided)
```

### Examples

**HTTP Tunnel:**
```bash
# Start local web server
python -m http.server 3000

# Create tunnel
./relay-client -server=relay.name:8084 -port=3000

# Access: http://tunnel-123456.relay.name
```

**TCP Tunnel (Game Server):**
```bash
# Start game server on port 25565
./minecraft-server

# Create TCP tunnel
./relay-client -server=relay.name:8084 -port=25565 -protocol=tcp

# Players connect to: relay.name:8081 (with subdomain in first packet)
```

**WebSocket Tunnel:**
```bash
# Start WebSocket server
node websocket-server.js

# Create tunnel
./relay-client -server=relay.name:8084 -port=8080 -protocol=ws

# Connect: ws://tunnel-123456.relay.name
```

## How It Differs from ngrok

| Feature | ngrok | relay.name |
|---------|-------|------------|
| **Domain** | ngrok.io (their domain) | relay.name (your domain) |
| **Subdomain** | Random (free) | Your choice or auto-generated |
| **DNS** | ngrok handles it | You control it (or we automate) |
| **Setup** | Just run client | Configure DNS once, then just run client |
| **Customization** | Limited | Full control |

## Architecture

### Server Components

1. **Tunnel Server** (Port 8084)
   - Accepts client connections
   - Manages tunnel lifecycle
   - Creates DNS records automatically

2. **HTTP Server** (Port 8080)
   - Receives public traffic
   - Checks for tunnels first
   - Falls back to DNS records

3. **Tunnel Manager**
   - Tracks active tunnels
   - Handles tunnel routing
   - Manages tunnel connections

### Client Components

1. **Connection**
   - Connects to tunnel server
   - Sends registration message
   - Maintains persistent connection

2. **Protocol Handler**
   - HTTP: Local server + tunnel forwarding
   - TCP: Direct bidirectional proxy
   - WebSocket: Message forwarding

## Tunnel Lifecycle

1. **Registration**
   ```
   Client → Server: {"type": "register", "protocol": "http", "target": "localhost:3000"}
   Server → Client: {"type": "registered", "subdomain": "tunnel-123456", "id": "tun_..."}
   ```

2. **DNS Creation** (if DNS provider configured)
   ```
   Server automatically creates: tunnel-123456.relay.name → YOUR_SERVER_IP
   ```

3. **Traffic Flow**
   ```
   User → http://tunnel-123456.relay.name
   → DNS resolves to YOUR_SERVER_IP
   → Server checks tunnel
   → Forwards through tunnel connection
   → Client receives request
   → Client forwards to localhost:3000
   → Response flows back
   ```

4. **Cleanup**
   ```
   Client disconnects → Server removes tunnel → DNS record remains (or delete manually)
   ```

## Database

Tunnels are stored in the database:

```sql
CREATE TABLE tunnels (
  id VARCHAR(255) PRIMARY KEY,
  subdomain VARCHAR(255) NOT NULL UNIQUE,
  target VARCHAR(500) NOT NULL,
  protocol VARCHAR(20) NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  is_active BOOLEAN DEFAULT 1
);
```

## API Endpoints (Future)

```bash
# List active tunnels
GET /api/tunnels

# Get tunnel info
GET /api/tunnels/:id

# Delete tunnel
DELETE /api/tunnels/:id
```

## Troubleshooting

### "Connection refused"
- Check server is running: `./relay-server`
- Verify tunnel port is open: `-tunnel=8084`
- Check firewall allows port 8084

### "Tunnel not found"
- Verify client is connected
- Check subdomain matches
- Check tunnel is active in database

### "DNS not resolving"
- Wait 5-60 minutes for propagation
- Check DNS provider is configured
- Verify DNS record was created

### "Target connection failed"
- Ensure local service is running
- Check target port is correct
- Verify firewall allows local connections

## Security Considerations

1. **Authentication**: Add API keys for tunnel creation
2. **Rate Limiting**: Limit tunnels per user
3. **Subdomain Validation**: Prevent subdomain conflicts
4. **TLS**: Use TLS for tunnel connections (future)
5. **Access Control**: Restrict who can create tunnels

## Future Enhancements

- [ ] WebSocket tunnel support
- [ ] UDP tunnel support
- [ ] TLS encryption for tunnels
- [ ] Authentication/API keys
- [ ] Tunnel management API
- [ ] Web dashboard for tunnels
- [ ] Tunnel statistics/monitoring
- [ ] Multiple tunnels per client
- [ ] Custom domains per tunnel

