# bothy — see PLAN.md §0 for the budgets these targets enforce.

BINARY   := bothy
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.Version=$(VERSION)

# PLAN.md §0: the core binary stays under 10 MB and the core source under ~5k
# lines. These are checked, not aspired to — a change that breaks one needs a
# written justification, not a quiet bump.
MAX_BINARY_BYTES := 10485760
MAX_SOURCE_LINES := 5500

.PHONY: all build test lint vet fmt budgets check clean install-binary

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
	@code=$$(cat $(SOURCES) | grep -v '^[[:space:]]*//' | grep -vc '^[[:space:]]*$$'); \
	total=$$(cat $(SOURCES) | wc -l); \
	echo "code:   $$code lines (budget $(MAX_CODE_LINES))"; \
	echo "total:  $$total lines (budget $(MAX_TOTAL_LINES), incl. comments)"; \
	if [ $$code -gt $(MAX_CODE_LINES) ]; then echo "over the code budget"; exit 1; fi; \
	if [ $$total -gt $(MAX_TOTAL_LINES) ]; then echo "over the total budget"; exit 1; fi

check: lint test budgets

# Named for what it installs. "install-local" collided with `bothy install`,
# which installs the workspace — two different things wearing one word.
install-binary: build
	install -Dm755 $(BINARY) $(HOME)/.local/bin/$(BINARY)
	@echo "installed to ~/.local/bin/$(BINARY)"

clean:
	rm -f $(BINARY)
