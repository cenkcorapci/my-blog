# Makefile for Cenk Corapci Blog

# Variables
BINARY_NAME=blog-gen
DIST_DIR=dist

.PHONY: all build test static clean run help

all: test static

clean-run: clean static run

build:
	@echo "Building generator binary..."
	go build -o $(BINARY_NAME) main.go

test: test-go test-js

test-go:
	@echo "Running Go tests..."
	go test -v ./...

test-js:
	@echo "Running JavaScript tests..."
	npm test

static:
	@echo "Generating static site..."
	go run main.go -dist $(DIST_DIR)

run:
	@echo "Running local preview server..."
	go run main.go -serve -dist $(DIST_DIR)

clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)
	rm -rf $(DIST_DIR)

help:
	@echo "Available targets:"
	@echo "  all     : Runs tests and generates the static site"
	@echo "  static  : Generates the static site in the dist/ directory"
	@echo "  run     : Generates and serves the site locally for preview"
	@echo "  clean   : Removes build artifacts"
