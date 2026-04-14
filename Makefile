BUILD_DIR := ./build
BINARY := inline-cli
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

# Cross-compilation targets: OS_ARCH pairs
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: build test lint clean install release release-binaries release-tarballs checksums

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

# Build binaries for all platforms
release-binaries:
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		output=$(BUILD_DIR)/$(BINARY)_$(VERSION)_$${os}_$${arch}/$(BINARY); \
		echo "building $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build $(LDFLAGS) -o $$output ./cmd/inline-cli || exit 1; \
	done

# Package each binary into a tarball matching install.sh's expected naming
release-tarballs: release-binaries
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		dir=$(BUILD_DIR)/$(BINARY)_$(VERSION)_$${os}_$${arch}; \
		tarball=$(BUILD_DIR)/$(BINARY)_$(VERSION)_$${os}_$${arch}.tar.gz; \
		echo "packaging $$tarball..."; \
		tar -czf $$tarball -C $$dir $(BINARY); \
	done

# Generate checksums for all tarballs
checksums: release-tarballs
	@cd $(BUILD_DIR) && shasum -a 256 $(BINARY)_$(VERSION)_*.tar.gz > checksums.txt
	@echo "checksums written to $(BUILD_DIR)/checksums.txt"

# Full release: binaries + tarballs + checksums
release: checksums
	@echo ""
	@echo "release artifacts in $(BUILD_DIR)/:"
	@ls -lh $(BUILD_DIR)/$(BINARY)_$(VERSION)_*.tar.gz
	@echo ""
	@cat $(BUILD_DIR)/checksums.txt
