# NOTE: This Makefile is only necessary if you
# plan on developing the msgp tool or library.
# You can still install msgp with `go get` or `go install`.

# generated integration test files
GGEN = ./gen_tests/def_gen.go ./gen_tests/def_gen_test.go

# generated unit test files
MGEN = ./msgp/defs_gen_test.go

SHELL := /bin/bash

BIN = $(GOBIN)/msgp

.PHONY: clean wipe install get-deps bench all

$(BIN): */*.go
	@go install ./...

install: $(BIN)

$(GGEN): ./gen_tests/def.go
	go generate ./gen_tests

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
	go generate ./gen_tests
	go test -v ./...
