BINARY_NAME := chronicle
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test lint clean install release-dry-run

build:
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/chronicle/

test:
	CGO_ENABLED=1 go test -v -race ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY_NAME)
	go clean -cache

install: build
	cp $(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)

release-dry-run:
	goreleaser release --snapshot --clean
