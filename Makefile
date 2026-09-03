# bothy — see PLAN.md §3 for the budgets these targets enforce.

BINARY   := bothy
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.Version=$(VERSION)

# PLAN.md §3: the core binary stays under 10 MB, and the core source under a
# line count. Checked, not aspired to.
#
# Two source limits: code is capped outright, and comments are capped as a
# share of it, so prose has to stay proportionate to what it explains rather
# than compete with it for room. See ADR-010, ADR-015 and ADR-021, with ADR-026
# for why the code cap is 6,000 and ADR-029 for why the ratio is 22.
MAX_BINARY_BYTES  := 10485760
MAX_CODE_LINES    := 6000
MAX_COMMENT_RATIO := 22

SOURCES := $(shell find cmd internal -name '*.go' -not -name '*_test.go')

.PHONY: all build test lint vet fmt budgets crossbuild check clean install-binary vendor srpm release release-tag copr ledger packet

all: check

# CGO_ENABLED=0 to match .goreleaser.yaml. Without it the binary `budgets`
# weighs against the 10 MB cap is not the binary that ships, and one built here
# and carried into another distro's container is linked against this machine's
# glibc -- which would test glibc compatibility rather than bothy.
build:
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/bothy

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
	@size=$$(wc -c < $(BINARY)); \
	echo "binary: $$size bytes (budget $(MAX_BINARY_BYTES))"; \
	if [ $$size -gt $(MAX_BINARY_BYTES) ]; then echo "over budget"; exit 1; fi
	@code=$$(cat $(SOURCES) | awk '/^[[:space:]]*\/\*/{b=1} b{if(/\*\//)b=0; next} !/^[[:space:]]*\/\// && NF' | wc -l); \
	comments=$$(cat $(SOURCES) | awk '/^[[:space:]]*\/\*/{b=1} b{c++; if(/\*\//)b=0; next} /^[[:space:]]*\/\//{c++} END{print c+0}'); \
	ratio=$$(( comments * 100 / code )); \
	echo "code:     $$code lines (budget $(MAX_CODE_LINES))"; \
	echo "comments: $$comments lines, $$ratio% of code (budget $(MAX_COMMENT_RATIO)%)"; \
	if [ $$code -gt $(MAX_CODE_LINES) ]; then echo "over the code budget"; exit 1; fi; \
	if [ $$ratio -gt $(MAX_COMMENT_RATIO) ]; then echo "over the comment budget"; exit 1; fi

# The platforms .goreleaser.yaml ships, plus windows -- which bothy does not
# support and does compile for today, with no build tags anywhere. Keeping it
# in the list costs a second and makes the day it stops compiling a decision
# rather than a discovery.
CROSS := darwin/amd64 darwin/arm64 linux/arm64 windows/amd64

# Compile for every target, without running anything.
#
# `make build` has no GOOS, so it only ever proves the host, and the only thing
# that built darwin was goreleaser -- which runs after the tag is pushed, on
# the one step that cannot be taken back. Building ./cmd/bothy reaches every
# package that ships; bootstrap is the only other one and it is tests.
crossbuild:
	@for t in $(CROSS); do \
	    echo "  $$t"; \
	    GOOS=$${t%/*} GOARCH=$${t#*/} CGO_ENABLED=0 go build -o /dev/null ./cmd/bothy || exit 1; \
	done

check: lint test crossbuild budgets

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

# Step one of a release: open the PR that bumps the spec.
#
#     make release VERSION=0.2.0
#
# main requires a pull request, so the version bump cannot be pushed straight
# to it. That is not just a rule to satisfy: Copr reads Version: from the spec
# at the tagged commit, so a tag whose commit still says the old version
# publishes an rpm under the wrong number. The bump has to land on main before
# the tag exists, which is exactly what the PR does.
# The vouching ledger (framework §7.1). Reports here; the release blocks.
ledger:
	@sh scripts/ledger.sh || true

# Fails unless the packet for VERSION exists and is answered (§7.5).
packet:
	@test -n "$(VERSION)" || { echo "usage: make packet VERSION=x.y.z"; exit 1; }
	@sh scripts/review-packet.sh --check "$(VERSION)"

