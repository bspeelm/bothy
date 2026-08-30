# bothy — see PLAN.md §0 for the budgets these targets enforce.

BINARY   := bothy
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.Version=$(VERSION)

# PLAN.md §0: the core binary stays under 10 MB and the core source under ~5k
# lines. These are checked, not aspired to — a change that breaks one needs a
# written justification, not a quiet bump.
MAX_BINARY_BYTES := 10485760
MAX_SOURCE_LINES := 5000

.PHONY: all build test lint vet fmt budgets check clean install-local

all: check

build:
	go build -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/bothy

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w $(shell find . -name '*.go' -not -path './.git/*')

lint: vet
	@unformatted=$$(gofmt -l $$(find . -name '*.go' -not -path './.git/*')); \
	if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi

budgets: build
	@size=$$(stat -c%s $(BINARY)); \
	echo "binary: $$size bytes (budget $(MAX_BINARY_BYTES))"; \
	if [ $$size -gt $(MAX_BINARY_BYTES) ]; then echo "over budget"; exit 1; fi
	@lines=$$(find cmd internal -name '*.go' -not -name '*_test.go' | xargs cat | wc -l); \
	echo "source: $$lines lines (budget $(MAX_SOURCE_LINES))"; \
	if [ $$lines -gt $(MAX_SOURCE_LINES) ]; then echo "over budget"; exit 1; fi

check: lint test budgets

install-local: build
	install -Dm755 $(BINARY) $(HOME)/.local/bin/$(BINARY)
	@echo "installed to ~/.local/bin/$(BINARY)"

clean:
	rm -f $(BINARY)
