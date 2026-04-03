BINARY_NAME=memoria
BUILD_DIR=bin
MODULE=github.com/cassiomarques/memoria
MAIN=./cmd/memoria
VERSION ?= dev
LDFLAGS = -s -w -X main.version=$(VERSION)

.PHONY: build run test lint clean release release-darwin-arm64 release-darwin-amd64

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN)

run:
	go run $(MAIN)

test:
	go test ./... -v -race -count=1

test-short:
	go test ./... -short -race -count=1

test-cover:
	go test ./... -race -count=1 -coverprofile=coverage.out -coverpkg=./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	go vet ./...

clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html

release-darwin-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN)

release-darwin-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN)

release: release-darwin-arm64 release-darwin-amd64
