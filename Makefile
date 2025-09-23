.PHONY: build test clean install docker release run help

BINARY_NAME=go-anta
BINARY_DIR=bin
GO=go
GOFLAGS=-v
DOCKER_IMAGE=go-anta
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.Version=${VERSION}"

help:
	@echo "Available targets:"
	@echo "  build       - Build the binary"
	@echo "  test        - Run tests"
	@echo "  clean       - Remove build artifacts"
	@echo "  install     - Install the binary to GOPATH/bin"
	@echo "  docker      - Build Docker image"
	@echo "  release     - Build release binaries for multiple platforms"
	@echo "  run         - Run the application"
	@echo "  fmt         - Format code"
	@echo "  lint        - Run linter"
	@echo "  vet         - Run go vet"
	@echo "  coverage    - Generate test coverage report"

build:
	@echo "Building ${BINARY_NAME}..."
	@mkdir -p ${BINARY_DIR}
	${GO} build ${GOFLAGS} ${LDFLAGS} -o ${BINARY_DIR}/${BINARY_NAME} cmd/go-anta/main.go
	@echo "Build complete: ${BINARY_DIR}/${BINARY_NAME}"

test:
	@echo "Running tests..."
	${GO} test ${GOFLAGS} ./...

test-verbose:
	@echo "Running tests with verbose output..."
	${GO} test -v ./...

coverage:
	@echo "Generating test coverage report..."
	${GO} test -v -coverprofile=coverage.out ./...
	${GO} tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

clean:
	@echo "Cleaning..."
	rm -rf ${BINARY_DIR}
	rm -f coverage.out coverage.html
	@echo "Clean complete"

install: build
	@echo "Installing ${BINARY_NAME}..."
	${GO} install cmd/go-anta/main.go
	@echo "Installation complete"

run: build
	@echo "Running ${BINARY_NAME}..."
	./${BINARY_DIR}/${BINARY_NAME}

fmt:
	@echo "Formatting code..."
	${GO} fmt ./...
	@echo "Formatting complete"

vet:
	@echo "Running go vet..."
	${GO} vet ./...
	@echo "Vet complete"

lint:
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin"; \
	fi

docker-build:
	@echo "Building Docker image..."
	docker build -t ${DOCKER_IMAGE}:${VERSION} .
	docker tag ${DOCKER_IMAGE}:${VERSION} ${DOCKER_IMAGE}:latest
	@echo "Docker image built: ${DOCKER_IMAGE}:${VERSION}"

docker-run:
	@echo "Running GANTA in Docker..."
	@if [ ! -f inventory.yaml ]; then \
		echo "Error: inventory.yaml not found. Please create it first."; \
		exit 1; \
	fi
	@if [ ! -f catalog.yaml ]; then \
		echo "Error: catalog.yaml not found. Please create it first."; \
		exit 1; \
	fi
	docker run --rm -it \
		-v $(PWD)/inventory.yaml:/data/inventory.yaml:ro \
		-v $(PWD)/catalog.yaml:/data/catalog.yaml:ro \
		-v $(PWD)/output:/data/output \
		--network host \
		${DOCKER_IMAGE}:${VERSION} nrfu -i /data/inventory.yaml -c /data/catalog.yaml

docker-check:
	@echo "Running connectivity check in Docker..."
	@if [ ! -f inventory.yaml ]; then \
		echo "Error: inventory.yaml not found. Please create it first."; \
		exit 1; \
	fi
	docker run --rm -it \
		-v $(PWD)/inventory.yaml:/data/inventory.yaml:ro \
		--network host \
		${DOCKER_IMAGE}:${VERSION} check -i /data/inventory.yaml

docker-shell:
	@echo "Starting shell in GANTA container..."
	docker run --rm -it \
		-v $(PWD):/data \
		--network host \
		${DOCKER_IMAGE}:${VERSION} bash

docker-compose-up:
	@echo "Starting GANTA with docker-compose..."
	docker-compose up -d

docker-compose-down:
	@echo "Stopping GANTA containers..."
	docker-compose down

docker-compose-logs:
	@echo "Showing GANTA container logs..."
	docker-compose logs -f

docker-push:
	@echo "Pushing Docker image to registry..."
	docker push ${DOCKER_IMAGE}:${VERSION}
	docker push ${DOCKER_IMAGE}:latest

docker-clean:
	@echo "Cleaning Docker images..."
	docker rmi ${DOCKER_IMAGE}:${VERSION} ${DOCKER_IMAGE}:latest || true
	docker system prune -f

release:
	@echo "Building release binaries..."
	@mkdir -p ${BINARY_DIR}
	
	@echo "Building for Linux AMD64..."
	GOOS=linux GOARCH=amd64 ${GO} build ${LDFLAGS} -o ${BINARY_DIR}/${BINARY_NAME}-linux-amd64 cmd/go-anta/main.go
	
	@echo "Building for Linux ARM64..."
	GOOS=linux GOARCH=arm64 ${GO} build ${LDFLAGS} -o ${BINARY_DIR}/${BINARY_NAME}-linux-arm64 cmd/go-anta/main.go
	
	@echo "Building for Darwin AMD64..."
	GOOS=darwin GOARCH=amd64 ${GO} build ${LDFLAGS} -o ${BINARY_DIR}/${BINARY_NAME}-darwin-amd64 cmd/go-anta/main.go
	
	@echo "Building for Darwin ARM64..."
	GOOS=darwin GOARCH=arm64 ${GO} build ${LDFLAGS} -o ${BINARY_DIR}/${BINARY_NAME}-darwin-arm64 cmd/go-anta/main.go
	
	@echo "Building for Windows AMD64..."
	GOOS=windows GOARCH=amd64 ${GO} build ${LDFLAGS} -o ${BINARY_DIR}/${BINARY_NAME}-windows-amd64.exe cmd/go-anta/main.go
	
	@echo "Release binaries built in ${BINARY_DIR}/"

deps:
	@echo "Downloading dependencies..."
	${GO} mod download
	${GO} mod tidy
	@echo "Dependencies updated"

check: fmt vet test
	@echo "All checks passed!"

ci: check coverage
	@echo "CI checks complete!"

.DEFAULT_GOAL := help