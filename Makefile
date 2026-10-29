BINDIR ?= $(HOME)/.local/bin

.PHONY: build install uninstall

build:
	go build -o bin/ccc ./cmd/ccc

install:
	GOBIN="$(BINDIR)" go install ./cmd/ccc

uninstall:
	rm -f "$(BINDIR)/ccc"
