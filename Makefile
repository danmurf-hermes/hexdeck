# hexdeck — common developer tasks in one place.
# Run `make help` to see the targets.

BINARY   := hexdeck
PKG      := ./cmd/hexdeck
BOARD    := .kanban
DEMO     := docs/demo

.PHONY: all build test vet fmt render-check coverage clean help

all: build

## build: compile the hexdeck binary into ./bin/hexdeck
build:
	go build -o bin/$(BINARY) $(PKG)

## test: run the full test suite with the race detector
test:
	go test -race ./...

## vet: run go vet
vet:
	go vet ./...

## fmt: check gofmt (fails on unformatted files)
fmt:
	@files=$$(gofmt -l .); \
	if [ -n "$$files" ]; then \
		echo "gofmt needed on these files:"; \
		echo "$$files"; \
		exit 1; \
	fi

## render-check: verify the committed board projections match the ops
render-check:
	bin/$(BINARY) render --check --dir $(DEMO)
	bin/$(BINARY) render --check --dir $(BOARD)
	bin/$(BINARY) render --svg --dir $(DEMO)
	cmp $(DEMO)/board.svg board.svg
	git diff --exit-code -- $(DEMO)/board.svg

## coverage: run tests and print total coverage (honest: includes subprocess E2E)
coverage:
	HEXDECK_E2E_COVER=1 go test -coverpkg=./... -count=1 -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

## clean: remove build artifacts
clean:
	rm -rf bin coverage.out

help:
	@grep -E '^## ' Makefile | sed 's/## //'
