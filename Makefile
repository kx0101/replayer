.PHONY: build clean test install

build:
	@echo "Building replayer tool..."
	@go build -o replayer .
	@go build -o mock-server cmd/mock-server/mock-server.go
	@go build -o mock-server-v2 cmd/mock-server/mock-server-v2.go
	@go build -o generate-logs cmd/generate-logs/generate-logs.go
	@go build -o nginx-converter cmd/nginx-converter/nginx-converter.go
	@echo "Build complete!"

release:
	@echo "Building optimized binaries..."
	@go build -ldflags="-s -w" -o replayer .
	@echo "Release build complete!"

clean:
	@rm -f replayer mock-server mock-server-v2 generate-logs nginx-converter
	@rm -f *.json *.html *.log
	@echo "Clean complete!"

test:
	@echo "Running tests..."
	@go test ./...
	@echo "Tests complete!"

install: build
	@echo "Installing to /usr/local/bin..."
	@sudo cp replayer /usr/local/bin/
	@echo "Installation complete!"

demo: build
	@echo "Starting demo..."

	@./generate-logs --output demo.json --count 50
	@./mock-server --port 8080 & echo $$! > .mock1.pid
	@./mock-server-v2 --port 8081 & echo $$! > .mock2.pid

	@sleep 1

	@./replayer --input-file demo.json --compare --html-report demo.html localhost:8080 localhost:8081

	@kill $$(cat .mock1.pid) $$(cat .mock2.pid)
	@rm .mock1.pid .mock2.pid

	@open demo.html || xdg-open demo.html
	@echo "Demo complete!"
