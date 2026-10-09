BINARY  := chatport
VERSION := $(shell cat VERSION)

.PHONY: build test vet example demo clean

## build: compile the single-file binary (version stamped from VERSION)
build:
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o $(BINARY) ./cmd/chatport

## test: run all tests
test:
	go test ./...

## vet: static checks
vet:
	go vet ./...

## example: rebuild the sample ChatGPT export zip from examples/sample-export
example:
	cd examples/sample-export && zip -FS -j ../sample-chatgpt-export.zip conversations.json

## demo: render assets/demo.gif from assets/demo.tape (requires vhs)
demo: build
	vhs assets/demo.tape

## clean: remove build output
clean:
	rm -f $(BINARY)
