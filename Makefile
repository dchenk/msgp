# This Makefile is necessary only if you want to help
# develop the msgp tool or library.
# You can still install msgp with `go get` or `go install`.

GGEN = ./tests/*_gen.go ./tests/*_gen_test.go

MGEN = ./msgp/defs_gen_test.go

SHELL := /bin/bash

BIN = $(GOBIN)/msgp

.PHONY: fmt clean wipe install get-deps bench all

$(BIN): */*.go
	@go install ./...

install: $(BIN)

$(GGEN): ./tests/def.go
	cd ./tests && go generate

$(MGEN): ./msgp/defs_test.go
	cd ./msgp && go generate

fmt:
	gofmt -s -w -e ./

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
	[ "${GIMME_ARCH}" == "amd64" ] && $(GOPATH)/bin/goveralls -service=travis-ci
