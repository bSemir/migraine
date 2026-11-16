.PHONY: build test lint demo

# Coverage is measured across all internal packages, so code exercised
# from another package's tests (the domain types, for one) is counted.
build:
	go build -trimpath -o migraine ./cmd/migraine

test:
	go test -race -coverpkg=./internal/... -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | tail -n 1

lint:
	go vet ./...
	golangci-lint run

# Records demo.gif with charmbracelet/vhs (needs vhs, ttyd and ffmpeg).
demo: build
	vhs demo.tape
