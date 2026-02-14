.PHONY: build test test-integration run clean

build:
	go build -o bin/replayer-cloud ./cmd/server

test:
	go test -v -race ./...

test-integration:
	go test -v -race -tags=integration ./...

run: build
	./bin/replayer-cloud

clean:
	rm -rf bin/
