.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: build
build:
	go build -v

.PHONY: test
test:
	go test -v ./... -bench=.

.DEFAULT_GOAL := build

