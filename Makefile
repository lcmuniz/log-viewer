.PHONY: build install run test lint clean

build:
	go build -o bin/log-viewer ./cmd/log-viewer

install:
	go install ./cmd/log-viewer

run:
	go run ./cmd/log-viewer

test:
	go test -race ./...

lint:
	gofmt -l . | (! grep .)
	go vet ./...

clean:
	rm -rf bin/
