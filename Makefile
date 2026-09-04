BINARY=terbash
VERSION?=dev
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE) -s -w"

.PHONY: build build-arm64 build-all clean test lint install

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/terbash

build-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY)-linux-arm64 ./cmd/terbash

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 ./cmd/terbash

build-windows-arm64:
	GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY)-windows-arm64.exe ./cmd/terbash

build-all: build-arm64 build-darwin-arm64 build-windows-arm64

clean:
	rm -f $(BINARY) $(BINARY)-*

test:
	go test -v ./...

lint:
	golangci-lint run ./...

install: build
	install -m 755 $(BINARY) $(DESTDIR)/usr/local/bin/$(BINARY)

mod-tidy:
	go mod tidy

mod-verify:
	go mod verify

run: build
	./$(BINARY)

dev:
	go run ./cmd/terbash