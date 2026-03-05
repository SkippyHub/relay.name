.PHONY: build run test clean install

build:
	go build -o relay-server main.go proxy.go udp.go dns_providers.go tunnel.go
	cd client && go build -o relay-client main.go

run: build
	./relay-server

test:
	go test ./...

clean:
	rm -f relay-server
	rm -f *.db *.db-shm *.db-wal

install:
	go mod download
	go mod tidy

dev:
	go run main.go proxy.go udp.go -http=8080 -tcp=8081 -udp=8082 -ws=8083


