.PHONY: build test audit fuzz bench check clean

build:
	mkdir -p bin
	go build -trimpath -o bin/tetris-optimizer ./cmd/tetris-optimizer
	go build -trimpath -o bin/tetris-visualizer ./cmd/tetris-visualizer

test:
	go test -race -cover ./...

audit: build
	python3 scripts/audit.py

fuzz:
	go test ./internal/parser -run='^$$' -fuzz=FuzzParse -fuzztime=10s -parallel=2
	go test ./internal/solver -run='^$$' -fuzz=FuzzTwoPieces -fuzztime=10s -parallel=2

bench:
	go test ./internal/solver -run='^$$' -bench=. -benchmem

check:
	test -z "$$(gofmt -l cmd internal)"
	go vet ./...
	go test -race -cover ./...
	$(MAKE) audit

clean:
	rm -rf bin coverage.out
