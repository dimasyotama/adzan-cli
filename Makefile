VERSION ?= 0.1.0
PREFIX  ?= $(HOME)/.local

# -s -w strips the symbol table and DWARF data; -trimpath removes build paths.
LDFLAGS := -s -w -X main.Version=$(VERSION)
GOFLAGS := -trimpath -ldflags "$(LDFLAGS)"

BIN := bin

.PHONY: all build test vet fmt clean install uninstall release size formula deb dist

all: build

build:
	@mkdir -p $(BIN)
	go build $(GOFLAGS) -o $(BIN)/adzan  ./cmd/adzan
	go build $(GOFLAGS) -o $(BIN)/adzand ./cmd/adzand
	go build $(GOFLAGS) -o $(BIN)/adzantray ./cmd/adzantray
	@echo
	@ls -lh $(BIN)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w ./cmd ./internal

# Both binaries must live in the same directory: the CLI finds the daemon
# next to itself.
install: build
	@mkdir -p $(PREFIX)/bin
	install -m 0755 $(BIN)/adzan     $(PREFIX)/bin/adzan
	install -m 0755 $(BIN)/adzand    $(PREFIX)/bin/adzand
	install -m 0755 $(BIN)/adzantray $(PREFIX)/bin/adzantray
	@echo "installed to $(PREFIX)/bin - make sure it is on your PATH"

uninstall:
	rm -f $(PREFIX)/bin/adzan $(PREFIX)/bin/adzand $(PREFIX)/bin/adzantray

clean:
	rm -rf $(BIN) dist

# Cross-compiled tarballs for the platforms we support today.
# adzantray needs cgo + Cocoa on darwin. Apple's clang can target either Mac
# arch from either Mac arch (`-arch x86_64`/`-arch arm64`), so as long as
# `make release` runs on a Mac at all, it can build adzantray for both darwin
# arches - it just can't build it anywhere else (no Cocoa toolchain there).
# linux/windows have no such requirement (pure Go, CGO_ENABLED=0 is fine),
# so they always get it.
HOST_OS := $(shell uname -s | tr A-Z a-z)

# TARGETS lets CI split the build across runners (a macOS runner for darwin,
# so adzantray gets built there too) - defaults to everything, for a plain
# local `make release`.
TARGETS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
release:
	@rm -rf dist && mkdir -p dist
	@for target in $(TARGETS); do \
		os=$${target%/*}; arch=$${target#*/}; ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		out=dist/adzan-$(VERSION)-$$os-$$arch; \
		mkdir -p $$out; \
		GOOS=$$os GOARCH=$$arch go build $(GOFLAGS) -o $$out/adzan$$ext  ./cmd/adzan  || exit 1; \
		GOOS=$$os GOARCH=$$arch go build $(GOFLAGS) -o $$out/adzand$$ext ./cmd/adzand || exit 1; \
		if [ "$$os" != "darwin" ]; then \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build $(GOFLAGS) -o $$out/adzantray$$ext ./cmd/adzantray || exit 1; \
		elif [ "$(HOST_OS)" = "darwin" ]; then \
			_cc_arch=$$arch; [ "$$arch" = "amd64" ] && _cc_arch=x86_64; \
			CGO_ENABLED=1 GOOS=darwin GOARCH=$$arch CC="clang -arch $$_cc_arch" go build $(GOFLAGS) -o $$out/adzantray ./cmd/adzantray || exit 1; \
		else \
			echo "skipping adzantray for $$target - not building on a Mac"; \
		fi; \
		cp README.md $$out/; \
		tar -czf $$out.tar.gz -C dist $$(basename $$out); \
		rm -rf $$out; \
		echo "built $$out.tar.gz"; \
	done
	@cd dist && (sha256sum *.tar.gz > checksums.txt 2>/dev/null || shasum -a 256 *.tar.gz > checksums.txt)
	@ls -lh dist

size: build
	@echo "stripped binary sizes:"
	@du -h $(BIN)/adzan $(BIN)/adzand

# Regenerates Formula/adzan.rb from the template, filling in the sha256 of
# whatever `make release` already built - never hand-edit the checksums, they
# only ever come from the tarballs that are actually about to be released.
# Deliberately does NOT depend on `release`: that target is destructive
# (rm -rf dist), so running `make deb` and `make formula` as separate `make`
# invocations after it - the normal way to chain targets - would each
# re-trigger it and wipe the other's output. Run `make release` yourself
# first, or use `make dist` to do all three in one invocation.
formula:
	@test -f dist/checksums.txt || { echo "dist/checksums.txt not found - run 'make release' first"; exit 1; }
	@sha() { grep " $$1\$$" dist/checksums.txt | cut -d' ' -f1; }; \
	sed -e "s/@VERSION@/$(VERSION)/g" \
	    -e "s/@SHA_DARWIN_ARM64@/$$(sha adzan-$(VERSION)-darwin-arm64.tar.gz)/" \
	    -e "s/@SHA_DARWIN_AMD64@/$$(sha adzan-$(VERSION)-darwin-amd64.tar.gz)/" \
	    -e "s/@SHA_LINUX_ARM64@/$$(sha adzan-$(VERSION)-linux-arm64.tar.gz)/" \
	    -e "s/@SHA_LINUX_AMD64@/$$(sha adzan-$(VERSION)-linux-amd64.tar.gz)/" \
	    Formula/adzan.rb.tmpl > Formula/adzan.rb
	@echo "wrote Formula/adzan.rb"

# .deb packages for the two Linux release tarballs, using dpkg-deb - the
# standard tool for this, not hand-rolled. Needs dpkg-deb: present by default
# on Debian/Ubuntu, `brew install dpkg` gets it on macOS.
# Deliberately does NOT depend on `release` - see the comment on `formula`.
deb:
	@command -v dpkg-deb >/dev/null 2>&1 || { \
		echo "dpkg-deb not found - run this on Linux, or 'brew install dpkg' here"; exit 1; }
	@test -f dist/adzan-$(VERSION)-linux-amd64.tar.gz || { echo "dist/adzan-$(VERSION)-linux-amd64.tar.gz not found - run 'make release' first"; exit 1; }
	@for arch in amd64 arm64; do \
		root=dist/deb-$$arch; rm -rf $$root; \
		mkdir -p $$root/usr/bin $$root/DEBIAN; \
		tar -xzf dist/adzan-$(VERSION)-linux-$$arch.tar.gz -C dist; \
		install -m 0755 dist/adzan-$(VERSION)-linux-$$arch/adzan     $$root/usr/bin/; \
		install -m 0755 dist/adzan-$(VERSION)-linux-$$arch/adzand    $$root/usr/bin/; \
		install -m 0755 dist/adzan-$(VERSION)-linux-$$arch/adzantray $$root/usr/bin/; \
		rm -rf dist/adzan-$(VERSION)-linux-$$arch; \
		sed -e "s/@VERSION@/$(VERSION)/" -e "s/@ARCH@/$$arch/" \
			packaging/deb-control.in > $$root/DEBIAN/control; \
		dpkg-deb --build --root-owner-group $$root dist/adzan_$(VERSION)_$$arch.deb || exit 1; \
		rm -rf $$root; \
	done
	@ls -lh dist/*.deb

# Everything a release needs, in the one order that is safe: release first
# (it wipes and repopulates dist/), then deb and formula reading from it -
# all as a single `make` invocation, so `release`'s prerequisite-free target
# only runs once and nothing gets wiped out from under the others.
dist: release deb formula
