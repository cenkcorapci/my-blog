# Makefile for Cenk Corapci Blog

# Variables
BINARY_NAME=blog
DIST_DIR=dist

.PHONY: all build test static clean run help

all: test build static

build:
	@echo "Building Go binary..."
	go build -o $(BINARY_NAME) main.go

test:
	@echo "Running tests..."
	go test -v ./...

static:
	@echo "Generating static site..."
	go run main.go -static

run:
	@echo "Running blog locally..."
	go run main.go

clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)
	rm -rf $(DIST_DIR)

help:
	@echo "Available targets:"
	@echo "  all     : Runs tests, builds binary, and generates static site"
	@echo "  build   : Builds the Go binary"
	@echo "  test    : Runs unit tests"
	@echo "  static  : Generates the static site in the dist/ directory"
	@echo "  run     : Runs the blog server locally"
	@echo "  clean   : Removes build artifacts"
