.PHONY: build install clean run

build:
	@echo "Building log-viewer..."
	@go build -o bin/log-viewer ./cmd/log-viewer
	@mkdir -p bin/static
	@cp static/* bin/static/
	@echo "Build complete! Binary: bin/log-viewer"

install:
	@echo "Installing log-viewer..."
	@go install ./cmd/log-viewer
	@echo "Install complete!"
	@echo "You can now run: log-viewer"

run:
	@go run ./cmd/log-viewer

clean:
	@rm -rf bin/
	@echo "Clean complete!"
