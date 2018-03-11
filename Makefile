# This Makefile is necessary only if you want to help
# develop the msgp tool or library.
# You can still install msgp with `go get` or `go install`.

GGEN = ./tests/def_gen.go ./tests/def_gen_test.go

MGEN = ./msgp/defs_gen_test.go

SHELL := /bin/bash

BIN = $(GOBIN)/msgp

.PHONY: clean wipe install get-deps bench all

$(BIN): */*.go
	@go install ./...

install: $(BIN)

$(GGEN): ./tests/def.go
	go generate ./tests

$(MGEN): ./msgp/defs_test.go
	go generate ./msgp

test: all
	go test -v ./...

bench: all
	go test -bench=. ./...

clean:
	$(RM) $(GGEN) $(MGEN)

wipe: clean
	$(RM) $(BIN)

get-deps:
	go get -d -t ./...

all: install $(GGEN) $(MGEN)

# Travis CI
travis:
	go get -d -t ./...
	go build -o "$${GOPATH%%:*}/bin/msgp" .
	go generate ./msgp
	go generate ./tests
	go test -v ./...
