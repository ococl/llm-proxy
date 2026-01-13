.PHONY: all clean build-all windows linux darwin

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

OUTPUT_DIR := dist
BINARY_NAME := llm-proxy

all: build-all

clean:
	rm -rf $(OUTPUT_DIR)/*.exe $(OUTPUT_DIR)/*-linux-* $(OUTPUT_DIR)/*-darwin-*

build-all: windows linux darwin
	@echo "构建完成，输出目录: $(OUTPUT_DIR)"

windows:
	@echo "构建 Windows 版本..."
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-windows-amd64.exe ./src
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-windows-arm64.exe ./src

linux:
	@echo "构建 Linux 版本..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-linux-amd64 ./src
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-linux-arm64 ./src

darwin:
	@echo "构建 macOS 版本..."
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-darwin-amd64 ./src
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-darwin-arm64 ./src

# 单平台快速构建
dev:
	go build -o $(OUTPUT_DIR)/$(BINARY_NAME).exe ./src

test:
	cd src && go test -v ./...
