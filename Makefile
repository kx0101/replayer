.PHONY: build build-cloud clean test lint release install

build:
	@echo "Building replayer CLI..."
	@cargo build --release -p replayer-cli
	@echo "Build complete!"

build-cloud:
	@echo "Building replayer-cloud server..."
	@cargo build --release -p replayer-cloud
	@echo "Cloud build complete!"

build-all: build build-cloud

release:
	@echo "Building optimized release binaries..."
	@cargo build --release
	@echo "Release build complete!"

clean:
	@cargo clean
	@echo "Clean complete!"

test:
	@echo "Running tests..."
	@cargo test --workspace
	@echo "Tests complete!"

lint:
	@echo "Running checks..."
	@cargo clippy --workspace -- -D warnings
	@cargo fmt --check
	@echo "Checks complete!"

fmt:
	@cargo fmt --all

install: build
	@echo "Installing to /usr/local/bin..."
	@sudo cp target/release/replayer /usr/local/bin/
	@echo "Installation complete!"

check:
	@cargo check --workspace
