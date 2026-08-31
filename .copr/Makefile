# Copr's "make_srpm" entry point.
#
# Copr clones this repository at the pushed ref and runs:
#
#     make -f .copr/Makefile srpm outdir=<some dir>
#
# so the whole release path becomes: push a tag -> webhook -> Copr builds.
# Nothing local, and no SRPM uploaded by hand.
#
# The tarball is built from the checkout rather than downloaded from the tag,
# even though the spec's Source0 names that URL. Building from the checkout is
# what makes this work the moment the tag is pushed, instead of only after
# GitHub has finished generating the archive for it.
#
# vendor/ is committed, so nothing here or in the build root needs a network.

srpm:
	@command -v rpmbuild >/dev/null || dnf -y install rpm-build
	@command -v git >/dev/null || dnf -y install git
	@test -n "$(outdir)" || { echo "outdir= is required"; exit 1; }
	@mkdir -p "$(outdir)"
	@v=$$(sed -n 's/^Version:[[:space:]]*//p' packaging/bothy.spec); \
	test -n "$$v" || { echo "no Version: in packaging/bothy.spec"; exit 1; }; \
	tmp=$$(mktemp -d); \
	mkdir -p "$$tmp/bothy-$$v" "$$tmp/SOURCES"; \
	git ls-files | tar -cf - -T - | tar -xf - -C "$$tmp/bothy-$$v"; \
	tar -czf "$$tmp/SOURCES/bothy-$$v.tar.gz" -C "$$tmp" "bothy-$$v"; \
	rpmbuild -bs packaging/bothy.spec \
	    --define "_sourcedir $$tmp/SOURCES" \
	    --define "_srcrpmdir $(outdir)"; \
	rc=$$?; rm -rf "$$tmp"; exit $$rc
