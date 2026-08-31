# bothy — see PLAN.md §3 for the budgets these targets enforce.

BINARY   := bothy
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.Version=$(VERSION)

# PLAN.md §3: the core binary stays under 10 MB and the core source under ~5k
# LOC. Checked, not aspired to.
#
# Two source limits, because one number could not tell code from prose. The
# principle says "LOC", and comments are not code — this project's comments are
# where the reasons live, and trimming them to pass a line count would delete
# the most valuable thing in the repository to satisfy a proxy for the thing it
# was meant to measure. So: code is capped at 5k, and total lines at 7k so the
# prose cannot grow without limit either. See docs/decisions.md ADR-010.
MAX_BINARY_BYTES := 10485760
MAX_CODE_LINES   := 5000
MAX_TOTAL_LINES  := 7000

SOURCES := $(shell find cmd internal -name '*.go' -not -name '*_test.go')

.PHONY: all build test lint vet fmt budgets check clean install-binary vendor srpm release copr

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

# --- packaging ---------------------------------------------------------------
# Copr build roots have no network, so the dependency is vendored and the spec
# builds with -mod=vendor and GOPROXY=off.
vendor:
	go mod vendor

# Build a source RPM from the working tree, laid out the way GitHub's tag
# tarball is, so what is tested locally is what Copr will build.
srpm: vendor
	@v=$$(sed -n 's/^Version:[[:space:]]*//p' packaging/$(BINARY).spec); 	rpmdev-setuptree; 	tmp=$$(mktemp -d); mkdir -p $$tmp/$(BINARY)-$$v; 	git ls-files | tar -cf - -T - | tar -xf - -C $$tmp/$(BINARY)-$$v; 	cp -r vendor $$tmp/$(BINARY)-$$v/; 	tar -czf $$HOME/rpmbuild/SOURCES/$(BINARY)-$$v.tar.gz -C $$tmp $(BINARY)-$$v; 	rm -rf $$tmp; 	rpmbuild -bs packaging/$(BINARY).spec

# Cut a release: bump the spec, commit, tag, push. Tagging is what triggers the
# GitHub release, which is what `curl | sh` and `go install @latest` both
# follow, so this is the only step those two need.
#
#     make release VERSION=0.2.0
release:
	@test -n "$(VERSION)" || { echo "usage: make release VERSION=x.y.z"; exit 1; }
	@echo "$(VERSION)" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "VERSION must be x.y.z"; exit 1; }
	@git diff --quiet || { echo "working tree is dirty; commit first"; exit 1; }
	@! git rev-parse -q --verify "refs/tags/v$(VERSION)" >/dev/null || { echo "v$(VERSION) already exists"; exit 1; }
	$(MAKE) check
	@scripts/bump-spec.sh "$(VERSION)"
	git add packaging/$(BINARY).spec
	git commit -m "build: $(VERSION)"
	git tag v$(VERSION)
	git push && git push origin v$(VERSION)
	@echo
	@echo "tagged v$(VERSION); GitHub Actions is building the release."
	@echo "once it is green:  make copr"

# Publish the current spec version to Copr. Kept separate from `release` so it
# runs after the GitHub release exists rather than racing it.
copr: srpm
	@v=$$(sed -n 's/^Version:[[:space:]]*//p' packaging/$(BINARY).spec); \
	copr-cli build $(BINARY) $$HOME/rpmbuild/SRPMS/$(BINARY)-$$v-1.*.src.rpm
