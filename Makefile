.PHONY: build build-cloud clean test install lint

build:
	@echo "Building replayer tool..."
	@go build -o replayer ./cmd/replayer
	@go build -o mock_server cmd/mock_server/mock_server.go
	@go build -o mock_server_v2 cmd/mock_server_v2/mock_server_v2.go
	@go build -o mock_server_v3 cmd/mock_server_v3/mock_server_v3.go
	@go build -o mock_server_tls cmd/mock_server_tls/mock_server_tls.go
	@go build -o generate_logs cmd/generate_logs/generate_logs.go
	@echo "Build complete!"

build-cloud:
	@echo "Building replayer-cloud server..."
	@cd cloud && go build -o ../replayer-cloud ./cmd/server
	@echo "Cloud build complete!"

build-all: build build-cloud

release:
	@echo "Building optimized binaries..."
	@go build -ldflags="-s -w" -o replayer ./cmd/replayer
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o replayer-linux ./cmd/replayer
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o replayer-macos-intel ./cmd/replayer
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o replayer-macos-arm ./cmd/replayer
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o replayer.exe ./cmd/replayer

	@echo "Release build complete!"

clean:
	@rm -f replayer replayer-cloud mock* generate_logs
	@rm -f *.json *.html *.log
	@rm -rf cloud/bin/
	@echo "Clean complete!"

test:
	@echo "Running tests..."
	@go test -v -race ./...
	@cd cloud && go test -v -race ./...
	@echo "Tests complete!"

lint:
	@echo "Running security checks..."
	@gosec ./cmd/... ./internal/...
	@cd cloud && gosec ./...
	@echo "Security checks complete!"

install: build
	@echo "Installing to /usr/local/bin..."
	@sudo cp replayer /usr/local/bin/
	@echo "Installation complete!"

demo: build
	@echo "Starting demo..."

	@./generate_logs --output demo.json --count 50

	@./mock_server --port 8080 & echo $$! > .mock1.pid
	@./mock_server_v2 --port 8081 & echo $$! > .mock2.pid
	@sleep 1

	@./replayer --input-file demo.json --compare --html-report demo.html \
		localhost:8080 localhost:8081 || true

	@kill $$(cat .mock1.pid) $$(cat .mock2.pid) 2>/dev/null || true
	@rm -f .mock1.pid .mock2.pid

	@open demo.html || xdg-open demo.html
	@echo "Demo complete!"
