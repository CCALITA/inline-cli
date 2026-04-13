BUILD_DIR := ./build
BINARY := inline-cli
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build test lint clean install

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/inline-cli

test:
	go test -race -cover ./...

lint:
	go vet ./...

clean:
	rm -rf $(BUILD_DIR)

install: build
	cp $(BUILD_DIR)/$(BINARY) $(GOPATH)/bin/ 2>/dev/null || cp $(BUILD_DIR)/$(BINARY) ~/go/bin/