release:
	@test -n "$(VERSION)" || { echo "usage: make release VERSION=x.y.z"; exit 1; }
	@echo "$(VERSION)" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "VERSION must be x.y.z"; exit 1; }
	@git diff --quiet || { echo "working tree is dirty; commit first"; exit 1; }
	@test "$$(git rev-parse --abbrev-ref HEAD)" = main || { echo "release starts from main"; exit 1; }
	@! git rev-parse -q --verify "refs/tags/v$(VERSION)" >/dev/null || { echo "v$(VERSION) already exists"; exit 1; }
	@! git rev-parse -q --verify "release/$(VERSION)" >/dev/null || { echo "branch release/$(VERSION) already exists"; exit 1; }
	@command -v gh >/dev/null || { echo "gh is not installed"; exit 1; }
	@gh auth status >/dev/null 2>&1 || { echo "gh is not logged in; run 'gh auth login'"; exit 1; }
	$(MAKE) check
	git switch -c release/$(VERSION)
	@scripts/bump-spec.sh "$(VERSION)"
	git add packaging/$(BINARY).spec
	git commit -m "build: $(VERSION)"
	git push -u origin release/$(VERSION)
	gh pr create --base main --head release/$(VERSION) \
	    --title "build: $(VERSION)" \
	    --body "Bumps the rpm spec to $(VERSION). Merging this, then \`make release-tag\`, cuts the release."
	@echo
	@echo "PR opened. Merge it, then:  git switch main && git pull && make release-tag"

# Step two: tag the merged bump. The tag is what everything downstream watches
# -- GitHub Actions builds the archives, the Copr webhook builds the rpms.
#
# Branch rules do not cover tags, so this needs no PR.
release-tag:
	@test "$$(git rev-parse --abbrev-ref HEAD)" = main || { echo "tag from main"; exit 1; }
	@git diff --quiet || { echo "working tree is dirty"; exit 1; }
	git fetch --quiet origin main
	@test "$$(git rev-parse HEAD)" = "$$(git rev-parse origin/main)" || \
	    { echo "main is not level with origin/main; run 'git pull'"; exit 1; }
	@v=$$(sed -n 's/^Version:[[:space:]]*//p' packaging/$(BINARY).spec); \
	test -n "$$v" || { echo "no Version: in the spec"; exit 1; }; \
	if git rev-parse -q --verify "refs/tags/v$$v" >/dev/null; then \
	    echo "v$$v is already tagged -- did the bump PR get merged?"; exit 1; fi; \
	sh scripts/review-packet.sh --check "$$v" || \
	    { echo "the release is blocked until the packet is answered"; exit 1; }; \
	test -f .copr/Makefile || { echo ".copr/Makefile is missing; Copr would have nothing to run"; exit 1; }; \
	echo "tagging v$$v at $$(git rev-parse --short HEAD)"; \
	git tag "v$$v" && git push origin "v$$v" && \
	echo && \
	echo "pushed v$$v. Actions is building the archives; the webhook is building the rpms." && \
	echo "  https://github.com/bspeelm/$(BINARY)/actions" && \
	echo "  https://copr.fedorainfracloud.org/coprs/bspeelman/$(BINARY)/builds/"

# Publish the tag to Copr. The package is an SCM package, so this hands Copr a
# ref and Copr does the rest: clone, .copr/Makefile, build. Nothing is built or
# uploaded from here, which is what keeps the published rpm honest -- it can
# only ever be the tag, never whatever happened to be in ~/rpmbuild.
#
# Kept separate from `release` so it follows the GitHub release rather than
# racing it.
#
# --nowait because copr-cli otherwise watches the build to completion, which
# means several minutes of no output from a command that looks hung. It prints
# the build URL instead; watch it there.
copr:
	@v=$$(sed -n 's/^Version:[[:space:]]*//p' packaging/$(BINARY).spec); \
	git rev-parse -q --verify "refs/tags/v$$v" >/dev/null || \
	    { echo "no tag v$$v -- run 'make release VERSION=$$v' first"; exit 1; }; \
	git cat-file -e "v$$v:.copr/Makefile" 2>/dev/null || \
	    { echo "tag v$$v has no .copr/Makefile, so Copr has nothing to run."; \
	      echo "tags cut before it was added cannot be built this way."; exit 1; }; \
	copr-cli buildscm $(BINARY) \
	    --clone-url https://github.com/bspeelm/$(BINARY) \
	    --commit "v$$v" \
	    --spec packaging/$(BINARY).spec \
	    --type git \
	    --method make_srpm \
	    --nowait
