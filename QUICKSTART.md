# Quick Start Guide

## Installation

```bash
# Install Go dependencies
go mod download

# Build the server
go build -o relay-server main.go proxy.go udp.go

# Or use Make
make build
```

## Running the Server

```bash
./relay-server
```

Default ports:
- HTTP: 8080
- TCP Tunneling: 8081
- UDP Tunneling: 8082
- WebSocket: 8083

## Create Your First DNS Record

```bash
curl -X POST http://localhost:8080/api/dns/records \
  -H "Content-Type: application/json" \
  -d '{
    "subdomain": "myapp",
    "target": "http://localhost:3000",
    "pattern_type": "subdomain_path",
    "protocol": "http",
    "user_id": 1
  }'
```

## Test HTTP Proxy

Once DNS is configured (`*.relay.name` → your server IP):

```bash
curl http://myapp.relay.name/api/test
```

## WebSocket for Games

```bash
# Create WebSocket record
curl -X POST http://localhost:8080/api/dns/records \
  -H "Content-Type: application/json" \
  -d '{
    "subdomain": "game",
    "target": "ws://localhost:8080",
    "pattern_type": "multi_word",
    "protocol": "ws",
    "user_id": 1
  }'

# Connect via: ws://game.relay.name
```

## TCP Tunnel for Game Servers

```bash
# Create TCP tunnel
curl -X POST http://localhost:8080/api/dns/records \
  -H "Content-Type: application/json" \
  -d '{
    "subdomain": "minecraft",
    "target": "localhost:25565",
    "pattern_type": "multi_word",
    "protocol": "tcp",
    "user_id": 1
  }'

# Connect to port 8081, send: "minecraft\n" + game data
```

## Development Mode

```bash
# Auto-rebuild and run (requires air or similar)
make dev

# Or manually:
go run main.go proxy.go udp.go
```

## List All Records

```bash
curl http://localhost:8080/api/dns/records
```

## Update a Record

```bash
curl -X PUT http://localhost:8080/api/dns/records/1 \
  -H "Content-Type: application/json" \
  -d '{
    "target": "http://new-target.com"
  }'
```

## Delete a Record

```bash
curl -X DELETE http://localhost:8080/api/dns/records/1
```


