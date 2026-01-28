# Makefile for Cenk Corapci Blog

# Variables
BINARY_NAME=blog
DIST_DIR=dist
FUNCTIONS_DIR=netlify/functions

.PHONY: all build test static clean run help build-functions

all: test build static

build: build-functions
	@echo "Building Go binary..."
	go build -o $(BINARY_NAME) main.go

build-functions:
	@echo "Building Netlify functions..."
	mkdir -p $(FUNCTIONS_DIR)
	go build -tags serverless -o $(FUNCTIONS_DIR)/blog serverless.go

test:
	@echo "Running tests..."
	go test -v ./...

static:
	@echo "Generating static site with front-end search..."
	go run main.go -static

run:
	@echo "Running blog locally with backend search..."
	go run main.go

clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)
	rm -rf $(DIST_DIR)
	rm -f $(FUNCTIONS_DIR)/blog

help:
	@echo "Available targets:"
	@echo "  all      : Runs tests, builds binary/functions, and generates static site"
	@echo "  build    : Builds the Go binary and Netlify functions"
	@echo "  static   : Generates the static site in the dist/ directory (Frontend Search)"
	@echo "  run      : Runs the blog server locally (Backend Search)"
	@echo "  clean    : Removes build artifacts"
